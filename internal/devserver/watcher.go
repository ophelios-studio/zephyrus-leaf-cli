// Package devserver implements the `leaf dev` runtime loop: static file
// server, file watcher, and WebSocket reload broadcaster.
package devserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchedDirs lists top-level dirs under the project root whose changes
// should trigger a rebuild. config.yml is handled separately because it's a
// single file, not a directory.
var WatchedDirs = []string{"content", "templates", "public", "app"}

// Watcher coalesces filesystem events from several directories into a single
// "please rebuild" signal, debounced to avoid rebuild-storms when editors
// write files in bursts.
type Watcher struct {
	root     string
	inner    *fsnotify.Watcher
	debounce time.Duration
	events   chan struct{}
}

// NewWatcher sets up recursive watches on WatchedDirs plus config.yml.
func NewWatcher(root string, debounce time.Duration) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		root:     root,
		inner:    fw,
		debounce: debounce,
		events:   make(chan struct{}, 1),
	}
	for _, d := range WatchedDirs {
		if err := w.addRecursive(filepath.Join(root, d)); err != nil {
			// Missing dir is not fatal: a bare project may not have `templates/`.
			continue
		}
	}
	_ = fw.Add(filepath.Join(root, "config.yml")) // errors tolerated
	return w, nil
}

// Events returns a receive-only channel that emits a single value per
// debounce window after any change is observed.
func (w *Watcher) Events() <-chan struct{} { return w.events }

// Run blocks, pumping events until ctx is canceled.
func (w *Watcher) Run(ctx context.Context) error {
	var timer *time.Timer
	fire := func() {
		select {
		case w.events <- struct{}{}:
		default: // coalesce: already queued
		}
	}
	for {
		select {
		case <-ctx.Done():
			_ = w.inner.Close()
			return ctx.Err()
		case ev, ok := <-w.inner.Events:
			if !ok {
				return nil
			}
			if w.isNoiseEvent(ev) {
				continue
			}
			// If the event created a new directory, add it to the watch list.
			if ev.Op&fsnotify.Create != 0 {
				if info, err := osStat(ev.Name); err == nil && info.IsDir() {
					_ = w.addRecursive(ev.Name)
				}
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(w.debounce, fire)
		case err, ok := <-w.inner.Errors:
			if !ok {
				return nil
			}
			_ = err // logged by caller via Errors() in future; for now swallow
		}
	}
}

func (w *Watcher) addRecursive(path string) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info != nil && info.IsDir() {
			return w.inner.Add(p)
		}
		return nil
	})
}

func (w *Watcher) isNoiseEvent(ev fsnotify.Event) bool {
	name := filepath.Base(ev.Name)
	// Editor swap / lockfiles, OS metadata, build output.
	if strings.HasPrefix(name, ".") {
		return true
	}
	if strings.HasSuffix(name, "~") || strings.HasSuffix(name, ".swp") || strings.HasSuffix(name, ".tmp") {
		return true
	}
	// Don't trigger on our own dist/ writes.
	rel, err := filepath.Rel(w.root, ev.Name)
	if err == nil && strings.HasPrefix(rel, "dist"+string(filepath.Separator)) {
		return true
	}
	return false
}
