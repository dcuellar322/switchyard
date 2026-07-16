// Package web embeds the release-mode Vue assets into the Go control plane.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// assets is populated by the frontend build before release compilation.
//
//go:embed all:dist
var assets embed.FS

// Handler returns a single-page-application file server.
func Handler() http.Handler {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "embedded UI unavailable", http.StatusServiceUnavailable)
		})
	}
	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requested == "." || requested == "" {
			requested = "index.html"
		}
		if _, err := fs.Stat(dist, requested); err == nil {
			files.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(dist, "index.html"); err != nil {
			http.Error(w, "build the Vue application before release serving", http.StatusServiceUnavailable)
			return
		}
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	})
}
