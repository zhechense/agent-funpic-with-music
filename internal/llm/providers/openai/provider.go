package openai

import (
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/zhe.chen/agent-funpic-act/internal/llm"
	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// Provider implements llm.Provider for OpenAI
type Provider struct {
	client  *openai.Client
	model   string
	timeout time.Duration
	enabled bool
}

// NewProvider creates a new OpenAI provider
func NewProvider(config types.OpenAIConfig) (*Provider, error) {
	if config.APIKey == "" {
		return &Provider{enabled: false}, nil
	}

	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.Organization != "" {
		clientConfig.OrgID = config.Organization
	}

	return &Provider{
		client:  openai.NewClientWithConfig(clientConfig),
		model:   config.Model,
		timeout: config.Timeout,
		enabled: true,
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "openai"
}

// IsEnabled returns whether the provider is configured
func (p *Provider) IsEnabled() bool {
	return p.enabled
}

// CreateConversation creates a new conversation session
func (p *Provider) CreateConversation(config *llm.FullAIConversationConfig) (llm.Conversation, error) {
	return NewConversation(p, config), nil
}
