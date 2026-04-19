package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Defaults describes where the CLI finds the framework scaffold. Exactly one
// of Root or FS is set. Root wins in dev builds (symlinks in vendor/ work);
// FS is used in release builds once go:embed is wired (M5).
type Defaults struct {
	Root string
	FS   fs.FS
}

// DefaultsSource returns the bundled framework scaffold.
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
	// When we wire go:embed in M5, this branch returns an embedded FS.
	return Defaults{}, errors.New("no embedded defaults in this build. Set LEAF_DEFAULTS_DIR to a local zephyrus-leaf checkout")
}
