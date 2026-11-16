package builder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/project"
)

func TestRunPostBuildHooks_NonExecutableFileNamesTheInterpreterFix(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hook.sh"), []byte("echo hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := runPostBuildHooks(context.Background(), root, []project.Hook{{Argv: []string{"./hook.sh", "arg"}}})
	if err == nil {
		t.Fatal("expected an error for a file the system cannot execute")
	}

	want := "hook [./hook.sh arg]: " + errors.Unwrap(err).Error() +
		"; hooks run without a shell and this file is not an executable for this system," +
		` name its interpreter first, for example ["bash", "./hook.sh", "arg"]`
	if err.Error() != want {
		t.Errorf("unexpected error\n got: %s\nwant: %s", err.Error(), want)
	}
	if !errors.Is(err, errBadExecutableFormat) {
		t.Errorf("error should wrap the original cause, got %v", err)
	}
}

func TestRunPostBuildHooks_MissingFileKeepsTheOriginalMessage(t *testing.T) {
	err := runPostBuildHooks(context.Background(), t.TempDir(), []project.Hook{{Argv: []string{"./absent-hook"}}})
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
	if got := err.Error(); got != "hook [./absent-hook]: "+errors.Unwrap(err).Error() {
		t.Errorf("a missing file must not get the interpreter hint, got: %s", got)
	}
}
