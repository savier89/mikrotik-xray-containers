#!/bin/bash
# Run all tests for sing-box container
# Usage: ./tests/run_tests.sh [SUB_URL]

set -e

SUB_URL="${1:-https://sub.chebu.site/api/sub/6fLSyfmd-q4tGvcR}"
IMAGE_NAME="test-sing-box:latest"
CONTAINER_NAME="test-sing-box"
PASS=0
FAIL=0

echo "========================================="
echo "sing-box Container Tests"
echo "========================================="
echo "SUB_URL: $SUB_URL"
echo ""

# Step 1: Build test image
echo "=== Building test image ==="
cd "$(dirname "$0")/../.."
docker buildx build \
    --platform linux/amd64 \
    --no-cache \
    --progress=plain \
    --tag "$IMAGE_NAME" \
    --output=type=docker \
    -f sing-box/Dockerfile \
    sing-box/ > /tmp/build.log 2>&1

if [ $? -ne 0 ]; then
    echo "ERROR: Build failed"
    tail -20 /tmp/build.log
    exit 1
fi
echo "✓ Build successful"
echo ""

# Step 2: Run entrypoint tests
echo "=== Running entrypoint tests ==="
docker run --rm \
    --name "${CONTAINER_NAME}-entrypoint" \
    -e SUB_URL="$SUB_URL" \
    "$IMAGE_NAME" \
    sh /tests/test_entrypoint.sh 2>&1 | tail -25

ENTRYPOINT_RESULT=$?
echo ""

# Step 3: Start container for API tests
echo "=== Starting container for API tests ==="
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
docker run -d \
    --name "$CONTAINER_NAME" \
    --network host \
    -e SUB_URL="$SUB_URL" \
    "$IMAGE_NAME"

# Wait for API to be ready
echo "Waiting for API..."
for i in $(seq 1 15); do
    if curl -s http://127.0.0.1:9090/api/health > /dev/null 2>&1; then
        echo "✓ API is ready!"
        break
    fi
    if [ $i -eq 15 ]; then
        echo "ERROR: API not responding after 15s"
        docker logs "$CONTAINER_NAME" | tail -20
        exit 1
    fi
    sleep 1
done
echo ""

# Step 4: Run API tests
echo "=== Running API tests ==="
cd "$(dirname "$0")"
python3 test_api.py
API_RESULT=$?
echo ""

# Step 5: Cleanup
echo "=== Cleanup ==="
docker rm -f "$CONTAINER_NAME" > /dev/null 2>&1
echo "✓ Container removed"
echo ""

# Summary
echo "========================================="
echo "Test Summary"
echo "========================================="
if [ $ENTRYPOINT_RESULT -eq 0 ]; then
    echo "✓ Entrypoint tests: PASS"
else
    echo "✗ Entrypoint tests: FAIL"
    FAIL=$((FAIL + 1))
fi

if [ $API_RESULT -eq 0 ]; then
    echo "✓ API tests: PASS"
else
    echo "✗ API tests: FAIL"
    FAIL=$((FAIL + 1))
fi

echo ""
if [ $FAIL -gt 0 ]; then
    echo "FAILED: $FAIL test suite(s) failed"
    exit 1
fi
echo "ALL TESTS PASSED!"
exit 0
