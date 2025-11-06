package llm

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ClaudeClient wraps the Anthropic Claude API for pipeline decision making
type ClaudeClient struct {
	client  anthropic.Client
	model   string
	timeout time.Duration
	enabled bool
}

// NewClaudeClient creates a new Claude API client
func NewClaudeClient(apiKey string, model string, timeout time.Duration) *ClaudeClient {
	if apiKey == "" {
		log.Println("[LLM] No API key provided, LLM features disabled")
		return &ClaudeClient{enabled: false}
	}

	return &ClaudeClient{
		client:  anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:   model,
		timeout: timeout,
		enabled: true,
	}
}

// IsEnabled returns whether LLM features are enabled
func (c *ClaudeClient) IsEnabled() bool {
	return c.enabled
}

// AnalyzeImage uses Claude to analyze the image and make pipeline decisions
// NOTE: This simplified version returns default decisions
// Vision API integration can be added later
func (c *ClaudeClient) AnalyzeImage(ctx context.Context, imagePath string) (*PipelineDecision, *LLMAnalysis, error) {
	if !c.enabled {
		return GetDefaultDecision(), nil, fmt.Errorf("LLM is disabled")
	}

	// Set timeout context
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	log.Printf("[LLM] Analyzing image: %s (using simplified mode)", imagePath)

	// TODO: Add Vision API integration
	// For now, return smart defaults based on typical use cases
	decision := &PipelineDecision{
		NeedSegment:      true,
		NeedLandmarks:    true,
		EnableMotion:     true,
		NeedMusic:        true,
		ImageDescription: "Image analysis (Vision API integration pending)",
		MusicMood:        "happy",
		MusicGenres:      []string{"pop", "electronic"},
		MusicCount:       5,
		Parameters: map[string]interface{}{
			"detect_confidence":    0.3,
			"landmark_confidence":  0.3,
			"motion_intensity":     1.0,
			"music_search_timeout": 30,
		},
		ErrorRecovery: map[string]string{
			"segment_person":     "use_original",
			"estimate_landmarks": "skip",
			"render_motion":      "static_image",
			"search_music":       "continue_without_music",
			"compose":            "fail",
		},
	}

	analysis := &LLMAnalysis{
		Decision:   decision,
		Model:      c.model,
		TokensUsed: 0, // Vision API not yet integrated
		ReasoningSteps: []string{
			"Using default configuration (Vision API integration pending)",
			"All stages enabled for maximum output quality",
		},
		ConfidenceScores: map[string]float64{
			"overall": 0.8,
		},
	}

	log.Printf("[LLM] Analysis complete (simplified mode)")
	return decision, analysis, nil
}
