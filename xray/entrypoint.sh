#!/bin/sh
# Xray entrypoint - generates config from environment variables and starts the proxy
#
# Required env vars:
#   SERVER_ADDRESS - Xray server hostname/IP
#   SERVER_PORT    - Xray server port (default: 443)
#   ID             - VLESS UUID
#
# Optional env vars:
#   ENCRYPTION     - VLESS encryption (default: none)
#   FLOW           - VLESS flow (e.g. xtls-rprx-vision)
#   NETWORK        - Transport network: tcp, ws, h2, grpc (default: tcp)
#   SECURITY       - TLS security: none, tls, reality (default: none)
#   SNI            - Server Name Indication for TLS/Reality
#   WS_PATH        - WebSocket path (for NETWORK=ws)
#   FP             - Fingerprint for Reality (e.g. chrome)
#   PBK            - PublicKey for Reality
#   SID            - ShortId for Reality
#   SPX            - SpiderX path for Reality
#   PQV            - Post-quantum value for Reality
#   LOGLEVEL       - Xray log level (default: warning)
#   SOCKS_PORT     - Local SOCKS port (default: 10800)

set -e

# Defaults
: "${ENCRYPTION:=none}"
: "${FLOW:=}"
: "${NETWORK:=tcp}"
: "${SECURITY:=none}"
: "${SNI:=}"
: "${WS_PATH:=/}"
: "${FP:=chrome}"
: "${PBK:=}"
: "${SID:=}"
: "${SPX:=/}"
: "${PQV:=}"
: "${LOGLEVEL:=warning}"
: "${SOCKS_PORT:=10800}"

if [ -z "$SERVER_ADDRESS" ] || [ -z "$ID" ]; then
    echo "ERROR: SERVER_ADDRESS and ID are required"
    exit 1
fi

echo "Starting Xray SOCKS proxy on port ${SOCKS_PORT}"
echo "Server: ${SERVER_ADDRESS}:${SERVER_PORT}"

# Build streamSettings dynamically
STREAM_SETTINGS=""

if [ "$SECURITY" = "reality" ]; then
    STREAM_SETTINGS=$(cat <<EOF
{
    "network": "${NETWORK}",
    "security": "reality",
    "realitySettings": {
        "fingerprint": "${FP}",
        "serverName": "${SNI}",
        "publicKey": "${PBK}",
        "shortId": "${SID}",
        "spx": "${SPX}"
    }
}
EOF
)
    # Add wsSettings if network is ws
    if [ "$NETWORK" = "ws" ]; then
        STREAM_SETTINGS=$(cat <<EOF
{
    "network": "ws",
    "security": "reality",
    "wsSettings": {
        "path": "${WS_PATH}"
    },
    "realitySettings": {
        "fingerprint": "${FP}",
        "serverName": "${SNI}",
        "publicKey": "${PBK}",
        "shortId": "${SID}",
        "spx": "${SPX}"
    }
}
EOF
)
    fi
elif [ "$SECURITY" = "tls" ]; then
    if [ "$NETWORK" = "ws" ]; then
        STREAM_SETTINGS=$(cat <<EOF
{
    "network": "ws",
    "security": "tls",
    "tlsSettings": {
        "serverName": "${SNI}"
    },
    "wsSettings": {
        "path": "${WS_PATH}"
    }
}
EOF
)
    else
        STREAM_SETTINGS=$(cat <<EOF
{
    "network": "${NETWORK}",
    "security": "tls",
    "tlsSettings": {
        "serverName": "${SNI}"
    }
}
EOF
)
    fi
else
    STREAM_SETTINGS=$(cat <<EOF
{
    "network": "${NETWORK}"
}
EOF
)
    if [ "$NETWORK" = "ws" ]; then
        STREAM_SETTINGS=$(cat <<EOF
{
    "network": "ws",
    "wsSettings": {
        "path": "${WS_PATH}"
    }
}
EOF
)
    fi
fi

# Build flow field (only for xtls-reality-vision)
FLOW_FIELD=""
if [ -n "$FLOW" ]; then
    FLOW_FIELD=",\"flow\": \"${FLOW}\""
fi

# Generate config
cat > /tmp/xray-config.json <<EOFCONFIG
{
    "log": {
        "loglevel": "${LOGLEVEL}"
    },
    "inbounds": [
        {
            "port": ${SOCKS_PORT},
            "listen": "0.0.0.0",
            "protocol": "socks",
            "settings": {
                "udp": true
            },
            "sniffing": {
                "enabled": true,
                "destOverride": ["http", "tls", "quic"],
                "routeOnly": true
            }
        }
    ],
    "outbounds": [
        {
            "protocol": "vless",
            "settings": {
                "vnext": [
                    {
                        "address": "${SERVER_ADDRESS}",
                        "port": ${SERVER_PORT},
                        "users": [
                            {
                                "id": "${ID}",
                                "encryption": "${ENCRYPTION}"${FLOW_FIELD}
                            }
                        ]
                    }
                ]
            },
            "streamSettings": ${STREAM_SETTINGS}
        }
    ]
}
EOFCONFIG

echo "Config generated:"
cat /tmp/xray-config.json
echo ""
echo "Launching Xray..."

exec /usr/local/bin/xray run -config /tmp/xray-config.json
