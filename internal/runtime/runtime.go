// Package runtime abstracts the PHP execution surface the CLI uses.
//
// Two implementations satisfy Runner:
//
//   - Exec (default, build tag !embed_php): shells out to system `php`. Used
//     for local development so contributors don't need a static-php-cli
//     toolchain to iterate on Go code.
//
//   - FrankenPHP (build tag embed_php): calls frankenphp.ExecuteScriptCLI
//     against the PHP interpreter statically linked into the release binary.
//
// Production releases are built with `-tags embed_php`. Go-side tests use the
// exec path with system PHP, or a Mock for unit coverage.
package runtime

import "context"

// Runner executes a PHP script and returns its exit code.
type Runner interface {
	// Run dispatches script with args inside cwd. Env entries are merged into
	// the child process's environment. Exit code 0 is success.
	Run(ctx context.Context, script string, args []string, cwd string, env map[string]string) (int, error)
}
