// Package webui serves the built Angular frontend. By default it serves the
// build embedded into the binary at compile time; setting FRONTEND_DIR at
// runtime overrides that with a directory on disk instead, so a new
// frontend build can be dropped onto an existing container/binary without
// rebuilding it.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// all: prefix is needed so a checkout with only the placeholder ./dist/.gitkeep
// (see that file for why it exists) still embeds successfully -- go:embed's
// default pattern excludes dotfiles, and an empty match is a build error.
//
//go:embed all:dist
var embedded embed.FS

// FS returns the static frontend filesystem to serve: os.DirFS(dir) if dir
// is non-empty, else the build embedded in the binary.
func FS(dir string) (fs.FS, error) {
	if dir != "" {
		return os.DirFS(dir), nil
	}
	return fs.Sub(embedded, "dist")
}

// Handler serves fsys as a single-page app: any request path that doesn't
// exist in fsys falls back to /index.html, so Angular's client-side router
// handles it -- equivalent to the original Caddyfile's
// `try_files {path} /index.html; file_server`.
func Handler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if clean == "." {
			clean = "index.html"
		}
		if _, err := fs.Stat(fsys, clean); err != nil {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
