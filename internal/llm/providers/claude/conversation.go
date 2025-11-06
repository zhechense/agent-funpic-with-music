package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/zhe.chen/agent-funpic-act/internal/llm"
)

// Conversation implements llm.Conversation for Claude
type Conversation struct {
	provider    *Provider
	config      *llm.FullAIConversationConfig
	toolAdapter *llm.ToolAdapter
	messages    []anthropic.MessageParam
	toolCalls   int
	tokensUsed  int
	startTime   time.Time
}

// NewConversation creates a new Claude conversation
func NewConversation(provider *Provider, config *llm.FullAIConversationConfig) *Conversation {
	return &Conversation{
		provider:   provider,
		config:     config,
		messages:   make([]anthropic.MessageParam, 0),
		startTime:  time.Now(),
	}
}

// SetToolAdapter sets the tool adapter for MCP tool integration
func (c *Conversation) SetToolAdapter(adapter *llm.ToolAdapter) {
	c.toolAdapter = adapter
}

// Execute runs the conversation loop
func (c *Conversation) Execute(ctx context.Context, imagePath string, duration float64, userPrompt string) (string, error) {
	log.Printf("[Claude] Starting conversation for image: %s (%.1fs)", imagePath, duration)
	// TODO: Integrate userPrompt into Claude conversation

	// 1. Read and encode image
	imageBase64, mediaType, err := llm.ReadAndEncodeImage(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	// 2. Discover tools
	tools, err := c.toolAdapter.DiscoverAndConvertTools(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to discover tools: %w", err)
	}

	// 3. Convert unified tools to Claude format
	claudeTools := c.convertToolsToClaudeFormat(tools)

	// 4. Create system prompt
	toolsDesc := c.toolAdapter.GetToolDescription()
	systemPrompt := llm.CreateVideoGenerationPrompt(duration, imagePath, toolsDesc)

	// 5. Create initial message
	var initialPrompt string
	if userPrompt != "" {
		initialPrompt = fmt.Sprintf("%s\n\nGenerate a %.1f-second animated video for this image.", userPrompt, duration)
	} else {
		initialPrompt = fmt.Sprintf("Please generate a %.1f-second animated video for this image.", duration)
	}
	initialMessage := anthropic.NewUserMessage(
		anthropic.NewImageBlockBase64(mediaType, imageBase64),
		anthropic.NewTextBlock(initialPrompt),
	)
	c.messages = append(c.messages, initialMessage)

	// 6. Conversation loop
	for round := 0; round < c.config.MaxRounds; round++ {
		log.Printf("[Claude] Round %d/%d", round+1, c.config.MaxRounds)

		// Check timeout
		if time.Since(c.startTime).Seconds() > float64(c.config.TimeoutSeconds) {
			return "", fmt.Errorf("conversation timeout after %d seconds", c.config.TimeoutSeconds)
		}

		// Check token limit
		if c.tokensUsed > c.config.MaxTokens {
			return "", fmt.Errorf("exceeded token limit: %d", c.config.MaxTokens)
		}

		// Call Claude API
		response, err := c.provider.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(c.config.Model),
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: systemPrompt},
			},
			Messages: c.messages,
			Tools:    claudeTools,
		})

		if err != nil {
			return "", fmt.Errorf("Claude API error at round %d: %w", round+1, err)
		}

		// Update metrics
		c.tokensUsed += int(response.Usage.InputTokens + response.Usage.OutputTokens)
		log.Printf("[Claude] Tokens: +%d input, +%d output (total: %d)",
			response.Usage.InputTokens, response.Usage.OutputTokens, c.tokensUsed)

		// Check cost limit
		estimatedCost := float64(c.tokensUsed) * 0.000003
		if estimatedCost > c.config.MaxCostUSD {
			return "", fmt.Errorf("exceeded cost limit: $%.4f", estimatedCost)
		}

		// Add assistant response
		assistantBlocks := c.convertContentBlocks(response.Content)
		c.messages = append(c.messages, anthropic.NewAssistantMessage(assistantBlocks...))

		// Handle stop reason
		switch response.StopReason {
		case "tool_use":
			log.Println("[Claude] Tool use requested")
			err := c.handleToolUse(ctx, response)
			if err != nil {
				log.Printf("[Claude] Tool execution error: %v", err)
			}
			continue

		case "end_turn":
			log.Println("[Claude] Conversation completed")
			return c.extractFinalResult(response), nil

		case "max_tokens":
			return "", fmt.Errorf("hit max tokens at round %d", round+1)

		case "stop_sequence":
			log.Println("[Claude] Stop sequence detected")
			return c.extractFinalResult(response), nil

		default:
			return "", fmt.Errorf("unexpected stop reason: %s", response.StopReason)
		}
	}

	return "", fmt.Errorf("exceeded max rounds: %d", c.config.MaxRounds)
}

// handleToolUse processes tool execution requests
func (c *Conversation) handleToolUse(ctx context.Context, response *anthropic.Message) error {
	var toolResultBlocks []anthropic.ContentBlockParamUnion

	for _, content := range response.Content {
		if content.Type == "tool_use" {
			c.toolCalls++

			log.Printf("[Claude] Tool Call #%d: %s", c.toolCalls, content.Name)

			// Execute tool
			var inputMap map[string]interface{}
			if err := json.Unmarshal(content.Input, &inputMap); err != nil {
				log.Printf("[Claude] Warning: Invalid tool input format: %v", err)
				inputMap = make(map[string]interface{})
			}

			result, err := c.toolAdapter.ExecuteToolCall(ctx, content.Name, inputMap)

			isError := err != nil
			if isError {
				result = fmt.Sprintf("Error: %v", err)
				log.Printf("[Claude] Tool execution failed: %v", err)
			} else {
				log.Printf("[Claude] Tool result: %d bytes", len(result))
			}

			// Add result
			toolResultBlocks = append(toolResultBlocks,
				anthropic.NewToolResultBlock(content.ID, result, isError))
		}
	}

	// Add all tool results
	if len(toolResultBlocks) > 0 {
		c.messages = append(c.messages, anthropic.NewUserMessage(toolResultBlocks...))
	}

	return nil
}

// extractFinalResult extracts text from Claude's response
func (c *Conversation) extractFinalResult(response *anthropic.Message) string {
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

// convertContentBlocks converts ContentBlockUnion to ContentBlockParamUnion
func (c *Conversation) convertContentBlocks(blocks []anthropic.ContentBlockUnion) []anthropic.ContentBlockParamUnion {
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

// convertToolsToClaudeFormat converts unified tools to Claude format
func (c *Conversation) convertToolsToClaudeFormat(tools []llm.UnifiedTool) []anthropic.ToolUnionParam {
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

// GetMetrics returns conversation metrics
func (c *Conversation) GetMetrics() llm.FullAIConversationMetrics {
	duration := time.Since(c.startTime).Seconds()
	costUSD := float64(c.tokensUsed) * 0.000003 // $3 per 1M tokens

	return llm.FullAIConversationMetrics{
		Rounds:     len(c.messages) / 2,
		ToolCalls:  c.toolCalls,
		TokensUsed: c.tokensUsed,
		Duration:   duration,
		CostUSD:    costUSD,
	}
}

// GetState returns current state (for debugging)
func (c *Conversation) GetState() interface{} {
	return map[string]interface{}{
		"messages":    len(c.messages),
		"tool_calls":  c.toolCalls,
		"tokens_used": c.tokensUsed,
	}
}
