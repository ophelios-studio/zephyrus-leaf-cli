//go:build !windows

package builder

import "syscall"

var errBadExecutableFormat error = syscall.ENOEXEC
