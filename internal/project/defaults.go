package project

import "io/fs"

// Defaults describes where the CLI finds the framework scaffold. Exactly one
// of Root or FS is set.
//   - Root: absolute path on disk. Dev-mode source, or an extracted embed.
//   - FS:   an fs.FS (typically from go:embed) for release-mode binaries.
type Defaults struct {
	Root string
	FS   fs.FS
}
