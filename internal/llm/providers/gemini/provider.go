package gemini

import (
	"context"
	"time"

	"google.golang.org/genai"
	"github.com/zhe.chen/agent-funpic-act/internal/llm"
	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// Provider implements llm.Provider for Google Gemini
type Provider struct {
	client  *genai.Client
	model   string
	timeout time.Duration
	enabled bool
}

// NewProvider creates a new Gemini provider
func NewProvider(config types.GoogleConfig) (*Provider, error) {
	if config.APIKey == "" {
		return &Provider{enabled: false}, nil
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: config.APIKey,
	})
	if err != nil {
		return nil, err
	}

	return &Provider{
		client:  client,
		model:   config.Model,
		timeout: config.Timeout,
		enabled: true,
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "gemini"
}

// IsEnabled returns whether the provider is configured
func (p *Provider) IsEnabled() bool {
	return p.enabled
}

// CreateConversation creates a new conversation session
func (p *Provider) CreateConversation(config *llm.FullAIConversationConfig) (llm.Conversation, error) {
	return NewConversation(p, config), nil
}
