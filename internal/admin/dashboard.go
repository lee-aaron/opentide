package admin

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// spaHandler serves the embedded React SPA with client-side routing support.
// Static files (JS, CSS, images) are served directly.
// All other paths get index.html for React Router to handle.
func spaHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("embedded dist/ not found: " + err.Error())
	}

	fileServer := http.StripPrefix("/admin/", http.FileServer(http.FS(sub)))
	indexHTML, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		panic("embedded index.html not found: " + err.Error())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip /admin prefix to get the file path within dist/
		path := strings.TrimPrefix(r.URL.Path, "/admin")
		path = strings.TrimPrefix(path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to serve the file directly
		if f, err := sub.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for client-side routes
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
}
