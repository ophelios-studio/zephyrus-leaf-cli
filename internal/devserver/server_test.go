package devserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newDistServer(t *testing.T, files map[string]string) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	for path, body := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return NewServer(dir, NewHub()), dir
}

func get(t *testing.T, s *Server, path string) (int, string, http.Header) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	body, _ := io.ReadAll(rr.Result().Body)
	return rr.Result().StatusCode, string(body), rr.Result().Header
}

func TestServer_ServesIndexAtRoot(t *testing.T) {
	s, _ := newDistServer(t, map[string]string{
		"index.html": "<html><body>hi</body></html>",
	})
	code, body, hdr := get(t, s, "/")
	if code != 200 {
		t.Fatalf("code %d", code)
	}
	if !strings.Contains(hdr.Get("Content-Type"), "text/html") {
		t.Errorf("content-type: %s", hdr.Get("Content-Type"))
	}
	if !strings.Contains(body, "EventSource") {
		t.Errorf("reload snippet missing: %s", body)
	}
	if !strings.Contains(body, "hi") {
		t.Errorf("original content missing: %s", body)
	}
}

func TestServer_InjectsBeforeBodyClose(t *testing.T) {
	html := []byte("<html><body>X</body></html>")
	out := InjectReload(html)
	// Snippet sits before the closing tag.
	if !strings.Contains(string(out), "X"+"<script>") {
		t.Errorf("snippet not immediately before </body>: %s", out)
	}
	if !strings.HasSuffix(string(out), "</body></html>") {
		t.Errorf("tail mangled: %s", out)
	}
}

func TestServer_InjectsWhenNoBodyTag(t *testing.T) {
	html := []byte("plain document")
	out := InjectReload(html)
	if !strings.HasPrefix(string(out), "plain document") {
		t.Errorf("prefix mangled: %s", out)
	}
	if !strings.Contains(string(out), "EventSource") {
		t.Errorf("snippet missing: %s", out)
	}
}

func TestServer_404FallsBackToSiteFile(t *testing.T) {
	s, _ := newDistServer(t, map[string]string{
		"404.html": "<html><body>custom missing</body></html>",
	})
	code, body, _ := get(t, s, "/does-not-exist")
	if code != 404 {
		t.Errorf("status %d", code)
	}
	if !strings.Contains(body, "custom missing") {
		t.Errorf("fallback 404 missing: %s", body)
	}
}

func TestServer_404Plain(t *testing.T) {
	s, _ := newDistServer(t, map[string]string{})
	code, body, _ := get(t, s, "/nothing")
	if code != 404 {
		t.Errorf("status %d", code)
	}
	if !strings.Contains(body, "404") {
		t.Errorf("body: %s", body)
	}
}

func TestServer_DirectoryServesIndex(t *testing.T) {
	s, _ := newDistServer(t, map[string]string{
		"guide/index.html": "<html><body>guide</body></html>",
	})
	code, body, _ := get(t, s, "/guide/")
	if code != 200 {
		t.Fatalf("code %d, body: %s", code, body)
	}
	if !strings.Contains(body, "guide") {
		t.Errorf("body: %s", body)
	}
}

func TestServer_RejectsPathTraversal(t *testing.T) {
	s, _ := newDistServer(t, map[string]string{"index.html": "ok"})
	code, _, _ := get(t, s, "/../etc/passwd")
	if code == 200 {
		t.Errorf("traversal not rejected")
	}
}
