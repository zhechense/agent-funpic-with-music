package client

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// TestToolExecutionError verifies handling of tool execution errors (isError=true)
func TestToolExecutionError(t *testing.T) {
	tests := []struct {
		name          string
		toolResponse  map[string]interface{}
		expectError   bool
		errorContains string
	}{
		{
			name: "tool execution error",
			toolResponse: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Invalid input: file not found",
					},
				},
				"isError": true,
			},
			expectError:   true,
			errorContains: "tool execution failed",
		},
		{
			name: "tool success",
			toolResponse: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Operation completed successfully",
					},
				},
				"isError": false,
			},
			expectError: false,
		},
		{
			name: "tool error with multiple content blocks",
			toolResponse: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Validation failed: invalid format",
					},
					{
						"type": "text",
						"text": "Additional error details",
					},
				},
				"isError": true,
			},
			expectError:   true,
			errorContains: "Validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTransport := NewMockTransport()

			// Configure initialization
			mockTransport.SetResponse("initialize", map[string]interface{}{
				"protocolVersion": "2025-03-26",
				"capabilities":    map[string]interface{}{},
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			})

			// Set tool response
			mockTransport.SetResponse("tools/call", tt.toolResponse)

			// Create client
			client := NewClient(mockTransport)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Initialize
			if err := client.Connect(ctx); err != nil {
				t.Fatalf("Connect failed: %v", err)
			}
			if err := client.Initialize(ctx); err != nil {
				t.Fatalf("Initialize failed: %v", err)
			}

			// Call tool
			result, err := client.CallTool(ctx, "test_tool", map[string]interface{}{
				"input": "test",
			})

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}

				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}

				// Result should still be returned even with error
				if result == nil {
					t.Error("Expected result to be returned even with isError=true")
				} else if !result.IsError {
					t.Error("Expected result.IsError to be true")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("Expected non-nil result")
				}
				if result.IsError {
					t.Error("Expected result.IsError to be false")
				}
			}
		})
	}
}

// TestToolErrorTypes verifies handling of different error content types
func TestToolErrorTypes(t *testing.T) {
	tests := []struct {
		name    string
		content []types.ContentBlock
	}{
		{
			name: "text error",
			content: []types.ContentBlock{
				{Type: "text", Text: "Error message"},
			},
		},
		{
			name: "resource error",
			content: []types.ContentBlock{
				{Type: "resource", URI: "file:///error.log"},
			},
		},
		{
			name: "multiple blocks",
			content: []types.ContentBlock{
				{Type: "text", Text: "Primary error"},
				{Type: "text", Text: "Stack trace"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTransport := NewMockTransport()

			mockTransport.SetResponse("initialize", map[string]interface{}{
				"protocolVersion": "2025-03-26",
				"capabilities":    map[string]interface{}{},
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			})

			// Convert content blocks to response format
			contentSlice := make([]map[string]interface{}, len(tt.content))
			for i, block := range tt.content {
				contentSlice[i] = map[string]interface{}{
					"type": block.Type,
					"text": block.Text,
					"uri":  block.URI,
				}
			}

			mockTransport.SetResponse("tools/call", map[string]interface{}{
				"content": contentSlice,
				"isError": true,
			})

			client := NewClient(mockTransport)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := client.Connect(ctx); err != nil {
				t.Fatalf("Connect failed: %v", err)
			}
			if err := client.Initialize(ctx); err != nil {
				t.Fatalf("Initialize failed: %v", err)
			}

			result, err := client.CallTool(ctx, "test_tool", nil)
			if err == nil {
				t.Fatal("Expected error for isError=true")
			}

			if result == nil {
				t.Fatal("Expected result to be returned")
			}

			if len(result.Content) != len(tt.content) {
				t.Errorf("Expected %d content blocks, got %d", len(tt.content), len(result.Content))
			}
		})
	}
}

// TestInvalidParameterError verifies handling of invalid parameters
func TestInvalidParameterError(t *testing.T) {
	mockTransport := NewMockTransport()

	mockTransport.SetResponse("initialize", map[string]interface{}{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]interface{}{},
		"serverInfo": map[string]interface{}{
			"name":    "test-server",
			"version": "1.0.0",
		},
	})

	client := NewClient(mockTransport)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Set JSON-RPC error for invalid parameters AFTER Initialize
	mockTransport.RequestErr = &JSONRPCError{
		Code:    -32602,
		Message: "Invalid params",
		Data:    "Required parameter 'file_path' is missing",
	}

	// Call with invalid parameters
	_, err := client.CallTool(ctx, "process_file", map[string]interface{}{
		"wrong_param": "value",
	})

	if err == nil {
		t.Fatal("Expected error for invalid parameters")
	}

	var jsonRPCErr *JSONRPCError
	if !errors.As(err, &jsonRPCErr) {
		t.Fatalf("Expected JSONRPCError, got %T: %v", err, err)
	}

	if jsonRPCErr.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", jsonRPCErr.Code)
	}
}

// TestToolErrorRecovery verifies error recovery and retry behavior
func TestToolErrorRecovery(t *testing.T) {
	mockTransport := NewMockTransport()

	mockTransport.SetResponse("initialize", map[string]interface{}{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]interface{}{},
		"serverInfo": map[string]interface{}{
			"name":    "test-server",
			"version": "1.0.0",
		},
	})

	client := NewClient(mockTransport)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// First call fails
	mockTransport.SetToolExecutionError("tools/call")
	_, err := client.CallTool(ctx, "flaky_tool", nil)
	if err == nil {
		t.Fatal("Expected first call to fail")
	}

	// Second call succeeds
	mockTransport.RequestErr = nil
	mockTransport.SetResponse("tools/call", map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": "Success on retry",
			},
		},
		"isError": false,
	})

	result, err := client.CallTool(ctx, "flaky_tool", nil)
	if err != nil {
		t.Fatalf("Expected second call to succeed, got error: %v", err)
	}

	if result.IsError {
		t.Error("Expected success result on retry")
	}
}
