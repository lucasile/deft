package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed web/build/*
var webAssets embed.FS

func GetFS() (fs.FS, error) {
	return fs.Sub(webAssets, "web/build")
}

func Handler(publicFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(publicFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if cleanPath == "." || cleanPath == "" {
			serveFile(publicFS, w, r, "index.html")
			return
		}

		if fileExists(publicFS, cleanPath) {
			servePath(fileServer, w, r, "/"+cleanPath)
			return
		}

		if path.Ext(cleanPath) == "" {
			htmlPath := cleanPath + ".html"
			if fileExists(publicFS, htmlPath) {
				servePath(fileServer, w, r, "/"+htmlPath)
				return
			}

			serveFile(publicFS, w, r, "index.html")
			return
		}

		http.NotFound(w, r)
	})
}

func serveFile(publicFS fs.FS, w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(publicFS, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(name, ".html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	_, _ = w.Write(data)
}

func servePath(handler http.Handler, w http.ResponseWriter, r *http.Request, requestPath string) {
	next := r.Clone(r.Context())
	next.URL.Path = requestPath
	handler.ServeHTTP(w, next)
}

func fileExists(publicFS fs.FS, name string) bool {
	info, err := fs.Stat(publicFS, name)
	return err == nil && !info.IsDir()
}
