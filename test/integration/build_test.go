// Integration tests for the full `leaf build` flow.
//
// These tests compile the binary, prepare a fixture project, and run it
// against a LEAF_DEFAULTS_DIR pointing at a local zephyrus-leaf checkout.
// Requires:
//   - system `php` on PATH
//   - LEAF_DEFAULTS_DIR (or a zephyrus-leaf checkout at the expected path)
//
// Tests skip when prerequisites are missing, so the suite is safe to run on
// CI runners that may not have PHP yet.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func defaultsDir(t *testing.T) string {
	t.Helper()
	if d := os.Getenv("LEAF_DEFAULTS_DIR"); d != "" {
		return d
	}
	// Fallback to the sibling checkout when running locally.
	guess := filepath.Clean(filepath.Join("..", "..", "..", "zephyrus-leaf"))
	abs, _ := filepath.Abs(guess)
	if _, err := os.Stat(abs); err == nil {
		return abs
	}
	t.Skip("set LEAF_DEFAULTS_DIR to a local zephyrus-leaf checkout to run this test")
	return ""
}

func requirePHP(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("php"); err != nil {
		t.Skip("system php not on PATH")
	}
}

// buildBinary compiles ./cmd/leaf into tmp and returns the path.
func buildBinary(t *testing.T) string {
	t.Helper()
	repoRoot, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	out := filepath.Join(tmp, "leaf")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/leaf")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build: %v", err)
	}
	return out
}

// prepareFixture copies testdata/minimal-site into a writable tempdir so the
// test doesn't mutate the checked-in fixture when the build writes dist/.
func prepareFixture(t *testing.T, name string) string {
	t.Helper()
	src, _ := filepath.Abs(filepath.Join("testdata", name))
	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dst
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func TestInit_ThenBuild(t *testing.T) {
	requirePHP(t)
	defaults := defaultsDir(t)
	bin := buildBinary(t)

	parent := t.TempDir()
	target := filepath.Join(parent, "my-site")

	initCmd := exec.Command(bin, "init", target)
	initCmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("leaf init failed: %v\n%s", err, out)
	}

	// Framework internals must not leak into the scaffold.
	for _, forbidden := range []string{"app", "bin", "vendor", "composer.json", "docker-compose.yml"} {
		if _, err := os.Stat(filepath.Join(target, forbidden)); err == nil {
			t.Errorf("init leaked framework file: %s", forbidden)
		}
	}
	// User-facing files must be present.
	for _, required := range []string{"content", "config.yml"} {
		if _, err := os.Stat(filepath.Join(target, required)); err != nil {
			t.Errorf("init missing user file: %s (%v)", required, err)
		}
	}

	// A build against the scaffolded site must succeed out of the box.
	buildCmd := exec.Command(bin, "build", "--dir", target)
	buildCmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("post-init build failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(target, "dist")); err != nil {
		t.Errorf("post-init build did not produce dist: %v", err)
	}
}

func TestInit_RefusesNonEmpty(t *testing.T) {
	defaults := defaultsDir(t)
	bin := buildBinary(t)

	parent := t.TempDir()
	target := filepath.Join(parent, "busy")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	initCmd := exec.Command(bin, "init", target)
	initCmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	out, err := initCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for non-empty target; got output:\n%s", out)
	}

	// --force allows it.
	forceCmd := exec.Command(bin, "init", "--force", target)
	forceCmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	if out, err := forceCmd.CombinedOutput(); err != nil {
		t.Fatalf("--force init failed: %v\n%s", err, out)
	}
}

func TestBuild_TemplateOverride(t *testing.T) {
	requirePHP(t)
	defaults := defaultsDir(t)
	bin := buildBinary(t)
	fixture := prepareFixture(t, "override-site")

	cmd := exec.Command(bin, "build", "--dir", fixture)
	cmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("leaf build failed: %v\n%s", err, out)
	}
	t.Logf("build output:\n%s", out)

	pagePath := filepath.Join(fixture, "dist", "guide", "page", "index.html")
	data, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("expected built page missing: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "CUSTOM_NAV_MARKER_42") {
		t.Errorf("template override did not win; marker absent from %s", pagePath)
	}
}

func TestBuild_Minimal(t *testing.T) {
	requirePHP(t)
	defaults := defaultsDir(t)
	bin := buildBinary(t)
	fixture := prepareFixture(t, "minimal-site")

	cmd := exec.Command(bin, "build", "--dir", fixture)
	cmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("leaf build failed: %v\n%s", err, out)
	}
	t.Logf("build output:\n%s", out)

	// Assert the rendered page exists and contains our sanity marker.
	pagePath := filepath.Join(fixture, "dist", "guide", "introduction", "index.html")
	data, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("expected built page missing: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "LEAF_INTEGRATION_OK") {
		t.Errorf("sanity marker missing from %s", pagePath)
	}
	if !strings.Contains(body, "Integration Fixture Heading") {
		t.Errorf("markdown heading not rendered in %s", pagePath)
	}
}
