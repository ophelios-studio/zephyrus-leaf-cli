package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/overlay"
	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/project"
	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/runtime"
)

// runBuild implements `leaf build`. Returns the process exit code.
func runBuild(args []string) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	projectDir := fs.String("dir", ".", "project root (directory containing config.yml)")
	keep := fs.Bool("keep-tmp", false, "leave the merged build dir on disk for inspection")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	root, err := filepath.Abs(*projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf build: resolve path: %v\n", err)
		return 1
	}

	cfg, err := project.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf build: %v\n", err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "leaf build: %v\n", err)
		return 1
	}

	defaults, err := project.DefaultsSource()
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf build: %v\n", err)
		return 1
	}

	workdir, err := os.MkdirTemp("", "leaf-build-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf build: create tempdir: %v\n", err)
		return 1
	}
	if *keep {
		fmt.Fprintf(os.Stderr, "leaf build: keeping workdir at %s\n", workdir)
	} else {
		defer os.RemoveAll(workdir)
	}

	// content/ and config.yml are user-owned. Defaults ship starter copies
	// for `leaf init`; they must not bleed into a build. User's templates/
	// is handled separately so it lands where PHP expects (app/Views/).
	if err := overlay.Merge(workdir, []overlay.Source{
		{Name: "defaults", Priority: 0, FS: defaults.FS, Root: defaults.Root, Skip: []string{"content", "config.yml", "dist", "cache", ".git"}},
		{Name: "user", Priority: 10, Root: root, Skip: []string{"dist", "templates"}},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "leaf build: merge project: %v\n", err)
		return 1
	}

	// User templates/ → workdir/app/Views/, overriding bundled defaults.
	userTemplates := filepath.Join(root, "templates")
	if _, err := os.Stat(userTemplates); err == nil {
		if err := overlay.Merge(filepath.Join(workdir, "app", "Views"), []overlay.Source{
			{Name: "user-templates", Root: userTemplates},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "leaf build: merge templates: %v\n", err)
			return 1
		}
	}

	entry := filepath.Join(workdir, "bin", "build.php")
	if _, err := os.Stat(entry); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "leaf build: bin/build.php missing from merged tree (defaults incomplete)\n")
			return 1
		}
		fmt.Fprintf(os.Stderr, "leaf build: stat entry: %v\n", err)
		return 1
	}

	ctx, cancel := signalContext()
	defer cancel()

	runner := runtime.Default()
	code, err := runner.Run(ctx, entry, nil, workdir, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf build: php: %v\n", err)
		return 1
	}
	if code != 0 {
		return code
	}

	// Copy the merged dist/ back to the user's project root.
	distSrc := filepath.Join(workdir, cfg.OutputPath)
	distDst := filepath.Join(root, cfg.OutputPath)
	if err := replaceDir(distDst, distSrc); err != nil {
		fmt.Fprintf(os.Stderr, "leaf build: publish dist: %v\n", err)
		return 1
	}

	return 0
}

// replaceDir atomically replaces dst with the contents of src.
func replaceDir(dst, src string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source %s: %w", src, err)
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return copyTree(src, dst)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
	}()
	return ctx, cancel
}
