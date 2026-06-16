#!/bin/sh
# Tests for entrypoint.sh subscription parsing and config generation
# Run inside the container: sh /tests/test_entrypoint.sh

set -e

PASS=0
FAIL=0
TOTAL=0

test() {
    TOTAL=$((TOTAL + 1))
    if [ "$2" = "0" ]; then
        PASS=$((PASS + 1))
        echo "  ✓ $1"
    else
        FAIL=$((FAIL + 1))
        echo "  ✗ $1"
    fi
}

echo "=== Entrypoint Tests ==="

# Test 1: Config file exists
[ -f /sing-box.json ]
test "Config file exists" "$?"

# Test 2: Config is valid JSON
jq . /sing-box.json > /dev/null 2>&1
test "Config is valid JSON" "$?"

# Test 3: Config has required sections
jq -e '.log' /sing-box.json > /dev/null 2>&1
test "Config has log section" "$?"

jq -e '.dns' /sing-box.json > /dev/null 2>&1
test "Config has dns section" "$?"

jq -e '.inbounds' /sing-box.json > /dev/null 2>&1
test "Config has inbounds section" "$?"

jq -e '.outbounds' /sing-box.json > /dev/null 2>&1
test "Config has outbounds section" "$?"

jq -e '.route' /sing-box.json > /dev/null 2>&1
test "Config has route section" "$?"

jq -e '.experimental' /sing-box.json > /dev/null 2>&1
test "Config has experimental section" "$?"

# Test 4: Clash API is enabled
jq -e '.experimental.clash_api' /sing-box.json > /dev/null 2>&1
test "Clash API is enabled" "$?"

# Test 5: TUN inbound exists
TUN_INBOUND=$(jq -r '.inbounds[] | select(.type=="tun") | .tag' /sing-box.json)
[ "$TUN_INBOUND" = "tun-in" ]
test "TUN inbound exists" "$?"

# Test 6: Mixed inbound for Clash API exists
MIXED_INBOUND=$(jq -r '.inbounds[] | select(.type=="mixed") | .tag' /sing-box.json)
[ "$MIXED_INBOUND" = "clash-api" ]
test "Mixed inbound for Clash API exists" "$?"

# Test 7: Proxy outbound exists
PROXY_OUTBOUND=$(jq -r '.outbounds[] | select(.tag=="proxy") | .tag' /sing-box.json)
[ "$PROXY_OUTBOUND" = "proxy" ]
test "Proxy outbound exists" "$?"

# Test 8: Direct outbound exists
DIRECT_OUTBOUND=$(jq -r '.outbounds[] | select(.tag=="direct") | .tag' /sing-box.json)
[ "$DIRECT_OUTBOUND" = "direct" ]
test "Direct outbound exists" "$?"

# Test 9: Block outbound exists
BLOCK_OUTBOUND=$(jq -r '.outbounds[] | select(.tag=="block") | .tag' /sing-box.json)
[ "$BLOCK_OUTBOUND" = "block" ]
test "Block outbound exists" "$?"

# Test 10: DNS server exists
DNS_SERVER=$(jq -r '.dns.servers[0].server' /sing-box.json)
[ -n "$DNS_SERVER" ]
test "DNS server exists" "$?"

# Test 11: sing-box binary exists
[ -f /usr/local/bin/sing-box ]
test "sing-box binary exists" "$?"

# Test 12: sing-box version
sing-box version > /dev/null 2>&1
test "sing-box version works" "$?"

# Test 13: sing-box check config
sing-box check -c /sing-box.json --disable-color > /dev/null 2>&1
test "sing-box check config passes" "$?"

# Test 14: API server exists
[ -f /api_server.py ]
test "API server script exists" "$?"

# Test 15: API server is executable
[ -x /api_server.py ]
test "API server is executable" "$?"

# Test 16: Python3 exists
python3 --version > /dev/null 2>&1
test "Python3 exists" "$?"

# Test 17: jq exists
jq --version > /dev/null 2>&1
test "jq exists" "$?"

# Test 18: wget exists
wget --version > /dev/null 2>&1
test "wget exists" "$?"

# Summary
echo ""
echo "=== Summary ==="
echo "Tests: $PASS/$TOTAL passed, $FAIL failed"

if [ $FAIL -gt 0 ]; then
    exit 1
fi
echo "All tests passed!"
exit 0
