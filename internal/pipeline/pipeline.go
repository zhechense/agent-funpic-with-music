package pipeline

import (
	"context"
	"fmt"
	"log"

	"github.com/zhe.chen/agent-funpic-act/internal/client"
	"github.com/zhe.chen/agent-funpic-act/internal/llm"
	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// Pipeline orchestrates the execution of all stages
type Pipeline struct {
	imagesorceryClient client.MCPClient // Background removal
	yoloClient         client.MCPClient // Pose estimation
	videoClient        client.MCPClient // Video composition
	musicClient        client.MCPClient // Music search
	llmProvider        llm.Provider     // Multi-provider LLM support
	enableMotion       bool
	maxRetries         int
	manifestPath       string
	aiMode             string // "lightweight" or "full_ai"
}

// NewPipeline creates a new pipeline executor
func NewPipeline(
	imagesorceryClient client.MCPClient,
	yoloClient client.MCPClient,
	videoClient client.MCPClient,
	musicClient client.MCPClient,
	llmProvider llm.Provider,
	enableMotion bool,
	maxRetries int,
	manifestPath string,
	aiMode string,
) *Pipeline {
	return &Pipeline{
		imagesorceryClient: imagesorceryClient,
		yoloClient:         yoloClient,
		videoClient:        videoClient,
		musicClient:        musicClient,
		llmProvider:        llmProvider,
		enableMotion:       enableMotion,
		maxRetries:         maxRetries,
		manifestPath:       manifestPath,
		aiMode:             aiMode,
	}
}

// Execute runs the pipeline with idempotent stage execution
func (p *Pipeline) Execute(ctx context.Context, input types.PipelineInput, pipelineID string) (*PipelineResult, error) {
	// Route to full AI mode if enabled
	if p.aiMode == "full_ai" && p.llmProvider != nil && p.llmProvider.IsEnabled() {
		log.Println("[AI Agent] Full AI mode enabled, routing to ExecuteWithAI")
		return p.ExecuteWithAI(ctx, input, pipelineID)
	}

	// Load or create manifest
	manifest, err := LoadManifest(p.manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	if manifest == nil {
		manifest = NewManifest(pipelineID, input)
		log.Printf("Created new pipeline manifest: %s", pipelineID)
	} else {
		log.Printf("Resuming pipeline: %s from stage %s", manifest.PipelineID, manifest.CurrentStage)
	}

	// Lightweight mode: Use default configuration
	// Note: For AI-driven decisions, use full_ai mode which leverages Provider interface
	var decision *llm.PipelineDecision
	if manifest.LLMAnalysis != nil {
		// Resume: use existing decision from manifest
		decision = manifest.LLMAnalysis.Decision
		log.Println("[AI Agent] Using existing decision from manifest")
	} else {
		// Use default configuration for all stages
		decision = llm.GetDefaultDecision()
		log.Println("[AI Agent] Using default configuration (lightweight mode)")
	}

	// Dynamic stage planning based on LLM decision
	stages := []types.PipelineStage{}
	if decision.NeedSegment {
		stages = append(stages, types.StageSegmentPerson)
	}
	if decision.NeedLandmarks {
		stages = append(stages, types.StageLandmarks)
	}
	if decision.EnableMotion {
		stages = append(stages, types.StageRenderMotion)
	}
	if decision.NeedMusic {
		stages = append(stages, types.StageSearchMusic)
	}
	// Always include compose stage
	stages = append(stages, types.StageCompose)

	log.Printf("[AI Agent] Executing %d stages: %v", len(stages), stages)

	// Execute stages sequentially
	for _, stage := range stages {
		// Check if stage already completed (idempotency)
		if manifest.IsStageCompleted(stage) {
			log.Printf("Stage %s already completed, skipping", stage)
			continue
		}

		// Check if we can retry this stage
		if !manifest.CanRetryStage(stage, p.maxRetries) {
			return nil, fmt.Errorf("stage %s exceeded max retries (%d)", stage, p.maxRetries)
		}

		// Execute stage with retry logic
		if err := p.executeStageWithRetry(ctx, stage, manifest); err != nil {
			// Save failed state
			manifest.FailStage(stage, err)
			if saveErr := manifest.Save(p.manifestPath); saveErr != nil {
				log.Printf("Warning: failed to save manifest after error: %v", saveErr)
			}
			return nil, fmt.Errorf("stage %s failed: %w", stage, err)
		}

		// Save progress after each stage
		if err := manifest.Save(p.manifestPath); err != nil {
			return nil, fmt.Errorf("failed to save manifest: %w", err)
		}

		log.Printf("Stage %s completed successfully", stage)
	}

	// Mark pipeline as complete
	manifest.CurrentStage = types.StageComplete
	if err := manifest.Save(p.manifestPath); err != nil {
		return nil, fmt.Errorf("failed to save final manifest: %w", err)
	}

	log.Printf("Pipeline %s completed successfully", pipelineID)
	return manifest.Result, nil
}

// ExecuteWithAI executes pipeline with full AI control via conversation loop
func (p *Pipeline) ExecuteWithAI(ctx context.Context, input types.PipelineInput, pipelineID string) (*PipelineResult, error) {
	log.Printf("[AI Agent] Starting full AI mode for pipeline: %s using provider: %s", pipelineID, p.llmProvider.Name())

	// 1. Create tool adapter with all MCP clients
	mcpClients := map[string]client.MCPClient{
		"imagesorcery": p.imagesorceryClient,
		"yolo":         p.yoloClient,
		"video":        p.videoClient,
		"music":        p.musicClient,
	}
	toolAdapter := llm.NewToolAdapter(mcpClients)

	// 2. Create conversation config with limits
	conversationConfig := &llm.FullAIConversationConfig{
		MaxRounds:      20,     // Max 20 conversation rounds
		MaxTokens:      100000, // Max 100k tokens
		MaxCostUSD:     0.50,   // Max $0.50
		TimeoutSeconds: 300,    // 5 minute timeout
		Model:          "",     // Use provider's default model
	}

	// 3. Create conversation from provider
	conversation, err := p.llmProvider.CreateConversation(conversationConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	// 4. Set tool adapter
	conversation.SetToolAdapter(toolAdapter)

	// 5. Execute conversation loop
	result, err := conversation.Execute(ctx, input.ImagePath, input.Duration, input.UserPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI conversation failed: %w", err)
	}

	// 6. Log metrics
	metrics := conversation.GetMetrics()
	log.Printf("[AI Agent] Conversation completed:")
	log.Printf("  - Rounds: %d", metrics.Rounds)
	log.Printf("  - Tool Calls: %d", metrics.ToolCalls)
	log.Printf("  - Tokens: %d", metrics.TokensUsed)
	log.Printf("  - Duration: %.2fs", metrics.Duration)
	log.Printf("  - Cost: $%.4f", metrics.CostUSD)

	// 7. Return result
	// Note: In full AI mode, the result is the LLM's final output
	// This might include the path to the final video or status message
	return &PipelineResult{
		FinalOutputPath: result, // LLM should return video path
	}, nil
}

// executeStageWithRetry executes a single stage with retry logic
func (p *Pipeline) executeStageWithRetry(ctx context.Context, stage types.PipelineStage, manifest *Manifest) error {
	stepFunc, err := GetStepForStage(stage)
	if err != nil {
		return err
	}

	// Mark stage as running
	manifest.StartStage(stage)
	log.Printf("Starting stage: %s", stage)

	// Execute the step
	if err := stepFunc(ctx, p, manifest); err != nil {
		return err
	}

	return nil
}

// GetStageOrder returns the ordered list of pipeline stages
func GetStageOrder() []types.PipelineStage {
	return []types.PipelineStage{
		types.StageSegmentPerson,
		types.StageLandmarks,
		types.StageRenderMotion,
		types.StageSearchMusic,
		types.StageCompose,
	}
}

// ValidateInput checks if the pipeline input is valid
func ValidateInput(input types.PipelineInput) error {
	if input.ImagePath == "" {
		return fmt.Errorf("image_path is required")
	}
	if input.Duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	return nil
}
