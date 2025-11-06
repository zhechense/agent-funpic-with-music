package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// ConversationState tracks the AI conversation state
type ConversationState struct {
	Messages      []anthropic.MessageParam
	ToolCallCount int
	TokensUsed    int
	StartTime     time.Time
	ImagePath     string
	Duration      float64
}

// ConversationConfig controls conversation limits
type ConversationConfig struct {
	MaxRounds      int     // Maximum conversation rounds
	MaxTokens      int     // Maximum total tokens
	MaxCostUSD     float64 // Maximum cost in USD
	TimeoutSeconds int     // Global timeout
	Model          string  // Claude model name
}

// ConversationMetrics tracks conversation performance
type ConversationMetrics struct {
	Rounds     int
	ToolCalls  int
	TokensUsed int
	Duration   time.Duration
	CostUSD    float64
}

// ConversationManager manages multi-turn conversations with Claude
type ConversationManager struct {
	client      *ClaudeClient
	toolAdapter *ToolAdapter
	state       *ConversationState
	config      *ConversationConfig
}

// NewConversationManager creates a new conversation manager
func NewConversationManager(
	client *ClaudeClient,
	toolAdapter *ToolAdapter,
	config *ConversationConfig,
) *ConversationManager {
	return &ConversationManager{
		client:      client,
		toolAdapter: toolAdapter,
		config:      config,
		state: &ConversationState{
			Messages:  make([]anthropic.MessageParam, 0),
			StartTime: time.Now(),
		},
	}
}

// Execute runs the conversation loop
func (m *ConversationManager) Execute(ctx context.Context, imagePath string, duration float64, userPrompt string) (string, error) {
	log.Printf("[AI Agent] Starting conversation for image: %s (%.1fs)", imagePath, duration)
	// TODO: Integrate userPrompt into Claude conversation

	m.state.ImagePath = imagePath
	m.state.Duration = duration

	// 1. Read and encode image
	imageBase64, mediaType, err := ReadAndEncodeImage(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	// 2. Discover available tools
	unifiedTools, err := m.toolAdapter.DiscoverAndConvertTools(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to discover tools: %w", err)
	}

	log.Printf("[AI Agent] Found %d tools available", len(unifiedTools))

	// Convert unified tools to Claude format
	claudeTools := m.convertToolsToClaudeFormat(unifiedTools)

	// 3. Create system prompt
	toolsDesc := m.toolAdapter.GetToolDescription()
	systemPrompt := CreateVideoGenerationPrompt(duration, imagePath, toolsDesc)

	// 4. Create initial user message with image
	var initialPrompt string
	if userPrompt != "" {
		initialPrompt = fmt.Sprintf("%s\n\nGenerate a %.1f-second animated video for this image.", userPrompt, duration)
	} else {
		initialPrompt = fmt.Sprintf("Please generate a %.1f-second animated video for this image.", duration)
	}
	initialMessage := CreateVisionMessage(imageBase64, mediaType, initialPrompt)
	m.state.Messages = append(m.state.Messages, initialMessage)

	// 5. Conversation loop
	for round := 0; round < m.config.MaxRounds; round++ {
		log.Printf("[AI Agent] Round %d/%d", round+1, m.config.MaxRounds)

		// Check timeout
		if time.Since(m.state.StartTime).Seconds() > float64(m.config.TimeoutSeconds) {
			return "", fmt.Errorf("conversation timeout after %d seconds", m.config.TimeoutSeconds)
		}

		// Check token limit
		if m.state.TokensUsed > m.config.MaxTokens {
			return "", fmt.Errorf("exceeded token limit: %d", m.config.MaxTokens)
		}

		// Call Claude
		response, err := m.client.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(m.config.Model),
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: systemPrompt},
			},
			Messages: m.state.Messages,
			Tools:    claudeTools,
		})

		if err != nil {
			return "", fmt.Errorf("Claude API error at round %d: %w", round+1, err)
		}

		// Update token usage
		inputTokens := int(response.Usage.InputTokens)
		outputTokens := int(response.Usage.OutputTokens)
		m.state.TokensUsed += inputTokens + outputTokens

		log.Printf("[AI Agent] Tokens: +%d input, +%d output (total: %d)",
			inputTokens, outputTokens, m.state.TokensUsed)

		// Check cost limit
		estimatedCost := float64(m.state.TokensUsed) * 0.000003 // Rough estimate
		if estimatedCost > m.config.MaxCostUSD {
			return "", fmt.Errorf("exceeded cost limit: $%.4f", estimatedCost)
		}

		// Add assistant's response to messages
		assistantBlocks := m.convertContentBlocks(response.Content)
		m.state.Messages = append(m.state.Messages, anthropic.NewAssistantMessage(assistantBlocks...))

		// Process response based on stop reason
		switch response.StopReason {
		case "tool_use":
			// Claude requests tool execution
			log.Println("[AI Agent] Claude requested tool execution")
			err := m.handleToolUse(ctx, response)
			if err != nil {
				log.Printf("[AI Agent] Tool execution error: %v", err)
				// Continue conversation to let Claude handle the error
			}
			continue

		case "end_turn":
			// Claude finished the task
			log.Println("[AI Agent] Claude completed the task")
			result := m.extractFinalResult(response)
			return result, nil

		case "max_tokens":
			return "", fmt.Errorf("hit max tokens per request at round %d", round+1)

		case "stop_sequence":
			log.Println("[AI Agent] Claude used stop sequence")
			result := m.extractFinalResult(response)
			return result, nil

		default:
			return "", fmt.Errorf("unexpected stop reason at round %d: %s", round+1, response.StopReason)
		}
	}

	return "", fmt.Errorf("exceeded max conversation rounds: %d", m.config.MaxRounds)
}

// handleToolUse processes tool use requests from Claude
func (m *ConversationManager) handleToolUse(ctx context.Context, response *anthropic.Message) error {
	var toolResultBlocks []anthropic.ContentBlockParamUnion

	for _, content := range response.Content {
		if content.Type == "tool_use" {
			m.state.ToolCallCount++

			log.Printf("[AI Agent] Tool Call #%d: %s", m.state.ToolCallCount, content.Name)

			// Convert input to map
			var inputMap map[string]interface{}
			if err := json.Unmarshal(content.Input, &inputMap); err != nil {
				log.Printf("[AI Agent] Warning: Invalid tool input format: %v", err)
				inputMap = make(map[string]interface{})
			}

			// Execute tool
			result, err := m.toolAdapter.ExecuteToolCall(ctx, content.Name, inputMap)

			// Format result
			isError := err != nil
			if isError {
				result = fmt.Sprintf("Error: %v", err)
				log.Printf("[AI Agent] Tool execution failed: %v", err)
			} else {
				log.Printf("[AI Agent] Tool result: %d bytes", len(result))
			}

			// Create tool result block
			toolResultBlocks = append(toolResultBlocks, anthropic.NewToolResultBlock(content.ID, result, isError))
		}
	}

	// Add all tool results as a single user message
	if len(toolResultBlocks) > 0 {
		m.state.Messages = append(m.state.Messages, anthropic.NewUserMessage(toolResultBlocks...))
	}
	return nil
}

// extractFinalResult extracts the final result from Claude's response
func (m *ConversationManager) extractFinalResult(response *anthropic.Message) string {
	var result string

	for _, content := range response.Content {
		if content.Type == "text" {
			result += content.Text
		}
	}

	if result == "" {
		result = "Task completed (no text output)"
	}

	return result
}

// GetMetrics returns conversation metrics
func (m *ConversationManager) GetMetrics() ConversationMetrics {
	duration := time.Since(m.state.StartTime)

	// Rough cost estimate: $3 per 1M tokens for Claude 3.5 Sonnet
	costUSD := float64(m.state.TokensUsed) * 0.000003

	// Calculate rounds (each user+assistant pair is one round)
	rounds := len(m.state.Messages) / 2

	return ConversationMetrics{
		Rounds:     rounds,
		ToolCalls:  m.state.ToolCallCount,
		TokensUsed: m.state.TokensUsed,
		Duration:   duration,
		CostUSD:    costUSD,
	}
}

// GetState returns the current conversation state (for debugging)
func (m *ConversationManager) GetState() *ConversationState {
	return m.state
}

// convertContentBlocks converts ContentBlockUnion to ContentBlockParamUnion
func (m *ConversationManager) convertContentBlocks(blocks []anthropic.ContentBlockUnion) []anthropic.ContentBlockParamUnion {
	result := make([]anthropic.ContentBlockParamUnion, len(blocks))
	for i, block := range blocks {
		switch block.Type {
		case "text":
			result[i] = anthropic.NewTextBlock(block.Text)
		case "thinking":
			result[i] = anthropic.NewThinkingBlock(block.Signature, block.Thinking)
		case "tool_use":
			result[i] = anthropic.NewToolUseBlock(block.ID, block.Input, block.Name)
		default:
			// Fallback: create text block with type info
			result[i] = anthropic.NewTextBlock(fmt.Sprintf("[%s block]", block.Type))
		}
	}
	return result
}

// convertToolsToClaudeFormat converts unified tools to Claude-specific format
func (m *ConversationManager) convertToolsToClaudeFormat(tools []UnifiedTool) []anthropic.ToolUnionParam {
	claudeTools := make([]anthropic.ToolUnionParam, len(tools))
	for i, tool := range tools {
		inputSchema := anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: tool.Parameters,
		}
		claudeTools[i] = anthropic.ToolUnionParamOfTool(inputSchema, tool.Name)
		// Set description if available
		if tool.Description != "" {
			claudeTools[i].OfTool.Description = anthropic.String(tool.Description)
		}
	}
	return claudeTools
}
