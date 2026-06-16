package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Config holds API configuration.
type Config struct {
	APIPort       int
	APIHost       string
	SingboxAddr   string
	SingboxToken  string
	AuthToken     string
	SubDir        string
	ConfigFile    string
	LogFile       string
	PIDFile       string
}

// Subscription represents a subscription profile.
type Subscription struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	Servers    []string `json:"servers"`
	Created    string   `json:"created"`
	Updated    string   `json:"updated"`
	ServerCount int     `json:"server_count"`
}

// State holds the application state.
type State struct {
	mu                sync.RWMutex
	subscriptions     []Subscription
	activeSubscription string
	selectedServer    int
	serverStats       map[string]interface{}
	startTime         time.Time
}

var (
	cfg   Config
	state State
)

func init() {
	cfg.APIPort = getEnvInt("API_PORT", 9090)
	cfg.APIHost = getEnvString("API_HOST", "0.0.0.0")
	cfg.SingboxAddr = getEnvString("SINGBOX_API_ADDR", "127.0.0.1:20123")
	cfg.SingboxToken = getEnvString("SINGBOX_API_TOKEN", "")
	cfg.AuthToken = getEnvString("API_AUTH_TOKEN", "")
	cfg.SubDir = getEnvString("SUB_DIR", "/etc/sing-box/subscriptions")
	cfg.ConfigFile = getEnvString("CONFIG_FILE", "/sing-box.json")
	cfg.LogFile = getEnvString("LOG_FILE", "/tmp/sing-box.log")
	cfg.PIDFile = getEnvString("PID_FILE", "/tmp/.singbox_pid")

	state.startTime = time.Now()
	state.serverStats = make(map[string]interface{})
	loadSubscriptions()
}

func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var v int
		fmt.Sscanf(val, "%d", &v)
		if v > 0 {
			return v
		}
	}
	return defaultVal
}

func loadSubscriptions() {
	data, err := os.ReadFile(cfg.SubDir + "/subscriptions.json")
	if err != nil {
		state.subscriptions = []Subscription{}
		return
	}
	json.Unmarshal(data, &state.subscriptions)
}

func saveSubscriptions() {
	os.MkdirAll(cfg.SubDir, 0755)
	data, _ := json.MarshalIndent(state.subscriptions, "", "  ")
	os.WriteFile(cfg.SubDir+"/subscriptions.json", data, 0644)
}

func getSingboxStatus() map[string]interface{} {
	result := map[string]interface{}{"running": false, "pid": nil, "uptime": 0}
	data, err := os.ReadFile(cfg.PIDFile)
	if err != nil {
		return result
	}
	var pid int
	fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
	if pid > 0 {
		process, err := os.FindProcess(pid)
		if err == nil {
			err = process.Signal(syscall.Signal(0))
			if err == nil {
				result["running"] = true
				result["pid"] = pid
				result["uptime"] = time.Since(state.startTime).Seconds()
			}
		}
	}
	return result
}

func getTrafficStats() map[string]interface{} {
	result := make(map[string]interface{})
	cmd := exec.Command("wget", "-qO-", "--timeout=5", fmt.Sprintf("http://%s/traffic", cfg.SingboxAddr))
	output, err := cmd.Output()
	if err != nil {
		result["error"] = err.Error()
		return result
	}
	var data map[string]interface{}
	if err := json.Unmarshal(output, &data); err != nil {
		result["error"] = err.Error()
		return result
	}
	result["upload"] = data["up"]
	result["download"] = data["down"]
	return result
}

func getConnections() map[string]interface{} {
	result := make(map[string]interface{})
	cmd := exec.Command("wget", "-qO-", "--timeout=5", fmt.Sprintf("http://%s/connections", cfg.SingboxAddr))
	output, err := cmd.Output()
	if err != nil {
		result["error"] = err.Error()
		return result
	}
	json.Unmarshal(output, &result)
	return result
}

func fetchSubscription(url string) (string, error) {
	cmd := exec.Command("wget", "-qO-", "--timeout=15", url)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	content := string(output)
	// Try base64 decode
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err == nil {
		decodedStr := string(decoded)
		if strings.Contains(decodedStr, "://") {
			content = decodedStr
		}
	}
	return content, nil
}

func parseServers(content string) []string {
	re := regexp.MustCompile(`(?:hysteria2|vless|vmess|trojan|ss)://[^\s"<,]+`)
	servers := re.FindAllString(content, -1)
	if servers == nil {
		return []string{}
	}
	return servers
}

func testServerLatency(serverURL string, timeout int) *int {
	re := regexp.MustCompile(`@([^:]+)`)
	matches := re.FindStringSubmatch(serverURL)
	if len(matches) < 2 {
		return nil
	}
	server := matches[1]
	start := time.Now()
	cmd := exec.Command("wget", "-qO-", "--timeout", fmt.Sprintf("%d", timeout), "--tries=1", fmt.Sprintf("https://%s", server))
	err := cmd.Run()
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return nil
	}
	return &latency
}

func reloadSingbox() bool {
	data, err := os.ReadFile(cfg.PIDFile)
	if err != nil {
		return false
	}
	var pid int
	fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
	if pid > 0 {
		process, _ := os.FindProcess(pid)
		if process != nil {
			return process.Signal(syscall.SIGHUP) == nil
		}
	}
	return false
}

func checkAuth(r *http.Request) bool {
	if cfg.AuthToken == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		return token == cfg.AuthToken
	}
	return false
}

func sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func readBody(r *http.Request) map[string]interface{} {
	body, _ := io.ReadAll(r.Body)
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	return data
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, 200, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	status := getSingboxStatus()
	status["api_port"] = cfg.APIPort
	status["subscriptions_count"] = len(state.subscriptions)
	status["active_subscription"] = state.activeSubscription
	status["selected_server"] = state.selectedServer
	sendJSON(w, 200, status)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, 200, getTrafficStats())
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, 200, getConnections())
}

func handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		state.mu.RLock()
		defer state.mu.RUnlock()
		sendJSON(w, 200, map[string]interface{}{
			"subscriptions": state.subscriptions,
			"active":        state.activeSubscription,
		})
		return
	}
	if r.Method == http.MethodPost {
		if !checkAuth(r) {
			sendJSON(w, 401, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		body := readBody(r)
		name, _ := body["name"].(string)
		url, _ := body["url"].(string)
		if name == "" {
			name = "Subscription"
		}
		if url == "" {
			sendJSON(w, 400, map[string]interface{}{"error": "URL required"})
			return
		}
		content, err := fetchSubscription(url)
		if err != nil {
			sendJSON(w, 500, map[string]interface{}{"error": "Failed to fetch subscription"})
			return
		}
		servers := parseServers(content)
		sub := Subscription{
			ID:          fmt.Sprintf("sub_%d", time.Now().Unix()),
			Name:        name,
			URL:         url,
			Servers:     servers,
			Created:     time.Now().Format(time.RFC3339),
			Updated:     time.Now().Format(time.RFC3339),
			ServerCount: len(servers),
		}
		state.mu.Lock()
		state.subscriptions = append(state.subscriptions, sub)
		saveSubscriptions()
		state.mu.Unlock()
		sendJSON(w, 201, map[string]interface{}{
			"subscription": sub,
			"message":      "Subscription added",
		})
		return
	}
	sendJSON(w, 404, map[string]interface{}{"error": "Not found"})
}

func handleDeleteSubscription(w http.ResponseWriter, r *http.Request, subID string) {
	if !checkAuth(r) {
		sendJSON(w, 401, map[string]interface{}{"error": "Unauthorized"})
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.activeSubscription == subID {
		state.activeSubscription = ""
	}
	newSubs := []Subscription{}
	for _, sub := range state.subscriptions {
		if sub.ID != subID {
			newSubs = append(newSubs, sub)
		}
	}
	state.subscriptions = newSubs
	saveSubscriptions()
	sendJSON(w, 200, map[string]interface{}{"message": fmt.Sprintf("Subscription %s removed", subID)})
}

func handleServers(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		state.mu.RLock()
		defer state.mu.RUnlock()
		if state.activeSubscription == "" {
			sendJSON(w, 400, map[string]interface{}{"error": "No active subscription"})
			return
		}
		var sub *Subscription
		for i := range state.subscriptions {
			if state.subscriptions[i].ID == state.activeSubscription {
				sub = &state.subscriptions[i]
				break
			}
		}
		if sub == nil {
			sendJSON(w, 404, map[string]interface{}{"error": "Subscription not found"})
			return
		}
		sendJSON(w, 200, map[string]interface{}{
			"subscription": sub.Name,
			"servers":      sub.Servers,
			"selected":     state.selectedServer,
			"stats":        state.serverStats,
		})
		return
	}
	if r.Method == http.MethodPost {
		if !checkAuth(r) {
			sendJSON(w, 401, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		path := r.URL.Path
		body := readBody(r)
		if strings.Contains(path, "/select") {
			handleServerSelect(w, body)
		} else if strings.Contains(path, "/test") {
			handleServerTest(w, body)
		} else {
			sendJSON(w, 404, map[string]interface{}{"error": "Not found"})
		}
		return
	}
	sendJSON(w, 404, map[string]interface{}{"error": "Not found"})
}

func handleServerSelect(w http.ResponseWriter, body map[string]interface{}) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.activeSubscription == "" {
		sendJSON(w, 400, map[string]interface{}{"error": "No active subscription"})
		return
	}
	var sub *Subscription
	for i := range state.subscriptions {
		if state.subscriptions[i].ID == state.activeSubscription {
			sub = &state.subscriptions[i]
			break
		}
	}
	if sub == nil || len(sub.Servers) == 0 {
		sendJSON(w, 400, map[string]interface{}{"error": "No servers available"})
		return
	}
	index := int(body["index"].(float64))
	if index >= len(sub.Servers) {
		sendJSON(w, 400, map[string]interface{}{"error": "Invalid server index"})
		return
	}
	state.selectedServer = index
	sendJSON(w, 200, map[string]interface{}{
		"selected": index,
		"server":   sub.Servers[index],
		"message":  "Server selected",
	})
}

func handleServerTest(w http.ResponseWriter, body map[string]interface{}) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.activeSubscription == "" {
		sendJSON(w, 400, map[string]interface{}{"error": "No active subscription"})
		return
	}
	var sub *Subscription
	for i := range state.subscriptions {
		if state.subscriptions[i].ID == state.activeSubscription {
			sub = &state.subscriptions[i]
			break
		}
	}
	if sub == nil || len(sub.Servers) == 0 {
		sendJSON(w, 400, map[string]interface{}{"error": "No servers available"})
		return
	}
	timeout := int(body["timeout"].(float64))
	if timeout == 0 {
		timeout = 5
	}
	results := []map[string]interface{}{}
	for i, server := range sub.Servers {
		latency := testServerLatency(server, timeout)
		result := map[string]interface{}{
			"index":  i,
			"server": server,
		}
		if latency != nil {
			result["latency"] = *latency
		} else {
			result["latency"] = nil
		}
		results = append(results, result)
		state.serverStats[fmt.Sprintf("server_%d", i)] = map[string]interface{}{
			"latency": latency,
			"tested":  time.Now().Format(time.RFC3339),
		}
	}
	// Find fastest
	var fastest map[string]interface{}
	minLatency := -1
	for _, r := range results {
		if lat, ok := r["latency"].(int); ok {
			if minLatency == -1 || lat < minLatency {
				minLatency = lat
				fastest = r
			}
		}
	}
	if fastest != nil {
		state.selectedServer = int(fastest["index"].(float64))
		sendJSON(w, 200, map[string]interface{}{
			"results":  results,
			"fastest":  fastest,
			"selected": state.selectedServer,
			"message":  fmt.Sprintf("Server %d selected (%dms)", state.selectedServer, minLatency),
		})
	} else {
		sendJSON(w, 200, map[string]interface{}{
			"results": results,
			"error":   "All servers unreachable",
			"message": "No server could be reached",
		})
	}
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		data, err := os.ReadFile(cfg.ConfigFile)
		if err != nil {
			sendJSON(w, 500, map[string]interface{}{"error": err.Error()})
			return
		}
		var config interface{}
		json.Unmarshal(data, &config)
		sendJSON(w, 200, config)
		return
	}
	if r.Method == http.MethodPost {
		if !checkAuth(r) {
			sendJSON(w, 401, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		if reloadSingbox() {
			sendJSON(w, 200, map[string]interface{}{"message": "sing-box reload signal sent"})
		} else {
			sendJSON(w, 500, map[string]interface{}{"error": "Failed to reload sing-box"})
		}
		return
	}
	sendJSON(w, 404, map[string]interface{}{"error": "Not found"})
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(cfg.LogFile)
	if err != nil {
		sendJSON(w, 200, map[string]interface{}{"logs": "Log file not found"})
		return
	}
	sendJSON(w, 200, map[string]interface{}{"logs": string(data)})
}

func router(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")

	switch path {
	case "/api/health":
		handleHealth(w, r)
	case "/api/status":
		handleStatus(w, r)
	case "/api/stats":
		handleStats(w, r)
	case "/api/connections":
		handleConnections(w, r)
	case "/api/subscriptions":
		handleSubscriptions(w, r)
	case "/api/servers", "/api/servers/select", "/api/servers/test":
		handleServers(w, r)
	case "/api/config", "/api/config/reload":
		handleConfig(w, r)
	case "/api/logs":
		handleLogs(w, r)
	default:
		// Check for subscription delete
		if strings.HasPrefix(path, "/api/subscriptions/") {
			subID := strings.TrimPrefix(path, "/api/subscriptions/")
			if r.Method == http.MethodDelete {
				handleDeleteSubscription(w, r, subID)
				return
			}
		}
		sendJSON(w, 404, map[string]interface{}{"error": "Not found"})
	}
}

func main() {
	addr := fmt.Sprintf("%s:%d", cfg.APIHost, cfg.APIPort)
	log.Printf("Management API started on %s", addr)
	if cfg.AuthToken != "" {
		log.Printf("Auth enabled: use Bearer token")
	}
	log.Printf("Endpoints:")
	log.Printf("  GET  /api/health")
	log.Printf("  GET  /api/status")
	log.Printf("  GET  /api/stats")
	log.Printf("  GET  /api/connections")
	log.Printf("  GET  /api/subscriptions")
	log.Printf("  POST /api/subscriptions")
	log.Printf("  DEL  /api/subscriptions/:id")
	log.Printf("  GET  /api/servers")
	log.Printf("  POST /api/servers/select")
	log.Printf("  POST /api/servers/test")
	log.Printf("  GET  /api/config")
	log.Printf("  POST /api/config/reload")
	log.Printf("  GET  /api/logs")

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	server := &http.Server{Handler: http.HandlerFunc(router)}
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
