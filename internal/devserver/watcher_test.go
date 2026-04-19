package devserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_EmitsOnWrite(t *testing.T) {
	root := t.TempDir()
	for _, d := range WatchedDirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	w, err := NewWatcher(root, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	// Give the watcher a tick to register watches.
	time.Sleep(100 * time.Millisecond)

	// Write a file into content/ and expect an event.
	target := filepath.Join(root, "content", "page.md")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-w.Events():
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("no event emitted on content write")
	}
}

func TestWatcher_IgnoresDotFilesAndSwap(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "content"), 0o755); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher(root, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Drop a .swp and a dotfile; should not trigger.
	if err := os.WriteFile(filepath.Join(root, "content", ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "content", "page.md.swp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-w.Events():
		t.Fatal("noise event emitted")
	case <-time.After(300 * time.Millisecond):
		// expected: no event within the debounce window
	}
}

func TestWatcher_Debounces(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "content"), 0o755); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher(root, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Rapid-fire writes should coalesce into a single event.
	for i := 0; i < 5; i++ {
		path := filepath.Join(root, "content", "p.md")
		if err := os.WriteFile(path, []byte{byte('a' + i)}, 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	got := 0
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case <-w.Events():
			got++
		case <-timeout:
			if got == 0 {
				t.Fatal("no coalesced event")
			}
			if got > 2 {
				t.Errorf("expected at most 2 coalesced events, got %d", got)
			}
			return
		}
	}
}
