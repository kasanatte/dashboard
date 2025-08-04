package assistant

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type MCPToolListResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	} `json:"result"`
	Error interface{} `json:"error,omitempty"`
}

func TestMCPToolList(t *testing.T) {
	fmt.Println("=== MCP 工具测试（模拟 ChatClient.prompt(\"有哪些工具可以使用\")） ===")

	// MCP 路径
	mcpPath := os.Getenv("KARMADA_MCP_SERVER_PATH")
	if mcpPath == "" {
		mcpPath = "/Users/ty/Documents/intern/project/dashboard/_output/bin/darwin/arm64/karmada-mcp-server"
		fmt.Printf("KARMADA_MCP_SERVER_PATH 未设置，使用默认路径：%s\n", mcpPath)
	}
	if _, err := os.Stat(mcpPath); err != nil {
		t.Fatalf("❌ MCP server 不存在: %v", err)
	}

	// kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/karmada.config"
	}
	if _, err := os.Stat(kubeconfig); err != nil {
		t.Fatalf("❌ kubeconfig 不存在: %v", err)
	}

	// 启动 MCP stdio
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, mcpPath, "stdio",
		"--karmada-kubeconfig="+kubeconfig,
		"--karmada-context=karmada-apiserver",
	)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("❌ 创建 stdin 失败: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("❌ 创建 stdout 失败: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("❌ MCP 启动失败: %v", err)
	}
	fmt.Println("✅ MCP 启动成功，进入 stdio 模式")

	reader := bufio.NewReader(stdout)

	// 发送初始化
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"go-test","version":"1.0"}}}` + "\n"
	toolsReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	stdin.Write([]byte(initReq))
	time.Sleep(500 * time.Millisecond) // 等初始化完成
	stdin.Write([]byte(toolsReq))

	timeout := time.After(10 * time.Second)
	var toolList MCPToolListResponse

	for {
		select {
		case <-timeout:
			t.Fatalf("❌ MCP 响应超时，请确认 MCP 是否返回 tools/list 响应")
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				continue
			}
			raw := strings.TrimSpace(string(line))
			if raw == "" {
				continue
			}
			fmt.Printf("[MCP 回应] %s\n", raw)

			// 判断是否是我们要的 id=2
			var msg map[string]interface{}
			if err := json.Unmarshal(line, &msg); err != nil {
				continue
			}
			if id, ok := msg["id"].(float64); ok && int(id) == 2 {
				json.Unmarshal(line, &toolList)
				goto Show
			}
		}
	}

Show:
	if toolList.Error != nil {
		t.Fatalf("❌ MCP 返回错误: %v", toolList.Error)
	}
	if len(toolList.Result.Tools) == 0 {
		t.Fatalf("⚠️ MCP 工具列表为空")
	}

	fmt.Println("\n✅ MCP 工具列表如下：")
	for _, tool := range toolList.Result.Tools {
		fmt.Printf(" - %s：%s\n", tool.Name, tool.Description)
	}

	cmd.Process.Kill()
	fmt.Println("\n=== MCP 工具测试完成 ===")
}
