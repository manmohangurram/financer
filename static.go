package main

import (
	_ "embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// The bundled default Yahoo endpoint config, copied into data/config on first
// run so the operator can edit it without rebuilding the image.
//go:embed config/yahoo.json
var defaultYahooConfig []byte

func writeDefaultYahooConfig(path string) error {
	return os.WriteFile(path, defaultYahooConfig, 0o644)
}

// spaHandler serves the built Vue app from dir with an SPA fallback: any path
// that does not resolve to a real file returns index.html so client-side
// routes (/accounts/rules, /settings, …) work on refresh. Returns nil when
// dist is absent so the server keeps serving only the API (dev mode).
func spaHandler(dir string) http.Handler {
	index, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		return nil
	}
	fileServer := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			clean := filepath.Clean(filepath.Join(dir, filepath.FromSlash(p)))
			if strings.HasPrefix(clean, dir+string(filepath.Separator)) {
				if f, err := os.Open(clean); err == nil {
					f.Close()
					fileServer.ServeHTTP(w, r)
					return
				}
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})
}
