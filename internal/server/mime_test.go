package server

import "testing"

func TestMimeFor(t *testing.T) {
	cases := map[string]string{
		"app.js":                 "text/javascript",
		"pdf.worker.min-yat.mjs": "text/javascript",
		"index.css":              "text/css",
		"manifest.webmanifest":   "application/manifest+json",
		"asset.json":             "application/json",
		"file.map":               "application/json",
		"font.woff2":             "font/woff2",
		"pic.webp":               "image/webp",
		"unknown.bin":            "application/octet-stream",
	}
	for name, want := range cases {
		if got := mimeFor(name); got != want {
			t.Fatalf("mimeFor(%q) = %q, want %q", name, got, want)
		}
	}
}
