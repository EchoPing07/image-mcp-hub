package web

import (
	"embed"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

//go:embed static/*
var staticFS embed.FS

// Handler serves the admin SPA. Any non-API, non-file path falls back to
// index.html so client-side routing works.
func Handler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// trim leading slash so FileServer paths resolve under static/
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		// SPA fallback for unknown routes (no file extension or missing file)
		if _, err := fs.Stat(sub, p); err != nil {
			r2 := new(http.Request)
			*r2 = *r
			r2.URL = cloneURL(r.URL, "/index.html")
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func cloneURL(u *url.URL, path string) *url.URL {
	cp := *u
	cp.Path = path
	cp.RawPath = ""
	return &cp
}
