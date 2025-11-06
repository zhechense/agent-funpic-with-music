package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// MockTransport is a mock implementation of Transport for testing
type MockTransport struct {
	// Behavior configuration
	StartErr         error
	RequestErr       error
	NotificationErr  error
	ResponseDelay    time.Duration
	RequestResponses map[string]interface{} // method -> response

	// State tracking
	Started       bool
	Closed        bool
	SentRequests  []MockRequest
	Notifications []MockNotification
}

// MockRequest records a request sent through the transport
type MockRequest struct {
	Method string
	Params interface{}
}

// MockNotification records a notification sent through the transport
type MockNotification struct {
	Method string
	Params interface{}
}

// NewMockTransport creates a new mock transport
func NewMockTransport() *MockTransport {
	return &MockTransport{
		RequestResponses: make(map[string]interface{}),
		SentRequests:     []MockRequest{},
		Notifications:    []MockNotification{},
	}
}

// Start initializes the mock transport
func (m *MockTransport) Start(ctx context.Context) error {
	if m.StartErr != nil {
		return m.StartErr
	}
	m.Started = true
	return nil
}

// SendRequest sends a mock request and returns configured response
func (m *MockTransport) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	// Record the request
	m.SentRequests = append(m.SentRequests, MockRequest{
		Method: method,
		Params: params,
	})

	// Simulate delay if configured
	if m.ResponseDelay > 0 {
		select {
		case <-time.After(m.ResponseDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Return configured error if set
	if m.RequestErr != nil {
		return nil, m.RequestErr
	}

	// Return configured response
	if resp, ok := m.RequestResponses[method]; ok {
		data, err := json.Marshal(resp)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal mock response: %w", err)
		}
		return data, nil
	}

	// Default empty response
	return json.RawMessage(`{}`), nil
}

// SendNotification sends a mock notification
func (m *MockTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	// Record the notification
	m.Notifications = append(m.Notifications, MockNotification{
		Method: method,
		Params: params,
	})

	if m.NotificationErr != nil {
		return m.NotificationErr
	}

	return nil
}

// Close shuts down the mock transport
func (m *MockTransport) Close() error {
	m.Closed = true
	return nil
}

// SetResponse configures a response for a specific method
func (m *MockTransport) SetResponse(method string, response interface{}) {
	m.RequestResponses[method] = response
}

// SetToolNotFoundError configures transport to return tool not found error
func (m *MockTransport) SetToolNotFoundError() {
	m.RequestErr = &JSONRPCError{
		Code:    -32000,
		Message: "Tool not found",
	}
}

// SetToolExecutionError configures a tool result with isError=true
func (m *MockTransport) SetToolExecutionError(method string) {
	m.SetResponse(method, map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": "Tool execution failed: invalid input",
			},
		},
		"isError": true,
	})
}

// SetTimeout configures transport to simulate timeout
func (m *MockTransport) SetTimeout(delay time.Duration) {
	m.ResponseDelay = delay
}

// GetRequestCount returns the number of requests sent
func (m *MockTransport) GetRequestCount() int {
	return len(m.SentRequests)
}

// GetNotificationCount returns the number of notifications sent
func (m *MockTransport) GetNotificationCount() int {
	return len(m.Notifications)
}

// GetLastRequest returns the most recent request
func (m *MockTransport) GetLastRequest() *MockRequest {
	if len(m.SentRequests) == 0 {
		return nil
	}
	return &m.SentRequests[len(m.SentRequests)-1]
}

// Reset clears all recorded state
func (m *MockTransport) Reset() {
	m.Started = false
	m.Closed = false
	m.SentRequests = []MockRequest{}
	m.Notifications = []MockNotification{}
	m.StartErr = nil
	m.RequestErr = nil
	m.NotificationErr = nil
	m.ResponseDelay = 0
}
