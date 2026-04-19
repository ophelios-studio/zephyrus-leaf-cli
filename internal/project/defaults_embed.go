//go:build embed_defaults

package project

import (
	"embed"
	"errors"
	"io/fs"
	"os"
)

//go:embed all:framework
var frameworkFS embed.FS

// DefaultsSource in embed builds prefers LEAF_DEFAULTS_DIR (for ad-hoc dev
// work against a different framework checkout), falling back to the
// statically-embedded framework tree.
func DefaultsSource() (Defaults, error) {
	if dir := os.Getenv("LEAF_DEFAULTS_DIR"); dir != "" {
		info, err := os.Stat(dir)
		if err == nil && info.IsDir() {
			return Defaults{Root: dir}, nil
		}
	}
	// framework/ is the embedded tree; strip the prefix so overlay.Merge
	// sees content/, app/, etc at the root of the FS.
	sub, err := fs.Sub(frameworkFS, "framework")
	if err != nil {
		return Defaults{}, err
	}
	// Sanity-check the staged tree is real (not just the .gitkeep marker).
	if _, err := fs.Stat(sub, "composer.json"); err != nil {
		return Defaults{}, errors.New("embedded framework is empty or incomplete. Run scripts/stage-defaults.sh before building with -tags embed_defaults")
	}
	return Defaults{FS: sub}, nil
}
