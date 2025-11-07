package openrouter

import (
	"net/http"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/zhe.chen/agent-funpic-act/internal/llm"
	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

const (
	// OpenRouter API base URL
	openRouterBaseURL = "https://openrouter.ai/api/v1"

	// OpenRouter headers
	httpReferer = "https://github.com/zhe.chen/agent-funpic-act"
	appTitle    = "agent-funpic-act"
)

// Provider implements llm.Provider for OpenRouter
type Provider struct {
	client  *openai.Client
	model   string
	timeout time.Duration
	enabled bool
}

// NewProvider creates a new OpenRouter provider
// OpenRouter uses OpenAI-compatible API with custom base URL
func NewProvider(config types.OpenRouterConfig) (*Provider, error) {
	if config.APIKey == "" {
		return &Provider{enabled: false}, nil
	}

	// Create OpenAI client config with OpenRouter base URL
	clientConfig := openai.DefaultConfig(config.APIKey)
	clientConfig.BaseURL = openRouterBaseURL

	// Create custom HTTP client with OpenRouter-specific headers
	customClient := &http.Client{
		Transport: &headerTransport{
			Base: http.DefaultTransport,
			Headers: map[string]string{
				"HTTP-Referer": httpReferer,
				"X-Title":      appTitle,
			},
		},
	}
	clientConfig.HTTPClient = customClient

	return &Provider{
		client:  openai.NewClientWithConfig(clientConfig),
		model:   config.Model,
		timeout: config.Timeout,
		enabled: true,
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "openrouter"
}

// IsEnabled returns whether the provider is configured
func (p *Provider) IsEnabled() bool {
	return p.enabled
}

// CreateConversation creates a new conversation session
func (p *Provider) CreateConversation(config *llm.FullAIConversationConfig) (llm.Conversation, error) {
	return NewConversation(p, config), nil
}

// headerTransport adds custom headers to HTTP requests
type headerTransport struct {
	Base    http.RoundTripper
	Headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add custom headers to the request
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}

	// Delegate to base transport
	return t.Base.RoundTrip(req)
}
