package project

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoad_Minimal(t *testing.T) {
	dir := writeConfig(t, `leaf:
  name: My Docs
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "My Docs" {
		t.Errorf("name: got %q", cfg.Name)
	}
	if cfg.ContentPath != "content" {
		t.Errorf("default content_path not applied: got %q", cfg.ContentPath)
	}
	if cfg.OutputPath != "dist" {
		t.Errorf("default output_path not applied: got %q", cfg.OutputPath)
	}
}

func TestLoad_Full(t *testing.T) {
	dir := writeConfig(t, `leaf:
  name: Zephyrus Leaf
  version: 1.0.0
  description: Static sites quietly crafted.
  github_url: https://github.com/ophelios-studio/zephyrus-leaf
  content_path: docs
  output_path: build
  base_url: /leaf
  production_url: https://leaf.ophelios.com
  sections:
    getting-started: Getting Started
    guides: Guides
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContentPath != "docs" || cfg.OutputPath != "build" {
		t.Errorf("custom paths not honored: content=%q output=%q", cfg.ContentPath, cfg.OutputPath)
	}
	if got, want := cfg.Sections["getting-started"], "Getting Started"; got != want {
		t.Errorf("section label: got %q want %q", got, want)
	}
	if cfg.ProductionURL != "https://leaf.ophelios.com" {
		t.Errorf("production_url: got %q", cfg.ProductionURL)
	}
}

func TestLoad_Missing(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("expected error for missing config.yml")
	}
}

func TestLoad_Invalid(t *testing.T) {
	dir := writeConfig(t, `leaf: {{{ broken yaml`)
	if _, err := Load(dir); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestNormalizeHooks(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want [][]string
	}{
		{
			name: "empty list",
			yaml: "leaf:\n  name: S\n  post_build: []\n",
			want: nil,
		},
		{
			name: "string entries",
			yaml: "leaf:\n  name: S\n  post_build:\n    - ./a.sh\n    - ./b.sh\n",
			want: [][]string{{"./a.sh"}, {"./b.sh"}},
		},
		{
			name: "argv sequence entries",
			yaml: "leaf:\n  name: S\n  post_build:\n    - [\"./deploy.sh\", \"--prod\"]\n    - [\"./opt.sh\"]\n",
			want: [][]string{{"./deploy.sh", "--prod"}, {"./opt.sh"}},
		},
		{
			name: "mixed string and argv",
			yaml: "leaf:\n  name: S\n  post_build:\n    - ./a.sh\n    - [\"./b.sh\", \"x\"]\n",
			want: [][]string{{"./a.sh"}, {"./b.sh", "x"}},
		},
		{
			name: "empty strings dropped",
			yaml: "leaf:\n  name: S\n  post_build:\n    - \"\"\n    - ./ok.sh\n    - []\n    - [\"./has-arg\", \"\"]\n",
			want: [][]string{{"./ok.sh"}, {"./has-arg"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeConfig(t, tc.yaml)
			cfg, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			got := cfg.NormalizeHooks()
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got %d (%#v), want %d (%#v)", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if !equalStringSlice(got[i].Argv, tc.want[i]) {
					t.Errorf("hook %d: got %v, want %v", i, got[i].Argv, tc.want[i])
				}
			}
		})
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestValidate_RequiresName(t *testing.T) {
	c := &Config{}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for missing name")
	}
	c.Name = "x"
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}
