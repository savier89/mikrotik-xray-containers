#!/bin/sh
# External entrypoint tests for sing-box container
# Run via docker exec

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

# Config tests
test "Config file exists" "$(ls /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Config is valid JSON" "$(jq . /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Config has log section" "$(jq -e '.log' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Config has dns section" "$(jq -e '.dns' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Config has inbounds section" "$(jq -e '.inbounds' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Config has outbounds section" "$(jq -e '.outbounds' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Config has route section" "$(jq -e '.route' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Config has experimental section" "$(jq -e '.experimental' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Clash API is enabled" "$(jq -e '.experimental.clash_api' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "TUN inbound exists" "$(jq -e '.inbounds[] | select(.type=="tun")' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Mixed inbound for Clash API exists" "$(jq -e '.inbounds[] | select(.type=="mixed")' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Proxy outbound exists" "$(jq -e '.outbounds[] | select(.tag=="proxy")' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Direct outbound exists" "$(jq -e '.outbounds[] | select(.tag=="direct")' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "Block outbound exists" "$(jq -e '.outbounds[] | select(.tag=="block")' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "DNS server exists" "$(jq -e '.dns.servers[]' /sing-box.json > /dev/null 2>&1 && echo 0 || echo 1)"
test "sing-box binary exists" "$(ls /usr/local/bin/sing-box > /dev/null 2>&1 && echo 0 || echo 1)"
test "sing-box version works" "$(sing-box version > /dev/null 2>&1 && echo 0 || echo 1)"
test "sing-box check config passes" "$(sing-box check -c /sing-box.json --disable-color > /dev/null 2>&1 && echo 0 || echo 1)"
test "API server binary exists" "$(ls /usr/local/bin/api_server > /dev/null 2>&1 && echo 0 || echo 1)"
test "API server is executable" "$(ls -l /usr/local/bin/api_server > /dev/null 2>&1 && echo 0 || echo 1)"
test "wget exists" "$(ls /usr/bin/wget > /dev/null 2>&1 && echo 0 || echo 1)"
test "jq exists" "$(ls /usr/bin/jq > /dev/null 2>&1 && echo 0 || echo 1)"

echo ""
echo "=== Summary ==="
echo "Tests: $TOTAL/$TOTAL passed, $FAIL failed"
if [ $FAIL -eq 0 ]; then
    echo "All tests passed!"
    exit 0
else
    echo "Some tests failed!"
    exit 1
fi
