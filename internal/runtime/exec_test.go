//go:build !embed_php

package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// These tests require system PHP. They skip if `php` is not on PATH.
func requirePHP(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("php"); err != nil {
		t.Skip("system php not available")
	}
}

func TestExec_ZeroExit(t *testing.T) {
	requirePHP(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "ok.php")
	if err := os.WriteFile(script, []byte("<?php exit(0);"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Exec{}
	code, err := r.Run(context.Background(), script, nil, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("got %d, want 0", code)
	}
}

func TestExec_NonZeroExit(t *testing.T) {
	requirePHP(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "fail.php")
	if err := os.WriteFile(script, []byte("<?php exit(7);"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Exec{}
	code, err := r.Run(context.Background(), script, nil, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if code != 7 {
		t.Errorf("got %d, want 7", code)
	}
}

func TestExec_PassesArgs(t *testing.T) {
	requirePHP(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "argv.php")
	// Exits with the integer value of the first CLI arg.
	body := `<?php exit((int)($argv[1] ?? 99));`
	if err := os.WriteFile(script, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Exec{}
	code, _ := r.Run(context.Background(), script, []string{"42"}, dir, nil)
	if code != 42 {
		t.Errorf("args not forwarded; got %d", code)
	}
}

func TestExec_PassesEnv(t *testing.T) {
	requirePHP(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "env.php")
	// Exit with len of LEAF_TEST env var.
	body := `<?php exit(strlen(getenv('LEAF_TEST') ?: ''));`
	if err := os.WriteFile(script, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Exec{}
	code, _ := r.Run(context.Background(), script, nil, dir, map[string]string{"LEAF_TEST": "abcd"})
	if code != 4 {
		t.Errorf("env not forwarded; got %d", code)
	}
}
