package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// StdioTransport implements Transport interface using stdio
type StdioTransport struct {
	command []string
	timeout time.Duration

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// Request tracking
	nextID      int
	pendingReqs map[int]chan *JSONRPCResponse
	mu          sync.Mutex

	// Background reader
	readerCtx    context.Context
	readerCancel context.CancelFunc
	readerDone   chan struct{}
}

// NewStdioTransport creates a stdio transport
func NewStdioTransport(command []string, timeout time.Duration) *StdioTransport {
	return &StdioTransport{
		command:     command,
		timeout:     timeout,
		pendingReqs: make(map[int]chan *JSONRPCResponse),
		nextID:      1,
		readerDone:  make(chan struct{}),
	}
}

// Start launches the subprocess and starts reading
func (t *StdioTransport) Start(ctx context.Context) error {
	if len(t.command) == 0 {
		return fmt.Errorf("command cannot be empty")
	}

	// Create command
	t.cmd = exec.CommandContext(ctx, t.command[0], t.command[1:]...)

	// Setup pipes
	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	t.stderr, err = t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start process
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Start background reader
	t.readerCtx, t.readerCancel = context.WithCancel(context.Background())
	go t.readLoop()
	go t.logStderr()

	return nil
}

// SendRequest sends a JSON-RPC request and waits for response
func (t *StdioTransport) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	t.mu.Lock()
	id := t.nextID
	t.nextID++
	respChan := make(chan *JSONRPCResponse, 1)
	t.pendingReqs[id] = respChan
	t.mu.Unlock()

	// Cleanup on exit
	defer func() {
		t.mu.Lock()
		delete(t.pendingReqs, id)
		t.mu.Unlock()
	}()

	// Build request
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Serialize and send
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	data = append(data, '\n')
	if _, err := t.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("request timeout: %w", timeoutCtx.Err())
	case <-t.readerDone:
		return nil, fmt.Errorf("transport closed")
	}
}

// SendNotification sends a JSON-RPC notification (no response)
func (t *StdioTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	data = append(data, '\n')
	if _, err := t.stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}

// Close shuts down the transport
func (t *StdioTransport) Close() error {
	// Cancel reader
	if t.readerCancel != nil {
		t.readerCancel()
	}

	// Close stdin to signal process to exit
	if t.stdin != nil {
		t.stdin.Close()
	}

	// Wait for process with timeout
	if t.cmd != nil && t.cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- t.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited
		case <-time.After(5 * time.Second):
			// Force kill
			t.cmd.Process.Kill()
		}
	}

	// Wait for reader to finish
	select {
	case <-t.readerDone:
	case <-time.After(1 * time.Second):
	}

	return nil
}

// readLoop continuously reads JSON-RPC responses from stdout
func (t *StdioTransport) readLoop() {
	defer close(t.readerDone)

	scanner := bufio.NewScanner(t.stdout)
	// Increase buffer size for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-t.readerCtx.Done():
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			// Invalid JSON, skip
			continue
		}

		// Route to pending request
		t.mu.Lock()
		if ch, ok := t.pendingReqs[resp.ID]; ok {
			ch <- &resp
		}
		t.mu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		// Log error (could add structured logging here)
		fmt.Printf("Error reading stdout: %v\n", err)
	}
}

// logStderr reads and logs stderr output
func (t *StdioTransport) logStderr() {
	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		// Could integrate with structured logging
		fmt.Printf("[SERVER STDERR] %s\n", scanner.Text())
	}
}
