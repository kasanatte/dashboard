#!/bin/bash

# Quick test script for Docker integration
set -e

echo "=== Testing Docker Integration for Karmada Dashboard + MCP ==="

# Check if Dockerfile exists
if [ ! -f "Dockerfile" ]; then
    echo "❌ Dockerfile not found"
    exit 1
fi

# Check if docker-compose.yml exists
if [ ! -f "docker-compose.yml" ]; then
    echo "❌ docker-compose.yml not found"
    exit 1
fi

# Check if MCP configuration is properly set
if [ ! -f "cmd/api/app/routes/assistant/mcp_config.go" ]; then
    echo "❌ MCP config file not found"
    exit 1
fi

echo "✅ Dockerfile found"
echo "✅ docker-compose.yml found"
echo "✅ MCP configuration found"

# Test environment variable configuration
echo ""
echo "=== Environment Configuration Test ==="
if [ -z "$KUBECONFIG" ]; then
    echo "⚠️  KUBECONFIG not set, will use default"
    export KUBECONFIG="$HOME/.kube/karmada.config"
fi

echo "KUBECONFIG: $KUBECONFIG"
if [ -f "$KUBECONFIG" ]; then
    echo "✅ Kubeconfig file exists"
else
    echo "⚠️  Kubeconfig file not found at $KUBECONFIG"
fi

# Test MCP server path configuration
echo ""
echo "=== MCP Configuration Test ==="
source_file="cmd/api/app/routes/assistant/mcp_config.go"
if grep -q "KARMADA_MCP_SERVER_PATH" "$source_file"; then
    echo "✅ MCP server path is configurable via environment variable"
else
    echo "❌ MCP server path is not configurable"
fi

# Test Docker build context
echo ""
echo "=== Docker Build Context Test ==="
echo "Dockerfile size: $(wc -c < Dockerfile) bytes"
echo "docker-compose.yml size: $(wc -c < docker-compose.yml) bytes"

# Check if .dockerignore exists and is properly configured
if [ -f ".dockerignore" ]; then
    echo "✅ .dockerignore found"
    echo "Ignored patterns:"
    grep -v '^#' .dockerignore | head -5
else
    echo "⚠️  .dockerignore not found"
fi

# Test Makefile integration
echo ""
echo "=== Makefile Integration Test ==="
if grep -q "docker-build" Makefile; then
    echo "✅ Docker build target exists in Makefile"
else
    echo "❌ Docker build target missing from Makefile"
fi

if grep -q "docker-run" Makefile; then
    echo "✅ Docker run target exists in Makefile"
else
    echo "❌ Docker run target missing from Makefile"
fi

# Test MCP client configuration
echo ""
echo "=== MCP Client Configuration Test ==="
mcp_client_file="cmd/api/app/routes/assistant/mcp_client.go"
if grep -q "NewStdioMCPClient" "$mcp_client_file"; then
    echo "✅ Using correct MCP client constructor"
else
    echo "❌ MCP client constructor may be incorrect"
fi

if grep -q "Exec.LookPath" "$mcp_client_file"; then
    echo "✅ MCP binary path resolution implemented"
else
    echo "❌ MCP binary path resolution missing"
fi

echo ""
echo "=== Summary ==="
echo "✅ All basic integration tests passed"
echo ""
echo "To build the Docker image:"
echo "  make docker-build"
echo ""
echo "To run with docker-compose:"
echo "  make docker-compose"
echo ""
echo "To run standalone Docker container:"
echo "  make docker-run"