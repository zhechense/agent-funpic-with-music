package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// Mark3LabsTransport wraps mark3labs/mcp-go client to implement our Transport interface
type Mark3LabsTransport struct {
	url         string
	timeout     time.Duration
	headers     map[string]string
	httpTrans   *transport.StreamableHTTP
	mcpClient   *client.Client
	initialized bool
}

// NewMark3LabsTransport creates a transport using mark3labs/mcp-go library
func NewMark3LabsTransport(url string, timeout time.Duration, headers map[string]string) *Mark3LabsTransport {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Mark3LabsTransport{
		url:     url,
		timeout: timeout,
		headers: headers,
	}
}

// Start initializes the transport
func (t *Mark3LabsTransport) Start(ctx context.Context) error {
	// Create Streamable HTTP transport with headers
	httpTransport, err := transport.NewStreamableHTTP(
		t.url,
		transport.WithContinuousListening(),
		transport.WithHTTPHeaders(t.headers),
	)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	t.httpTrans = httpTransport

	// Create MCP client
	t.mcpClient = client.NewClient(httpTransport)

	// Start the client
	if err := t.mcpClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start client: %w", err)
	}

	return nil
}

// SendRequest sends a JSON-RPC request and waits for response
func (t *Mark3LabsTransport) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	// Handle initialize specially
	if method == "initialize" {
		initParams, ok := params.(InitializeRequest)
		if !ok {
			return nil, fmt.Errorf("invalid initialize params type")
		}

		initRequest := mcp.InitializeRequest{
			Params: mcp.InitializeParams{
				ProtocolVersion: initParams.ProtocolVersion,
				Capabilities:    mcp.ClientCapabilities{},
				ClientInfo: mcp.Implementation{
					Name:    initParams.ClientInfo.Name,
					Version: initParams.ClientInfo.Version,
				},
			},
		}

		initResult, err := t.mcpClient.Initialize(ctx, initRequest)
		if err != nil {
			return nil, fmt.Errorf("initialize failed: %w", err)
		}

		t.initialized = true

		// Convert InitializeResult to our format
		response := InitializeResponse{
			ProtocolVersion: initResult.ProtocolVersion,
			ServerInfo: ServerInfo{
				Name:    initResult.ServerInfo.Name,
				Version: initResult.ServerInfo.Version,
			},
		}

		return json.Marshal(response)
	}

	// Handle tools/list
	if method == "tools/list" {
		if !t.initialized {
			return nil, fmt.Errorf("client not initialized")
		}

		toolsRequest := mcp.ListToolsRequest{}
		toolsResult, err := t.mcpClient.ListTools(ctx, toolsRequest)
		if err != nil {
			return nil, fmt.Errorf("list tools failed: %w", err)
		}

		// Convert to our format
		var tools []types.Tool
		for _, tool := range toolsResult.Tools {
			var schema map[string]interface{}
			// Marshal and unmarshal to convert ToolInputSchema to map
			if schemaBytes, err := json.Marshal(tool.InputSchema); err == nil {
				json.Unmarshal(schemaBytes, &schema)
			}

			tools = append(tools, types.Tool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: schema,
			})
		}

		response := ToolsListResponse{
			Tools: tools,
		}

		return json.Marshal(response)
	}

	// Handle tools/call
	if method == "tools/call" {
		if !t.initialized {
			return nil, fmt.Errorf("client not initialized")
		}

		callParams, ok := params.(CallToolRequest)
		if !ok {
			return nil, fmt.Errorf("invalid tools/call params type")
		}

		callRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      callParams.Name,
				Arguments: callParams.Arguments,
			},
		}

		result, err := t.mcpClient.CallTool(ctx, callRequest)
		if err != nil {
			return nil, fmt.Errorf("call tool failed: %w", err)
		}

		return json.Marshal(result)
	}

	return nil, fmt.Errorf("unsupported method: %s", method)
}

// SendNotification sends a JSON-RPC notification
func (t *Mark3LabsTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	// mark3labs client handles initialized notification internally
	return nil
}

// Close shuts down the transport
func (t *Mark3LabsTransport) Close() error {
	if t.mcpClient != nil {
		return t.mcpClient.Close()
	}
	return nil
}
