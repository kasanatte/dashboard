package assistant

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"k8s.io/klog/v2"
)

type MCPServerConfig struct {
	Type    string            `json:"type"`
	Name    string            `json:"name"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	BaseURL string            `json:"baseUrl,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Enabled bool              `json:"enabled,omitempty"`
}

type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
	Global     struct {
		KarmadaKubeConfig string `json:"karmadaKubeConfig,omitempty"`
		KarmadaContext    string `json:"karmadaContext,omitempty"`
		AutoDiscovery     bool   `json:"autoDiscovery,omitempty"`
	} `json:"global,omitempty"`
}

var (
	mcpConfig  *MCPConfig
	configOnce sync.Once
	// Support multiple configuration paths
	configFilePaths = []string{
		"/etc/karmada-dashboard/mcp-config.json",
		"./config/mcp-config.json",
		"$HOME/.karmada/mcp-config.json",
	}
)

func loadMCPConfig() *MCPConfig {
	configOnce.Do(func() {
		config := &MCPConfig{
			MCPServers: make(map[string]MCPServerConfig),
		}

		// First, try to load from config file
		configLoaded := false
		for _, path := range configFilePaths {
			expandedPath := expandPath(path)
			if _, err := os.Stat(expandedPath); err == nil {
				if err := loadConfigFromFile(config, expandedPath); err == nil {
					klog.Infof("Loaded MCP configuration from %s", expandedPath)
					configLoaded = true
					break
				} else {
					klog.Warningf("Failed to load MCP config from %s: %v", expandedPath, err)
				}
			}
		}

		// If no config file found or empty, use environment-based configuration
		if !configLoaded || len(config.MCPServers) == 0 {
			loadEnvironmentConfig(config)
		}

		// Validate and set defaults
		validateConfig(config)

		mcpConfig = config
	})
	return mcpConfig
}

func loadConfigFromFile(config *MCPConfig, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, config)
}

func loadEnvironmentConfig(config *MCPConfig) {
	kubeconfig := getKubeConfigPath()
	context := getKarmadaContext()

	// Check if MCP is disabled via environment
	if os.Getenv("KARMADA_MCP_DISABLED") == "true" {
		klog.Info("MCP features disabled via KARMADA_MCP_DISABLED environment variable")
		return
	}

	// Check for karmada-mcp-server in various locations
	serverPath := findMCPBinary()
	if serverPath == "" {
		klog.Info("karmada-mcp-server binary not found, MCP features will be disabled")
		return
	}

	config.MCPServers["karmada-mcp-server"] = MCPServerConfig{
		Type:    "stdio",
		Name:    "karmada-mcp-server",
		Command: serverPath,
		Args: []string{
			"stdio",
			"--karmada-kubeconfig=" + kubeconfig,
			"--karmada-context=" + context,
		},
		Enabled: true,
	}
}

func findMCPBinary() string {
	// Check environment variable first
	if path := os.Getenv("KARMADA_MCP_SERVER_PATH"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		klog.Warningf("KARMADA_MCP_SERVER_PATH points to non-existent file: %s", path)
	}

	// Check common installation paths
	paths := []string{
		"/usr/local/bin/karmada-mcp-server",
		"/usr/bin/karmada-mcp-server",
		"/opt/karmada/bin/karmada-mcp-server",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Check PATH
	if path, err := exec.LookPath("karmada-mcp-server"); err == nil {
		return path
	}

	return ""
}

func getKubeConfigPath() string {
	// Environment variable takes precedence
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}

	// Check if we have a specific karmada config
	homeDir := getHomeDir()
	karmadaConfig := filepath.Join(homeDir, ".kube", "karmada.config")
	if _, err := os.Stat(karmadaConfig); err == nil {
		return karmadaConfig
	}

	// Fallback to standard kubeconfig
	return filepath.Join(homeDir, ".kube", "config")
}

func getKarmadaContext() string {
	if context := os.Getenv("KARMADA_CONTEXT"); context != "" {
		return context
	}
	return "karmada-apiserver"
}

func getHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return "/root"
}

func expandPath(path string) string {
	return os.ExpandEnv(path)
}

func validateConfig(config *MCPConfig) {
	// Set global defaults
	if config.Global.KarmadaKubeConfig == "" {
		config.Global.KarmadaKubeConfig = getKubeConfigPath()
	}
	if config.Global.KarmadaContext == "" {
		config.Global.KarmadaContext = getKarmadaContext()
	}

	// Validate and update server configurations
	for name, server := range config.MCPServers {
		if server.Type == "" {
			server.Type = "stdio"
		}
		if !server.Enabled {
			delete(config.MCPServers, name)
			continue
		}

		// Update kubeconfig references
		for i, arg := range server.Args {
			if server.Type == "stdio" {
				if arg == "--karmada-kubeconfig" || arg == "--karmada-kubeconfig=" {
					server.Args[i] = "--karmada-kubeconfig=" + config.Global.KarmadaKubeConfig
				}
				if arg == "--karmada-context" || arg == "--karmada-context=" {
					server.Args[i] = "--karmada-context=" + config.Global.KarmadaContext
				}
			}
		}
	}
}

func GetMCPConfig() *MCPConfig {
	return loadMCPConfig()
}
