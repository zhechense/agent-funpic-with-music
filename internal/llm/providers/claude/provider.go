package claude

import (
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/zhe.chen/agent-funpic-act/internal/llm"
	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// Provider implements llm.Provider for Anthropic Claude
type Provider struct {
	client  anthropic.Client
	model   string
	timeout time.Duration
	enabled bool
}

// NewProvider creates a new Claude provider
func NewProvider(config types.AnthropicConfig) (*Provider, error) {
	if config.APIKey == "" {
		return &Provider{enabled: false}, nil
	}

	return &Provider{
		client:  anthropic.NewClient(option.WithAPIKey(config.APIKey)),
		model:   config.Model,
		timeout: config.Timeout,
		enabled: true,
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "anthropic"
}

// IsEnabled returns whether the provider is configured
func (p *Provider) IsEnabled() bool {
	return p.enabled
}

// CreateConversation creates a new conversation session
func (p *Provider) CreateConversation(config *llm.FullAIConversationConfig) (llm.Conversation, error) {
	return NewConversation(p, config), nil
}
