package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/overlay"
	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/project"
)

// Paths that eject must not touch. These are user-owned and must survive.
var ejectSkip = []string{
	"content",
	"config.yml",
	"templates",
	"public",
	"dist",
	"cache",
	".git",
	".github",
	".gitignore",
	".DS_Store",
	"CLAUDE.md",
}

func runEject(args []string) int {
	fs := flag.NewFlagSet("eject", flag.ContinueOnError)
	projectDir := fs.String("dir", ".", "project root (directory containing config.yml)")
	force := fs.Bool("force", false, "proceed even when framework files already exist")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	root, err := filepath.Abs(*projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf eject: %v\n", err)
		return 1
	}

	// Must look like a Leaf site: config.yml required.
	if _, err := os.Stat(filepath.Join(root, "config.yml")); err != nil {
		fmt.Fprintf(os.Stderr, "leaf eject: %s does not look like a leaf site (config.yml missing)\n", root)
		return 1
	}

	// Refuse if already ejected (composer.json + app/ typically present).
	if _, err := os.Stat(filepath.Join(root, "composer.json")); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "leaf eject: composer.json already exists; use --force to overwrite\n")
		return 1
	}

	defaults, err := project.DefaultsSource()
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf eject: %v\n", err)
		return 1
	}

	if err := overlay.Merge(root, []overlay.Source{
		{Name: "framework", Root: defaults.Root, FS: defaults.FS, Skip: ejectSkip},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "leaf eject: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stdout, "Ejected framework into %s\n", root)
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Next:")
	fmt.Fprintln(os.Stdout, "  composer install")
	fmt.Fprintln(os.Stdout, "  composer build")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "You now manage a full Zephyrus Leaf project. The `leaf` CLI")
	fmt.Fprintln(os.Stdout, "can still build it, but composer dev / composer build work too.")
	return 0
}
