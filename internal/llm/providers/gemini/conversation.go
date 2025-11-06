package gemini

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"google.golang.org/genai"
	"github.com/zhe.chen/agent-funpic-act/internal/llm"
)

// Conversation implements llm.Conversation for Gemini
type Conversation struct {
	provider    *Provider
	config      *llm.FullAIConversationConfig
	toolAdapter *llm.ToolAdapter
	chat        *genai.Chat
	toolCalls   int
	tokensUsed  int
	startTime   time.Time
}

// NewConversation creates a new Gemini conversation
func NewConversation(provider *Provider, config *llm.FullAIConversationConfig) *Conversation {
	return &Conversation{
		provider:  provider,
		config:    config,
		startTime: time.Now(),
	}
}

// SetToolAdapter sets the tool adapter for MCP tool integration
func (c *Conversation) SetToolAdapter(adapter *llm.ToolAdapter) {
	c.toolAdapter = adapter
}

// Execute runs the conversation loop
func (c *Conversation) Execute(ctx context.Context, imagePath string, duration float64, userPrompt string) (string, error) {
	log.Printf("[Gemini] Starting conversation for image: %s (%.1fs)", imagePath, duration)
	if userPrompt != "" {
		log.Printf("[Gemini] User request: %s", userPrompt)
	}

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

	// 3. Convert unified tools to Gemini format
	geminiTools := c.convertToolsToGeminiFormat(tools)

	// 4. Create system instruction
	toolsDesc := c.toolAdapter.GetToolDescription()
	systemPrompt := llm.CreateVideoGenerationPrompt(duration, imagePath, toolsDesc)

	// 5. Create chat configuration
	chatConfig := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(systemPrompt)},
		},
		Tools: geminiTools,
	}

	// 6. Create chat session with empty history
	// Use provider's default model if not specified in config
	model := c.config.Model
	if model == "" {
		model = c.provider.model
		log.Printf("[Gemini] Using provider's default model: %s", model)
	}

	var chatErr error
	c.chat, chatErr = c.provider.client.Chats.Create(ctx, model, chatConfig, []*genai.Content{})
	if chatErr != nil {
		return "", fmt.Errorf("failed to create chat: %w", chatErr)
	}

	// 7. Prepare initial message with image
	var initialPrompt string
	if userPrompt != "" {
		// User provided specific request
		initialPrompt = fmt.Sprintf("%s\n\nGenerate a %.1f-second animated video for this image.", userPrompt, duration)
	} else {
		// Default request
		initialPrompt = fmt.Sprintf("Please generate a %.1f-second animated video for this image.", duration)
	}

	imageData, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	initialParts := []genai.Part{
		*genai.NewPartFromBytes(imageData, mediaType),
		*genai.NewPartFromText(initialPrompt),
	}

	// 8. Conversation loop
	maxRounds := c.config.MaxRounds
	if maxRounds == 0 {
		maxRounds = 20
	}

	for round := 0; round < maxRounds; round++ {
		log.Printf("[Gemini] Round %d/%d", round+1, maxRounds)

		// Check timeout
		if time.Since(c.startTime).Seconds() > float64(c.config.TimeoutSeconds) {
			return "", fmt.Errorf("conversation timeout after %d seconds", c.config.TimeoutSeconds)
		}

		// Check token limit
		if c.tokensUsed > c.config.MaxTokens {
			return "", fmt.Errorf("exceeded token limit: %d", c.config.MaxTokens)
		}

		// Send message (only on first round)
		var resp *genai.GenerateContentResponse
		var err error

		if round == 0 {
			resp, err = c.chat.SendMessage(ctx, initialParts...)
			if err != nil {
				return "", fmt.Errorf("Gemini API error at round %d: %w", round+1, err)
			}
		} else {
			// Subsequent rounds will get response from handleToolCalls
			break
		}

		// Process responses in a loop (for handling multiple tool call rounds)
		for {
			// Update token usage
			if resp.UsageMetadata != nil {
				inputTokens := int(resp.UsageMetadata.PromptTokenCount)
				outputTokens := int(resp.UsageMetadata.CandidatesTokenCount)
				c.tokensUsed += inputTokens + outputTokens
				log.Printf("[Gemini] Tokens: +%d input, +%d output (total: %d)",
					inputTokens, outputTokens, c.tokensUsed)
			}

			// Check cost limit
			estimatedCost := float64(c.tokensUsed) * 0.000001
			if estimatedCost > c.config.MaxCostUSD {
				return "", fmt.Errorf("exceeded cost limit: $%.4f", estimatedCost)
			}

			// Check if we have a valid candidate
			if len(resp.Candidates) == 0 {
				return "", fmt.Errorf("no candidates in response")
			}

			candidate := resp.Candidates[0]

			// Check for tool calls
			hasToolCalls := false
			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					hasToolCalls = true
					break
				}
			}

			if hasToolCalls {
				// Execute tool calls and get Gemini's next response
				log.Println("[Gemini] Processing tool calls")
				nextResp, err := c.handleToolCalls(ctx, candidate.Content.Parts)
				if err != nil {
					log.Printf("[Gemini] Tool execution error: %v", err)
					return "", fmt.Errorf("tool execution failed: %w", err)
				}
				if nextResp == nil {
					return "", fmt.Errorf("no response after tool execution")
				}
				// Continue processing with the new response
				resp = nextResp
				continue
			}

			// No tool calls - extract final result
			result := c.extractTextFromParts(candidate.Content.Parts)
			if result != "" {
				log.Println("[Gemini] Conversation completed")
				return result, nil
			}

			// If we get here with no text and no tool calls, something is wrong
			return "", fmt.Errorf("no text or tool calls in response")
		}
	}

	return "", fmt.Errorf("exceeded max conversation rounds: %d", maxRounds)
}

// handleToolCalls processes tool calls from Gemini and sends results back
// Returns Gemini's response after seeing the function results
func (c *Conversation) handleToolCalls(ctx context.Context, parts []*genai.Part) (*genai.GenerateContentResponse, error) {
	var functionResponses []genai.Part

	for _, part := range parts {
		if part.FunctionCall != nil {
			c.toolCalls++
			toolName := part.FunctionCall.Name
			log.Printf("[Gemini] Tool Call #%d: %s", c.toolCalls, toolName)

			// Convert args to map
			inputMap := make(map[string]interface{})
			for k, v := range part.FunctionCall.Args {
				inputMap[k] = v
			}

			// Execute tool
			result, err := c.toolAdapter.ExecuteToolCall(ctx, toolName, inputMap)

			// Create function response
			var response genai.Part
			if err != nil {
				log.Printf("[Gemini] Tool execution failed: %v", err)
				response = *genai.NewPartFromFunctionResponse(toolName, map[string]interface{}{
					"error":  err.Error(),
					"result": result,
				})
			} else {
				log.Printf("[Gemini] Tool result: %d bytes", len(result))
				response = *genai.NewPartFromFunctionResponse(toolName, map[string]interface{}{
					"result": result,
				})
			}

			functionResponses = append(functionResponses, response)
		}
	}

	// Send all function responses back to Gemini and get its response
	if len(functionResponses) > 0 {
		resp, err := c.chat.SendMessage(ctx, functionResponses...)
		if err != nil {
			return nil, fmt.Errorf("failed to send function responses: %w", err)
		}
		return resp, nil
	}

	return nil, nil
}

// extractTextFromParts extracts text from Gemini response parts
func (c *Conversation) extractTextFromParts(parts []*genai.Part) string {
	var result string
	for _, part := range parts {
		if part.Text != "" {
			result += part.Text
		}
	}
	return result
}

// convertToolsToGeminiFormat converts unified tools to Gemini format
func (c *Conversation) convertToolsToGeminiFormat(tools []llm.UnifiedTool) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	declarations := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		// Convert JSON Schema to Gemini Schema
		schema := c.convertJSONSchemaToGemini(tool.Parameters)

		declarations = append(declarations, &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  schema,
		})
	}

	return []*genai.Tool{
		{
			FunctionDeclarations: declarations,
		},
	}
}

// convertJSONSchemaToGemini converts JSON Schema map to Gemini Schema
func (c *Conversation) convertJSONSchemaToGemini(params map[string]interface{}) *genai.Schema {
	if params == nil || len(params) == 0 {
		// Return a valid empty object schema
		return &genai.Schema{
			Type: genai.TypeObject,
		}
	}

	schema := &genai.Schema{
		Type: genai.TypeObject,
	}

	// Extract properties if they exist
	if props, ok := params["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)

		for propName, propValue := range props {
			if propDef, ok := propValue.(map[string]interface{}); ok {
				schema.Properties[propName] = c.convertPropertySchema(propDef)
			}
		}
	}

	// Extract required fields if they exist
	if required, ok := params["required"].([]interface{}); ok {
		schema.Required = make([]string, 0, len(required))
		for _, req := range required {
			if reqStr, ok := req.(string); ok {
				schema.Required = append(schema.Required, reqStr)
			}
		}
	}

	return schema
}

// convertPropertySchema converts a single property's JSON Schema to Gemini Schema
func (c *Conversation) convertPropertySchema(propDef map[string]interface{}) *genai.Schema {
	propSchema := &genai.Schema{}

	// Convert type
	if typeStr, ok := propDef["type"].(string); ok {
		propSchema.Type = c.mapJSONTypeToGemini(typeStr)
	}

	// Convert description
	if desc, ok := propDef["description"].(string); ok {
		propSchema.Description = desc
	}

	// Convert enum
	if enumVals, ok := propDef["enum"].([]interface{}); ok {
		propSchema.Enum = make([]string, 0, len(enumVals))
		for _, val := range enumVals {
			if strVal, ok := val.(string); ok {
				propSchema.Enum = append(propSchema.Enum, strVal)
			}
		}
	}

	// Handle array items
	if propSchema.Type == genai.TypeArray {
		if items, ok := propDef["items"].(map[string]interface{}); ok {
			propSchema.Items = c.convertPropertySchema(items)
		}
	}

	// Handle nested objects
	if propSchema.Type == genai.TypeObject {
		if props, ok := propDef["properties"].(map[string]interface{}); ok {
			propSchema.Properties = make(map[string]*genai.Schema)
			for nestedName, nestedDef := range props {
				if nestedDefMap, ok := nestedDef.(map[string]interface{}); ok {
					propSchema.Properties[nestedName] = c.convertPropertySchema(nestedDefMap)
				}
			}
		}
	}

	return propSchema
}

// mapJSONTypeToGemini maps JSON Schema types to Gemini Schema types
func (c *Conversation) mapJSONTypeToGemini(jsonType string) genai.Type {
	switch jsonType {
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	case "object":
		return genai.TypeObject
	default:
		return genai.TypeString // Default to string
	}
}

// GetMetrics returns conversation metrics
func (c *Conversation) GetMetrics() llm.FullAIConversationMetrics {
	duration := time.Since(c.startTime).Seconds()
	costUSD := float64(c.tokensUsed) * 0.000001 // Approximate Gemini pricing

	return llm.FullAIConversationMetrics{
		Rounds:     1, // Simplified for now
		ToolCalls:  c.toolCalls,
		TokensUsed: c.tokensUsed,
		Duration:   duration,
		CostUSD:    costUSD,
	}
}

// GetState returns current state (for debugging)
func (c *Conversation) GetState() interface{} {
	return map[string]interface{}{
		"tool_calls":  c.toolCalls,
		"tokens_used": c.tokensUsed,
	}
}
