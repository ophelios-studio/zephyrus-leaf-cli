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
	"runtime"
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
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
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
		// Preserve the source file mode so executables stay executable
		// (integration tests rely on shell scripts being runnable).
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

// TestBuild_Standalone exercises the embed_defaults path: binary built with
// the framework baked in, LEAF_DEFAULTS_DIR unset, init + build should still
// succeed. Skips when the local environment can't compile with that tag
// (no staged framework tree).
func TestBuild_MultiLocaleContent(t *testing.T) {
	requirePHP(t)
	defaults := defaultsDir(t)
	bin := buildBinary(t)
	fixture := prepareFixture(t, "multi-locale-site")

	cmd := exec.Command(bin, "build", "--dir", fixture)
	cmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("leaf build failed: %v\n%s", err, out)
	}

	type check struct {
		path, marker, label string
	}
	cases := []check{
		// English default
		{"dist/getting-started/intro/index.html", "DEFAULT_INTRO_MARKER", "en intro"},
		{"dist/getting-started/installation/index.html", "ENGLISH_ONLY_MARKER", "en installation"},
		// French: translated intro, fallback installation, French-only section
		{"dist/fr/getting-started/intro/index.html", "FRENCH_INTRO_MARKER", "fr intro (translated)"},
		{"dist/fr/getting-started/installation/index.html", "ENGLISH_ONLY_MARKER", "fr installation (fallback)"},
		{"dist/fr/concepts/overview/index.html", "FRENCH_ONLY_SECTION_MARKER", "fr-only concepts"},
	}
	for _, c := range cases {
		data, err := os.ReadFile(filepath.Join(fixture, c.path))
		if err != nil {
			t.Errorf("[%s] missing %s: %v", c.label, c.path, err)
			continue
		}
		if !strings.Contains(string(data), c.marker) {
			t.Errorf("[%s] %s missing from %s", c.label, c.marker, c.path)
		}
	}

	// French-only content must NOT appear in the English build.
	if _, err := os.Stat(filepath.Join(fixture, "dist", "concepts", "overview", "index.html")); err == nil {
		t.Errorf("French-only section leaked into the English dist")
	}

	// Per-locale search indices must exist; each locale's index surfaces
	// its own translated content.
	enSearch, err := os.ReadFile(filepath.Join(fixture, "dist", "search.json"))
	if err != nil {
		t.Fatalf("english search.json missing: %v", err)
	}
	if !strings.Contains(string(enSearch), "DEFAULT_INTRO_MARKER") {
		t.Errorf("english search index doesn't contain default intro text")
	}
	if strings.Contains(string(enSearch), "FRENCH_INTRO_MARKER") {
		t.Errorf("english search index leaked French content")
	}

	frSearch, err := os.ReadFile(filepath.Join(fixture, "dist", "fr", "search.json"))
	if err != nil {
		t.Fatalf("french search.json missing: %v", err)
	}
	if !strings.Contains(string(frSearch), "FRENCH_INTRO_MARKER") {
		t.Errorf("french search index missing translated intro")
	}
	if !strings.Contains(string(frSearch), "FRENCH_ONLY_SECTION_MARKER") {
		t.Errorf("french search index missing french-only section")
	}
	if !strings.Contains(string(frSearch), "ENGLISH_ONLY_MARKER") {
		t.Errorf("french search index should include English fallback pages")
	}
}

func TestBuild_PostBuildHooks(t *testing.T) {
	requirePHP(t)
	defaults := defaultsDir(t)
	bin := buildBinary(t)
	fixture := prepareFixture(t, "hooks-site")

	// Clean any stale sentinels from a prior run.
	os.Remove(filepath.Join(fixture, "hook-marker.txt"))
	os.Remove(filepath.Join(fixture, "hook-stamp.txt"))

	cmd := exec.Command(bin, "build", "--dir", fixture)
	cmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("leaf build failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Running post_build hooks") {
		t.Errorf("expected hook announcement in build log; got:\n%s", out)
	}

	marker, err := os.ReadFile(filepath.Join(fixture, "hook-marker.txt"))
	if err != nil {
		t.Fatalf("first hook did not run (hook-marker.txt missing): %v", err)
	}
	if !strings.Contains(string(marker), "FIRST_HOOK_RAN") {
		t.Errorf("first hook wrote unexpected contents: %q", marker)
	}

	stamp, err := os.ReadFile(filepath.Join(fixture, "hook-stamp.txt"))
	if err != nil {
		t.Fatalf("second hook did not run (hook-stamp.txt missing): %v", err)
	}
	if !strings.Contains(string(stamp), "SECOND_HOOK_ARG=second-hook") {
		t.Errorf("second hook did not receive argv arg: %q", stamp)
	}
}

func TestBuild_PostBuildHookFailureFailsBuild(t *testing.T) {
	requirePHP(t)
	defaults := defaultsDir(t)
	bin := buildBinary(t)
	fixture := prepareFixture(t, "hooks-site")

	// Replace the first hook with a failing one.
	failing := filepath.Join(fixture, "scripts", "touch-marker.sh")
	if err := os.WriteFile(failing, []byte("#!/usr/bin/env bash\nexit 17\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "build", "--dir", fixture)
	cmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	out, _ := cmd.CombinedOutput()
	if cmd.ProcessState.ExitCode() == 0 {
		t.Fatalf("expected non-zero exit when hook fails; got 0\n%s", out)
	}
	if !strings.Contains(string(out), "exited 17") {
		t.Errorf("expected hook exit code in output; got:\n%s", out)
	}
	// Second hook must not run after the first failed.
	if _, err := os.Stat(filepath.Join(fixture, "hook-stamp.txt")); err == nil {
		t.Errorf("second hook ran despite first hook failing")
	}
}

func TestBuild_CustomPages(t *testing.T) {
	requirePHP(t)
	defaults := defaultsDir(t)
	bin := buildBinary(t)
	fixture := prepareFixture(t, "pages-site")

	cmd := exec.Command(bin, "build", "--dir", fixture)
	cmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("leaf build failed: %v\n%s", err, out)
	}

	// /about/index.html contains the about marker.
	about, err := os.ReadFile(filepath.Join(fixture, "dist", "about", "index.html"))
	if err != nil {
		t.Fatalf("/about missing: %v", err)
	}
	if !strings.Contains(string(about), "CUSTOM_PAGE_ABOUT_MARKER") {
		t.Errorf("about marker missing; got:\n%s", about)
	}
	if !strings.Contains(string(about), "/about") {
		t.Errorf("pagePath not propagated to template")
	}

	// /contact/index.html contains the contact marker.
	contact, err := os.ReadFile(filepath.Join(fixture, "dist", "contact", "index.html"))
	if err != nil {
		t.Fatalf("/contact missing: %v", err)
	}
	if !strings.Contains(string(contact), "CUSTOM_PAGE_CONTACT_MARKER") {
		t.Errorf("contact marker missing; got:\n%s", contact)
	}

	// Docs page still builds alongside the custom pages.
	if _, err := os.Stat(filepath.Join(fixture, "dist", "guide", "intro", "index.html")); err != nil {
		t.Errorf("docs page missing after enabling pages feature: %v", err)
	}
}

func TestBuild_Standalone(t *testing.T) {
	requirePHP(t)
	repoRoot, _ := filepath.Abs(filepath.Join("..", ".."))
	markerPath := filepath.Join(repoRoot, "internal", "project", "framework", "composer.json")
	if _, err := os.Stat(markerPath); err != nil {
		t.Skip("framework not staged; run 'go run ./scripts/stage -src <zephyrus-leaf>' first")
	}

	tmp := t.TempDir()
	out := filepath.Join(tmp, "leaf")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	build := exec.Command("go", "build", "-tags", "embed_defaults", "-o", out, "./cmd/leaf")
	build.Dir = repoRoot
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("go build -tags embed_defaults: %v", err)
	}

	target := filepath.Join(tmp, "mysite")
	// Deliberately empty environment except PATH (LEAF_DEFAULTS_DIR MUST NOT be set).
	clean := []string{"PATH=" + os.Getenv("PATH")}

	initCmd := exec.Command(out, "init", target)
	initCmd.Env = clean
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("standalone init: %v\n%s", err, out)
	}
	buildCmd := exec.Command(out, "build", "--dir", target)
	buildCmd.Env = clean
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("standalone build: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(target, "dist", "getting-started", "introduction", "index.html")); err != nil {
		t.Errorf("expected dist page missing: %v", err)
	}
}

func TestEject_RestoresFramework(t *testing.T) {
	defaults := defaultsDir(t)
	bin := buildBinary(t)

	// Start from an init'd site.
	target := filepath.Join(t.TempDir(), "site")
	initCmd := exec.Command(bin, "init", target)
	initCmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

	// Eject.
	ejectCmd := exec.Command(bin, "eject", "--dir", target)
	ejectCmd.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	if out, err := ejectCmd.CombinedOutput(); err != nil {
		t.Fatalf("eject failed: %v\n%s", err, out)
	}

	// Framework files must now be present.
	for _, required := range []string{"app", "bin", "composer.json"} {
		if _, err := os.Stat(filepath.Join(target, required)); err != nil {
			t.Errorf("eject did not add %s: %v", required, err)
		}
	}
	// User files must survive.
	for _, keep := range []string{"content", "config.yml"} {
		if _, err := os.Stat(filepath.Join(target, keep)); err != nil {
			t.Errorf("eject destroyed %s: %v", keep, err)
		}
	}

	// Second eject without --force must refuse.
	refuse := exec.Command(bin, "eject", "--dir", target)
	refuse.Env = append(os.Environ(), "LEAF_DEFAULTS_DIR="+defaults)
	if out, err := refuse.CombinedOutput(); err == nil {
		t.Fatalf("expected refuse on re-eject; got:\n%s", out)
	}
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
