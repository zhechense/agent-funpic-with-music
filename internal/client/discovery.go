package client

import (
	"fmt"

	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// ValidateTools checks if required tools are available on the server
func ValidateTools(available []types.Tool, required []string) error {
	toolMap := make(map[string]bool)
	for _, tool := range available {
		toolMap[tool.Name] = true
	}

	var missing []string
	for _, req := range required {
		if !toolMap[req] {
			missing = append(missing, req)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required tools: %v", missing)
	}

	return nil
}

// CreateClient creates an MCP client from server configuration
func CreateClient(config types.ServerConfig) (MCPClient, error) {
	var transport Transport

	switch config.Transport {
	case "stdio":
		if len(config.Command) == 0 {
			return nil, fmt.Errorf("command required for stdio transport")
		}
		transport = NewStdioTransport(config.Command, config.Timeout)

	case "http":
		if config.URL == "" {
			return nil, fmt.Errorf("url required for http transport")
		}
		// Use mark3labs/mcp-go library for reliable Streamable HTTP support
		transport = NewMark3LabsTransport(config.URL, config.Timeout, config.Headers)

	default:
		return nil, fmt.Errorf("unsupported transport type: %s", config.Transport)
	}

	return NewClient(transport), nil
}
