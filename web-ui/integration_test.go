package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	webUIBase = "http://localhost:11501"
	apiBase   = "http://localhost:9090"
)

var testTimeout = 10 * time.Second

func httpGet(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, body
}

func httpPost(t *testing.T, url string, data interface{}) (*http.Response, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	jsonData, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(jsonData)))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, body
}

func httpDelete(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, body
}

// ============================================
// STATIC FILES
// ============================================

func TestIntegration_StaticRootPage(t *testing.T) {
	resp, body := httpGet(t, webUIBase+"/")
	if resp.StatusCode != 200 {
		t.Errorf("Root page: expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "SING-BOX") {
		t.Error("Root page missing SING-BOX")
	}
}

func TestIntegration_StaticCSSExists(t *testing.T) {
	resp, body := httpGet(t, webUIBase+"/")
	if resp.StatusCode != 200 {
		t.Fatal("Root page not accessible")
	}
	if !strings.Contains(string(body), ".css") {
		t.Error("Root page missing CSS reference")
	}
	_ = body
}

// ============================================
// HEALTH API
// ============================================

func TestIntegration_HealthEndpoint(t *testing.T) {
	resp, body := httpGet(t, webUIBase+"/api/health")
	if resp.StatusCode != 200 {
		t.Errorf("Health: expected 200, got %d", resp.StatusCode)
	}
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if data["status"] != "ok" {
		t.Errorf("Health status: expected 'ok', got %v", data["status"])
	}
	if _, ok := data["timestamp"]; !ok {
		t.Error("Health missing timestamp")
	}
}

func TestIntegration_HealthDirectAPI(t *testing.T) {
	resp, body := httpGet(t, apiBase+"/api/health")
	if resp.StatusCode != 200 {
		t.Errorf("Direct health: expected 200, got %d", resp.StatusCode)
	}
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if data["status"] != "ok" {
		t.Errorf("Direct health status: expected 'ok', got %v", data["status"])
	}
}

// ============================================
// STATUS API
// ============================================

func TestIntegration_StatusEndpoint(t *testing.T) {
	resp, body := httpGet(t, webUIBase+"/api/status")
	if resp.StatusCode != 200 {
		t.Errorf("Status: expected 200, got %d", resp.StatusCode)
	}
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if _, ok := data["api_port"]; !ok {
		t.Error("Status missing api_port")
	}
	if _, ok := data["running"]; !ok {
		t.Error("Status missing running field")
	}
}

// ============================================
// SUBSCRIPTIONS API
// ============================================

func TestIntegration_GetSubscriptions(t *testing.T) {
	resp, body := httpGet(t, webUIBase+"/api/subscriptions")
	if resp.StatusCode != 200 {
		t.Errorf("Get subscriptions: expected 200, got %d", resp.StatusCode)
	}
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if _, ok := data["subscriptions"]; !ok {
		t.Error("Subscriptions missing 'subscriptions' field")
	}
}

func TestIntegration_AddSubscription(t *testing.T) {
	payload := map[string]interface{}{
		"name":    "Test Sub",
		"url":     "manual",
		"servers": []string{"vless://test@test.example.com:443#Test Server"},
	}
	resp, body := httpPost(t, webUIBase+"/api/subscriptions", payload)
	if resp.StatusCode != 201 {
		t.Fatalf("Add subscription: expected 201, got %d, body: %s", resp.StatusCode, string(body))
	}
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	sub := data["subscription"].(map[string]interface{})
	if sub["name"] != "Test Sub" {
		t.Errorf("Subscription name: expected 'Test Sub', got %v", sub["name"])
	}
	// Clean up
	httpDelete(t, fmt.Sprintf("%s/api/subscriptions/%s", webUIBase, sub["id"]))
}

func TestIntegration_ActivateSubscription(t *testing.T) {
	// Add subscription
	payload := map[string]interface{}{
		"name":    "Activate Test",
		"url":     "manual",
		"servers": []string{"vless://test@test.example.com:443#Activate Test Server"},
	}
	resp, body := httpPost(t, webUIBase+"/api/subscriptions", payload)
	if resp.StatusCode != 201 {
		t.Fatalf("Failed to add subscription: %s", string(body))
	}
	var addData map[string]interface{}
	json.Unmarshal(body, &addData)
	sub := addData["subscription"].(map[string]interface{})
	subID := sub["id"].(string)

	// Activate
	resp, body = httpPost(t, fmt.Sprintf("%s/api/subscriptions/%s/activate", webUIBase, subID), nil)
	if resp.StatusCode != 200 {
		t.Errorf("Activate: expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if data["active_subscription"] != subID {
		t.Errorf("Active subscription: expected %s, got %v", subID, data["active_subscription"])
	}

	// Clean up
	httpDelete(t, fmt.Sprintf("%s/api/subscriptions/%s", webUIBase, subID))
}

func TestIntegration_DeleteSubscription(t *testing.T) {
	// Add first
	payload := map[string]interface{}{
		"name":    "Delete Test",
		"url":     "manual",
		"servers": []string{"vless://test@test.example.com:443#Delete Test Server"},
	}
	resp, body := httpPost(t, webUIBase+"/api/subscriptions", payload)
	if resp.StatusCode != 201 {
		t.Fatalf("Failed to add subscription: %s", string(body))
	}
	var addData map[string]interface{}
	json.Unmarshal(body, &addData)
	sub := addData["subscription"].(map[string]interface{})
	subID := sub["id"].(string)

	// Delete
	resp, body = httpDelete(t, fmt.Sprintf("%s/api/subscriptions/%s", webUIBase, subID))
	if resp.StatusCode != 200 {
		t.Errorf("Delete: expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
}

// ============================================
// SERVERS API
// ============================================

func TestIntegration_GetServers(t *testing.T) {
	resp, body := httpGet(t, webUIBase+"/api/servers")
	// Should return 200 (with servers) or 400 (no active subscription)
	if resp.StatusCode != 200 && resp.StatusCode != 400 {
		t.Errorf("Get servers: expected 200 or 400, got %d", resp.StatusCode)
	}
	_ = body
}

// ============================================
// CONFIG API
// ============================================

func TestIntegration_GetConfig(t *testing.T) {
	resp, body := httpGet(t, webUIBase+"/api/config")
	if resp.StatusCode != 200 {
		t.Errorf("Config: expected 200, got %d", resp.StatusCode)
	}
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if _, ok := data["dns"]; !ok {
		t.Error("Config missing dns field")
	}
}

// ============================================
// LOGS API
// ============================================

func TestIntegration_GetLogs(t *testing.T) {
	resp, body := httpGet(t, webUIBase+"/api/logs")
	if resp.StatusCode != 200 {
		t.Errorf("Logs: expected 200, got %d", resp.StatusCode)
	}
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if _, ok := data["logs"]; !ok {
		t.Error("Logs missing 'logs' field")
	}
}

// ============================================
// CORS
// ============================================

func TestIntegration_CORSPresent(t *testing.T) {
	resp, _ := httpGet(t, webUIBase+"/api/health")
	cors := resp.Header.Get("Access-Control-Allow-Origin")
	if cors != "*" {
		t.Errorf("CORS header: expected '*', got '%s'", cors)
	}
}

func TestIntegration_OPTIONSPreflight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodOptions, webUIBase+"/api/health", nil)
	req.Header.Set("Origin", "http://example.com")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		t.Errorf("OPTIONS: expected 200, got %d", resp.StatusCode)
	}
	methods := resp.Header.Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("OPTIONS missing Allow-Methods header")
	}
}

// ============================================
// API PROXY
// ============================================

func TestIntegration_APIProxyForwards(t *testing.T) {
	directResp, directBody := httpGet(t, apiBase+"/api/health")
	proxyResp, proxyBody := httpGet(t, webUIBase+"/api/health")

	if directResp.StatusCode != proxyResp.StatusCode {
		t.Errorf("Proxy status mismatch: direct=%d, proxy=%d", directResp.StatusCode, proxyResp.StatusCode)
	}
	if string(directBody) != string(proxyBody) {
		t.Error("Proxy body mismatch between direct and proxied response")
	}
}
