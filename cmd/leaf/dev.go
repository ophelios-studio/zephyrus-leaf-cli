package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/builder"
	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/devserver"
	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/project"
)

func runDev(args []string) int {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	projectDir := fs.String("dir", ".", "project root (directory containing config.yml)")
	addr := fs.String("addr", ":8080", "address to serve on")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	root, err := filepath.Abs(*projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf dev: %v\n", err)
		return 1
	}

	cfg, err := project.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf dev: %v\n", err)
		return 1
	}
	distRoot := filepath.Join(root, cfg.OutputPath)

	ctx, cancel := signalContext()
	defer cancel()

	// Initial build so dist/ exists before the server comes up.
	if code, err := builder.Build(ctx, builder.Options{ProjectRoot: root}); err != nil || code != 0 {
		if err != nil {
			fmt.Fprintf(os.Stderr, "leaf dev: initial build: %v\n", err)
		}
		return 1
	}

	hub := devserver.NewHub()
	srv := devserver.NewServer(distRoot, hub)
	httpSrv := &http.Server{Addr: *addr, Handler: srv}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf dev: listen %s: %v\n", *addr, err)
		return 1
	}

	go func() {
		_ = httpSrv.Serve(ln)
	}()

	fmt.Fprintf(os.Stdout, "leaf dev: serving %s at http://%s\n", root, ln.Addr().String())

	watcher, err := devserver.NewWatcher(root, 250*time.Millisecond)
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaf dev: watcher: %v\n", err)
		return 1
	}
	watchCtx, cancelWatch := context.WithCancel(ctx)
	defer cancelWatch()
	go func() { _ = watcher.Run(watchCtx) }()

	// Main loop: on every debounced event, rebuild and broadcast reload.
	for {
		select {
		case <-ctx.Done():
			_ = httpSrv.Shutdown(context.Background())
			return 0
		case <-watcher.Events():
			fmt.Fprintln(os.Stdout, "leaf dev: change detected, rebuilding...")
			if code, err := builder.Build(ctx, builder.Options{ProjectRoot: root}); err != nil {
				fmt.Fprintf(os.Stderr, "leaf dev: build failed: %v\n", err)
				continue
			} else if code != 0 {
				fmt.Fprintf(os.Stderr, "leaf dev: build exited %d\n", code)
				continue
			}
			hub.Broadcast("reload")
		}
	}
}
