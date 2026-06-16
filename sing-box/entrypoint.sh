#!/bin/sh
# sing-box entrypoint for MikroTik RouterOS
# Generates /sing-box.json from ENV, validates, runs sing-box
# Compatible with ash/dash (Alpine)

# Support running tests (generate config first, then run command)
if [ "$1" = "test" ] || [ "$1" = "sh" ] || [ "$1" = "python3" ]; then
    # Generate config first
    GENERATE_CONFIG=true
fi

set -e

: "${REMOTE_PORT:=443}"
: "${FLOW:=}"
: "${FINGER_PRINT:=chrome}"
: "${PUBLIC_KEY:=}"
: "${SHORT_ID:=}"
: "${SPIDER_X:=/}"
: "${NETWORK:=tcp}"
: "${WS_PATH:=/}"
: "${LOG_LEVEL:=warn}"
: "${DNS_UPSTREAM:=8.8.8.8,8.8.4.4}"
: "${DNS_TYPE:=udp}"
: "${DNS_DOH_URL:=https://dns.google/dns-query}"
: "${TUN_STACK:=system}"
: "${TUN_MTU:=1500}"
: "${WHITELIST_MODE:=}"
: "${RULESETS:=}"
: "${DOMAINS:=}"
: "${DIRECT_IPS:=}"

# ===== API support =====
: "${API_PORT:=9090}"
: "${API_HOST:=0.0.0.0}"
: "${API_AUTH_TOKEN:=}"
: "${SINGBOX_API_PORT:=20123}"
: "${SINGBOX_API_TOKEN:=}"

# ===== Subscription support =====
: "${SUB_URL:=}"
: "${SUB_SELECT:=auto}"
: "${SUB_REFRESH:=0}"
: "${SUB_USER_AGENT:=curl/8.0.0}"
: "${SUB_TEST_TIMEOUT:=5}"
: "${SUB_TEST_COUNT:=3}"

parse_subscription() {
    echo "Fetching subscription..."
    local SUB_CONTENT
    SUB_CONTENT=$(wget -qO- --user-agent="$SUB_USER_AGENT" --timeout=15 "$SUB_URL" 2>/dev/null) || {
        echo "ERROR: Failed to fetch subscription from $SUB_URL"
        return 1
    }
    
    local DECODED="$SUB_CONTENT"
    local B64_DECODED=$(echo "$SUB_CONTENT" | base64 -d 2>/dev/null || true)
    if [ -n "$B64_DECODED" ] && echo "$B64_DECODED" | grep -q "://"; then
        DECODED="$B64_DECODED"
    fi
    
    local LINKS
    LINKS=$(echo "$DECODED" | grep -oE '(hysteria2|vless|vmess|trojan|ss)://[^[:space:]"<>,]+' || true)
    
    if [ -z "$LINKS" ]; then
        echo "ERROR: No servers found in subscription"
        return 1
    fi
    
    echo "Found servers:"
    echo "$LINKS" | awk '{print NR": "$0}'
    
    local SELECTED=""
    case "$SUB_SELECT" in
        index:*|idx:*)
            IDX=$(echo "$SUB_SELECT" | cut -d: -f2)
            SELECTED=$(echo "$LINKS" | sed -n "${IDX}p")
            ;;
        protocol:*|proto:*)
            PROTO_FILTER=$(echo "$SUB_SELECT" | cut -d: -f2)
            SELECTED=$(echo "$LINKS" | grep -i "$PROTO_FILTER" | head -1)
            ;;
        random)
            TOTAL=$(echo "$LINKS" | wc -l)
            RAND_LINE=$(( (RANDOM % TOTAL) + 1 ))
            SELECTED=$(echo "$LINKS" | sed -n "${RAND_LINE}p")
            echo "Random selection: line $RAND_LINE of $TOTAL"
            ;;
        fastest)
            echo "Testing servers (timeout: ${SUB_TEST_TIMEOUT}s, attempts: ${SUB_TEST_COUNT})..."
            BEST_TIME=999999
            BEST_LINK=""
            echo "$LINKS" | while IFS= read -r link; do
                SRV=$(echo "$link" | sed -n 's|.*@\([^:]*\).*|\1|p')
                [ -z "$SRV" ] && continue
                START=$(date +%s%N 2>/dev/null || echo "0")
                wget -qO- --timeout="$SUB_TEST_TIMEOUT" --tries="$SUB_TEST_COUNT" "https://$SRV" 2>/dev/null && {
                    END=$(date +%s%N 2>/dev/null || echo "0")
                    if [ "$START" != "0" ] && [ "$END" != "0" ]; then
                        LATENCY=$(( (END - START) / 1000000 ))
                        echo "  $SRV: ${LATENCY}ms"
                        if [ "$LATENCY" -lt "$BEST_TIME" ]; then
                            BEST_TIME=$LATENCY
                            BEST_LINK="$link"
                        fi
                    fi
                } || echo "  $SRV: timeout"
            done
            SELECTED=$(cat /tmp/.fastest_link 2>/dev/null || echo "$LINKS" | head -1)
            [ -z "$SELECTED" ] && SELECTED=$(echo "$LINKS" | head -1)
            ;;
        auto|first|"")
            SELECTED=$(echo "$LINKS" | head -1)
            ;;
    esac
    
    if [ -z "$SELECTED" ]; then
        echo "ERROR: No server matched selection: $SUB_SELECT"
        return 1
    fi
    
    echo "Selected: $SELECTED"
    
    case "$SELECTED" in
        hysteria2://*)
            export URL="$SELECTED"
            ;;
        vless://*)
            VLESS_UUID=$(echo "$SELECTED" | sed -n 's|vless://\([^@]*\)@.*|\1|p')
            VLESS_REST=$(echo "$SELECTED" | sed -n 's|vless://[^@]*@\([^?]*\).*|\1|p')
            VLESS_SERVER=$(echo "$VLESS_REST" | cut -d: -f1)
            VLESS_PORT=$(echo "$VLESS_REST" | cut -d: -f2)
            VLESS_PARAMS=$(echo "$SELECTED" | sed 's/.*?//')
            VLESS_SNI=$(echo "$VLESS_PARAMS" | grep -o 'sni=[^&]*' | cut -d= -f2)
            VLESS_FLOW=$(echo "$VLESS_PARAMS" | grep -o 'flow=[^&]*' | cut -d= -f2)
            VLESS_PUBKEY=$(echo "$VLESS_PARAMS" | grep -o 'pbk=[^&]*' | cut -d= -f2)
            VLESS_SID=$(echo "$VLESS_PARAMS" | grep -o 'sid=[^&]*' | cut -d= -f2)
            VLESS_NET=$(echo "$VLESS_PARAMS" | grep -o 'type=[^&]*' | cut -d= -f2)
            VLESS_PATH=$(echo "$VLESS_PARAMS" | grep -o 'path=[^&]*' | cut -d= -f2)
            VLESS_REALITY=$(echo "$VLESS_PARAMS" | grep -co 'reality=true' || echo "0")
            VLESS_SECURITY=$(echo "$VLESS_PARAMS" | grep -o 'security=[^&]*' | cut -d= -f2)
            VLESS_MODE=$(echo "$VLESS_PARAMS" | grep -o 'mode=[^&]*' | cut -d= -f2)
            VLESS_HOST=$(echo "$VLESS_PARAMS" | grep -o 'host=[^&]*' | cut -d= -f2)
            
            export REMOTE_ADDRESS="$VLESS_SERVER"
            export REMOTE_PORT="${VLESS_PORT:-443}"
            export ID="$VLESS_UUID"
            export SERVER_NAME="$VLESS_SNI"
            [ -n "$VLESS_FLOW" ] && export FLOW="$VLESS_FLOW"
            [ "$VLESS_REALITY" != "0" ] && {
                export PUBLIC_KEY="$VLESS_PUBKEY"
                export SHORT_ID="$VLESS_SID"
            }
            [ -n "$VLESS_NET" ] && export NETWORK="$VLESS_NET"
            [ -n "$VLESS_PATH" ] && export WS_PATH="$VLESS_PATH"
            [ -n "$VLESS_SECURITY" ] && export SECURITY="$VLESS_SECURITY"
            [ -n "$VLESS_MODE" ] && export XHTTP_MODE="$VLESS_MODE"
            [ -n "$VLESS_HOST" ] && export XHTTP_HOST="$VLESS_HOST"
            ;;
        vmess://*)
            VM_UUID=$(echo "$SELECTED" | sed -n 's|vmess://\([^@]*\)@.*|\1|p')
            VM_REST=$(echo "$SELECTED" | sed -n 's|vmess://[^@]*@\([^?]*\).*|\1|p')
            VM_SERVER=$(echo "$VM_REST" | cut -d: -f1)
            VM_PORT=$(echo "$VM_REST" | cut -d: -f2)
            VM_SNI=$(echo "$SELECTED" | grep -o 'sni=[^&]*' | cut -d= -f2 || true)
            
            export REMOTE_ADDRESS="$VM_SERVER"
            export REMOTE_PORT="${VM_PORT:-443}"
            export ID="$VM_UUID"
            export SERVER_NAME="${VM_SNI:-$VM_SERVER}"
            ;;
        trojan://*)
            T_PASS=$(echo "$SELECTED" | sed -n 's|trojan://\([^@]*\)@.*|\1|p')
            T_REST=$(echo "$SELECTED" | sed -n 's|trojan://[^@]*@\([^?]*\).*|\1|p')
            T_SERVER=$(echo "$T_REST" | cut -d: -f1)
            T_PORT=$(echo "$T_REST" | cut -d: -f2)
            T_SNI=$(echo "$SELECTED" | grep -o 'sni=[^&]*' | cut -d= -f2 || true)
            
            export REMOTE_ADDRESS="$T_SERVER"
            export REMOTE_PORT="${T_PORT:-443}"
            export ID="$T_PASS"
            export SERVER_NAME="${T_SNI:-$T_SERVER}"
            export PROTO_FORCE="trojan"
            ;;
        *)
            echo "ERROR: Unsupported protocol in link"
            return 1
            ;;
    esac
    
    return 0
}

# Detect protocol
if [ -n "$SUB_URL" ]; then
    PROTO="subscription"
    if ! parse_subscription; then
        exit 1
    fi
    if [ -n "$URL" ]; then
        PROTO="hysteria2"
    elif [ -n "$PROTO_FORCE" ]; then
        PROTO="vless"
    elif [ -n "$REMOTE_ADDRESS" ] && [ -n "$ID" ] && [ -n "$SERVER_NAME" ]; then
        PROTO="vless"
    fi
elif [ -n "$URL" ]; then
    PROTO="hysteria2"
elif [ -n "$REMOTE_ADDRESS" ] && [ -n "$ID" ] && [ -n "$SERVER_NAME" ]; then
    PROTO="vless"
else
    echo "ERROR: Set SUB_URL (subscription), URL (hysteria2), or REMOTE_ADDRESS+ID+SERVER_NAME (vless)"
    exit 1
fi

echo "=== sing-box for MikroTik ==="
echo "Protocol: $PROTO"

# Build outbound JSON
OUTBOUND=""
if [ "$PROTO" = "vless" ] || [ "$PROTO" = "subscription" ]; then
    ACTUAL_PROTO="vless"
    [ "$PROTO_FORCE" = "trojan" ] && ACTUAL_PROTO="trojan"
    if [ "$ACTUAL_PROTO" = "trojan" ]; then
        OUTBOUND="\"type\":\"trojan\",\"server\":\"${REMOTE_ADDRESS}\",\"server_port\":${REMOTE_PORT},\"password\":\"${ID}\""
    else
        OUTBOUND="\"type\":\"vless\",\"server\":\"${REMOTE_ADDRESS}\",\"server_port\":${REMOTE_PORT},\"uuid\":\"${ID}\""
        [ -n "$FLOW" ] && OUTBOUND="${OUTBOUND},\"flow\":\"${FLOW}\""
    fi
    OUTBOUND="${OUTBOUND},\"tls\":{\"enabled\":true,\"server_name\":\"${SERVER_NAME}\",\"utls\":{\"enabled\":true,\"fingerprint\":\"${FINGER_PRINT}\"}"
    if [ -n "$PUBLIC_KEY" ]; then
        OUTBOUND="${OUTBOUND},\"reality\":{\"enabled\":true,\"public_key\":\"${PUBLIC_KEY}\",\"short_id\":\"${SHORT_ID}\"}"
    fi
    OUTBOUND="${OUTBOUND}}"
    if [ "$NETWORK" = "ws" ]; then
        OUTBOUND="${OUTBOUND},\"transport\":{\"type\":\"ws\",\"path\":\"${WS_PATH}\"}"
    elif [ "$NETWORK" = "grpc" ]; then
        OUTBOUND="${OUTBOUND},\"transport\":{\"type\":\"grpc\",\"service_name\":\"${WS_PATH}\"}"
    elif [ "$NETWORK" = "xhttp" ]; then
        : "${XHTTP_MODE:=stream-one}"
        : "${XHTTP_HOST:=}"
        DECODED_PATH=$(printf '%b' "$(echo "${WS_PATH}" | sed 's/%\([0-9A-Fa-f]\{2\}\)/\\x\1/g' 2>/dev/null || echo "${WS_PATH}")" 2>/dev/null || echo "${WS_PATH}")
        XHTTP_CONFIG="\"transport\":{\"type\":\"xhttp\",\"host\":\"${XHTTP_HOST}\",\"mode\":\"${XHTTP_MODE}\",\"x_padding_bytes\":\"100-1000\""
        [ -n "$DECODED_PATH" ] && XHTTP_CONFIG="${XHTTP_CONFIG},\"path\":\"${DECODED_PATH}\""
        XHTTP_CONFIG="${XHTTP_CONFIG}}"
        OUTBOUND="${OUTBOUND},${XHTTP_CONFIG}"
    fi
elif [ "$PROTO" = "hysteria2" ]; then
    H2_PASS=$(echo "$URL" | sed -n 's|hysteria2://\([^@]*\)@.*|\1|p')
    H2_SRV=$(echo "$URL" | sed -n 's|hysteria2://[^@]*@\([^:?]*\).*|\1|p')
    H2_PORT=$(echo "$URL" | sed -n 's|hysteria2://[^@]*@[^:]*:\([0-9]*\).*|\1|p')
    H2_SNI=$(echo "$URL" | grep -o 'sni=[^&]*' | head -1 | cut -d= -f2)
    H2_AUTH=$(echo "$URL" | grep -o 'auth=[^&]*' | head -1 | cut -d= -f2 | sed 's/%20/ /g')
    H2_INSECURE=$(echo "$URL" | grep -co 'insecure=1' || echo "0")
    : "${H2_PORT:=${REMOTE_PORT}}"
    : "${H2_SNI:=${SERVER_NAME}}"
    : "${H2_PASS:=${H2_AUTH}}"
    OUTBOUND="\"type\":\"hysteria2\",\"server\":\"${H2_SRV}\",\"server_port\":${H2_PORT},\"password\":\"${H2_PASS}\",\"tls\":{\"enabled\":true,\"server_name\":\"${H2_SNI}\""
    [ "$H2_INSECURE" != "0" ] && OUTBOUND="${OUTBOUND},\"insecure\":true"
    OUTBOUND="${OUTBOUND}}"
fi

# Build DNS
DNS_FIRST=$(echo "$DNS_UPSTREAM" | cut -d, -f1)

if [ "$DNS_TYPE" = "doh" ]; then
    DNS_JSON=$(jq -n \
      --arg dns_first "$DNS_FIRST" \
      '{
        servers: [
          {type: "udp", tag: "dns-local", server: $dns_first, server_port: 53},
          {type: "https", tag: "dns-remote", server: "dns.google", server_port: 443, path: "/dns-query", domain_resolver: "dns-local"}
        ],
        final: "dns-remote",
        strategy: "prefer_ipv4",
        reverse_mapping: true
      }')
else
    DNS_JSON=$(jq -n \
      --arg dns_first "$DNS_FIRST" \
      '{
        servers: [
          {type: "udp", tag: "dns-local", server: $dns_first, server_port: 53}
        ],
        final: "dns-local",
        strategy: "prefer_ipv4",
        reverse_mapping: true
      }')
fi

# Build rules
RULES_JSON='[{"action":"sniff"},{"protocol":"dns","action":"hijack-dns"},{"ip_cidr":["10.0.0.0/8","172.16.0.0/12","192.168.0.0/16","fc00::/7"],"outbound":"direct"}]'

if [ -n "$DIRECT_IPS" ]; then
    IPS_JSON=$(echo "$DIRECT_IPS" | tr ',' '\n' | sed 's/^ *//;s/ *$//' | awk 'NF{printf "%s\"%s\"", (NR>1?",":""), $0}')
    RULES_JSON=$(echo "$RULES_JSON" | jq --argjson ips "[${IPS_JSON}]" '. + [{"ip_cidr":$ips,"outbound":"direct"}]')
fi

if [ -n "$DOMAINS" ]; then
    DOM_JSON=$(echo "$DOMAINS" | tr ',' '\n' | sed 's/^ *//;s/ *$//' | awk 'NF{printf "%s\"%s\"", (NR>1?",":""), $0}')
    RULES_JSON=$(echo "$RULES_JSON" | jq --argjson doms "[${DOM_JSON}]" '. + [{"domain_suffix":$doms,"outbound":"direct"}]')
fi

# Rulesets
RS_DEF_JSON="[]"
if [ -n "$RULESETS" ]; then
    IDX=0
    OLD_IFS="$IFS"
    IFS=','
    for RURL in $RULESETS; do
        IFS="$OLD_IFS"
        TAG="ruleset-${IDX}"
        RFORMAT="binary"
        case "$RURL" in *.json) RFORMAT="source" ;; esac
        RS_DEF_JSON=$(echo "$RS_DEF_JSON" | jq --arg tag "$TAG" --arg fmt "$RFORMAT" --arg url "$RURL" '. + [{"tag":$tag,"type":"remote","format":$fmt,"url":$url,"download_detour":"proxy"}]')
        IDX=$((IDX + 1))
    done
    IFS="$OLD_IFS"
fi

FINAL="proxy"
[ "$WHITELIST_MODE" = "1" ] && FINAL="direct"

# Write config
jq -n \
  --argjson dns "$DNS_JSON" \
  --argjson outbound "{\"tag\":\"proxy\",${OUTBOUND}}" \
  --argjson rules "$RULES_JSON" \
  --argjson rule_set "$RS_DEF_JSON" \
  --arg final "$FINAL" \
  --arg log_level "$LOG_LEVEL" \
  --argjson tun_mtu "$TUN_MTU" \
  --arg tun_stack "$TUN_STACK" \
  --arg clash_api_addr "127.0.0.1" \
  --arg clash_api_port "$SINGBOX_API_PORT" \
  '{
    log: {level: $log_level, timestamp: true},
    dns: $dns,
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
      },
      {
        type: "mixed",
        tag: "clash-api",
        listen: $clash_api_addr,
        listen_port: ($clash_api_port | tonumber)
      }
    ],
    outbounds: [$outbound, {tag: "direct", type: "direct"}, {tag: "block", type: "block"}],
    route: {
      default_domain_resolver: "dns-local",
      auto_detect_interface: true,
      rule_set: $rule_set,
      rules: $rules,
      final: $final
    },
    experimental: {
      cache_file: {enabled: true},
      clash_api: {
        external_controller: ($clash_api_addr + ":" + $clash_api_port),
        default_mode: "Global"
      }
    }
  }' > /sing-box.json

echo "Config generated:"
cat /sing-box.json
echo ""

echo "Validating..."
sing-box check -c /sing-box.json --disable-color || exit 1
echo "OK."

# If running tests, execute the command instead of starting sing-box
if [ "$GENERATE_CONFIG" = "true" ]; then
    exec "$@"
fi

echo "Starting sing-box..."

# Start sing-box in background
/usr/local/bin/sing-box run -c /sing-box.json > /tmp/sing-box.log 2>&1 &
SINGBOX_PID=$!
echo "$SINGBOX_PID" > /tmp/.singbox_pid

echo "sing-box started (PID: $SINGBOX_PID)"

# Start management API
export SINGBOX_API_ADDR="127.0.0.1:${SINGBOX_API_PORT}"
export SINGBOX_API_TOKEN="${SINGBOX_API_TOKEN}"
export API_AUTH_TOKEN="${API_AUTH_TOKEN}"
export API_HOST="${API_HOST}"
/usr/local/bin/api_server &
API_PID=$!
echo "Management API started (PID: $API_PID, port: ${API_HOST}:${API_PORT})"

# Wait for either process to exit
wait -n $SINGBOX_PID $API_PID
EXIT_CODE=$?
if [ $EXIT_CODE -eq 0 ]; then
    echo "sing-box exited, stopping API..."
    kill $API_PID 2>/dev/null
else
    echo "API exited, stopping sing-box..."
    kill $SINGBOX_PID 2>/dev/null
fi
wait
