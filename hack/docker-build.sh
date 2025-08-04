#!/bin/bash

# Docker build script for karmada-dashboard with integrated MCP server
set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Source version
VERSION=${VERSION:-$("${SCRIPT_DIR}/version.sh")}
REGISTRY=${REGISTRY:-docker.io/karmada}
IMAGE_NAME=${IMAGE_NAME:-karmada-dashboard}
IMAGE_TAG=${IMAGE_TAG:-${VERSION}}

# Build arguments
BUILD_PLATFORMS=${BUILD_PLATFORMS:-linux/amd64,linux/arm64}
BUILD_ARGS="--build-arg VERSION=${VERSION}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Building karmada-dashboard with integrated MCP server...${NC}"
echo "Version: ${VERSION}"
echo "Registry: ${REGISTRY}"
echo "Image: ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
echo "Platforms: ${BUILD_PLATFORMS}"

# Change to root directory
cd "${ROOT_DIR}"

# Build the Docker image
if command -v docker buildx >/dev/null 2>&1; then
    echo -e "${YELLOW}Using docker buildx for multi-platform build...${NC}"
    docker buildx build \
        --platform "${BUILD_PLATFORMS}" \
        ${BUILD_ARGS} \
        -t "${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}" \
        -t "${REGISTRY}/${IMAGE_NAME}:latest" \
        --push=false \
        .
else
    echo -e "${YELLOW}Using standard docker build...${NC}"
    docker build \
        ${BUILD_ARGS} \
        -t "${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}" \
        -t "${REGISTRY}/${IMAGE_NAME}:latest" \
        .
fi

echo -e "${GREEN}Build completed successfully!${NC}"
echo "To run the image:"
echo "  docker run -it --rm -p 8000:8000 -v \$HOME/.kube/karmada.config:/home/karmada/.kube/karmada.config:ro ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
echo ""
echo "Or use docker-compose:"
echo "  docker-compose up"
echo ""
echo "To push to registry:"
echo "  docker push ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"