package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/zhe.chen/agent-funpic-act/internal/llm"
	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// Manifest represents the pipeline execution state
type Manifest struct {
	// Metadata
	PipelineID string    `json:"pipeline_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Input parameters
	Input types.PipelineInput `json:"input"`

	// LLM analysis and decision (AI Agent feature)
	LLMAnalysis *llm.LLMAnalysis `json:"llm_analysis,omitempty"`

	// Current execution state
	CurrentStage types.PipelineStage `json:"current_stage"`
	Stages       map[types.PipelineStage]*StageState `json:"stages"`

	// Final result
	Result *PipelineResult `json:"result,omitempty"`
}

// StageState tracks the state of a single pipeline stage
type StageState struct {
	Status     types.StageStatus `json:"status"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
	RetryCount int               `json:"retry_count"`
	Error      string            `json:"error,omitempty"`
	Output     json.RawMessage   `json:"output,omitempty"` // Stage-specific output
}

// PipelineResult contains the final output
type PipelineResult struct {
	SegmentedImagePath string   `json:"segmented_image_path,omitempty"`
	LandmarksData      string   `json:"landmarks_data,omitempty"`
	MotionVideoPath    string   `json:"motion_video_path,omitempty"`
	MusicTracks        []string `json:"music_tracks,omitempty"`
	FinalOutputPath    string   `json:"final_output_path,omitempty"`
}

// NewManifest creates a new pipeline manifest
func NewManifest(pipelineID string, input types.PipelineInput) *Manifest {
	now := time.Now()
	return &Manifest{
		PipelineID:   pipelineID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Input:        input,
		CurrentStage: types.StageInit,
		Stages:       make(map[types.PipelineStage]*StageState),
	}
}

// LoadManifest reads manifest from file
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No existing manifest
		}
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// Save writes manifest to file atomically
func (m *Manifest) Save(path string) error {
	m.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Write to temp file first for atomicity
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename manifest: %w", err)
	}

	return nil
}

// GetStageState returns the state for a stage, creating if needed
func (m *Manifest) GetStageState(stage types.PipelineStage) *StageState {
	if m.Stages[stage] == nil {
		m.Stages[stage] = &StageState{
			Status: types.StatusPending,
		}
	}
	return m.Stages[stage]
}

// StartStage marks a stage as running
func (m *Manifest) StartStage(stage types.PipelineStage) {
	state := m.GetStageState(stage)
	now := time.Now()
	state.Status = types.StatusRunning
	state.StartedAt = &now
	m.CurrentStage = stage
}

// CompleteStage marks a stage as completed with output
func (m *Manifest) CompleteStage(stage types.PipelineStage, output interface{}) error {
	state := m.GetStageState(stage)
	now := time.Now()
	state.Status = types.StatusCompleted
	state.CompletedAt = &now

	if output != nil {
		data, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal stage output: %w", err)
		}
		state.Output = data
	}

	return nil
}

// FailStage marks a stage as failed with error message
func (m *Manifest) FailStage(stage types.PipelineStage, err error) {
	state := m.GetStageState(stage)
	state.Status = types.StatusFailed
	state.Error = err.Error()
	state.RetryCount++
}

// SkipStage marks a stage as skipped
func (m *Manifest) SkipStage(stage types.PipelineStage) {
	state := m.GetStageState(stage)
	state.Status = types.StatusSkipped
}

// IsStageCompleted checks if a stage was already completed
func (m *Manifest) IsStageCompleted(stage types.PipelineStage) bool {
	state := m.Stages[stage]
	return state != nil && state.Status == types.StatusCompleted
}

// CanRetryStage checks if a stage can be retried
func (m *Manifest) CanRetryStage(stage types.PipelineStage, maxRetries int) bool {
	state := m.Stages[stage]
	if state == nil {
		return true
	}
	return state.RetryCount < maxRetries
}
