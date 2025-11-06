package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// MCPClient defines the interface for interacting with MCP servers
type MCPClient interface {
	// Connect establishes connection to the MCP server
	Connect(ctx context.Context) error

	// Initialize performs MCP protocol initialization handshake
	Initialize(ctx context.Context) error

	// ListTools retrieves available tools from the server
	ListTools(ctx context.Context) ([]types.Tool, error)

	// CallTool invokes a tool with given arguments
	CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*types.ToolCallResult, error)

	// Close terminates the connection
	Close() error

	// GetServerInfo returns server name and version
	GetServerInfo() (name, version string)
}

// Transport defines the interface for MCP transport layers
type Transport interface {
	// Start initializes the transport
	Start(ctx context.Context) error

	// SendRequest sends a JSON-RPC request and waits for response
	SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error)

	// SendNotification sends a JSON-RPC notification (no response expected)
	SendNotification(ctx context.Context, method string, params interface{}) error

	// Close shuts down the transport
	Close() error
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// InitializeRequest represents MCP initialize request parameters
type InitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

// ClientInfo represents client identification
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResponse represents MCP initialize response
type InitializeResponse struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    ServerCapabilities     `json:"capabilities"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
}

// ServerCapabilities represents server capabilities
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourceCapability  `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourceCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type LoggingCapability struct{}

// ServerInfo represents server identification
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolsListResponse represents response from tools/list
type ToolsListResponse struct {
	Tools      []types.Tool `json:"tools"`
	NextCursor *string      `json:"nextCursor,omitempty"`
}

// CallToolRequest represents parameters for tools/call
type CallToolRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// Client implements MCPClient interface
type Client struct {
	transport  Transport
	serverName string
	serverVer  string
	nextID     int
}

// NewClient creates a new MCP client with the given transport
func NewClient(transport Transport) *Client {
	return &Client{
		transport: transport,
		nextID:    1,
	}
}

// Connect establishes connection to the MCP server
func (c *Client) Connect(ctx context.Context) error {
	return c.transport.Start(ctx)
}

// Initialize performs MCP protocol initialization
func (c *Client) Initialize(ctx context.Context) error {
	// Send initialize request
	initReq := InitializeRequest{
		ProtocolVersion: "2025-03-26",
		Capabilities: map[string]interface{}{
			"roots": map[string]interface{}{
				"listChanged": false,
			},
		},
		ClientInfo: ClientInfo{
			Name:    "agent-funpic-act",
			Version: "1.0.0",
		},
	}

	resultBytes, err := c.transport.SendRequest(ctx, "initialize", initReq)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	var initResp InitializeResponse
	if err := json.Unmarshal(resultBytes, &initResp); err != nil {
		return fmt.Errorf("failed to parse initialize response: %w", err)
	}

	c.serverName = initResp.ServerInfo.Name
	c.serverVer = initResp.ServerInfo.Version

	// Send initialized notification
	if err := c.transport.SendNotification(ctx, "notifications/initialized", nil); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	return nil
}

// ListTools retrieves available tools from the server
func (c *Client) ListTools(ctx context.Context) ([]types.Tool, error) {
	resultBytes, err := c.transport.SendRequest(ctx, "tools/list", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("tools/list request failed: %w", err)
	}

	var listResp ToolsListResponse
	if err := json.Unmarshal(resultBytes, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse tools/list response: %w", err)
	}

	return listResp.Tools, nil
}

// CallTool invokes a tool with given arguments
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*types.ToolCallResult, error) {
	req := CallToolRequest{
		Name:      name,
		Arguments: arguments,
	}

	resultBytes, err := c.transport.SendRequest(ctx, "tools/call", req)
	if err != nil {
		return nil, fmt.Errorf("tools/call request failed: %w", err)
	}

	var result types.ToolCallResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools/call response: %w", err)
	}

	// Check if the tool returned an error
	if result.IsError {
		return &result, fmt.Errorf("tool execution failed: %s", result.Content[0].Text)
	}

	return &result, nil
}

// Close terminates the connection
func (c *Client) Close() error {
	return c.transport.Close()
}

// GetServerInfo returns server name and version
func (c *Client) GetServerInfo() (name, version string) {
	return c.serverName, c.serverVer
}
