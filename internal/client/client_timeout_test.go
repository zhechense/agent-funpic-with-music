package client

import (
	"context"
	"testing"
	"time"
)

// TestClientTimeout verifies that client requests timeout correctly
func TestClientTimeout(t *testing.T) {
	tests := []struct {
		name          string
		contextTimeout time.Duration
		responseDelay time.Duration
		expectTimeout bool
	}{
		{
			name:          "request completes before timeout",
			contextTimeout: 2 * time.Second,
			responseDelay: 100 * time.Millisecond,
			expectTimeout: false,
		},
		{
			name:          "request times out",
			contextTimeout: 100 * time.Millisecond,
			responseDelay: 2 * time.Second,
			expectTimeout: true,
		},
		{
			name:          "immediate timeout",
			contextTimeout: 1 * time.Millisecond,
			responseDelay: 1 * time.Second,
			expectTimeout: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock transport with configured delay
			mockTransport := NewMockTransport()
			mockTransport.SetTimeout(tt.responseDelay)
			mockTransport.SetResponse("initialize", map[string]interface{}{
				"protocolVersion": "2025-03-26",
				"capabilities":    map[string]interface{}{},
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			})
			mockTransport.SetResponse("tools/call", map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "success",
					},
				},
				"isError": false,
			})

			// Create client
			client := NewClient(mockTransport)

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			// Connect and initialize
			if err := client.Connect(ctx); err != nil {
				t.Fatalf("Connect failed: %v", err)
			}

			if err := client.Initialize(ctx); err != nil {
				if tt.expectTimeout {
					// Expected timeout during initialization
					if ctx.Err() == context.DeadlineExceeded {
						return
					}
				}
				t.Fatalf("Initialize failed: %v", err)
			}

			// Call tool
			_, err := client.CallTool(ctx, "testTool", map[string]interface{}{
				"arg": "value",
			})

			if tt.expectTimeout {
				if err == nil {
					t.Fatal("Expected timeout error, got nil")
				}
				if ctx.Err() != context.DeadlineExceeded {
					t.Fatalf("Expected context deadline exceeded, got: %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestClientCancellation verifies that cancelled contexts stop requests
func TestClientCancellation(t *testing.T) {
	mockTransport := NewMockTransport()
	mockTransport.SetTimeout(5 * time.Second) // Long delay
	mockTransport.SetResponse("tools/call", map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": "success"},
		},
		"isError": false,
	})

	client := NewClient(mockTransport)

	ctx, cancel := context.WithCancel(context.Background())

	// Start request in background
	errChan := make(chan error, 1)
	go func() {
		_, err := client.CallTool(ctx, "testTool", nil)
		errChan <- err
	}()

	// Cancel context after short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for error
	err := <-errChan
	if err == nil {
		t.Fatal("Expected error from cancelled context, got nil")
	}

	if ctx.Err() != context.Canceled {
		t.Fatalf("Expected context cancelled error, got: %v", err)
	}
}
