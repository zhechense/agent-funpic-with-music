package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/zhe.chen/agent-funpic-act/internal/llm"
)

// Conversation implements llm.Conversation for OpenRouter
type Conversation struct {
	provider    *Provider
	config      *llm.FullAIConversationConfig
	toolAdapter *llm.ToolAdapter
	messages    []openai.ChatCompletionMessage
	toolCalls   int
	tokensUsed  int
	startTime   time.Time
}

// NewConversation creates a new OpenRouter conversation
func NewConversation(provider *Provider, config *llm.FullAIConversationConfig) *Conversation {
	return &Conversation{
		provider:  provider,
		config:    config,
		messages:  make([]openai.ChatCompletionMessage, 0),
		startTime: time.Now(),
	}
}

// SetToolAdapter sets the tool adapter for MCP tool integration
func (c *Conversation) SetToolAdapter(adapter *llm.ToolAdapter) {
	c.toolAdapter = adapter
}

// Execute runs the conversation loop
func (c *Conversation) Execute(ctx context.Context, imagePath string, duration float64, userPrompt string) (string, error) {
	log.Printf("[OpenRouter] Starting conversation for image: %s (%.1fs)", imagePath, duration)
	log.Printf("[OpenRouter] User request: %s", userPrompt)

	// 1. Read and encode image
	imageBase64, _, err := llm.ReadAndEncodeImage(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	// 2. Discover tools
	tools, err := c.toolAdapter.DiscoverAndConvertTools(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to discover tools: %w", err)
	}

	// 3. Convert unified tools to OpenAI format (OpenRouter uses OpenAI-compatible format)
	openaiTools := c.convertToolsToOpenAIFormat(tools)

	// 4. Create system message
	toolsDesc := c.toolAdapter.GetToolDescription()
	systemPrompt := llm.CreateVideoGenerationPrompt(duration, imagePath, toolsDesc)
	c.messages = append(c.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt,
	})

	// 5. Create initial user message with image
	var initialPrompt string
	if userPrompt != "" {
		initialPrompt = fmt.Sprintf("%s\n\nGenerate a %.1f-second animated video for this image.", userPrompt, duration)
	} else {
		initialPrompt = fmt.Sprintf("Please generate a %.1f-second animated video for this image.", duration)
	}
	c.messages = append(c.messages, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{
			{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL: fmt.Sprintf("data:image/jpeg;base64,%s", imageBase64),
				},
			},
			{
				Type: openai.ChatMessagePartTypeText,
				Text: initialPrompt,
			},
		},
	})

	// 6. Conversation loop
	for round := 0; round < c.config.MaxRounds; round++ {
		log.Printf("[OpenRouter] Round %d/%d", round+1, c.config.MaxRounds)

		// Check timeout
		if time.Since(c.startTime).Seconds() > float64(c.config.TimeoutSeconds) {
			return "", fmt.Errorf("conversation timeout after %d seconds", c.config.TimeoutSeconds)
		}

		// Check token limit
		if c.tokensUsed > c.config.MaxTokens {
			return "", fmt.Errorf("exceeded token limit: %d", c.config.MaxTokens)
		}

		// Call OpenRouter API (using OpenAI-compatible client)
		resp, err := c.provider.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    c.provider.model,
			Messages: c.messages,
			Tools:    openaiTools,
		})

		if err != nil {
			return "", fmt.Errorf("OpenRouter API error at round %d: %w", round+1, err)
		}

		// Update metrics
		c.tokensUsed += resp.Usage.PromptTokens + resp.Usage.CompletionTokens
		log.Printf("[OpenRouter] Tokens: +%d input, +%d output (total: %d)",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, c.tokensUsed)

		// Process response
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no choices in response at round %d", round+1)
		}

		choice := resp.Choices[0]
		c.messages = append(c.messages, choice.Message)

		// Check for tool calls
		if len(choice.Message.ToolCalls) > 0 {
			log.Println("[OpenRouter] Tool calls requested")
			err := c.handleToolCalls(ctx, choice.Message.ToolCalls)
			if err != nil {
				log.Printf("[OpenRouter] Tool execution error: %v", err)
			}
			continue
		}

		// Check finish reason
		switch choice.FinishReason {
		case openai.FinishReasonStop:
			log.Println("[OpenRouter] Conversation completed")
			return choice.Message.Content, nil

		case openai.FinishReasonLength:
			return "", fmt.Errorf("hit max tokens at round %d", round+1)

		case openai.FinishReasonContentFilter:
			return "", fmt.Errorf("content filtered at round %d", round+1)

		default:
			// Continue conversation
			if choice.Message.Content != "" {
				return choice.Message.Content, nil
			}
		}
	}

	return "", fmt.Errorf("exceeded max rounds: %d", c.config.MaxRounds)
}

// handleToolCalls processes tool execution requests
func (c *Conversation) handleToolCalls(ctx context.Context, toolCalls []openai.ToolCall) error {
	var toolMessages []openai.ChatCompletionMessage

	for _, toolCall := range toolCalls {
		c.toolCalls++
		log.Printf("[OpenRouter] Tool Call #%d: %s", c.toolCalls, toolCall.Function.Name)

		// Parse arguments
		var inputMap map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &inputMap); err != nil {
			log.Printf("[OpenRouter] Warning: Invalid tool arguments: %v", err)
			inputMap = make(map[string]interface{})
		}

		// Execute tool
		result, err := c.toolAdapter.ExecuteToolCall(ctx, toolCall.Function.Name, inputMap)

		// Format result
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			log.Printf("[OpenRouter] Tool execution failed: %v", err)
		} else {
			log.Printf("[OpenRouter] Tool result: %d bytes", len(result))
		}

		// Add tool response message
		toolMessages = append(toolMessages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    result,
			ToolCallID: toolCall.ID,
		})
	}

	// Add all tool responses to conversation
	c.messages = append(c.messages, toolMessages...)
	return nil
}

// convertToolsToOpenAIFormat converts unified tools to OpenAI format
func (c *Conversation) convertToolsToOpenAIFormat(tools []llm.UnifiedTool) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}

	openaiTools := make([]openai.Tool, len(tools))
	for i, tool := range tools {
		// Create parameters as map
		params := map[string]interface{}{
			"type":       "object",
			"properties": tool.Parameters,
		}

		openaiTools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		}
	}

	return openaiTools
}

// GetMetrics returns conversation metrics
// Note: Cost is set to $0.00 as OpenRouter pricing varies by model
func (c *Conversation) GetMetrics() llm.FullAIConversationMetrics {
	duration := time.Since(c.startTime).Seconds()

	return llm.FullAIConversationMetrics{
		Rounds:     len(c.messages) / 2,
		ToolCalls:  c.toolCalls,
		TokensUsed: c.tokensUsed,
		Duration:   duration,
		CostUSD:    0.00, // OpenRouter pricing varies by model, not tracked
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
