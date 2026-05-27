#!/bin/sh
# hev-socks5-tunnel entrypoint for MikroTik
#
# Required env vars:
#   SOCKS5_ADDR - Xray container IP (e.g. 172.17.0.2)
#   SOCKS5_PORT - Xray SOCKS port (default: 10800)
#
# Optional env vars:
#   TUN_NAME    - TUN interface name (default: tun0)
#   MTU         - TUN interface MTU (default: 8500)
#   TUN_IPV4    - TUN interface IP (default: 198.18.0.1)
#   TABLE       - IP routing table ID (default: 20)
#   MARK        - Firewall mark for bypass rules (default: 438)
#   UDP_MODE    - SOCKS5 UDP mode: fullcone, udp, tcp (default: udp)
#   LOG_LEVEL   - Log level: emerg, alert, crit, error, warn, info, debug (default: warn)
#   GATEWAY     - MikroTik gateway IP (veth gateway, default: 172.17.0.1)
#   TZ          - Timezone (default: UTC)

set -e

# Defaults
: "${SOCKS5_PORT:=10800}"
: "${TUN_NAME:=tun0}"
: "${MTU:=8500}"
: "${TUN_IPV4:=198.18.0.1}"
: "${TABLE:=20}"
: "${MARK:=438}"
: "${UDP_MODE:=udp}"
: "${LOG_LEVEL:=warn}"
: "${GATEWAY:=172.17.0.1}"
: "${TZ:=UTC}"

if [ -z "$SOCKS5_ADDR" ]; then
    echo "ERROR: SOCKS5_ADDR is required (Xray container IP)"
    exit 1
fi

export TZ

echo "=== hev-socks5-tunnel for MikroTik ==="
echo "SOCKS5: ${SOCKS5_ADDR}:${SOCKS5_PORT}"
echo "TUN: ${TUN_NAME} (${TUN_IPV4}, MTU ${MTU})"
echo "Route table: ${TABLE}, Mark: ${MARK}"
echo "UDP mode: ${UDP_MODE}"
echo "Gateway: ${GATEWAY}"
echo ""

# Generate YAML config
cat > /etc/hev-socks5-tunnel/hev.yml <<EOF
tunnel:
  name: ${TUN_NAME}
  mtu: ${MTU}
  multi-queue: false
  ipv4: ${TUN_IPV4}

socks5:
  port: ${SOCKS5_PORT}
  address: ${SOCKS5_ADDR}
  udp: '${UDP_MODE}'
  mark: ${MARK}

misc:
  log-level: ${LOG_LEVEL}
EOF

echo "Config:"
cat /etc/hev-socks5-tunnel/hev.yml
echo ""

# Setup TUN interface
echo "Setting up ${TUN_NAME}..."
ip link del ${TUN_NAME} 2>/dev/null || true
ip tuntap add dev ${TUN_NAME} mode tun
ip addr add ${TUN_IPV4}/30 dev ${TUN_NAME}
ip link set dev ${TUN_NAME} up

# Setup routing
# Default route through TUN
ip route del default 2>/dev/null || true
ip route add default dev ${TUN_NAME} table ${TABLE}
ip rule add lookup ${TABLE} pref ${TABLE}

# Bypass route: traffic to SOCKS5 server goes through gateway (not through TUN)
ip route add ${SOCKS5_ADDR}/32 via ${GATEWAY}

# Bypass route: DNS traffic goes through gateway
ip route add 1.1.1.1/32 via ${GATEWAY}
ip route add 8.8.8.8/32 via ${GATEWAY}
ip route add 8.8.4.4/32 via ${GATEWAY}

# Bypass local networks
ip route add 10.0.0.0/8 via ${GATEWAY}
ip route add 172.16.0.0/12 via ${GATEWAY}
ip route add 192.168.0.0/16 via ${GATEWAY}

# Disable reverse path filter for TUN interface
sysctl -w net.ipv4.conf.all.rp_filter=0 2>/dev/null || true
sysctl -w net.ipv4.conf.${TUN_NAME}.rp_filter=0 2>/dev/null || true

echo "Routing table:"
ip route show table ${TABLE}
echo ""
echo "Default route:"
ip route show default
echo ""

# Start hev-socks5-tunnel in background
echo "Starting hev-socks5-tunnel..."
/usr/local/bin/hev-socks5-tunnel /etc/hev-socks5-tunnel/hev.yml &

# Keep container alive
echo "Container ready. Waiting..."
exec /sbin/init
