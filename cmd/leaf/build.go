package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/builder"
)

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

	ctx, cancel := signalContext()
	defer cancel()

	code, err := builder.Build(ctx, builder.Options{
		ProjectRoot: root,
		KeepTmp:     *keep,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf build: %v\n", err)
		return 1
	}
	return code
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
