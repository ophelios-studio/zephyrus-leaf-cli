// Package builder wraps the overlay+runtime+publish flow so both `leaf build`
// and `leaf dev` can drive it.
package builder

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/overlay"
	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/project"
	"github.com/ophelios-studio/zephyrus-leaf-cli/internal/runtime"
)

type Options struct {
	ProjectRoot string
	KeepTmp     bool
	Runner      runtime.Runner // defaults to runtime.Default() when nil
}

// Build runs one full build cycle. Returns the PHP exit code (0 on success) or
// a Go error for pipeline failures (merge, missing config, etc.).
func Build(ctx context.Context, opts Options) (int, error) {
	cfg, err := project.Load(opts.ProjectRoot)
	if err != nil {
		return 0, err
	}
	if err := cfg.Validate(); err != nil {
		return 0, err
	}

	defaults, err := project.DefaultsSource()
	if err != nil {
		return 0, err
	}

	workdir, err := os.MkdirTemp("", "leaf-build-")
	if err != nil {
		return 0, fmt.Errorf("create tempdir: %w", err)
	}
	if !opts.KeepTmp {
		defer os.RemoveAll(workdir)
	}

	if err := overlay.Merge(workdir, []overlay.Source{
		{Name: "defaults", Priority: 0, FS: defaults.FS, Root: defaults.Root, Skip: []string{"content", "config.yml", "dist", "cache", ".git"}},
		{Name: "user", Priority: 10, Root: opts.ProjectRoot, Skip: []string{"dist", "templates"}},
	}); err != nil {
		return 0, fmt.Errorf("merge project: %w", err)
	}

	userTemplates := filepath.Join(opts.ProjectRoot, "templates")
	if _, err := os.Stat(userTemplates); err == nil {
		if err := overlay.Merge(filepath.Join(workdir, "app", "Views"), []overlay.Source{
			{Name: "user-templates", Root: userTemplates},
		}); err != nil {
			return 0, fmt.Errorf("merge templates: %w", err)
		}
	}

	entry := filepath.Join(workdir, "bin", "build.php")
	if _, err := os.Stat(entry); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, errors.New("bin/build.php missing from merged tree (defaults incomplete)")
		}
		return 0, err
	}

	runner := opts.Runner
	if runner == nil {
		runner = runtime.Default()
	}
	// Binary tier drives hooks itself (post-publish, cwd = real project).
	// Tell leaf-core to skip PHP-side hook execution.
	code, err := runner.Run(ctx, entry, nil, workdir, map[string]string{
		"LEAF_SKIP_HOOKS": "1",
	})
	if err != nil {
		return 0, fmt.Errorf("php: %w", err)
	}
	if code != 0 {
		return code, nil
	}

	distSrc := filepath.Join(workdir, cfg.OutputPath)
	distDst := filepath.Join(opts.ProjectRoot, cfg.OutputPath)
	if err := replaceDir(distDst, distSrc); err != nil {
		return 0, fmt.Errorf("publish dist: %w", err)
	}

	// Run config-declared post_build hooks after dist/ is in place. Hooks
	// run with cwd = the user's real project root (not the tempdir) so
	// anything they produce is reachable and persistent.
	if err := runPostBuildHooks(ctx, opts.ProjectRoot, cfg.NormalizeHooks()); err != nil {
		return 1, err
	}

	return 0, nil
}

// runPostBuildHooks executes each hook in order. Stdout/stderr inherit the
// parent's, so hook output streams live. First non-zero exit aborts the chain.
func runPostBuildHooks(ctx context.Context, projectRoot string, hooks []project.Hook) error {
	if len(hooks) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Running post_build hooks...")
	for _, hook := range hooks {
		if len(hook.Argv) == 0 {
			continue
		}
		fmt.Fprintf(os.Stdout, "  -> %v\n", hook.Argv)
		cmd := exec.CommandContext(ctx, hook.Argv[0], hook.Argv[1:]...)
		cmd.Dir = projectRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return fmt.Errorf("hook %v exited %d", hook.Argv, exitErr.ExitCode())
			}
			return fmt.Errorf("hook %v: %w", hook.Argv, err)
		}
	}
	return nil
}

func replaceDir(dst, src string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source %s: %w", src, err)
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return copyTree(src, dst)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
