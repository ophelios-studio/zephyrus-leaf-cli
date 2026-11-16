package main

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRunEject_AddsFrontControllerAndKeepsUserFiles(t *testing.T) {
	defaults := t.TempDir()
	write(t, filepath.Join(defaults, "composer.json"), "TEMPLATE COMPOSER")
	write(t, filepath.Join(defaults, "README.md"), "TEMPLATE README")
	write(t, filepath.Join(defaults, "config.yml"), "TEMPLATE CONFIG")
	write(t, filepath.Join(defaults, "app", "Kernel.php"), "TEMPLATE KERNEL")
	write(t, filepath.Join(defaults, "public", "index.php"), "TEMPLATE INDEX")
	write(t, filepath.Join(defaults, "public", "assets", "app.css"), "TEMPLATE CSS")
	write(t, filepath.Join(defaults, "public", "favicon.ico"), "TEMPLATE ICON")
	t.Setenv("LEAF_DEFAULTS_DIR", defaults)

	project := t.TempDir()
	write(t, filepath.Join(project, "config.yml"), "USER CONFIG")
	write(t, filepath.Join(project, "README.md"), "USER README")
	write(t, filepath.Join(project, "public", "favicon.ico"), "USER ICON")

	if code := runEject([]string{"-dir", project}); code != 0 {
		t.Fatalf("runEject exit code %d", code)
	}

	expected := map[string]string{
		"composer.json":         "TEMPLATE COMPOSER",
		"app/Kernel.php":        "TEMPLATE KERNEL",
		"public/index.php":      "TEMPLATE INDEX",
		"public/assets/app.css": "TEMPLATE CSS",
		"public/favicon.ico":    "USER ICON",
		"README.md":             "USER README",
		"config.yml":            "USER CONFIG",
	}
	for rel, want := range expected {
		if got := read(t, filepath.Join(project, filepath.FromSlash(rel))); got != want {
			t.Errorf("%s: got %q, want %q", rel, got, want)
		}
	}
}
