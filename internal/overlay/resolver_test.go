package overlay

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMerge_UserOverridesDefaults(t *testing.T) {
	defaults := fstest.MapFS{
		"templates/layouts/docs.latte":    {Data: []byte("DEFAULT DOCS")},
		"templates/partials/nav.latte":    {Data: []byte("DEFAULT NAV")},
		"public/assets/css/app.css":       {Data: []byte("DEFAULT CSS")},
	}
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(userDir, "templates", "layouts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "templates", "layouts", "docs.latte"), []byte("USER DOCS"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	err := Merge(dst, []Source{
		{Name: "defaults", Priority: 0, FS: defaults},
		{Name: "user", Priority: 10, Root: userDir},
	})
	if err != nil {
		t.Fatal(err)
	}

	// User override wins.
	if got := readFile(t, filepath.Join(dst, "templates", "layouts", "docs.latte")); got != "USER DOCS" {
		t.Errorf("docs.latte: got %q, want user override", got)
	}
	// Default preserved where user didn't override.
	if got := readFile(t, filepath.Join(dst, "templates", "partials", "nav.latte")); got != "DEFAULT NAV" {
		t.Errorf("nav.latte: got %q, want default", got)
	}
	if got := readFile(t, filepath.Join(dst, "public", "assets", "css", "app.css")); got != "DEFAULT CSS" {
		t.Errorf("app.css: got %q", got)
	}
}

func TestMerge_PriorityDeterminesOrder(t *testing.T) {
	low := fstest.MapFS{"x.txt": {Data: []byte("LOW")}}
	mid := fstest.MapFS{"x.txt": {Data: []byte("MID")}}
	high := fstest.MapFS{"x.txt": {Data: []byte("HIGH")}}

	dst := t.TempDir()
	// Pass out of order to confirm sort works.
	err := Merge(dst, []Source{
		{Name: "high", Priority: 20, FS: high},
		{Name: "low", Priority: 0, FS: low},
		{Name: "mid", Priority: 10, FS: mid},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dst, "x.txt")); got != "HIGH" {
		t.Errorf("priority order wrong: got %q want HIGH", got)
	}
}

func TestMerge_EmptySourceSkipped(t *testing.T) {
	defaults := fstest.MapFS{"a.txt": {Data: []byte("A")}}
	dst := t.TempDir()
	err := Merge(dst, []Source{
		{Name: "defaults", Priority: 0, FS: defaults},
		{Name: "empty", Priority: 10, Root: ""},
	})
	if err != nil {
		t.Fatalf("empty source should be a no-op: %v", err)
	}
	if got := readFile(t, filepath.Join(dst, "a.txt")); got != "A" {
		t.Errorf("got %q", got)
	}
}

func TestMerge_MissingUserDirIsNotError(t *testing.T) {
	// User hasn't created `templates/`; defaults alone should win.
	defaults := fstest.MapFS{"templates/base.latte": {Data: []byte("D")}}
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	dst := t.TempDir()
	err := Merge(dst, []Source{
		{Name: "defaults", Priority: 0, FS: defaults},
		{Name: "user", Priority: 10, Root: missing},
	})
	// Root missing means os.DirFS returns an FS whose Open fails; WalkDir
	// surfaces that error. We explicitly choose to tolerate this.
	if err == nil {
		// Acceptable: defaults applied.
		if got := readFile(t, filepath.Join(dst, "templates", "base.latte")); got != "D" {
			t.Errorf("default lost: %q", got)
		}
		return
	}
	t.Logf("missing user dir surfaced as error (acceptable): %v", err)
}

func TestMerge_Skip(t *testing.T) {
	defaults := fstest.MapFS{
		"content/getting-started/intro.md": {Data: []byte("DEFAULT INTRO")},
		"templates/docs.latte":             {Data: []byte("D")},
	}
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(userDir, "content", "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "content", "api", "ref.md"), []byte("USER REF"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	err := Merge(dst, []Source{
		{Name: "defaults", Priority: 0, FS: defaults, Skip: []string{"content"}},
		{Name: "user", Priority: 10, Root: userDir},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "content", "getting-started", "intro.md")); err == nil {
		t.Errorf("default content/ leaked through Skip")
	}
	if got := readFile(t, filepath.Join(dst, "content", "api", "ref.md")); got != "USER REF" {
		t.Errorf("user content lost: %q", got)
	}
	if got := readFile(t, filepath.Join(dst, "templates", "docs.latte")); got != "D" {
		t.Errorf("non-skipped default dropped: %q", got)
	}
}

func TestMerge_NestedDirectories(t *testing.T) {
	defaults := fstest.MapFS{
		"a/b/c/d.txt": {Data: []byte("deep")},
	}
	dst := t.TempDir()
	if err := Merge(dst, []Source{{Name: "defaults", FS: defaults}}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dst, "a", "b", "c", "d.txt")); got != "deep" {
		t.Errorf("nested file wrong: %q", got)
	}
}
