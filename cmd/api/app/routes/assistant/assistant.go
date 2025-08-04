/*
Copyright 2024 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sashabaranov/go-openai"
	"k8s.io/klog/v2"

	"github.com/karmada-io/dashboard/cmd/api/app/router"
)

func init() {
	router.V1().POST("/assistant", Answering)
	router.V1().GET("/assistant/status", GetStatus)
}

// AnsweringRequest represents the user request body
type AnsweringRequest struct {
	Prompt string `json:"prompt"`
}

// StreamResponse is used for SSE stream response
type StreamResponse struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
}

// Answering handles user questions to LLM with MCP tool support.
func Answering(c *gin.Context) {
	var request AnsweringRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		klog.Errorf("Failed to bind request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Initialize MCP client manager
	mcpManager := GetMCPClientManager()
	tools := mcpManager.GetTools()

	// Configure OpenAI client
	config := openai.DefaultConfig("xxx")
	config.BaseURL = "https://api.moonshot.cn/v1"
	client := openai.NewClientWithConfig(config)

	// Prepare system message based on available tools
	var systemMessage string
	if len(tools) > 0 {
		toolsDescription := buildToolsDescription(tools)
		systemMessage = fmt.Sprintf(`You are a helpful assistant with access to Kubernetes cluster management tools.

Available tools:
%s

When you need to use a tool, respond with a JSON object in this format:
{
  "tool_call": {
    "server": "karmada-mcp-server",
    "tool": "tool_name",
    "arguments": {"param1": "value1", "param2": "value2"}
  }
}

After receiving tool results, incorporate them naturally into your response.`, toolsDescription)
	} else {
		systemMessage = `You are a helpful assistant for Karmada cluster management. 

Note: Advanced cluster management tools are currently unavailable. 
Please provide general guidance about Karmada concepts, best practices, and configuration help.

If you need to check specific cluster resources, please ensure the karmada-mcp-server is properly configured and accessible.`
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemMessage,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: request.Prompt,
		},
	}

	resp, err := client.CreateChatCompletionStream(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    "kimi-k2-0711-preview",
			Messages: messages,
		},
	)
	if err != nil {
		klog.Errorf("Failed to create chat completion stream: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get response from LLM"})
		return
	}
	defer resp.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	var buffer strings.Builder

	for {
		response, err := resp.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			klog.Errorf("Error receiving stream response: %v", err)
			return
		}

		content := response.Choices[0].Delta.Content
		if content != "" {
			buffer.WriteString(content)

			// Check for tool call
			if strings.Contains(content, "}") {
				fullContent := buffer.String()
				if toolCall, ok := parseToolCall(fullContent); ok {
					// Send tool call notification
					toolCallMsg := StreamResponse{
						Type:    "tool_call",
						Content: toolCall,
					}
					if msg, err := json.Marshal(toolCallMsg); err == nil {
						fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
						c.Writer.Flush()
					}

					// Execute tool call
					result, err := executeToolCall(mcpManager, toolCall)
					if err != nil {
						klog.Errorf("Failed to execute tool call: %v", err)
						result = map[string]interface{}{
							"error": err.Error(),
						}
					}

					// Send tool result
					toolResultMsg := StreamResponse{
						Type:    "tool_result",
						Content: result,
					}
					if msg, err := json.Marshal(toolResultMsg); err == nil {
						fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
						c.Writer.Flush()
					}

					buffer.Reset()
					continue
				}
			}

			// Send regular content
			textMsg := StreamResponse{
				Type:    "text",
				Content: content,
			}
			if msg, err := json.Marshal(textMsg); err == nil {
				fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
				c.Writer.Flush()
			}
		}
	}

	// Send completion signal
	completionMsg := StreamResponse{
		Type:    "completion",
		Content: nil,
	}
	if msg, err := json.Marshal(completionMsg); err == nil {
		fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
		c.Writer.Flush()
	}
}

func buildToolsDescription(tools []mcp.Tool) string {
	var descriptions strings.Builder
	for _, tool := range tools {
		descriptions.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description))
		if tool.InputSchema.Properties != nil {
			for propName, prop := range tool.InputSchema.Properties {
				if propMap, ok := prop.(map[string]interface{}); ok {
					description := ""
					typeStr := ""
					if desc, ok := propMap["description"].(string); ok {
						description = desc
					}
					if typ, ok := propMap["type"].(string); ok {
						typeStr = typ
					}
					descriptions.WriteString(fmt.Sprintf("  - %s: %s (%s)\n", propName, description, typeStr))
				}
			}
		}
	}
	return descriptions.String()
}

func parseToolCall(content string) (map[string]interface{}, bool) {
	start := strings.Index(content, "{")
	if start == -1 {
		return nil, false
	}

	// Find the last complete JSON object
	braceCount := 0
	end := -1
	for i := start; i < len(content); i++ {
		if content[i] == '{' {
			braceCount++
		} else if content[i] == '}' {
			braceCount--
			if braceCount == 0 {
				end = i + 1
				break
			}
		}
	}

	if end == -1 {
		return nil, false
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(content[start:end]), &result); err != nil {
		return nil, false
	}

	if toolCall, ok := result["tool_call"].(map[string]interface{}); ok {
		return toolCall, true
	}

	return nil, false
}

func executeToolCall(mcpManager *MCPClientManager, toolCall map[string]interface{}) (map[string]interface{}, error) {
	server, _ := toolCall["server"].(string)
	tool, _ := toolCall["tool"].(string)
	arguments, _ := toolCall["arguments"].(map[string]interface{})

	if server == "" || tool == "" {
		return nil, fmt.Errorf("invalid tool call format")
	}

	// Default to karmada-mcp-server if server is not specified
	if server != "karmada-mcp-server" {
		server = "karmada-mcp-server"
	}

	result, err := mcpManager.ExecuteTool(context.Background(), server, tool, arguments)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool":    tool,
		"result":  result.Content,
		"isError": result.IsError,
	}, nil
}

// GetStatus returns the current MCP server status
func GetStatus(c *gin.Context) {
	mcpManager := GetMCPClientManager()

	// Ensure initialization is complete
	if err := mcpManager.WaitForInitialization(2 * time.Second); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "initializing",
			"error":  err.Error(),
		})
		return
	}

	status := mcpManager.GetStatus()
	c.JSON(http.StatusOK, status)
}
