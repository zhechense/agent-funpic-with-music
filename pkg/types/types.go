package types

import "time"

// Config represents the application configuration
type Config struct {
	Servers  map[string]ServerConfig `yaml:"servers"`
	Pipeline PipelineConfig          `yaml:"pipeline"`
	LLM      LLMConfig               `yaml:"llm"`
}

// ServerConfig defines MCP server connection parameters
type ServerConfig struct {
	Name         string            `yaml:"name"`
	Command      []string          `yaml:"command"`           // For stdio transport
	URL          string            `yaml:"url"`               // For HTTP transport
	Transport    string            `yaml:"transport"`         // "stdio" or "http"
	Timeout      time.Duration     `yaml:"timeout"`
	Headers      map[string]string `yaml:"headers,omitempty"` // HTTP headers (e.g., Authorization)
	Capabilities struct {
		Tools []string `yaml:"tools"`
	} `yaml:"capabilities"`
}

// PipelineConfig defines pipeline execution parameters
type PipelineConfig struct {
	EnableMotion bool   `yaml:"enable_motion"`
	MaxRetries   int    `yaml:"max_retries"`
	ManifestPath string `yaml:"manifest_path"`
}

// LLMConfig defines LLM/AI Agent configuration
type LLMConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Provider string        `yaml:"provider"` // "anthropic", "google", "openai"
	Mode     string        `yaml:"mode"`     // "lightweight" or "full_ai"
	FullAI FullAIConfig `yaml:"full_ai"`

	// Provider-specific configurations
	Anthropic AnthropicConfig `yaml:"anthropic"`
	Google    GoogleConfig    `yaml:"google"`
	OpenAI    OpenAIConfig    `yaml:"openai"`
}

// FullAIConfig defines limits for full AI agent mode
type FullAIConfig struct {
	MaxRounds      int     `yaml:"max_rounds"`       // Max conversation rounds
	MaxTokens      int     `yaml:"max_tokens"`       // Max total tokens
	MaxCostUSD     float64 `yaml:"max_cost_usd"`     // Max cost in USD
	TimeoutSeconds int     `yaml:"timeout_seconds"`  // Global timeout
}

// AnthropicConfig for Claude
type AnthropicConfig struct {
	APIKey  string        `yaml:"api_key"`
	Model   string        `yaml:"model"`   // e.g., "claude-3-5-sonnet-20241022"
	Timeout time.Duration `yaml:"timeout"`
}

// GoogleConfig for Gemini
type GoogleConfig struct {
	APIKey  string        `yaml:"api_key"`
	Model   string        `yaml:"model"`   // e.g., "gemini-2.0-flash-exp"
	Project string        `yaml:"project"` // GCP project ID (optional, for Vertex AI)
	Timeout time.Duration `yaml:"timeout"`
}

// OpenAIConfig for GPT models
type OpenAIConfig struct {
	APIKey       string        `yaml:"api_key"`
	Model        string        `yaml:"model"`        // e.g., "gpt-4o"
	Organization string        `yaml:"organization"` // Optional
	Timeout      time.Duration `yaml:"timeout"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolCallResult represents the result of a tool invocation
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError"`
}

// ContentBlock represents a content item in tool result
type ContentBlock struct {
	Type string `json:"type"` // "text", "image", "resource"
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
	URI  string `json:"uri,omitempty"`
}

// PipelineInput contains the initial pipeline parameters
type PipelineInput struct {
	ImagePath  string
	Duration   float64 // Target duration in seconds
	UserPrompt string  // User's request (e.g., "make a shake animation")
	OutputDir  string  // Output directory for final result files
	TempDir    string  // Temporary directory for intermediate files
}

// PipelineStage represents a stage in the execution pipeline
type PipelineStage string

const (
	StageInit           PipelineStage = "init"
	StageSegmentPerson  PipelineStage = "segment_person"
	StageLandmarks      PipelineStage = "estimate_landmarks"
	StageRenderMotion   PipelineStage = "render_motion"
	StageSearchMusic    PipelineStage = "search_music"
	StageCompose        PipelineStage = "compose"
	StageComplete       PipelineStage = "complete"
)

// StageStatus represents the execution status of a stage
type StageStatus string

const (
	StatusPending   StageStatus = "pending"
	StatusRunning   StageStatus = "running"
	StatusCompleted StageStatus = "completed"
	StatusFailed    StageStatus = "failed"
	StatusSkipped   StageStatus = "skipped"
)
