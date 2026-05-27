#!/bin/bash
# Build and push containers to local Docker registry
#
# Usage:
#   ./build.sh [ARCH]
#
# ARCH: amd64 | arm64 | arm (default: all)
#
# Registry URL is configurable via REGISTRY variable below.
# Default: localhost:5000 (for local registry on the build machine)

set -e

# ===== Configuration =====
REGISTRY="${REGISTRY:-localhost:5000}"
PROJECT="mikrotik-xray"
TAG="${TAG:-latest}"

# ===== Architecture mapping =====
ARCH_MAP=""
case "${1:-all}" in
    amd64)  ARCH_MAP="amd64" ;;
    arm64)  ARCH_MAP="arm64" ;;
    arm)    ARCH_MAP="arm" ;;
    all)    ARCH_MAP="amd64 arm64 arm" ;;
    *)      echo "Usage: $0 [amd64|arm64|arm|all]"; exit 1 ;;
esac

# ===== Colors =====
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[BUILD]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# ===== Check prerequisites =====
if ! command -v docker &> /dev/null; then
    echo "ERROR: docker is not installed"
    exit 1
fi

# ===== Build Xray container =====
build_xray() {
    local arch="$1"
    local platform=""

    case "$arch" in
        amd64) platform="linux/amd64" ;;
        arm64) platform="linux/arm64" ;;
        arm)   platform="linux/arm/v7" ;;
    esac

    local image_name="${REGISTRY}/${PROJECT}/xray:${TAG}-${arch}"

    log "Building Xray (${arch}, ${platform})..."
    log "Image: ${image_name}"

    docker buildx build \
        --platform "${platform}" \
        --no-cache \
        --progress=plain \
        --tag "${image_name}" \
        --output=type=docker \
        -f xray/Dockerfile \
        xray/

    log "Pushing Xray (${arch}) to ${REGISTRY}..."
    docker push "${image_name}"
    log "Done: Xray (${arch})"
    echo ""
}

# ===== Build hev-socks5-tunnel container =====
build_hev() {
    local arch="$1"
    local platform=""

    case "$arch" in
        amd64) platform="linux/amd64" ;;
        arm64) platform="linux/arm64" ;;
        arm)   platform="linux/arm/v7" ;;
    esac

    local image_name="${REGISTRY}/${PROJECT}/hev-socks5-tunnel:${TAG}-${arch}"

    log "Building hev-socks5-tunnel (${arch}, ${platform})..."
    log "Image: ${image_name}"

    docker buildx build \
        --platform "${platform}" \
        --no-cache \
        --progress=plain \
        --tag "${image_name}" \
        --output=type=docker \
        -f hev-socks5-tunnel/Dockerfile \
        hev-socks5-tunnel/

    log "Pushing hev-socks5-tunnel (${arch}) to ${REGISTRY}..."
    docker push "${image_name}"
    log "Done: hev-socks5-tunnel (${arch})"
    echo ""
}

# ===== Main =====
echo "========================================"
echo " MikroTik Xray Container Builder"
echo "========================================"
echo "Registry: ${REGISTRY}"
echo "Project:  ${PROJECT}"
echo "Tag:      ${TAG}"
echo "Arch:     ${ARCH_MAP}"
echo "========================================"
echo ""

# Check if registry is accessible
if ! docker info 2>/dev/null | grep -q "Registry"; then
    warn "Could not verify registry connectivity"
    warn "Make sure registry is running: docker run -d -p 5000:5000 registry:2"
fi

for arch in ${ARCH_MAP}; do
    build_xray "${arch}"
    build_hev "${arch}"
done

echo "========================================"
log "All images built and pushed!"
echo "========================================"
echo ""
echo "Images in ${REGISTRY}:"
echo "  ${PROJECT}/xray:${TAG}-amd64"
echo "  ${PROJECT}/xray:${TAG}-arm64"
echo "  ${PROJECT}/xray:${TAG}-arm"
echo "  ${PROJECT}/hev-socks5-tunnel:${TAG}-amd64"
echo "  ${PROJECT}/hev-socks5-tunnel:${TAG}-arm64"
echo "  ${PROJECT}/hev-socks5-tunnel:${TAG}-arm"
echo ""
echo "On MikroTik, pull with:"
echo "  /container remote-image=${REGISTRY}/${PROJECT}/xray:${TAG}-arm64"
echo "  /container remote-image=${REGISTRY}/${PROJECT}/hev-socks5-tunnel:${TAG}-arm64"
