# Karmada MCP Integration Guide

This guide explains how to set up and use the Karmada MCP (Model Context Protocol) integration in the Karmada Dashboard.

## Overview

The Karmada Dashboard now includes built-in AI assistance powered by the Karmada-MCP-Server, enabling natural language management of multi-cluster environments through a ChatUI interface.

## Features

- **Natural Language Cluster Management**: Ask questions about your clusters in plain English
- **Zero Configuration**: Works out of the box with automatic discovery
- **Real-time Status**: Live MCP server status indicator in the UI
- **Tool Transparency**: See exactly what tools are being used and their results
- **Graceful Degradation**: Falls back to general guidance when MCP is unavailable

## Quick Start

### Prerequisites

1. **Karmada Dashboard**: Ensure you have the latest version of Karmada Dashboard
2. **Karmada-MCP-Server**: Install the karmada-mcp-server binary

### Installation

#### Option 1: Binary Installation

```bash
# Download the latest karmada-mcp-server
wget https://github.com/karmada-io/karmada-mcp-server/releases/latest/download/karmada-mcp-server-linux-amd64
chmod +x karmada-mcp-server-linux-amd64
sudo mv karmada-mcp-server-linux-amd64 /usr/local/bin/karmada-mcp-server
```

#### Option 2: Docker (Coming Soon)

```bash
# Future Docker installation
docker pull karmada/karmada-mcp-server:latest
```

## Configuration

### Environment Variables

The integration supports flexible configuration through environment variables:

```bash
# MCP server binary location
export KARMADA_MCP_SERVER_PATH=/usr/local/bin/karmada-mcp-server

# Karmada configuration
export KUBECONFIG=/path/to/karmada.config
export KARMADA_CONTEXT=karmada-apiserver

# Disable MCP (fallback to general guidance)
export KARMADA_MCP_DISABLED=true
```

### Configuration File

Create `/etc/karmada-dashboard/mcp-config.json`:

```json
{
  "global": {
    "karmadaKubeConfig": "/path/to/karmada.config",
    "karmadaContext": "karmada-apiserver",
    "autoDiscovery": true
  },
  "mcpServers": {
    "karmada-mcp-server": {
      "type": "stdio",
      "name": "karmada-mcp-server",
      "command": "karmada-mcp-server",
      "args": [
        "stdio",
        "--karmada-kubeconfig=/path/to/karmada.config",
        "--karmada-context=karmada-apiserver"
      ],
      "enabled": true
    }
  }
}
```

### Configuration Paths

The system automatically searches for configuration in these locations:

1. `/etc/karmada-dashboard/mcp-config.json`
2. `./config/mcp-config.json`
3. `$HOME/.karmada/mcp-config.json`

## Usage

### Accessing the AI Assistant

1. Navigate to the Karmada Dashboard
2. Click on "AI Assistant" in the navigation menu
3. Start asking questions about your clusters

### Example Queries

#### Cluster Management
- "List all my clusters"
- "Show me the status of cluster prod-east"
- "Which clusters are ready?"

#### Workload Management
- "Show me all deployments in namespace default"
- "List pods with status not running"
- "Create a new deployment called nginx in cluster prod-west"

#### Policy Management
- "Show me propagation policies"
- "List override policies"
- "Create a policy to deploy nginx to all clusters"

#### Resource Discovery
- "What resources are available?"
- "Show me services across all clusters"
- "List configmaps in the kube-system namespace"

### Understanding the Interface

#### Status Indicators

- **🟢 MCP Ready**: Full functionality available
- **🟡 MCP Not Ready**: Limited functionality (general guidance only)
- **🔴 MCP Unavailable**: MCP server not found or disabled

#### Tool Call Transparency

When the AI uses tools to interact with your clusters, you'll see:

1. **Tool Call**: Shows which tool is being used and its parameters
2. **Tool Result**: Displays the actual output from your clusters
3. **Error Handling**: Clear error messages if something goes wrong

## Advanced Configuration

### Multiple MCP Servers

You can configure multiple MCP servers for different purposes:

```json
{
  "mcpServers": {
    "karmada-mcp-server": {
      "type": "stdio",
      "command": "karmada-mcp-server",
      "args": ["stdio"],
      "enabled": true
    },
    "custom-mcp-server": {
      "type": "stdio", 
      "command": "/path/to/custom-mcp-server",
      "args": ["stdio", "--config", "/path/to/config"],
      "enabled": true
    }
  }
}
```

### Environment-Specific Configuration

#### Development
```bash
# Use local karmada-mcp-server
export KARMADA_MCP_SERVER_PATH=./bin/karmada-mcp-server
export KUBECONFIG=~/.kube/karmada-dev.config
```

#### Production
```bash
# Use system installation
export KARMADA_MCP_SERVER_PATH=/usr/local/bin/karmada-mcp-server
export KUBECONFIG=/etc/karmada/karmada.config
```

## Troubleshooting

### Common Issues

#### MCP Server Not Found

**Symptoms**: Status shows "MCP Not Ready"

**Solutions**:
1. Install karmada-mcp-server binary
2. Set `KARMADA_MCP_SERVER_PATH` environment variable
3. Check if binary is in system PATH
4. Verify binary has execute permissions

#### Connection Issues

**Symptoms**: Tools return connection errors

**Solutions**:
1. Verify `KUBECONFIG` points to valid karmada configuration
2. Check if `karmada-context` exists in your kubeconfig
3. Ensure karmada-apiserver is accessible

#### Permission Issues

**Symptoms**: Tools return permission denied errors

**Solutions**:
1. Ensure dashboard service account has necessary RBAC permissions
2. Check if kubeconfig has correct user credentials
3. Verify cluster connectivity

### Debug Mode

Enable debug logging:

```bash
# Backend logs
export LOG_LEVEL=debug

# Frontend browser console
# Open browser dev tools and check Network tab for /api/v1/assistant requests
```

### Status API

Check MCP status programmatically:

```bash
curl http://localhost:8080/api/v1/assistant/status
```

Expected response:
```json
{
  "enabled": true,
  "initialized": true,
  "servers": 1,
  "total_tools": 15,
  "karmada-mcp-server": {
    "tools_count": 15
  }
}
```

## Security Considerations

- **API Keys**: The dashboard uses environment variables for LLM API keys
- **RBAC**: Ensure proper RBAC is configured for the dashboard service account
- **Network Security**: MCP communication is done via stdio (secure)
- **Audit Logging**: All tool calls are logged for security audit

## Performance Optimization

### Resource Limits

The MCP server runs with reasonable defaults:

- **Memory**: ~50MB per server instance
- **CPU**: Minimal usage when idle
- **Timeout**: 30 seconds per tool call
- **Connection**: Automatic reconnection on failure

### Scaling

For large clusters:
1. Increase tool call timeout via environment variables
2. Consider running multiple MCP server instances
3. Monitor memory usage and adjust as needed

## Contributing

### Adding New MCP Tools

To extend the MCP server with custom tools:

1. Fork the karmada-mcp-server repository
2. Add new tool definitions
3. Submit pull request with documentation
4. Update dashboard configuration examples

### Dashboard Integration

To add new dashboard features:
1. Extend the MCP client in `cmd/api/app/routes/assistant/`
2. Add corresponding UI components in `ui/apps/dashboard/src/pages/assistant/`
3. Update this documentation
4. Add comprehensive tests

## Support

- **GitHub Issues**: https://github.com/karmada-io/dashboard/issues
- **Karmada Slack**: #karmada-users on CNCF Slack
- **Documentation**: https://karmada.io/docs/