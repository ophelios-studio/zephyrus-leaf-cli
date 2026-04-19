package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()

	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "  leaf dev ready\n")
	fmt.Fprintf(os.Stdout, "  \u279C %s\n", friendlyURL(ln.Addr().String()))
	fmt.Fprintf(os.Stdout, "  Watching %s\n", root)
	fmt.Fprintf(os.Stdout, "  Press Ctrl+C to stop\n")
	fmt.Fprintln(os.Stdout)

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
			fmt.Fprintln(os.Stdout, "\nleaf dev: shutting down...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer shutdownCancel()
			if err := httpSrv.Shutdown(shutdownCtx); err != nil {
				// SSE streams don't close on graceful shutdown. Force-close.
				_ = httpSrv.Close()
			}
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

// friendlyURL rewrites a net.Listen address so wildcard binds read as
// "localhost" instead of "[::]" or "0.0.0.0", which users never remember.
func friendlyURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if host == "" || host == "::" || host == "[::]" || host == "0.0.0.0" {
		host = "localhost"
	} else if strings.HasPrefix(addr, "[::]:") {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}
