package main

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

//go:embed dist
var distFS embed.FS

var apiTarget *url.URL

func init() {
	targetStr := os.Getenv("API_TARGET")
	if targetStr == "" {
		targetStr = "http://127.0.0.1:9090"
	}
	var err error
	apiTarget, err = url.Parse(targetStr)
	if err != nil {
		log.Fatalf("Invalid API_TARGET: %v", err)
	}
}

func apiProxy(w http.ResponseWriter, r *http.Request) {
	proxy := httputil.NewSingleHostReverseProxy(apiTarget)

	// Remove trailing slash from path
	r.URL.Path = strings.TrimRight(r.URL.Path, "/")

	// Modify request for proxy
	r.Host = apiTarget.Host

	// Set custom transport for better error handling
	proxy.Transport = &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
	}

	// Error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		log.Printf("Proxy error: %v", err)
		http.Error(w, `{"error":"Backend unavailable"}`, http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)
}

func apiHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Try to reach the backend
	resp, err := http.Get(apiTarget.String() + "/api/health")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unavailable","error":"Backend not reachable"}`))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// API endpoints — proxy to sing-box API
	if strings.HasPrefix(path, "/api/") {
		apiProxy(w, r)
		return
	}

	// Health check for web-ui itself
	if path == "/health" {
		apiHealthCheck(w, r)
		return
	}

	// Static files
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Fatal(err)
	}
	http.FileServer(http.FS(sub)).ServeHTTP(w, r)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "11501"
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("Web UI server starting on :%s", port)
	log.Printf("API target: %s", apiTarget.String())

	http.HandleFunc("/", corsMiddleware(router))

	log.Printf("Server running on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
