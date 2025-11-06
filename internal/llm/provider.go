package llm

import (
	"context"
)

// Provider abstracts different LLM providers (Claude, Gemini, OpenAI)
type Provider interface {
	// Name returns the provider name
	Name() string

	// CreateConversation starts a new conversation session
	CreateConversation(config *FullAIConversationConfig) (Conversation, error)

	// IsEnabled returns whether the provider is configured with valid credentials
	IsEnabled() bool
}

// Conversation manages a multi-turn conversation with tool calling support
type Conversation interface {
	// SetToolAdapter sets the tool adapter for MCP tool integration
	SetToolAdapter(adapter *ToolAdapter)

	// Execute runs the conversation loop with vision input and tool access
	// userPrompt: Optional user request (e.g., "make a shake animation")
	Execute(ctx context.Context, imagePath string, duration float64, userPrompt string) (string, error)

	// GetMetrics returns conversation performance metrics
	GetMetrics() FullAIConversationMetrics

	// GetState returns the current conversation state (for debugging)
	GetState() interface{}
}

// FullAIConversationConfig controls conversation limits for full AI mode
type FullAIConversationConfig struct {
	MaxRounds      int     // Maximum conversation rounds
	MaxTokens      int     // Maximum total tokens
	MaxCostUSD     float64 // Maximum cost in USD
	TimeoutSeconds int     // Global timeout
	Model          string  // Model name (provider-specific)
}

// FullAIConversationMetrics tracks conversation performance for full AI mode
type FullAIConversationMetrics struct {
	Rounds     int
	ToolCalls  int
	TokensUsed int
	Duration   float64 // seconds
	CostUSD    float64
}

// NewProvider factory has been moved to cmd/agent/main.go to avoid import cycles.
// Each provider package (providers/claude, providers/gemini, providers/openai)
// exports a NewProvider function that main.go calls directly.
