package llm

import (
	"context"
	"fmt"
	"log"

	"github.com/zhe.chen/agent-funpic-act/internal/client"
	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// ToolAdapter converts MCP tools to unified format for use with any LLM provider
type ToolAdapter struct {
	mcpClients map[string]client.MCPClient // server_name -> client
	toolsCache []UnifiedTool               // cached unified tool definitions
}

// NewToolAdapter creates a new tool adapter
func NewToolAdapter(clients map[string]client.MCPClient) *ToolAdapter {
	return &ToolAdapter{
		mcpClients: clients,
	}
}

// DiscoverAndConvertTools discovers all MCP tools and converts them to unified format
func (a *ToolAdapter) DiscoverAndConvertTools(ctx context.Context) ([]UnifiedTool, error) {
	if a.toolsCache != nil {
		return a.toolsCache, nil
	}

	var unifiedTools []UnifiedTool

	// Discover tools from each MCP server
	for serverName, mcpClient := range a.mcpClients {
		log.Printf("[Tool Adapter] Discovering tools from %s...", serverName)

		// List available tools
		tools, err := mcpClient.ListTools(ctx)
		if err != nil {
			log.Printf("[Tool Adapter] Warning: Failed to list tools from %s: %v", serverName, err)
			continue
		}

		log.Printf("[Tool Adapter] Found %d tools from %s", len(tools), serverName)

		// Convert each MCP tool to unified format
		for _, tool := range tools {
			unifiedTool := a.convertMCPToolToUnified(serverName, tool)
			unifiedTools = append(unifiedTools, unifiedTool)
		}
	}

	a.toolsCache = unifiedTools
	log.Printf("[Tool Adapter] Total tools available: %d", len(unifiedTools))
	return unifiedTools, nil
}

// convertMCPToolToUnified converts a single MCP tool to unified format
func (a *ToolAdapter) convertMCPToolToUnified(serverName string, tool types.Tool) UnifiedTool {
	// Prefix tool name with server name to avoid conflicts
	toolName := fmt.Sprintf("%s__%s", serverName, tool.Name)

	// Add server context to description
	description := fmt.Sprintf("[%s] %s", serverName, tool.Description)

	return UnifiedTool{
		Name:        toolName,
		Description: description,
		Parameters:  tool.InputSchema,
	}
}

// ExecuteToolCall executes a Claude tool call by routing to the appropriate MCP client
func (a *ToolAdapter) ExecuteToolCall(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error) {
	// Parse tool name: "server__tool"
	serverName, mcpToolName, err := a.parseToolName(toolName)
	if err != nil {
		return "", err
	}

	// Get MCP client
	mcpClient, ok := a.mcpClients[serverName]
	if !ok {
		return "", fmt.Errorf("MCP server %s not found", serverName)
	}

	log.Printf("[Tool Adapter] Executing %s.%s", serverName, mcpToolName)

	// Call MCP tool
	result, err := mcpClient.CallTool(ctx, mcpToolName, arguments)
	if err != nil {
		return "", fmt.Errorf("MCP tool %s failed: %w", toolName, err)
	}

	// Check for errors in result
	if result.IsError {
		if len(result.Content) > 0 {
			return "", fmt.Errorf("tool execution error: %s", result.Content[0].Text)
		}
		return "", fmt.Errorf("tool execution error (no details)")
	}

	// Extract result text
	if len(result.Content) == 0 {
		return "", fmt.Errorf("tool returned no content")
	}

	// Combine all content blocks
	var resultText string
	for _, block := range result.Content {
		if block.Type == "text" {
			resultText += block.Text
		}
	}

	log.Printf("[Tool Adapter] Tool result: %d bytes", len(resultText))
	return resultText, nil
}

// parseToolName splits "server__tool" into ("server", "tool")
func (a *ToolAdapter) parseToolName(toolName string) (string, string, error) {
	// Find "__" separator
	for i := 0; i < len(toolName)-1; i++ {
		if toolName[i] == '_' && toolName[i+1] == '_' {
			return toolName[:i], toolName[i+2:], nil
		}
	}
	return "", "", fmt.Errorf("invalid tool name format: %s (expected: server__tool)", toolName)
}

// GetToolDescription returns a human-readable description of all available tools
func (a *ToolAdapter) GetToolDescription() string {
	if a.toolsCache == nil {
		return "No tools available"
	}

	desc := fmt.Sprintf("You have access to %d tools from MCP servers:\n\n", len(a.toolsCache))

	// Group by server
	servers := make(map[string][]string)
	for _, tool := range a.toolsCache {
		serverName, mcpName, _ := a.parseToolName(tool.Name)
		servers[serverName] = append(servers[serverName], mcpName)
	}

	// Format description
	for server, tools := range servers {
		desc += fmt.Sprintf("**%s**:\n", server)
		for _, tool := range tools {
			desc += fmt.Sprintf("  - %s\n", tool)
		}
		desc += "\n"
	}

	return desc
}
