//go:build !embed_php

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Default Runner used when the binary is not statically linked against libphp.
func Default() Runner { return &Exec{Binary: "php"} }

// Exec shells out to the system PHP CLI.
type Exec struct {
	Binary string // defaults to "php" if empty
}

func (e *Exec) Run(ctx context.Context, script string, args []string, cwd string, env map[string]string) (int, error) {
	bin := e.Binary
	if bin == "" {
		bin = "php"
	}
	cmdArgs := append([]string{script}, args...)
	cmd := exec.CommandContext(ctx, bin, cmdArgs...)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = mergedEnv(env)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, fmt.Errorf("exec %s: %w", bin, err)
	}
	return 0, nil
}

func mergedEnv(extra map[string]string) []string {
	base := os.Environ()
	for k, v := range extra {
		base = append(base, k+"="+v)
	}
	return base
}
