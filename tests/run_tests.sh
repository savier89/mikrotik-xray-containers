#!/bin/bash
# External test runner for sing-box container
# Usage: ./tests/run_tests.sh [SUB_URL]

set -e

SUB_URL="${1:-https://sub.chebu.site/api/sub/6fLSyfmd-q4tGvcR}"
IMAGE_NAME="test-sing-box:latest"
CONTAINER_NAME="test-sb"
TEST_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$TEST_DIR")"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== External Test Runner ==="
echo "SUB_URL: $SUB_URL"
echo "IMAGE: $IMAGE_NAME"
echo ""

# Build test image
echo -e "${YELLOW}Building test image...${NC}"
cd "$PROJECT_DIR"
docker buildx build \
    --platform linux/amd64 \
    --no-cache \
    --progress=plain \
    --tag "$IMAGE_NAME" \
    --output=type=docker \
    -f sing-box/Dockerfile \
    sing-box/ 2>&1 | tail -5

# Clean up
docker rm -f $CONTAINER_NAME 2>/dev/null || true

# Start container
echo -e "${YELLOW}Starting container...${NC}"
docker run -d --name $CONTAINER_NAME --network host \
    -e SUB_URL="$SUB_URL" \
    "$IMAGE_NAME"

# Wait for API
echo -e "${YELLOW}Waiting for API...${NC}"
for i in $(seq 1 30); do
    if curl -s http://127.0.0.1:9090/api/health > /dev/null 2>&1; then
        echo "API is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo -e "${RED}API not ready after 30s${NC}"
        docker logs $CONTAINER_NAME
        exit 1
    fi
    sleep 1
done

# Run tests
echo ""
echo -e "${YELLOW}Running API tests...${NC}"
python3 "$TEST_DIR/test_api.py"
API_RESULT=$?

echo ""
echo -e "${YELLOW}Running entrypoint tests...${NC}"
docker cp "$TEST_DIR/test_entrypoint.sh" "$CONTAINER_NAME:/tmp/test_entrypoint.sh"
docker exec "$CONTAINER_NAME" sh /tmp/test_entrypoint.sh
ENTRYPOINT_RESULT=$?

# Clean up
echo ""
echo -e "${YELLOW}Cleaning up...${NC}"
docker rm -f $CONTAINER_NAME 2>/dev/null || true

# Results
echo ""
echo "=== Test Results ==="
if [ $API_RESULT -eq 0 ] && [ $ENTRYPOINT_RESULT -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    [ $API_RESULT -ne 0 ] && echo "API tests failed"
    [ $ENTRYPOINT_RESULT -ne 0 ] && echo "Entrypoint tests failed"
    exit 1
fi
