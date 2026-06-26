package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
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

// ServerInfo holds a server URL with its remark (name)
type ServerInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Subscription represents a subscription profile.
type Subscription struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	URL         string      `json:"url"`
	Servers     []ServerInfo `json:"servers"`
	Created     string      `json:"created"`
	Updated     string      `json:"updated"`
	ServerCount int         `json:"server_count"`
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
	// clash_api /traffic returns streaming data, use head -1 to get first line only
	cmd := exec.Command("sh", "-c", fmt.Sprintf("wget -qO- --timeout=1 --tries=1 http://%s/traffic 2>/dev/null | head -1", cfg.SingboxAddr))
	output, err := cmd.Output()
	if err != nil {
		result["error"] = err.Error()
		return result
	}
	line := strings.TrimSpace(string(output))
	if line == "" {
		result["error"] = "empty response"
		return result
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		result["error"] = err.Error()
		return result
	}
	result["upload"] = data["up"]
	result["download"] = data["down"]
	return result
}

func getConnections() map[string]interface{} {
	result := make(map[string]interface{})
	// clash_api /connections returns streaming data, use head -1 to get first line only
	cmd := exec.Command("sh", "-c", fmt.Sprintf("wget -qO- --timeout=1 --tries=1 http://%s/connections 2>/dev/null | head -1", cfg.SingboxAddr))
	output, err := cmd.Output()
	if err != nil {
		result["error"] = err.Error()
		return result
	}
	line := strings.TrimSpace(string(output))
	if line == "" {
		result["error"] = "empty response"
		return result
	}
	json.Unmarshal([]byte(line), &result)
	return result
}

func fetchSubscription(url string) (string, error) {
	// Try curl first (better TLS support)
	cmd := exec.Command("curl", "-s", "--max-time", "15", "-k", url)
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		content := string(output)
		// Try base64 decode
		decoded, decodeErr := base64.StdEncoding.DecodeString(content)
		if decodeErr == nil {
			decodedStr := string(decoded)
			if strings.Contains(decodedStr, "://") {
				content = decodedStr
			}
		}
		return content, nil
	}
	// Fallback to wget
	cmd = exec.Command("wget", "-qO-", "--timeout=15", "--no-check-certificate", url)
	output, err = cmd.Output()
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

func parseServerInfos(content string) []ServerInfo {
	re := regexp.MustCompile(`(?:hysteria2|vless|vmess|trojan|ss)://[^\s"<,]+`)
	rawURLs := re.FindAllString(content, -1)
	result := make([]ServerInfo, 0, len(rawURLs))
	for _, raw := range rawURLs {
		name := ""
		hashIdx := strings.Index(raw, "#")
		if hashIdx >= 0 {
			name = raw[hashIdx+1:]
			if decoded, err := url.PathUnescape(name); err == nil {
				name = decoded
			}
		}
		if name == "" {
			name = fmt.Sprintf("Server #%d", len(result))
		}
		result = append(result, ServerInfo{Name: name, URL: raw})
	}
	return result
}

// isDirectServerURL checks if the URL is a direct server link (not a subscription URL)
func isDirectServerURL(url string) bool {
	return strings.HasPrefix(url, "vless://") ||
		strings.HasPrefix(url, "vmess://") ||
		strings.HasPrefix(url, "trojan://") ||
		strings.HasPrefix(url, "hysteria2://") ||
		strings.HasPrefix(url, "ss://")
}

func testServerLatency(serverURL string, timeout int) *int {
	cfg, err := parseServerURL(serverURL)
	if err != nil {
		return nil
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// TCP dial
	dialer := net.Dialer{Timeout: time.Duration(timeout) * time.Second}
	addr := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		log.Printf("test latency %s: TCP dial failed: %v", addr, err)
		return nil
	}
	defer conn.Close()

	// TLS handshake if security=tls
	if cfg.Security == "tls" || cfg.Protocol == "trojan" || cfg.Protocol == "vless" {
		tlsConn := net.Conn(conn)
		// For TLS, we do a basic handshake
		tlsConfig := &tls.Config{
			ServerName:         cfg.SNI,
			InsecureSkipVerify: cfg.Insecure,
		}
		tlsConn = tls.Client(conn, tlsConfig)
		if err := tlsConn.(*tls.Conn).Handshake(); err != nil {
			log.Printf("test latency %s: TLS handshake failed: %v", addr, err)
			return nil
		}
		conn = tlsConn
	}

	latency := int(time.Since(start).Milliseconds())
	return &latency
}

// ProtocolConfig holds parsed server connection details
type ProtocolConfig struct {
	Protocol    string
	Server      string
	Port        int
	UUID        string
	Password    string
	SNI         string
	Transport   string
	Path        string
	Host        string
	Mode        string
	Encryption  string
	Security    string
	Flow        string
	PublicKey   string
	ShortID     string
	Insecure    bool
	Extra       string // xhttp extra JSON
	Fingerprint string // tls utls fingerprint
	ALPN        string // tls alpn
	Version     string // for ss
	Method      string // for ss
}

// parseServerURL detects protocol and parses the URL dynamically
func parseServerURL(rawURL string) (*ProtocolConfig, error) {
	var cfg ProtocolConfig

	// Detect protocol
	if strings.HasPrefix(rawURL, "vless://") {
		cfg.Protocol = "vless"
		rawURL = strings.TrimPrefix(rawURL, "vless://")
	} else if strings.HasPrefix(rawURL, "vmess://") {
		cfg.Protocol = "vmess"
		// vmess uses base64 encoded JSON
		rawURL = strings.TrimPrefix(rawURL, "vmess://")
		decoded, err := base64.StdEncoding.DecodeString(rawURL)
		if err != nil {
			return nil, fmt.Errorf("invalid vmess URL: %w", err)
		}
		var vmess map[string]interface{}
		if err := json.Unmarshal(decoded, &vmess); err != nil {
			return nil, fmt.Errorf("invalid vmess JSON: %w", err)
		}
		cfg.UUID = vmess["id"].(string)
		cfg.Server = vmess["add"].(string)
		cfg.Port = int(vmess["port"].(float64))
		cfg.Transport = vmess["net"].(string)
		cfg.Encryption = vmess["scy"].(string)
		if sni, ok := vmess["ps"].(string); ok {
			cfg.SNI = sni
		}
		return &cfg, nil
	} else if strings.HasPrefix(rawURL, "trojan://") {
		cfg.Protocol = "trojan"
		rawURL = strings.TrimPrefix(rawURL, "trojan://")
	} else if strings.HasPrefix(rawURL, "hysteria2://") {
		cfg.Protocol = "hysteria2"
		rawURL = strings.TrimPrefix(rawURL, "hysteria2://")
	} else if strings.HasPrefix(rawURL, "ss://") {
		cfg.Protocol = "shadowsocks"
		rawURL = strings.TrimPrefix(rawURL, "ss://")
	} else {
		return nil, fmt.Errorf("unsupported protocol")
	}

	// Extract credentials (before @)
	parts := strings.SplitN(rawURL, "@", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid URL format")
	}

	// Protocol-specific credential parsing
	switch cfg.Protocol {
	case "vless":
		cfg.UUID = parts[0]
	case "trojan", "hysteria2":
		cfg.Password = parts[0]
	case "shadowsocks":
		// ss://base64(method:password)@server:port
		decoded, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			cfg.Method = parts[0]
		} else {
			decodedStr := string(decoded)
			sepIdx := strings.Index(decodedStr, ":")
			if sepIdx >= 0 {
				cfg.Method = decodedStr[:sepIdx]
				cfg.Password = decodedStr[sepIdx+1:]
			} else {
				cfg.Method = decodedStr
			}
		}
	}

	// Extract server:port and query params
	rest := parts[1]
	hashIdx := strings.Index(rest, "#")
	if hashIdx >= 0 {
		rest = rest[:hashIdx]
	}

	// Split by ? to get server:port and params
	qIdx := strings.Index(rest, "?")
	serverPort := rest
	if qIdx >= 0 {
		serverPort = rest[:qIdx]
		params := rest[qIdx+1:]
		// Parse all params
		for _, param := range strings.Split(params, "&") {
			kv := strings.SplitN(param, "=", 2)
			if len(kv) == 2 {
				key := kv[0]
				val := kv[1]
				if decoded, err := url.PathUnescape(val); err == nil {
					val = decoded
				}
				switch key {
				case "sni", "peer", "obfsParam":
					cfg.SNI = val
				case "type", "obfs":
					cfg.Transport = val
				case "path", "obfsPath":
					cfg.Path = val
				case "host":
					cfg.Host = val
				case "mode":
					cfg.Mode = val
				case "security":
					cfg.Security = val
				case "flow":
					cfg.Flow = val
				case "pbk", "publicKey":
					cfg.PublicKey = val
				case "sid":
					cfg.ShortID = val
				case "insecure":
					cfg.Insecure = val == "1" || val == "true"
				case "encryption":
					cfg.Encryption = val
				case "extra":
					cfg.Extra = val
				case "fp":
					cfg.Fingerprint = val
				case "alpn":
					cfg.ALPN = val
				}
			}
		}
	}

	// Parse server:port
	hostPort := strings.Split(serverPort, ":")
	if len(hostPort) != 2 {
		return nil, fmt.Errorf("invalid server:port")
	}
	cfg.Server = hostPort[0]
	var err error
	cfg.Port, err = strconv.Atoi(hostPort[1])
	if err != nil {
		return nil, fmt.Errorf("invalid port")
	}

	// Default SNI to server if not set
	if cfg.SNI == "" {
		cfg.SNI = cfg.Server
	}

	return &cfg, nil
}

// parseVlessURL parses a vless:// URL and returns the server config
func parseVlessURL(rawURL string) (server string, port int, uuid string, sni string, transport string, path string, host string, mode string, err error) {
	cfg, err := parseServerURL(rawURL)
	if err != nil {
		return "", 0, "", "", "", "", "", "", err
	}
	return cfg.Server, cfg.Port, cfg.UUID, cfg.SNI, cfg.Transport, cfg.Path, cfg.Host, cfg.Mode, nil
}

// parseServerConfig returns full JSON config for a server URL
func parseServerConfig(rawURL string) map[string]interface{} {
	cfg, err := parseServerURL(rawURL)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
			"url":   rawURL,
		}
	}

	config := map[string]interface{}{
		"url":      rawURL,
		"protocol": cfg.Protocol,
		"server":   cfg.Server,
		"port":     cfg.Port,
		"sni":      cfg.SNI,
		"tls":      cfg.Security == "tls",
	}

	switch cfg.Protocol {
	case "vless":
		config["uuid"] = cfg.UUID
		config["encryption"] = cfg.Encryption
		config["security"] = cfg.Security
		config["flow"] = cfg.Flow
		config["transport"] = cfg.Transport
		if cfg.Transport == "xhttp" || cfg.Transport == "ws" {
			config["path"] = cfg.Path
			config["host"] = cfg.Host
		}
		if cfg.Transport == "xhttp" {
			config["mode"] = cfg.Mode
			if cfg.Extra != "" {
				config["extra"] = cfg.Extra
			}
		}
		if cfg.Fingerprint != "" {
			config["fingerprint"] = cfg.Fingerprint
		}
		if cfg.ALPN != "" {
			config["alpn"] = cfg.ALPN
		}
	case "vmess":
		config["uuid"] = cfg.UUID
		config["encryption"] = cfg.Encryption
		config["transport"] = cfg.Transport
		if cfg.Transport == "ws" {
			config["path"] = cfg.Path
			config["host"] = cfg.Host
		}
	case "trojan":
		config["password"] = cfg.Password
		config["security"] = cfg.Security
		config["transport"] = cfg.Transport
		if cfg.Transport == "ws" {
			config["path"] = cfg.Path
			config["host"] = cfg.Host
		}
	case "hysteria2":
		config["password"] = cfg.Password
		config["insecure"] = cfg.Insecure
		if cfg.PublicKey != "" {
			config["obfs"] = "salamander"
		}
	case "shadowsocks":
		config["method"] = cfg.Method
		config["password"] = cfg.Password
	}

	return config
}

// updateSingboxOutbound updates the sing-box config with the selected server
func updateSingboxOutbound(serverURL string) error {
	serverCfg, err := parseServerURL(serverURL)
	if err != nil {
		return fmt.Errorf("failed to parse server URL: %w", err)
	}

	// Read current config
	data, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Build outbound config based on protocol
	outbound := buildOutbound(serverCfg)

	// Update outbound in config
	if outbounds, ok := config["outbounds"].([]interface{}); ok && len(outbounds) > 0 {
		outbounds[0] = outbound
	}

	// Write updated config
	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cfg.ConfigFile, newData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	log.Printf("Updated sing-box outbound (protocol: %s, server: %s:%d)", serverCfg.Protocol, serverCfg.Server, serverCfg.Port)
	return nil
}

// buildOutbound creates a sing-box outbound config based on protocol
func buildOutbound(cfg *ProtocolConfig) map[string]interface{} {
	outbound := map[string]interface{}{
		"tag":         "proxy",
		"server":      cfg.Server,
		"server_port": cfg.Port,
	}

	switch cfg.Protocol {
	case "vless":
		outbound["type"] = "vless"
		outbound["uuid"] = cfg.UUID
		outbound["flow"] = cfg.Flow
		outbound["tls"] = buildTLS(cfg.SNI, cfg.Security, cfg.Insecure)
		outbound["transport"] = buildTransport(cfg.Transport, cfg.Path, cfg.Host, cfg.Mode)

	case "vmess":
		outbound["type"] = "vmess"
		outbound["uuid"] = cfg.UUID
		outbound["security"] = cfg.Encryption
		outbound["alter_id"] = 0
		outbound["tls"] = buildTLS(cfg.SNI, cfg.Security, cfg.Insecure)
		outbound["transport"] = buildTransport(cfg.Transport, cfg.Path, cfg.Host, cfg.Mode)

	case "trojan":
		outbound["type"] = "trojan"
		outbound["password"] = cfg.Password
		outbound["tls"] = buildTLS(cfg.SNI, cfg.Security, cfg.Insecure)
		outbound["transport"] = buildTransport(cfg.Transport, cfg.Path, cfg.Host, cfg.Mode)

	case "hysteria2":
		outbound["type"] = "hysteria2"
		outbound["password"] = cfg.Password
		outbound["tls"] = buildTLS(cfg.SNI, cfg.Security, cfg.Insecure)
		if cfg.PublicKey != "" {
			outbound["obfs"] = map[string]interface{}{
				"type":    "salamander",
				"secret": cfg.PublicKey,
			}
		}

	case "shadowsocks":
		outbound["type"] = "shadowsocks"
		outbound["method"] = cfg.Method
		outbound["password"] = cfg.Password
	}

	return outbound
}

// buildTLS creates TLS config
func buildTLS(sni, security string, insecure bool) map[string]interface{} {
	if security == "" || security == "none" {
		return nil
	}

	tls := map[string]interface{}{
		"enabled":     true,
		"server_name": sni,
		"insecure":    insecure,
		"utls": map[string]interface{}{
			"enabled":     true,
			"fingerprint": "chrome",
		},
	}

	if security == "reality" {
		tls["reality"] = map[string]interface{}{
			"enabled":    true,
			"public_key": "", // Will be filled from URL params
		}
	}

	return tls
}

// buildTransport creates transport config
func buildTransport(transport, path, host, mode string) map[string]interface{} {
	if transport == "" || transport == "tcp" {
		return nil
	}

	tr := map[string]interface{}{
		"type": transport,
	}

	switch transport {
	case "ws":
		tr["path"] = path
		if host != "" {
			tr["headers"] = map[string]interface{}{
				"Host": host,
			}
		}
	case "grpc":
		tr["path"] = path
		tr["idle_timeout"] = "15s"
		tr["ping_timeout"] = "15s"
		tr["permit_without_stream"] = true
	case "http", "xhttp":
		tr["host"] = host
		tr["path"] = path
		if mode != "" {
			tr["mode"] = mode
		}
		if transport == "xhttp" {
			tr["x_padding_bytes"] = "100-500"
		}
	case "httpupgrade":
		tr["host"] = host
		tr["path"] = path
	}

	return tr
}

func reloadSingbox() bool {
	// Find sing-box PID
	pidFile := "/tmp/.singbox_pid"
	data, err := os.ReadFile(pidFile)
	if err != nil {
		log.Printf("Failed to read sing-box PID file: %v", err)
		return false
	}
	pidStr := strings.TrimSpace(string(data))
	if pidStr == "" {
		log.Printf("sing-box PID is empty")
		return false
	}

	var pid int
	_, err = fmt.Sscanf(pidStr, "%d", &pid)
	if err != nil {
		log.Printf("Invalid sing-box PID: %v", err)
		return false
	}

	// Send SIGHUP to reload config
	process, err := os.FindProcess(pid)
	if err != nil {
		log.Printf("Failed to find sing-box process: %v", err)
		return false
	}

	err = process.Signal(syscall.SIGHUP)
	if err != nil {
		log.Printf("Failed to send SIGHUP to sing-box: %v", err)
		return false
	}

	log.Printf("sing-box reloaded (PID: %d)", pid)
	return true
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
		subURL, _ := body["url"].(string)
		if name == "" {
			name = "Subscription"
		}
		if subURL == "" {
			sendJSON(w, 400, map[string]interface{}{"error": "URL required"})
			return
		}
		var servers []ServerInfo
		if subURL == "manual" {
			if srvs, ok := body["servers"].([]interface{}); ok {
				for idx, s := range srvs {
					if srv, ok := s.(string); ok {
						servers = append(servers, ServerInfo{Name: fmt.Sprintf("Server #%d", idx), URL: srv})
					}
				}
			}
		} else if isDirectServerURL(subURL) {
			// Direct server URL (vless://, trojan://, etc.)
			name := ""
			hashIdx := strings.Index(subURL, "#")
			if hashIdx >= 0 {
				name = subURL[hashIdx+1:]
				if decoded, err := url.PathUnescape(name); err == nil {
					name = decoded
				}
			}
			if name == "" {
				name = "Direct Server"
			}
			servers = []ServerInfo{{Name: name, URL: subURL}}
		} else {
			content, err := fetchSubscription(subURL)
			if err != nil {
				sendJSON(w, 500, map[string]interface{}{"error": "Failed to fetch subscription"})
				return
			}
			servers = parseServerInfos(content)
		}
		sub := Subscription{
			ID:          fmt.Sprintf("sub_%d", time.Now().Unix()),
			Name:        name,
			URL:         subURL,
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
		state.selectedServer = -1
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

func handleActivateSubscription(w http.ResponseWriter, r *http.Request, subID string) {
	if !checkAuth(r) {
		sendJSON(w, 401, map[string]interface{}{"error": "Unauthorized"})
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	for _, sub := range state.subscriptions {
		if sub.ID == subID {
			state.activeSubscription = subID
			saveSubscriptions()
			sendJSON(w, 200, map[string]interface{}{
				"message":           "Subscription activated",
				"active_subscription": subID,
			})
			return
		}
	}
	sendJSON(w, 404, map[string]interface{}{"error": "Subscription not found"})
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
		} else if strings.Contains(path, "/test-config") {
			handleServerTestConfig(w, body)
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

	// Update sing-box config with the selected server
	serverURL := sub.Servers[index].URL
	if err := updateSingboxOutbound(serverURL); err != nil {
		log.Printf("Warning: failed to update sing-box outbound: %v", err)
	}

	// Parse server config for JSON output
	serverConfig := parseServerConfig(serverURL)

	// Reload sing-box to apply changes
	if reloadSingbox() {
		log.Printf("sing-box reloaded with server %d", index)
	}

	sendJSON(w, 200, map[string]interface{}{
		"selected":    index,
		"server":      sub.Servers[index],
		"config":      serverConfig,
		"message":     "Server selected",
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
		latency := testServerLatency(server.URL, timeout)
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
		var idx int
		switch v := fastest["index"].(type) {
		case float64:
			idx = int(v)
		case int:
			idx = v
		default:
			idx = 0
		}
		state.selectedServer = idx
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

func handleServerTestConfig(w http.ResponseWriter, body map[string]interface{}) {
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
	if sub == nil || len(sub.Servers) == 0 {
		sendJSON(w, 400, map[string]interface{}{"error": "No servers available"})
		return
	}
	index := int(body["index"].(float64))
	if index >= len(sub.Servers) {
		sendJSON(w, 400, map[string]interface{}{"error": "Invalid server index"})
		return
	}

	serverURL := sub.Servers[index].URL
	cfg, err := parseServerURL(serverURL)
	if err != nil {
		sendJSON(w, 400, map[string]interface{}{"error": err.Error()})
		return
	}

	result := map[string]interface{}{
		"valid": true,
	}

	// TCP dial
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	addr := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		result["valid"] = false
		result["message"] = fmt.Sprintf("Connection failed: TCP dial error: %v", err)
		sendJSON(w, 200, result)
		return
	}
	defer conn.Close()

	// TLS handshake if needed
	if cfg.Security == "tls" || cfg.Protocol == "trojan" || cfg.Protocol == "vless" {
		tlsConfig := &tls.Config{
			ServerName:         cfg.SNI,
			InsecureSkipVerify: cfg.Insecure,
		}
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			result["valid"] = false
			result["message"] = fmt.Sprintf("Connection failed: TLS handshake error: %v", err)
			sendJSON(w, 200, result)
			return
		}
		conn = tlsConn
	}

	latency := int(time.Since(start).Milliseconds())
	result["message"] = "Config valid, connection successful"
	result["latency"] = latency
	sendJSON(w, 200, result)
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

func handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if !checkAuth(r) {
			sendJSON(w, 401, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		body := readBody(r)
		action := body["action"].(string)

		if action == "connect" {
			// Reload sing-box with current config
			if !reloadSingbox() {
				sendJSON(w, 500, map[string]interface{}{
					"error":  "Failed to reload sing-box",
					"status": "disconnected",
				})
				return
			}

			// Actually establish connection by sending traffic through the proxy
			time.Sleep(1 * time.Second)
			client := &http.Client{
				Timeout: 10 * time.Second,
				Transport: &http.Transport{
					Proxy: http.ProxyURL(&url.URL{
						Scheme: "http",
						Host:   "127.0.0.1:20123",
					}),
				},
			}

			resp, err := client.Get("https://1.1.1.1")
			if err != nil {
				log.Printf("Connect failed: %v", err)
				sendJSON(w, 500, map[string]interface{}{
					"error":  fmt.Sprintf("Connection failed: %v", err),
					"status": "disconnected",
				})
				return
			}
			resp.Body.Close()

			sendJSON(w, 200, map[string]interface{}{
				"message": "Connected",
				"status":  "connected",
			})
			return
		} else if action == "disconnect" {
			// Disconnect by stopping sing-box process
			cmd := exec.Command("kill", "$(pgrep -f 'sing-box run')")
			cmd.Run()
			time.Sleep(500 * time.Millisecond)
			sendJSON(w, 200, map[string]interface{}{
				"message": "Disconnected",
				"status":  "disconnected",
			})
			return
		}
		sendJSON(w, 400, map[string]interface{}{"error": "Invalid action"})
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
	case "/api/servers", "/api/servers/select", "/api/servers/test", "/api/servers/test-config":
		handleServers(w, r)
	case "/api/config", "/api/config/reload":
		handleConfig(w, r)
	case "/api/connect":
		handleConnect(w, r)
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
			if r.Method == http.MethodPost && strings.HasSuffix(subID, "/activate") {
				handleActivateSubscription(w, r, strings.TrimSuffix(subID, "/activate"))
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
