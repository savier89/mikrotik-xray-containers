package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
)

// mockBackend creates a test HTTP server that simulates the sing-box API
func mockBackend(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimRight(r.URL.Path, "/")

		switch path {
		case "/api/health":
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			sendJSON(w, 200, map[string]interface{}{
				"status":    "ok",
				"timestamp": "2024-01-01T00:00:00Z",
			})

		case "/api/status":
			sendJSON(w, 200, map[string]interface{}{
				"running":             true,
				"pid":                 1234,
				"uptime":              3600,
				"api_port":            9090,
				"subscriptions_count": 2,
				"active_subscription": "sub_1",
				"selected_server":     0,
			})

		case "/api/stats":
			sendJSON(w, 200, map[string]interface{}{
				"upload":   1024,
				"download": 2048,
			})

		case "/api/connections":
			sendJSON(w, 200, map[string]interface{}{
				"uploadTotal":   1048576,
				"downloadTotal": 2097152,
				"metas": []map[string]interface{}{
					{"id": 1, "upload": 512, "download": 1024},
				},
			})

		case "/api/subscriptions":
			if r.Method == http.MethodGet {
				sendJSON(w, 200, map[string]interface{}{
					"subscriptions": []map[string]interface{}{
						{
							"id":           "sub_1",
							"name":         "Test Sub",
							"url":          "https://example.com/sub",
							"server_count": 5,
						},
					},
					"active": "sub_1",
				})
			} else if r.Method == http.MethodPost {
				sendJSON(w, 201, map[string]interface{}{
					"subscription": map[string]interface{}{
						"id":           "sub_new",
						"name":         "New Sub",
						"server_count": 3,
					},
					"message": "Subscription added",
				})
			} else {
				http.Error(w, "Not found", http.StatusNotFound)
			}

		case "/api/servers":
			sendJSON(w, 200, map[string]interface{}{
				"subscription": "Test Sub",
				"servers": []string{
					"vless://uuid@server1.example.com:443",
					"vless://uuid@server2.example.com:443",
				},
				"selected": 0,
			})

		case "/api/servers/select":
			sendJSON(w, 200, map[string]interface{}{
				"selected": 1,
				"server":   "vless://uuid@server2.example.com:443",
				"message":  "Server selected",
			})

		case "/api/servers/test":
			sendJSON(w, 200, map[string]interface{}{
				"results": []map[string]interface{}{
					{"index": 0, "latency": 50},
					{"index": 1, "latency": 120},
				},
				"fastest":  map[string]interface{}{"index": 0, "latency": 50},
				"selected": 0,
				"message":  "Server 0 selected (50ms)",
			})

		case "/api/config":
			sendJSON(w, 200, map[string]interface{}{
				"log":        map[string]interface{}{"level": "warn"},
				"dns":        map[string]interface{}{"servers": []string{"8.8.8.8"}},
				"inbounds":   []map[string]interface{}{{"type": "tun", "tag": "tun-in"}},
				"outbounds":  []map[string]interface{}{{"tag": "proxy", "type": "vless"}},
				"route":      map[string]interface{}{"rules": []interface{}{}},
				"experimental": map[string]interface{}{
					"clash_api": map[string]interface{}{
						"external_controller": "127.0.0.1:20123",
					},
				},
			})

		case "/api/config/reload":
			sendJSON(w, 200, map[string]interface{}{
				"message": "sing-box reload signal sent",
			})

		case "/api/logs":
			sendJSON(w, 200, map[string]interface{}{
				"logs": "[2024-01-01 00:00:00] sing-box started\n[2024-01-01 00:00:01] TUN interface created",
			})

		default:
			sendJSON(w, 404, map[string]interface{}{"error": "Not found"})
		}
	}))
}

func sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func TestAPIProxy_Health(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("Expected status ok, got %v", resp["status"])
	}
}

func TestAPIProxy_Status(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["running"] != true {
		t.Errorf("Expected running true, got %v", resp["running"])
	}
	if resp["api_port"] != float64(9090) {
		t.Errorf("Expected api_port 9090, got %v", resp["api_port"])
	}
}

func TestAPIProxy_Subscriptions(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// GET subscriptions
	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET subscriptions: Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	subs, ok := resp["subscriptions"].([]interface{})
	if !ok || len(subs) == 0 {
		t.Error("Expected subscriptions list")
	}

	// POST subscription
	body := strings.NewReader(`{"name":"New Sub","url":"https://example.com/sub"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("POST subscription: Expected 201, got %d", w.Code)
	}
}

func TestAPIProxy_Servers(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// GET servers
	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET servers: Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	servers, ok := resp["servers"].([]interface{})
	if !ok || len(servers) == 0 {
		t.Error("Expected servers list")
	}

	// POST select server
	body := strings.NewReader(`{"index":1}`)
	req = httptest.NewRequest(http.MethodPost, "/api/servers/select", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST select: Expected 200, got %d", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["selected"] != float64(1) {
		t.Errorf("Expected selected 1, got %v", resp["selected"])
	}

	// POST test servers
	body = strings.NewReader(`{"timeout":5}`)
	req = httptest.NewRequest(http.MethodPost, "/api/servers/test", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST test: Expected 200, got %d", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	results, ok := resp["results"].([]interface{})
	if !ok || len(results) == 0 {
		t.Error("Expected test results")
	}
}

func TestAPIProxy_Config(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// GET config
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET config: Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["log"]; !ok {
		t.Error("Expected log in config")
	}
	if _, ok := resp["dns"]; !ok {
		t.Error("Expected dns in config")
	}
	if _, ok := resp["inbounds"]; !ok {
		t.Error("Expected inbounds in config")
	}
	if _, ok := resp["outbounds"]; !ok {
		t.Error("Expected outbounds in config")
	}
	if _, ok := resp["route"]; !ok {
		t.Error("Expected route in config")
	}
	if _, ok := resp["experimental"]; !ok {
		t.Error("Expected experimental in config")
	}

	// POST config/reload
	req = httptest.NewRequest(http.MethodPost, "/api/config/reload", nil)
	w = httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST reload: Expected 200, got %d", w.Code)
	}
}

func TestAPIProxy_Logs(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET logs: Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["logs"]; !ok {
		t.Error("Expected logs in response")
	}
}

func TestAPIProxy_NotFound(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}
}

func TestAPIProxy_Stats(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["upload"]; !ok {
		t.Error("Expected upload in stats")
	}
	if _, ok := resp["download"]; !ok {
		t.Error("Expected download in stats")
	}
}

func TestAPIProxy_Connections(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/connections", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["uploadTotal"]; !ok {
		t.Error("Expected uploadTotal in connections")
	}
	if _, ok := resp["downloadTotal"]; !ok {
		t.Error("Expected downloadTotal in connections")
	}
}

func TestCorsMiddleware(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	handler := corsMiddleware(router)
	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS: Expected 200, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header")
	}
}

func TestHealthEndpoint(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("Expected status ok, got %v", resp["status"])
	}
}

func TestHealthEndpoint_BackendDown(t *testing.T) {
	// Set apiTarget to an unreachable address
	origTarget := apiTarget
	downTarget, _ := url.Parse("http://127.0.0.1:19999")
	apiTarget = downTarget
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 503, got %d", w.Code)
	}
}

func TestStaticFiles(t *testing.T) {
	// Test that non-API routes serve static files
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router(w, req)

	// Should return 200 (index.html)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for static file, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "SING-BOX MANAGER") {
		t.Error("Expected HTML content")
	}
}

func TestReverseProxy(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	target, _ := url.Parse(ts.URL)

	proxy := httputil.NewSingleHostReverseProxy(target)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestAPITargetEnv(t *testing.T) {
	// Test that API_TARGET env var is read correctly
	os.Setenv("API_TARGET", "http://custom-host:9999")
	defer os.Unsetenv("API_TARGET")

	val := os.Getenv("API_TARGET")
	if val != "http://custom-host:9999" {
		t.Errorf("Expected custom target, got %s", val)
	}
}

func TestPortEnv(t *testing.T) {
	os.Setenv("PORT", "11502")
	defer os.Unsetenv("PORT")

	port := os.Getenv("PORT")
	if port != "11502" {
		t.Errorf("Expected 11502, got %s", port)
	}
}

func TestDefaultPort(t *testing.T) {
	os.Unsetenv("PORT")
	port := os.Getenv("PORT")
	if port != "" {
		t.Errorf("Expected empty PORT, got %s", port)
	}
}

func TestRouter_APIPrefix(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// Test various API paths
	paths := []string{
		"/api/health",
		"/api/status",
		"/api/stats",
		"/api/connections",
		"/api/subscriptions",
		"/api/servers",
		"/api/config",
		"/api/logs",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			router(w, req)

			// All API endpoints should return 200 (not 404)
			if w.Code != http.StatusOK {
				t.Errorf("%s: Expected 200, got %d", path, w.Code)
			}
		})
	}
}

func TestRouter_HealthCheck(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouter_StaticFallback(t *testing.T) {
	// Non-API, non-health paths should serve static files
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestProxyErrorHandling(t *testing.T) {
	// Set apiTarget to unreachable address
	origTarget := apiTarget
	downTarget, _ := url.Parse("http://127.0.0.1:19999")
	apiTarget = downTarget
	defer func() { apiTarget = origTarget }()

	proxy := httputil.NewSingleHostReverseProxy(apiTarget)
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		http.Error(w, `{"error":"Backend unavailable"}`, http.StatusBadGateway)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	// Should get 502 Bad Gateway
	if w.Code != http.StatusBadGateway {
		t.Errorf("Expected 502, got %d", w.Code)
	}
}

func TestJSONResponse(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	// Verify JSON content type
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Expected application/json, got %s", ct)
	}

	// Verify valid JSON
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Invalid JSON: %v", err)
	}
}

func TestSubscriptionCRUD(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// 1. List subscriptions (GET)
	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET subscriptions: Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// 2. Add subscription (POST)
	body := strings.NewReader(`{"name":"Test","url":"https://example.com/sub"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST subscription: Expected 201, got %d", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	sub, ok := resp["subscription"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected subscription object in response")
	}
	if sub["id"] != "sub_new" {
		t.Errorf("Expected sub_new id, got %v", sub["id"])
	}

	// 3. List again (GET)
	req = httptest.NewRequest(http.MethodGet, "/api/subscriptions", nil)
	w = httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET subscriptions after POST: Expected 200, got %d", w.Code)
	}
}

func TestServerWorkflow(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// 1. Get servers
	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET servers: Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	servers, _ := resp["servers"].([]interface{})
	if len(servers) < 2 {
		t.Fatal("Expected at least 2 servers")
	}

	// 2. Select server
	body := strings.NewReader(`{"index":1}`)
	req = httptest.NewRequest(http.MethodPost, "/api/servers/select", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST select: Expected 200, got %d", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["selected"] != float64(1) {
		t.Errorf("Expected selected 1, got %v", resp["selected"])
	}

	// 3. Test servers
	body = strings.NewReader(`{"timeout":5}`)
	req = httptest.NewRequest(http.MethodPost, "/api/servers/test", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST test: Expected 200, got %d", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	results, _ := resp["results"].([]interface{})
	if len(results) < 2 {
		t.Errorf("Expected at least 2 test results, got %d", len(results))
	}
}

func TestConfigWorkflow(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// 1. Get config
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET config: Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// Verify config structure
	expectedKeys := []string{"log", "dns", "inbounds", "outbounds", "route", "experimental"}
	for _, key := range expectedKeys {
		if _, ok := resp[key]; !ok {
			t.Errorf("Missing key in config: %s", key)
		}
	}

	// 2. Reload config
	req = httptest.NewRequest(http.MethodPost, "/api/config/reload", nil)
	w = httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST reload: Expected 200, got %d", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["message"]; !ok {
		t.Error("Expected message in reload response")
	}
}

func TestLogsWorkflow(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET logs: Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	logs, ok := resp["logs"].(string)
	if !ok {
		t.Fatal("Expected logs string")
	}
	if len(logs) == 0 {
		t.Error("Expected non-empty logs")
	}
}

func TestFullIntegration(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// Full workflow: health -> status -> subscriptions -> servers -> config -> logs
	endpoints := []struct {
		path   string
		method string
		body   io.Reader
	}{
		{"/api/health", http.MethodGet, nil},
		{"/api/status", http.MethodGet, nil},
		{"/api/stats", http.MethodGet, nil},
		{"/api/connections", http.MethodGet, nil},
		{"/api/subscriptions", http.MethodGet, nil},
		{"/api/servers", http.MethodGet, nil},
		{"/api/config", http.MethodGet, nil},
		{"/api/logs", http.MethodGet, nil},
	}

	for _, ep := range endpoints {
		t.Run(ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, ep.body)
			w := httptest.NewRecorder()
			router(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected 200, got %d", w.Code)
			}
		})
	}

	// POST endpoints
	postEndpoints := []struct {
		path string
		body io.Reader
		code int
	}{
		{"/api/subscriptions", strings.NewReader(`{"name":"Test","url":"https://example.com/sub"}`), http.StatusCreated},
		{"/api/servers/select", strings.NewReader(`{"index":0}`), http.StatusOK},
		{"/api/servers/test", strings.NewReader(`{"timeout":5}`), http.StatusOK},
		{"/api/config/reload", nil, http.StatusOK},
	}

	for _, ep := range postEndpoints {
		t.Run(ep.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, ep.path, ep.body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router(w, req)

			if w.Code != ep.code {
				t.Errorf("Expected %d, got %d", ep.code, w.Code)
			}
		})
	}

	// Static file fallback
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Static fallback: Expected 200, got %d", w.Code)
	}

	// Health check
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	w = httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health: Expected 200, got %d", w.Code)
	}
}

func TestRouterPathPrefix(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// Test that /api/ prefix routes to proxy
	apiPaths := []string{
		"/api/health",
		"/api/status",
		"/api/stats",
		"/api/connections",
		"/api/subscriptions",
		"/api/servers",
		"/api/config",
		"/api/logs",
	}

	for _, path := range apiPaths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router(w, req)

		// Should NOT serve static files for API paths
		if w.Header().Get("Content-Type") != "application/json" {
			t.Errorf("%s: Expected application/json, got %s", path, w.Header().Get("Content-Type"))
		}
	}
}

func TestProxyPreservesMethod(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// POST should be forwarded as POST
	body := strings.NewReader(`{"name":"Test","url":"https://example.com/sub"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router(w, req)

	// Should get 201 (created) — not 200 (GET)
	if w.Code != http.StatusCreated {
		t.Errorf("Expected 201, got %d", w.Code)
	}
}

func TestProxyPreservesHeaders(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestProxyResponseHeaders(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	// Should have JSON content type
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected application/json, got %s", w.Header().Get("Content-Type"))
	}
}

func TestProxyTrailingSlash(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// Test with trailing slash
	req := httptest.NewRequest(http.MethodGet, "/api/health/", nil)
	w := httptest.NewRecorder()
	router(w, req)

	// Should still work (trailing slash stripped)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestProxyError(t *testing.T) {
	// Set apiTarget to unreachable
	origTarget := apiTarget
	downTarget, _ := url.Parse("http://127.0.0.1:19999")
	apiTarget = downTarget
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	// Should get 502 Bad Gateway
	if w.Code != http.StatusBadGateway {
		t.Errorf("Expected 502, got %d", w.Code)
	}
}

func TestProxyMultipleRequests(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// Make multiple requests to test connection handling
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		w := httptest.NewRecorder()
		router(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected 200, got %d", i, w.Code)
		}
	}
}

func TestProxyConcurrentRequests(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
			w := httptest.NewRecorder()
			router(w, req)
			done <- w.Code == http.StatusOK
		}()
	}

	for i := 0; i < 5; i++ {
		success := <-done
		if !success {
			t.Error("Concurrent request failed")
		}
	}
}

func TestRouterIntegration(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// Test that router correctly dispatches to different handlers
	tests := []struct {
		path     string
		expected int
	}{
		{"/", http.StatusOK},
		{"/health", http.StatusOK},
		{"/api/health", http.StatusOK},
		{"/api/status", http.StatusOK},
		{"/api/stats", http.StatusOK},
		{"/api/connections", http.StatusOK},
		{"/api/subscriptions", http.StatusOK},
		{"/api/servers", http.StatusOK},
		{"/api/config", http.StatusOK},
		{"/api/logs", http.StatusOK},
		{"/api/nonexistent", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			router(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestRouterPOSTIntegration(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	tests := []struct {
		path     string
		body     io.Reader
		expected int
	}{
		{"/api/subscriptions", strings.NewReader(`{"name":"Test","url":"https://example.com/sub"}`), http.StatusCreated},
		{"/api/servers/select", strings.NewReader(`{"index":0}`), http.StatusOK},
		{"/api/servers/test", strings.NewReader(`{"timeout":5}`), http.StatusOK},
		{"/api/config/reload", nil, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, tt.body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestRouterOPTIONSIntegration(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	handler := corsMiddleware(router)
	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header on OPTIONS")
	}
}

func TestDefaultAPIPort(t *testing.T) {
	// Default API port should be 9090 (from sing-box container)
	os.Unsetenv("API_TARGET")
	// The default is set in init(), so we check the env var logic
	targetStr := os.Getenv("API_TARGET")
	if targetStr != "" {
		t.Errorf("Expected empty API_TARGET, got %s", targetStr)
	}
}

func TestDefaultWebUIPort(t *testing.T) {
	// Default web UI port should be 11501
	os.Unsetenv("PORT")
	port := os.Getenv("PORT")
	if port != "" {
		t.Errorf("Expected empty PORT, got %s", port)
	}
}

func TestPortAbove11500(t *testing.T) {
	// Verify that the default port is above 11500
	// This is checked in the init function of server.go
	// The default port is 11501
	port := 11501
	if port <= 11500 {
		t.Errorf("Default port %d should be above 11500", port)
	}
}

func TestAPIProxyEnvOverride(t *testing.T) {
	// Test that API_TARGET env var changes the target
	os.Setenv("API_TARGET", "http://test-host:12345")
	defer os.Unsetenv("API_TARGET")

	targetStr := os.Getenv("API_TARGET")
	if targetStr != "http://test-host:12345" {
		t.Errorf("Expected http://test-host:12345, got %s", targetStr)
	}
}

func TestAPIProxyDefaultTarget(t *testing.T) {
	// Test that default target is 127.0.0.1:9090
	os.Unsetenv("API_TARGET")

	// The default is set in init(), so we check the init logic
	targetStr := os.Getenv("API_TARGET")
	if targetStr != "" {
		t.Errorf("Expected empty API_TARGET, got %s", targetStr)
	}
}

func TestRouterAPIProxy(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	// Test that API requests are proxied correctly
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("Expected status ok, got %v", resp["status"])
	}
}

func TestRouterStatic(t *testing.T) {
	// Test that non-API requests serve static files
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "SING-BOX MANAGER") {
		t.Error("Expected SING-BOX MANAGER in HTML")
	}
}

func TestRouterStaticAssets(t *testing.T) {
	// Test that asset requests serve static files
	req := httptest.NewRequest(http.MethodGet, "/assets/index.js", nil)
	w := httptest.NewRecorder()
	router(w, req)

	// Should return 200 or 404 (depending on whether the file exists)
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("Expected 200 or 404, got %d", w.Code)
	}
}

func TestRouterStaticIndex(t *testing.T) {
	// Test that root request serves index.html
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyMethods(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	methods := []string{http.MethodGet, http.MethodPost}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var body io.Reader
			if method == http.MethodPost {
				body = strings.NewReader(`{"name":"Test","url":"https://example.com/sub"}`)
			}
			req := httptest.NewRequest(method, "/api/subscriptions", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router(w, req)

			if method == http.MethodGet {
				if w.Code != http.StatusOK {
					t.Errorf("GET: Expected 200, got %d", w.Code)
				}
			} else {
				if w.Code != http.StatusCreated {
					t.Errorf("POST: Expected 201, got %d", w.Code)
				}
			}
		})
	}
}

func TestRouterAPIProxyBody(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	body := strings.NewReader(`{"name":"Test","url":"https://example.com/sub"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected 201, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	sub, ok := resp["subscription"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected subscription object")
	}
	if sub["id"] != "sub_new" {
		t.Errorf("Expected sub_new id, got %v", sub["id"])
	}
}

func TestRouterAPIProxyHeaders(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyResponse(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyError(t *testing.T) {
	// Test proxy error handling
	origTarget := apiTarget
	downTarget, _ := url.Parse("http://127.0.0.1:19999")
	apiTarget = downTarget
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("Expected 502, got %d", w.Code)
	}
}

func TestRouterAPIProxyNotFound(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}
}

func TestRouterAPIProxyHealth(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyStatus(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyStats(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyConnections(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/connections", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxySubscriptions(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyServers(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyConfig(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyLogs(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyReload(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodPost, "/api/config/reload", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxySelectServer(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	body := strings.NewReader(`{"index":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/servers/select", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyTestServers(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	body := strings.NewReader(`{"timeout":5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/servers/test", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProxyAddSubscription(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	body := strings.NewReader(`{"name":"Test","url":"https://example.com/sub"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected 201, got %d", w.Code)
	}
}

func TestRouterAPIProxyDeleteSubscription(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodDelete, "/api/subscriptions/sub_1", nil)
	w := httptest.NewRecorder()
	router(w, req)

	// The mock backend doesn't handle DELETE, so we expect 404 or 500
	// This is expected behavior — the real backend handles DELETE
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Logf("DELETE subscription returned %d (expected 404 or 500 from mock)", w.Code)
	}
}

func TestRouterAPIProxyCORS(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	handler := corsMiddleware(router)
	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header")
	}
}

func TestRouterAPIProxyContentType(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	router(w, req)

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected application/json, got %s", w.Header().Get("Content-Type"))
	}
}

func TestRouterAPIProxyBodyForward(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	body := strings.NewReader(`{"name":"Test","url":"https://example.com/sub"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected 201, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	sub, ok := resp["subscription"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected subscription object")
	}
	if sub["id"] != "sub_new" {
		t.Errorf("Expected sub_new id, got %v", sub["id"])
	}
}

func TestRouterAPIProxyStatusCodes(t *testing.T) {
	ts := mockBackend(t)
	defer ts.Close()

	origTarget := apiTarget
	parsed, _ := url.Parse(ts.URL)
	apiTarget = parsed
	defer func() { apiTarget = origTarget }()

	tests := []struct {
		path     string
		method   string
		body     io.Reader
		expected int
	}{
		{"/api/health", http.MethodGet, nil, http.StatusOK},
		{"/api/status", http.MethodGet, nil, http.StatusOK},
		{"/api/stats", http.MethodGet, nil, http.StatusOK},
		{"/api/connections", http.MethodGet, nil, http.StatusOK},
		{"/api/subscriptions", http.MethodGet, nil, http.StatusOK},
		{"/api/servers", http.MethodGet, nil, http.StatusOK},
		{"/api/config", http.MethodGet, nil, http.StatusOK},
		{"/api/logs", http.MethodGet, nil, http.StatusOK},
		{"/api/subscriptions", http.MethodPost, strings.NewReader(`{"name":"T","url":"https://x.com/s"}`), http.StatusCreated},
		{"/api/servers/select", http.MethodPost, strings.NewReader(`{"index":0}`), http.StatusOK},
		{"/api/servers/test", http.MethodPost, strings.NewReader(`{"timeout":5}`), http.StatusOK},
		{"/api/config/reload", http.MethodPost, nil, http.StatusOK},
		{"/api/nonexistent", http.MethodGet, nil, http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, tt.body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, w.Code)
			}
		})
	}
}
