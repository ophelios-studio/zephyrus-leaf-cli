package runtime

import "context"

// Mock is a test double. Records invocations and returns a preset exit code.
type Mock struct {
	ExitCode int
	Err      error
	Calls    []MockCall
}

type MockCall struct {
	Script string
	Args   []string
	Cwd    string
	Env    map[string]string
}

func (m *Mock) Run(ctx context.Context, script string, args []string, cwd string, env map[string]string) (int, error) {
	m.Calls = append(m.Calls, MockCall{Script: script, Args: args, Cwd: cwd, Env: env})
	return m.ExitCode, m.Err
}
