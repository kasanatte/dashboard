package assistant

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/klog/v2"
)

type MCPClientManager struct {
	clients     map[string]client.MCPClient
	tools       map[string][]mcp.Tool
	mu          sync.RWMutex
	initialized bool
	enabled     bool
	initError   error
}

var (
	manager     *MCPClientManager
	managerOnce sync.Once
)

func GetMCPClientManager() *MCPClientManager {
	managerOnce.Do(func() {
		manager = &MCPClientManager{
			clients: make(map[string]client.MCPClient),
			tools:   make(map[string][]mcp.Tool),
			enabled: true, // Enabled by default, but can be gracefully degraded
		}

		// Synchronous initialization, but in a background goroutine to avoid blocking the main thread
		go manager.initializeSync()
	})
	return manager
}

// initializeSync performs synchronous MCP initialization with graceful fallback
func (m *MCPClientManager) initializeSync() {
	config := GetMCPConfig()
	if len(config.MCPServers) == 0 {
		klog.Info("No MCP servers configured, MCP features disabled")
		m.markInitialized(false)
		return
	}

	for name, serverConfig := range config.MCPServers {
		if err := m.initializeServer(name, serverConfig); err != nil {
			klog.Warningf("MCP server %s initialization failed: %v, MCP features disabled", name, err)
			m.markInitialized(false)
			return
		}
	}

	m.markInitialized(true)
}

// initializeServer initializes a single MCP server
func (m *MCPClientManager) initializeServer(name string, config MCPServerConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if the binary exists
	if config.Type == "stdio" {
		if _, err := exec.LookPath(config.Command); err != nil {
			return fmt.Errorf("MCP binary not found: %s", config.Command)
		}
	}

	// Set up environment variables
	env := os.Environ()
	if config.Env != nil {
		for k, v := range config.Env {
			env = append(env, k+"="+v)
		}
	}

	// Use mcp-go client to connect directly
	cli, err := client.NewStdioMCPClient(config.Command, env, config.Args...)
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %w", err)
	}

	// Initialize session
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "karmada-dashboard",
		Version: "1.0.0",
	}

	_, err = cli.Initialize(ctx, initRequest)
	if err != nil {
		cli.Close()
		return fmt.Errorf("failed to initialize MCP session: %w", err)
	}

	// Get tool list
	toolsRequest := mcp.ListToolsRequest{}
	toolsResponse, err := cli.ListTools(ctx, toolsRequest)
	if err != nil {
		cli.Close()
		return fmt.Errorf("failed to list tools: %w", err)
	}

	m.mu.Lock()
	m.clients[name] = cli
	m.tools[name] = toolsResponse.Tools
	m.mu.Unlock()

	klog.Infof("MCP server %s initialized successfully with %d tools", name, len(toolsResponse.Tools))
	return nil
}

// markInitialized marks the manager as initialized (or not)
func (m *MCPClientManager) markInitialized(enabled bool) {
	m.mu.Lock()
	m.initialized = true
	m.enabled = enabled
	m.mu.Unlock()
}

// IsReady returns true if MCP is ready to use
func (m *MCPClientManager) IsReady() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized && m.enabled && len(m.clients) > 0
}

// GetTools returns available tools
func (m *MCPClientManager) GetTools() []mcp.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if !m.enabled {
		return nil
	}
	
	var allTools []mcp.Tool
	for _, tools := range m.tools {
		allTools = append(allTools, tools...)
	}
	return allTools
}

// ExecuteTool executes a tool by name
func (m *MCPClientManager) ExecuteTool(ctx context.Context, serverName string, toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if !m.enabled {
		return nil, fmt.Errorf("MCP is not enabled")
	}

	cli, exists := m.clients[serverName]
	if !exists {
		return nil, fmt.Errorf("MCP server %s not found", serverName)
	}

	// Use context with timeout for tool calls
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return cli.CallTool(callCtx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: arguments,
		},
	})
}

// Close closes all MCP clients
func (m *MCPClientManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, cli := range m.clients {
		if err := cli.Close(); err != nil {
			klog.Errorf("Error closing MCP client %s: %v", name, err)
		}
		delete(m.clients, name)
		delete(m.tools, name)
	}
}

// WaitForInitialization waits for MCP initialization to complete
func (m *MCPClientManager) WaitForInitialization(timeout time.Duration) error {
	start := time.Now()
	for {
		m.mu.RLock()
		initialized := m.initialized
		initError := m.initError
		m.mu.RUnlock()
		
		if initialized {
			return initError
		}
		
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for MCP initialization")
		}
		
		time.Sleep(100 * time.Millisecond)
	}
}

// GetStatus returns MCP status information
func (m *MCPClientManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	status := map[string]interface{}{
		"enabled":     m.enabled,
		"initialized": m.initialized,
		"servers":     len(m.clients),
		"total_tools": len(m.GetTools()),
	}
	
	for serverName, tools := range m.tools {
		status[serverName] = map[string]interface{}{
			"tools_count": len(tools),
		}
	}
	
	return status
}

