package llm

// PipelineDecision represents LLM's decision on how to execute the pipeline
type PipelineDecision struct {
	// Stage execution decisions
	NeedSegment   bool `json:"need_segment"`    // Whether to perform background removal
	NeedLandmarks bool `json:"need_landmarks"`  // Whether to perform pose estimation
	EnableMotion  bool `json:"enable_motion"`   // Whether to apply animation
	NeedMusic     bool `json:"need_music"`      // Whether to search and add music

	// Dynamic parameters for each stage
	Parameters map[string]interface{} `json:"parameters"` // Stage-specific parameters

	// Error recovery strategies
	ErrorRecovery map[string]string `json:"error_recovery"` // Stage -> recovery action

	// Content understanding
	ImageDescription string   `json:"image_description"` // LLM's description of image content
	MusicMood        string   `json:"music_mood"`        // Suggested music mood (happy, calm, energetic, etc.)
	MusicGenres      []string `json:"music_genres"`      // Suggested music genres
	MusicCount       int      `json:"music_count"`       // Number of music tracks to search
}

// LLMAnalysis stores the complete LLM analysis result for the pipeline
type LLMAnalysis struct {
	// Decision made by LLM
	Decision *PipelineDecision `json:"decision"`

	// Reasoning steps
	ReasoningSteps []string `json:"reasoning_steps,omitempty"` // LLM's reasoning process

	// Confidence scores for each decision
	ConfidenceScores map[string]float64 `json:"confidence_scores,omitempty"`

	// Model information
	Model      string `json:"model"`       // Claude model used (e.g., "claude-3-5-sonnet-20241022")
	TokensUsed int    `json:"tokens_used"` // Total tokens consumed
}

// GetDefaultDecision returns default pipeline decision when LLM is unavailable
func GetDefaultDecision() *PipelineDecision {
	return &PipelineDecision{
		NeedSegment:      true,
		NeedLandmarks:    true,
		EnableMotion:     true,
		NeedMusic:        true,
		ImageDescription: "Default configuration - LLM analysis skipped",
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
}
