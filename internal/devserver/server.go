package devserver

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// reloadSnippet is injected just before </body> in every HTML response. It
// subscribes to an SSE endpoint and reloads the page when the server pushes
// "reload".
const reloadSnippet = `<script>(function(){var es=new EventSource('/__leaf/reload');es.onmessage=function(e){if(e.data==='reload')location.reload()};})();</script>`

// Server serves a user's dist/ directory with HTML-injection for live reload
// and a WebSocket endpoint the watcher pings on rebuild.
type Server struct {
	DistRoot string
	Hub      *Hub
}

func NewServer(distRoot string, hub *Hub) *Server {
	return &Server{DistRoot: distRoot, Hub: hub}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/__leaf/reload" {
		s.Hub.Handler()(w, r)
		return
	}

	// Resolve path to a file under DistRoot. Falls back to /index.html for
	// directory paths.
	reqPath := r.URL.Path
	if reqPath == "" || strings.HasSuffix(reqPath, "/") {
		reqPath += "index.html"
	}
	clean := filepath.Clean(reqPath)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, "\x00") {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	file := filepath.Join(s.DistRoot, filepath.FromSlash(strings.TrimPrefix(clean, "/")))

	info, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			s.serve404(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		file = filepath.Join(file, "index.html")
	}

	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			s.serve404(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Content-type: trust extension for the usual cases. HTML gets injection.
	ext := strings.ToLower(filepath.Ext(file))
	switch ext {
	case ".html", ".htm":
		data = InjectReload(data)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	}
	// Disable caching so the next reload always sees fresh content.
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// InjectReload places the live-reload snippet just before </body>. If no
// </body> exists (e.g. fragment or non-HTML), append to the end of the doc.
func InjectReload(html []byte) []byte {
	idx := bytes.LastIndex(bytes.ToLower(html), []byte("</body>"))
	if idx < 0 {
		return append(html, []byte(reloadSnippet)...)
	}
	out := make([]byte, 0, len(html)+len(reloadSnippet))
	out = append(out, html[:idx]...)
	out = append(out, []byte(reloadSnippet)...)
	out = append(out, html[idx:]...)
	return out
}

func (s *Server) serve404(w http.ResponseWriter, r *http.Request) {
	// Respect the site's built 404 if present.
	if data, err := os.ReadFile(filepath.Join(s.DistRoot, "404.html")); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(InjectReload(data))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "404 not found: %s\n", r.URL.Path)
}
