// stage resolves symlinks and copies the zephyrus-leaf framework tree into
// internal/project/framework/ for go:embed. Cross-platform replacement for
// the old rsync-based stage-defaults.sh.
//
//	go run ./scripts/stage -src /path/to/zephyrus-leaf
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// skipPrefix returns true if rel (forward-slash-separated relative path)
// matches a path we should not include in the embedded framework.
func skipPrefix(rel string) bool {
	// Top-level dirs/files excluded entirely.
	topExact := map[string]bool{
		"dist":              true,
		"cache":             true,
		".git":              true,
		".github":           true,
		".idea":             true,
		".vscode":           true,
		"node_modules":      true,
		"docker-compose.yml": true,
		"CLAUDE.md":         true,
		"tests":             true,
		"phpunit.xml":       true,
	}
	// First path segment.
	first := rel
	if i := strings.Index(rel, "/"); i >= 0 {
		first = rel[:i]
	}
	if topExact[first] {
		return true
	}
	// Nested vendor excludes, applied at path depth 3+:
	// vendor/<owner>/<pkg>/<excluded>...
	if strings.HasPrefix(rel, "vendor/") {
		parts := strings.Split(rel, "/")
		if len(parts) >= 4 {
			last := parts[3]
			nestedExact := map[string]bool{
				"vendor": true, "tests": true, "test": true, "Tests": true,
				"docs": true, "doc": true, "examples": true,
				".github": true, ".git": true,
			}
			if nestedExact[last] {
				return true
			}
			if len(parts) == 4 {
				// Root-of-package files we can drop.
				switch last {
				case "phpunit.xml", "phpunit.xml.dist", ".editorconfig",
					".gitattributes", ".gitignore", ".scrutinizer.yml",
					".travis.yml":
					return true
				}
				if strings.HasSuffix(last, ".md") || strings.HasSuffix(last, ".rst") {
					return true
				}
			}
		}
	}
	// Drop .DS_Store wherever it appears.
	if filepath.Base(rel) == ".DS_Store" {
		return true
	}
	return false
}

func main() {
	src := flag.String("src", "", "path to a zephyrus-leaf checkout (with vendor/ installed)")
	dst := flag.String("dst", "internal/project/framework", "destination for the staged tree")
	flag.Parse()

	if *src == "" {
		fmt.Fprintln(os.Stderr, "stage: -src is required")
		os.Exit(2)
	}
	srcAbs, err := filepath.Abs(*src)
	if err != nil {
		fatal(err)
	}
	if _, err := os.Stat(filepath.Join(srcAbs, "vendor")); err != nil {
		fatal(fmt.Errorf("%s/vendor missing; run 'composer install' there first", srcAbs))
	}
	dstAbs, err := filepath.Abs(*dst)
	if err != nil {
		fatal(err)
	}

	// Preserve any top-level marker files we own (like .gitkeep).
	keep := map[string][]byte{}
	entries, _ := os.ReadDir(dstAbs)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			if b, err := os.ReadFile(filepath.Join(dstAbs, e.Name())); err == nil {
				keep[e.Name()] = b
			}
		}
	}

	if err := os.RemoveAll(dstAbs); err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(dstAbs, 0o755); err != nil {
		fatal(err)
	}
	for name, body := range keep {
		if err := os.WriteFile(filepath.Join(dstAbs, name), body, 0o644); err != nil {
			fatal(err)
		}
	}

	walk(srcAbs, dstAbs)

	stamp := time.Now().UTC().Format(time.RFC3339)
	_ = os.WriteFile(filepath.Join(dstAbs, ".staged"), []byte(stamp+"\n"), 0o644)
	fmt.Printf("stage: staged framework from %s to %s\n", srcAbs, dstAbs)
}

func walk(src, dst string) {
	walkRel(src, dst, "")
}

func walkRel(srcRoot, dstRoot, rel string) {
	srcPath := filepath.Join(srcRoot, rel)
	info, err := os.Lstat(srcPath)
	if err != nil {
		fatal(err)
	}
	// Resolve symlinks so the embedded tree has no links.
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(srcPath)
		if err != nil {
			fatal(fmt.Errorf("resolve symlink %s: %w", srcPath, err))
		}
		info, err = os.Stat(resolved)
		if err != nil {
			fatal(err)
		}
		srcPath = resolved
	}
	relForward := filepath.ToSlash(rel)
	if rel != "" && skipPrefix(relForward) {
		return
	}
	if info.IsDir() {
		dstPath := filepath.Join(dstRoot, rel)
		if rel != "" {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				fatal(err)
			}
		}
		children, err := os.ReadDir(srcPath)
		if err != nil {
			fatal(err)
		}
		for _, c := range children {
			childRel := filepath.Join(rel, c.Name())
			walkRelResolved(srcPath, dstRoot, childRel)
		}
		return
	}
	if !info.Mode().IsRegular() {
		return
	}
	copyFile(srcPath, filepath.Join(dstRoot, rel))
}

// walkRelResolved is the recursive helper when the caller has already
// resolved a parent dir via symlink; we walk children starting from
// parentSrc (not the original src root).
func walkRelResolved(parentSrc, dstRoot, rel string) {
	srcPath := filepath.Join(parentSrc, filepath.Base(rel))
	info, err := os.Lstat(srcPath)
	if err != nil {
		fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(srcPath)
		if err != nil {
			fatal(fmt.Errorf("resolve symlink %s: %w", srcPath, err))
		}
		info, err = os.Stat(resolved)
		if err != nil {
			fatal(err)
		}
		srcPath = resolved
	}
	relForward := filepath.ToSlash(rel)
	if skipPrefix(relForward) {
		return
	}
	dstPath := filepath.Join(dstRoot, rel)
	if info.IsDir() {
		if err := os.MkdirAll(dstPath, 0o755); err != nil {
			fatal(err)
		}
		children, err := os.ReadDir(srcPath)
		if err != nil {
			fatal(err)
		}
		for _, c := range children {
			walkRelResolved(srcPath, dstRoot, filepath.Join(rel, c.Name()))
		}
		return
	}
	if !info.Mode().IsRegular() {
		return
	}
	copyFile(srcPath, dstPath)
}

func copyFile(src, dst string) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		fatal(err)
	}
	in, err := os.Open(src)
	if err != nil {
		fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		fatal(err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "stage:", err)
	os.Exit(1)
}
