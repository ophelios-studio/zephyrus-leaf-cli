//go:build !embed_defaults

package project

import (
	"errors"
	"fmt"
	"os"
)

// DefaultsSource in dev builds resolves from LEAF_DEFAULTS_DIR. This build
// variant does not bake the framework into the binary, so incremental Go
// builds stay fast.
func DefaultsSource() (Defaults, error) {
	if dir := os.Getenv("LEAF_DEFAULTS_DIR"); dir != "" {
		info, err := os.Stat(dir)
		if err != nil {
			return Defaults{}, fmt.Errorf("LEAF_DEFAULTS_DIR=%s: %w", dir, err)
		}
		if !info.IsDir() {
			return Defaults{}, fmt.Errorf("LEAF_DEFAULTS_DIR=%s is not a directory", dir)
		}
		return Defaults{Root: dir}, nil
	}
	return Defaults{}, errors.New("no embedded defaults in this build. Set LEAF_DEFAULTS_DIR, or rebuild with -tags embed_defaults after running scripts/stage-defaults.sh")
}
