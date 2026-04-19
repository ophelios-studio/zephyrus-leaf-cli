package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/project"
	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/runtime"
)

// runBuild implements `leaf build`. Returns the process exit code.
func runBuild(args []string) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	projectDir := fs.String("dir", ".", "project root (directory containing config.yml)")
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

	// TODO(M1.overlay): merge embedded defaults + user project into a tempdir
	// and point the PHP runtime at it. For now, this returns a stub until
	// the phar + scaffolds are wired up.
	_ = runtime.Default()
	ctx, cancel := signalContext()
	defer cancel()
	_ = ctx

	fmt.Fprintf(os.Stderr, "leaf build: not fully wired yet (site %q resolves, phar pipeline pending)\n", cfg.Name)
	return 0
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
