package builder

import "syscall"

// ERROR_BAD_EXE_FORMAT (193), not exported by the syscall package.
var errBadExecutableFormat error = syscall.Errno(193)
