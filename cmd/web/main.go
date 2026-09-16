// Command web serves the frontend in ./frontend and reverse-proxies /api/* to
// the JSON API started by the root main.go.
//
// It exists only so the browser and the API share an origin: the API sends no
// CORS headers and registers no OPTIONS routes, so a page loaded from a
// different port could not call it at all.
//
// Usage:
//
//	go run ./cmd/web                       # serve :8080, proxy to :8070
//	WEB_PORT=3000 API_URL=http://localhost:9000 go run ./cmd/web
package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	webPort := env("WEB_PORT", "8080")
	apiURL := env("API_URL", "http://localhost:"+env("PORT", "8070"))
	dir := env("WEB_DIR", "frontend")

	target, err := url.Parse(apiURL)
	if err != nil {
		log.Fatalf("bad API_URL %q: %v", apiURL, err)
	}

	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		log.Fatalf("no index.html in %q (run from the repo root, or set WEB_DIR): %v", dir, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy %s %s: %v", r.Method, r.URL.Path, err)
		http.Error(w, `{"error":{"message":"API unreachable"}}`, http.StatusBadGateway)
	}

	mux := http.NewServeMux()

	// Strip the /api prefix so /api/items reaches the API as /items.
	mux.Handle("/api/", http.StripPrefix("/api", proxy))

	files := http.FileServer(http.Dir(dir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// The frontend is a single page with hash routing, so anything that is
		// not an existing asset falls back to index.html.
		if r.URL.Path != "/" && !hasAsset(dir, r.URL.Path) {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	})

	log.Printf("frontend on http://localhost:%s  (proxying /api -> %s)", webPort, target)
	if err := http.ListenAndServe(":"+webPort, mux); err != nil {
		log.Fatal(err)
	}
}

func hasAsset(dir, reqPath string) bool {
	clean := filepath.Clean(strings.TrimPrefix(reqPath, "/"))
	if clean == "." || strings.HasPrefix(clean, "..") {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, clean))
	return err == nil && !info.IsDir()
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
