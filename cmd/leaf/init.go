package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/overlay"
	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/project"
)

// Paths that live inside the CLI binary at build time, not in a user's init
// scaffold. Anything PHP-glue, vendored, or build-only is excluded so the
// user's new project only contains files they're meant to author.
var initSkip = []string{
	// Framework plumbing (PHP tier, hidden from CLI users)
	"app",
	"bin",
	"vendor",
	"src",
	"composer.json",
	"composer.lock",
	"phpunit.xml",
	"tests",
	"public/index.php",
	"public/router.php",
	// Build output and caches
	"dist",
	"cache",
	"locale",
	// Project-meta / dev-only files from the source template
	"docker-compose.yml",
	"CLAUDE.md",
	"README.md",
	"LICENSE",
	".codequill",
	// Git & tooling metadata
	".git",
	".github",
	".gitignore",
	".gitattributes",
	".editorconfig",
	// OS / staging markers
	".DS_Store",
	".staged",
	".gitkeep",
}

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	force := fs.Bool("force", false, "overwrite existing files in the target directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: leaf init [--force] <name>")
		return 2
	}
	name := fs.Arg(0)

	target, err := filepath.Abs(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf init: resolve path: %v\n", err)
		return 1
	}

	if err := ensureEmpty(target, *force); err != nil {
		fmt.Fprintf(os.Stderr, "leaf init: %v\n", err)
		return 1
	}

	defaults, err := project.DefaultsSource()
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf init: %v\n", err)
		return 1
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "leaf init: %v\n", err)
		return 1
	}

	if err := overlay.Merge(target, []overlay.Source{
		{Name: "scaffold", Root: defaults.Root, FS: defaults.FS, Skip: initSkip},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "leaf init: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stdout, "Initialized leaf site at %s\n", target)
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Next:")
	fmt.Fprintf(os.Stdout, "  cd %s\n", name)
	fmt.Fprintln(os.Stdout, "  leaf dev     # live preview at http://localhost:8080")
	return 0
}

func ensureEmpty(dir string, force bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	if force {
		return nil
	}
	return fmt.Errorf("target %s is not empty (use --force to overwrite)", dir)
}
