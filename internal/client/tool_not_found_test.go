package client

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestToolNotFound verifies proper handling of tool not found errors
func TestToolNotFound(t *testing.T) {
	tests := []struct {
		name         string
		toolName     string
		errorCode    int
		errorMessage string
		expectError  bool
	}{
		{
			name:         "tool not found",
			toolName:     "nonexistent_tool",
			errorCode:    -32000,
			errorMessage: "Tool not found",
			expectError:  true,
		},
		{
			name:         "method not found",
			toolName:     "invalid_method",
			errorCode:    -32601,
			errorMessage: "Method not found",
			expectError:  true,
		},
		{
			name:         "internal server error",
			toolName:     "failing_tool",
			errorCode:    -32603,
			errorMessage: "Internal error",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock transport
			mockTransport := NewMockTransport()

			// Configure initialization responses
			mockTransport.SetResponse("initialize", map[string]interface{}{
				"protocolVersion": "2025-03-26",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			})

			// Create client
			client := NewClient(mockTransport)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Connect and initialize
			if err := client.Connect(ctx); err != nil {
				t.Fatalf("Connect failed: %v", err)
			}

			if err := client.Initialize(ctx); err != nil {
				t.Fatalf("Initialize failed: %v", err)
			}

			// Set error response for tool call AFTER Initialize
			mockTransport.RequestErr = &JSONRPCError{
				Code:    tt.errorCode,
				Message: tt.errorMessage,
			}

			// Attempt to call the tool
			result, err := client.CallTool(ctx, tt.toolName, map[string]interface{}{
				"param": "value",
			})

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}

				// Verify error is JSONRPCError with correct code (unwrap wrapped error)
				var jsonRPCErr *JSONRPCError
				if !errors.As(err, &jsonRPCErr) {
					t.Fatalf("Expected JSONRPCError, got %T: %v", err, err)
				}

				if jsonRPCErr.Code != tt.errorCode {
					t.Errorf("Expected error code %d, got %d", tt.errorCode, jsonRPCErr.Code)
				}

				if jsonRPCErr.Message != tt.errorMessage {
					t.Errorf("Expected error message %q, got %q", tt.errorMessage, jsonRPCErr.Message)
				}

				if result != nil {
					t.Error("Expected nil result for error case")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestValidateTools verifies tool validation logic
func TestValidateTools(t *testing.T) {
	mockTransport := NewMockTransport()

	// Configure responses
	mockTransport.SetResponse("initialize", map[string]interface{}{
		"protocolVersion": "2025-03-26",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": true,
			},
		},
		"serverInfo": map[string]interface{}{
			"name":    "test-server",
			"version": "1.0.0",
		},
	})

	mockTransport.SetResponse("tools/list", map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "toolA",
				"description": "Tool A",
				"inputSchema": map[string]interface{}{
					"type": "object",
				},
			},
			{
				"name":        "toolB",
				"description": "Tool B",
				"inputSchema": map[string]interface{}{
					"type": "object",
				},
			},
		},
	})

	client := NewClient(mockTransport)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect and initialize
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// List tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(tools) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(tools))
	}

	// Test validation with available tools
	err = ValidateTools(tools, []string{"toolA", "toolB"})
	if err != nil {
		t.Errorf("Validation failed for available tools: %v", err)
	}

	// Test validation with missing tool
	err = ValidateTools(tools, []string{"toolA", "toolC"})
	if err == nil {
		t.Error("Expected validation error for missing tool")
	}
}

// TestToolNotFoundAfterList verifies behavior when tool exists in list but fails on call
func TestToolNotFoundAfterList(t *testing.T) {
	mockTransport := NewMockTransport()

	// Tool appears in list
	mockTransport.SetResponse("initialize", map[string]interface{}{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]interface{}{},
		"serverInfo": map[string]interface{}{
			"name":    "test-server",
			"version": "1.0.0",
		},
	})

	mockTransport.SetResponse("tools/list", map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "disappearing_tool",
				"description": "Tool that disappears",
				"inputSchema": map[string]interface{}{},
			},
		},
	})

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

	// Tool exists in list
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "disappearing_tool" {
		t.Fatal("Tool not found in list")
	}

	// But calling it fails with tool not found
	mockTransport.RequestErr = &JSONRPCError{
		Code:    -32000,
		Message: "Tool not found",
	}

	_, err = client.CallTool(ctx, "disappearing_tool", nil)
	if err == nil {
		t.Fatal("Expected tool not found error")
	}

	var jsonRPCErr *JSONRPCError
	if !errors.As(err, &jsonRPCErr) {
		t.Fatalf("Expected JSONRPCError, got %T: %v", err, err)
	}
	if jsonRPCErr.Code != -32000 {
		t.Errorf("Expected error code -32000, got %d", jsonRPCErr.Code)
	}
}
