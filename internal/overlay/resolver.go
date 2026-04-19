// Package overlay merges embedded defaults with the user's project tree into
// a single directory that the PHP runtime treats as ROOT_DIR.
//
// Order of precedence (last write wins):
//   1. Embedded defaults (templates, assets, fallback config)
//   2. User project files (content/, templates/, public/, config.yml)
//
// The merged tree lives under a tempdir for the duration of the build and is
// cleaned up afterwards.
package overlay

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Source describes one layer of the overlay stack. A Source with higher
// Priority wins. Zero-value FS means "use the filesystem at Root."
type Source struct {
	Name     string // "defaults", "user", etc. Used in error messages.
	Priority int    // higher wins
	FS       fs.FS  // if nil, OS filesystem under Root is used
	Root     string // absolute path when FS is nil
	Skip     []string // top-level path prefixes this source must not contribute
}

// Merge walks each source in priority order (ascending) and writes files into
// dst, overwriting earlier layers with later ones. The dst directory must
// exist and should be empty.
func Merge(dst string, sources []Source) error {
	ordered := make([]Source, len(sources))
	copy(ordered, sources)
	// Insertion sort by priority ascending; small N, keeps order stable.
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && ordered[j-1].Priority > ordered[j].Priority; j-- {
			ordered[j-1], ordered[j] = ordered[j], ordered[j-1]
		}
	}
	for _, src := range ordered {
		if err := copyLayer(src, dst); err != nil {
			return fmt.Errorf("overlay source %q: %w", src.Name, err)
		}
	}
	return nil
}

func copyLayer(src Source, dst string) error {
	// OS-backed path: follow symlinks so dev-mode vendor checkouts work.
	if src.FS == nil {
		if src.Root == "" {
			return nil
		}
		return copyOSDir(src.Root, dst, src.Skip)
	}
	// Abstract fs.FS (e.g. go:embed): no symlinks possible, walk directly.
	fsys := src.FS
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		if shouldSkip(path, src.Skip) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(fsys, path, target)
	})
}

// copyOSDir walks the filesystem tree at root, following symlinks (including
// directory symlinks, common in local Composer workspace setups), and writes
// a real-file copy under dst. Missing roots are not an error (the user may
// not have a templates/ dir yet, for example).
func copyOSDir(root, dst string, skips []string) error {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return walkCopy(root, dst, "", skips, make(map[string]bool))
}

func walkCopy(srcRoot, dstRoot, rel string, skips []string, seen map[string]bool) error {
	if rel != "" && shouldSkip(rel, skips) {
		return nil
	}

	srcPath := filepath.Join(srcRoot, rel)
	info, err := os.Lstat(srcPath)
	if err != nil {
		return err
	}

	// Follow symlinks, with cycle protection via resolved-path set.
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(srcPath)
		if err != nil {
			return fmt.Errorf("resolve symlink %s: %w", srcPath, err)
		}
		if seen[resolved] {
			return nil // cycle, stop
		}
		seen[resolved] = true
		info, err = os.Stat(resolved)
		if err != nil {
			return err
		}
		srcPath = resolved
	}

	dstPath := filepath.Join(dstRoot, rel)

	if info.IsDir() {
		if rel != "" {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return err
			}
		}
		entries, err := os.ReadDir(srcPath)
		if err != nil {
			return err
		}
		for _, e := range entries {
			childRel := filepath.Join(rel, e.Name())
			// Recurse from the original srcRoot so relative paths stay consistent;
			// but the data comes from the resolved srcPath via an intermediate.
			if err := walkCopyAt(srcPath, e.Name(), dstRoot, childRel, skips, seen); err != nil {
				return err
			}
		}
		return nil
	}

	if !info.Mode().IsRegular() {
		return nil
	}
	return copyOSFile(srcPath, dstPath)
}

// walkCopyAt handles a child entry whose source lives under a (possibly
// resolved-via-symlink) directory different from the original root. relFromDst
// is kept so the destination layout mirrors the logical source tree.
func walkCopyAt(parentSrc, childName, dstRoot, relFromDst string, skips []string, seen map[string]bool) error {
	if shouldSkip(relFromDst, skips) {
		return nil
	}
	childSrc := filepath.Join(parentSrc, childName)
	info, err := os.Lstat(childSrc)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(childSrc)
		if err != nil {
			return fmt.Errorf("resolve symlink %s: %w", childSrc, err)
		}
		if seen[resolved] {
			return nil
		}
		seen[resolved] = true
		info, err = os.Stat(resolved)
		if err != nil {
			return err
		}
		childSrc = resolved
	}
	dstPath := filepath.Join(dstRoot, relFromDst)
	if info.IsDir() {
		if err := os.MkdirAll(dstPath, 0o755); err != nil {
			return err
		}
		entries, err := os.ReadDir(childSrc)
		if err != nil {
			return err
		}
		for _, e := range entries {
			childRel := filepath.Join(relFromDst, e.Name())
			if err := walkCopyAt(childSrc, e.Name(), dstRoot, childRel, skips, seen); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	return copyOSFile(childSrc, dstPath)
}

func copyOSFile(srcPath, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func shouldSkip(path string, skips []string) bool {
	for _, s := range skips {
		if path == s || strings.HasPrefix(path, s+"/") {
			return true
		}
	}
	return false
}

func copyFile(fsys fs.FS, srcPath, dstPath string) error {
	in, err := fsys.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
