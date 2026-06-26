package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ServerConfig holds all possible connection parameters extracted dynamically
type ServerConfig struct {
	Protocol   string
	Server     string
	Port       int
	RawParams  map[string]string
	RawQuery   url.Values
}

// ParseSubscriptionURL detects protocol and parses any subscription link format
func ParseSubscriptionURL(rawURL string) (*ServerConfig, error) {
	cfg := &ServerConfig{
		RawParams: make(map[string]string),
	}

	// Detect protocol from URL prefix
	if !strings.Contains(rawURL, "://") {
		return nil, fmt.Errorf("invalid URL: missing ://")
	}

	parts := strings.SplitN(rawURL, "://", 2)
	cfg.Protocol = strings.ToLower(parts[0])
	rest := parts[1]

	// Handle vmess base64 format
	if cfg.Protocol == "vmess" {
		return parseVmessBase64(rest)
	}

	// Handle ss base64 format
	if cfg.Protocol == "ss" || cfg.Protocol == "shadowsocks" {
		return parseShadowsocks(rest)
	}

	// Standard format: [auth]@server:port?query#remark
	// auth can be uuid, password, method:password, etc.
	atIdx := strings.Index(rest, "@")
	if atIdx < 0 {
		return nil, fmt.Errorf("invalid URL: missing @")
	}

	auth := rest[:atIdx]
	rest = rest[atIdx+1:]

	// Extract remark from #
	remark := ""
	if hashIdx := strings.Index(rest, "#"); hashIdx >= 0 {
		remark = rest[hashIdx+1:]
		rest = rest[:hashIdx]
		cfg.RawParams["remark"] = remark
	}

	// Extract query parameters
	cfg.RawQuery = make(url.Values)
	if qIdx := strings.Index(rest, "?"); qIdx >= 0 {
		queryStr := rest[qIdx+1:]
		rest = rest[:qIdx]
		if parsed, err := url.ParseQuery(queryStr); err == nil {
			for k, v := range parsed {
				if len(v) > 0 {
					cfg.RawQuery[k] = v
					cfg.RawParams[k] = v[0]
				}
			}
		}
	}

	// Parse server:port
	if !strings.Contains(rest, ":") {
		return nil, fmt.Errorf("invalid server:port format")
	}
	hostPort := strings.Split(rest, ":")
	cfg.Server = hostPort[0]
	port, err := strconv.Atoi(hostPort[1])
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}
	cfg.Port = port

	// Protocol-specific auth parsing
	switch cfg.Protocol {
	case "vless":
		cfg.RawParams["uuid"] = auth
	case "trojan":
		cfg.RawParams["password"] = auth
	case "hysteria2", "h2":
		cfg.RawParams["password"] = auth
	case "http":
		// http://base64(username:password)@server:port
		decoded, err := base64.StdEncoding.DecodeString(auth)
		if err != nil {
			cfg.RawParams["username"] = auth
		} else {
			sep := strings.Index(string(decoded), ":")
			if sep >= 0 {
				cfg.RawParams["username"] = string(decoded[:sep])
				cfg.RawParams["password"] = string(decoded[sep+1:])
			} else {
				cfg.RawParams["username"] = string(decoded)
			}
		}
	}

	return cfg, nil
}

// parseVmessBase64 parses vmess://base64(json) format
func parseVmessBase64(encoded string) (*ServerConfig, error) {
	cfg := &ServerConfig{
		Protocol:  "vmess",
		RawParams: make(map[string]string),
		RawQuery:  make(url.Values),
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Try URL-safe base64
		decoded, err = base64.URLEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("invalid vmess base64: %w", err)
		}
	}

	var vmess map[string]interface{}
	if err := json.Unmarshal(decoded, &vmess); err != nil {
		return nil, fmt.Errorf("invalid vmess JSON: %w", err)
	}

	// Extract all fields dynamically
	for k, v := range vmess {
		cfg.RawParams[k] = fmt.Sprintf("%v", v)
	}

	cfg.Server = vmess["add"].(string)
	if port, ok := vmess["port"].(float64); ok {
		cfg.Port = int(port)
	} else if port, ok := vmess["port"].(string); ok {
		cfg.Port, _ = strconv.Atoi(port)
	}

	return cfg, nil
}

// parseShadowsocks parses ss://base64(method:password)@server:port format
func parseShadowsocks(rest string) (*ServerConfig, error) {
	cfg := &ServerConfig{
		Protocol:  "shadowsocks",
		RawParams: make(map[string]string),
		RawQuery:  make(url.Values),
	}

	atIdx := strings.Index(rest, "@")
	if atIdx < 0 {
		return nil, fmt.Errorf("invalid ss URL: missing @")
	}

	auth := rest[:atIdx]
	rest = rest[atIdx+1:]

	// Try base64 decode first
	decoded, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		// Try URL-safe base64
		decoded, err = base64.URLEncoding.DecodeString(auth)
	}

	if err == nil {
		decodedStr := string(decoded)
		sep := strings.Index(decodedStr, ":")
		if sep >= 0 {
			cfg.RawParams["method"] = decodedStr[:sep]
			cfg.RawParams["password"] = decodedStr[sep+1:]
		} else {
			cfg.RawParams["method"] = decodedStr
		}
	} else {
		// Plain text method:password
		sep := strings.Index(auth, ":")
		if sep >= 0 {
			cfg.RawParams["method"] = auth[:sep]
			cfg.RawParams["password"] = auth[sep+1:]
		} else {
			cfg.RawParams["method"] = auth
		}
	}

	// Extract remark
	if hashIdx := strings.Index(rest, "#"); hashIdx >= 0 {
		cfg.RawParams["remark"] = rest[hashIdx+1:]
		rest = rest[:hashIdx]
	}

	// Extract query params
	if qIdx := strings.Index(rest, "?"); qIdx >= 0 {
		queryStr := rest[qIdx+1:]
		rest = rest[:qIdx]
		if parsed, err := url.ParseQuery(queryStr); err == nil {
			for k, v := range parsed {
				if len(v) > 0 {
					cfg.RawQuery[k] = v
					cfg.RawParams[k] = v[0]
				}
			}
		}
	}

	// Parse server:port
	if !strings.Contains(rest, ":") {
		return nil, fmt.Errorf("invalid server:port format")
	}
	hostPort := strings.Split(rest, ":")
	cfg.Server = hostPort[0]
	port, err := strconv.Atoi(hostPort[1])
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}
	cfg.Port = port

	return cfg, nil
}

// GetParam safely gets a parameter value with default
func (c *ServerConfig) GetParam(key string, defaultVal string) string {
	if val, ok := c.RawParams[key]; ok && val != "" {
		return val
	}
	return defaultVal
}

// GetIntParam safely gets an integer parameter with default
func (c *ServerConfig) GetIntParam(key string, defaultVal int) int {
	if val, ok := c.RawParams[key]; ok && val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return defaultVal
}

// GetBoolParam safely gets a boolean parameter
func (c *ServerConfig) GetBoolParam(key string) bool {
	val := c.GetParam(key, "")
	return val == "1" || val == "true" || val == "yes"
}
