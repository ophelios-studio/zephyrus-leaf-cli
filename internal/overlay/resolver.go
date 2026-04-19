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
)

// Source describes one layer of the overlay stack. A Source with higher
// Priority wins. Zero-value FS means "use the filesystem at Root."
type Source struct {
	Name     string // "defaults", "user", etc. Used in error messages.
	Priority int    // higher wins
	FS       fs.FS  // if nil, OS filesystem under Root is used
	Root     string // absolute path when FS is nil
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
	fsys := src.FS
	if fsys == nil {
		if src.Root == "" {
			return nil // empty source, skip
		}
		fsys = os.DirFS(src.Root)
	}
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		target := filepath.Join(dst, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(fsys, path, target)
	})
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
