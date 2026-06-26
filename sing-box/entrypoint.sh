#!/bin/sh
# sing-box entrypoint for MikroTik RouterOS
# Generates a minimal default config, then starts sing-box + API + Web UI
# All configuration is done through the Web UI (API Server)
# Compatible with ash/dash (Alpine)

set -e

# ===== Environment defaults =====
: "${LOG_LEVEL:=warn}"
: "${DNS_UPSTREAM:=8.8.8.8}"
: "${TUN_STACK:=system}"
: "${TUN_MTU:=1500}"

# ===== API =====
: "${API_PORT:=9090}"
: "${API_HOST:=0.0.0.0}"
: "${API_AUTH_TOKEN:=}"
: "${SINGBOX_API_PORT:=20123}"
: "${SINGBOX_API_TOKEN:=}"

# ===== Web UI =====
: "${WEBUI_PORT:=11501}"

echo "=== sing-box for MikroTik ==="
echo "Log level: $LOG_LEVEL"

# Generate minimal sing-box config (placeholder — Web UI will configure)
jq -n \
  --arg log_level "$LOG_LEVEL" \
  --arg dns_upstream "$DNS_UPSTREAM" \
  --argjson tun_mtu "$TUN_MTU" \
  --arg tun_stack "$TUN_STACK" \
  --arg clash_api_port "$SINGBOX_API_PORT" \
  '{
    log: {level: $log_level, timestamp: true},
    dns: {
      servers: [
        {type: "udp", tag: "dns-local", server: $dns_upstream, server_port: 53}
      ],
      final: "dns-local",
      strategy: "prefer_ipv4"
    },
    inbounds: [
      {
        type: "tun",
        tag: "tun-in",
        interface_name: "tun0",
        address: ["198.18.0.1/30"],
        mtu: $tun_mtu,
        auto_route: true,
        auto_redirect: true,
        strict_route: true,
        stack: $tun_stack,
        route_exclude_address: ["192.168.0.0/16", "172.16.0.0/12", "10.0.0.0/8", "fc00::/7"]
      }
    ],
    outbounds: [
      {tag: "proxy", type: "direct"},
      {tag: "direct", type: "direct"},
      {tag: "block", type: "block"}
    ],
    route: {
      default_domain_resolver: "dns-local",
      auto_detect_interface: true,
      rules: [
        {action: "sniff"},
        {protocol: "dns", action: "hijack-dns"},
        {ip_cidr: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"], outbound: "direct"}
      ],
      final: "direct"
    },
    experimental: {
      cache_file: {enabled: true},
      clash_api: {
        external_controller: ("127.0.0.1:" + $clash_api_port),
        default_mode: "Global"
      }
    }
  }' > /sing-box.json

echo "Config generated (minimal default)"

# Validate config
/sing-box check -c /sing-box.json --disable-color || exit 1
echo "Config OK."

# Start sing-box in background
/sing-box run -c /sing-box.json > /tmp/sing-box.log 2>&1 &
SINGBOX_PID=$!
echo "$SINGBOX_PID" > /tmp/.singbox_pid
echo "sing-box started (PID: $SINGBOX_PID)"

# Start management API
export SINGBOX_API_ADDR="127.0.0.1:${SINGBOX_API_PORT}"
export SINGBOX_API_TOKEN="${SINGBOX_API_TOKEN}"
export API_AUTH_TOKEN="${API_AUTH_TOKEN}"
export API_HOST="${API_HOST}"
/api_server &
API_PID=$!
echo "Management API started (PID: $API_PID, port: ${API_HOST}:${API_PORT})"

# Start web UI server
export API_TARGET="http://127.0.0.1:${API_PORT}"
export PORT="$WEBUI_PORT"
/web_ui_server &
WEBUI_PID=$!
echo "Web UI started (PID: $WEBUI_PID, port: ${WEBUI_PORT})"

# Wait for any process to exit
wait -n $SINGBOX_PID $API_PID $WEBUI_PID
EXIT_CODE=$?
if [ $EXIT_CODE -eq 0 ]; then
    echo "sing-box exited, stopping API and Web UI..."
    kill $API_PID $WEBUI_PID 2>/dev/null
else
    echo "Process exited, stopping all..."
    kill $SINGBOX_PID $API_PID $WEBUI_PID 2>/dev/null
fi
wait
