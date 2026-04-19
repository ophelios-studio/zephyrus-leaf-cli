//go:build embed_php

// This file is the FrankenPHP-backed Runner. It is only compiled when the
// `embed_php` build tag is set, which requires a CGO toolchain and a static
// libphp. The release CI handles that build; local dev uses exec.go.
//
// TODO(M5): implement against github.com/php/frankenphp. For now, a stub so
// `go build -tags embed_php` fails loudly with a clear message.

package runtime

import (
	"context"
	"errors"
)

func Default() Runner { return &frankenPHP{} }

type frankenPHP struct{}

func (f *frankenPHP) Run(ctx context.Context, script string, args []string, cwd string, env map[string]string) (int, error) {
	return -1, errors.New("frankenphp runtime not yet wired up; rebuild without -tags embed_php to use system PHP")
}
