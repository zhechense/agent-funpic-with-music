package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// StepFunc represents a pipeline step function
type StepFunc func(ctx context.Context, p *Pipeline, manifest *Manifest) error

// ExecuteSegmentPerson - Use ImageSorcery detect + fill to remove background
func ExecuteSegmentPerson(ctx context.Context, p *Pipeline, manifest *Manifest) error {
	imagePath := manifest.Input.ImagePath

	// Get absolute path
	absPath, err := filepath.Abs(imagePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get confidence threshold from LLM decision (AI Agent feature)
	confidence := 0.3 // default
	if manifest.LLMAnalysis != nil && manifest.LLMAnalysis.Decision != nil {
		if conf, ok := manifest.LLMAnalysis.Decision.Parameters["detect_confidence"].(float64); ok {
			confidence = conf
			log.Printf("[AI Agent] Using LLM confidence: %.2f", confidence)
		}
	}

	// Step 1: Detect person using ImageSorcery's detect tool with segmentation
	detectArgs := map[string]interface{}{
		"input_path":      absPath,
		"confidence":      confidence, // Dynamic parameter from LLM
		"return_geometry": true,
		"geometry_format": "polygon", // Get polygon coordinates
	}

	detectResult, err := p.imagesorceryClient.CallTool(ctx, "detect", detectArgs)
	if err != nil {
		return fmt.Errorf("detect tool failed: %w", err)
	}

	if len(detectResult.Content) == 0 {
		return fmt.Errorf("detect returned no content")
	}

	// Parse detection results to extract person polygons
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(detectResult.Content[0].Text), &response); err != nil {
		return fmt.Errorf("failed to parse detection results: %w", err)
	}

	// Extract detections array
	detections, ok := response["detections"].([]interface{})
	if !ok || len(detections) == 0 {
		return fmt.Errorf("no detections found in image")
	}

	// Find the first person detection with polygon
	var personPolygon []interface{}
	for _, det := range detections {
		detMap := det.(map[string]interface{})
		if detMap["class"] == "person" {
			if poly, exists := detMap["polygon"]; exists {
				personPolygon = poly.([]interface{})
				break
			}
		}
	}

	if len(personPolygon) == 0 {
		return fmt.Errorf("no person with polygon found in image")
	}

	// Step 2: Use fill tool to make everything EXCEPT the person transparent
	// When invert_areas=true with invert, the background is removed
	// Use opacity=0 to make the background fully transparent
	outputPath := filepath.Join(manifest.Input.TempDir, "segmented_person.png")

	// Convert to absolute path for ImageSorcery MCP server
	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute output path: %w", err)
	}

	fillArgs := map[string]interface{}{
		"input_path":   absPath,
		"areas":        []map[string]interface{}{
			{
				"polygon": personPolygon,
				"opacity": 0.0, // Fully transparent background
			},
		},
		"invert_areas": true,  // Fill background (everything except person)
		"output_path":  absOutputPath,
	}

	fillResult, err := p.imagesorceryClient.CallTool(ctx, "fill", fillArgs)
	if err != nil {
		return fmt.Errorf("fill tool failed: %w", err)
	}

	// Fill tool returns the output path as text
	if len(fillResult.Content) > 0 {
		resultText := fillResult.Content[0].Text
		// Try parsing as JSON first
		var fillResponse map[string]interface{}
		if err := json.Unmarshal([]byte(resultText), &fillResponse); err == nil {
			// It's JSON, extract output_path
			if outputPathStr, ok := fillResponse["output_path"].(string); ok {
				outputPath = outputPathStr
			}
		} else {
			// It's plain text (file path)
			outputPath = resultText
		}
	}

	if err := manifest.CompleteStage(types.StageSegmentPerson, map[string]string{
		"segmented_path": outputPath,
	}); err != nil {
		return err
	}

	if manifest.Result == nil {
		manifest.Result = &PipelineResult{}
	}
	manifest.Result.SegmentedImagePath = outputPath

	return nil
}

// ExecuteEstimateLandmarks estimates pose landmarks using YOLO pose model
func ExecuteEstimateLandmarks(ctx context.Context, p *Pipeline, manifest *Manifest) error {
	// Get segmented image from previous stage, fallback to original if not available
	imagePath := manifest.Result.SegmentedImagePath
	if imagePath == "" {
		imagePath = manifest.Input.ImagePath
	}

	// Get confidence threshold from LLM decision (AI Agent feature)
	confidence := 0.3 // default
	if manifest.LLMAnalysis != nil && manifest.LLMAnalysis.Decision != nil {
		if conf, ok := manifest.LLMAnalysis.Decision.Parameters["landmark_confidence"].(float64); ok {
			confidence = conf
			log.Printf("[AI Agent] Using LLM landmark confidence: %.2f", confidence)
		}
	}

	// Use YOLO's analyze_image_from_path with pose model
	args := map[string]interface{}{
		"image_path": imagePath,
		"model_name": "yolov8n-pose.pt",
		"confidence": confidence, // Dynamic parameter from LLM
	}

	result, err := p.yoloClient.CallTool(ctx, "analyze_image_from_path", args)
	if err != nil {
		return fmt.Errorf("analyze_image_from_path (pose) tool failed: %w", err)
	}

	// Extract landmarks data (17 COCO keypoints)
	if len(result.Content) == 0 {
		return fmt.Errorf("pose estimation returned no content")
	}

	landmarksJSON := result.Content[0].Text

	output := map[string]interface{}{
		"landmarks": landmarksJSON,
	}

	if err := manifest.CompleteStage(types.StageLandmarks, output); err != nil {
		return err
	}

	// Store in final result
	manifest.Result.LandmarksData = landmarksJSON

	return nil
}

// ExecuteRenderMotion generates "happy head shake" animation using FFmpeg rotate
func ExecuteRenderMotion(ctx context.Context, p *Pipeline, manifest *Manifest) error {
	imagePath := manifest.Result.SegmentedImagePath
	if imagePath == "" {
		imagePath = manifest.Input.ImagePath
	}

	duration := manifest.Input.Duration
	outputPath := filepath.Join(manifest.Input.TempDir, "headshake_animation.mp4")

	// Use FFmpeg to create rotation animation (head shake effect)
	// Rotate angle: -10 to +10 degrees, 2 complete cycles
	rotateExpr := "rotate=10*PI/180*sin(4*PI*t):c=none"

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-loop", "1",
		"-i", imagePath,
		"-vf", rotateExpr,
		"-t", strconv.FormatFloat(duration, 'f', 1, 64),
		"-r", "15", // 15 fps
		"-pix_fmt", "yuv420p",
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg head shake failed: %w, output: %s", err, output)
	}

	if err := manifest.CompleteStage(types.StageRenderMotion, map[string]string{
		"video_path": outputPath,
	}); err != nil {
		return err
	}

	manifest.Result.MotionVideoPath = outputPath
	return nil
}

// ExecuteSearchMusic searches for happy music from Epidemic Sound
func ExecuteSearchMusic(ctx context.Context, p *Pipeline, manifest *Manifest) error {
	// Get music parameters from LLM decision (AI Agent feature)
	musicCount := 5 // default
	musicMood := "happy" // default
	if manifest.LLMAnalysis != nil && manifest.LLMAnalysis.Decision != nil {
		if count, ok := manifest.LLMAnalysis.Decision.MusicCount, manifest.LLMAnalysis.Decision.MusicCount > 0; ok {
			musicCount = count
		}
		if mood := manifest.LLMAnalysis.Decision.MusicMood; mood != "" {
			musicMood = mood
		}
		log.Printf("[AI Agent] Searching for %s music (count: %d)", musicMood, musicCount)
	} else {
		log.Println("Searching for music from Epidemic Sound...")
	}

	// Use SearchRecordings with empty args to get music
	// The query parameter requires a complex RecordingsQuery object which is not documented
	// Using empty args returns default results we can filter
	args := map[string]interface{}{
		"first": musicCount, // Dynamic parameter from LLM
	}

	log.Printf("Calling Epidemic Sound 'SearchRecordings' tool")
	result, err := p.musicClient.CallTool(ctx, "SearchRecordings", args)
	if err != nil {
		log.Printf("Music search failed (will skip music): %v", err)
		// If search fails (e.g., token expired), skip music
		manifest.SkipStage(types.StageSearchMusic)
		manifest.Result.MusicTracks = []string{}
		return nil
	}

	log.Printf("Music search succeeded! Got %d content blocks", len(result.Content))

	// Parse music results - extract track information from JSON
	var musicTracks []string
	if len(result.Content) > 0 {
		// The result is GraphQL JSON response with recordings data
		// Parse to extract track titles and preview URLs
		log.Printf("Music result contains %d bytes of data", len(result.Content[0].Text))

		// For now, just save the first 500 chars for display
		preview := result.Content[0].Text
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		log.Printf("Music tracks found: %s", preview)

		musicTracks = []string{"Music tracks available (see manifest for details)"}
		manifest.Result.MusicTracks = musicTracks
	} else {
		log.Println("No music tracks returned")
		manifest.Result.MusicTracks = []string{}
	}

	stageData := map[string]interface{}{
		"track_count": len(musicTracks),
	}
	if len(result.Content) > 0 {
		stageData["data"] = result.Content[0].Text
	}

	if err := manifest.CompleteStage(types.StageSearchMusic, stageData); err != nil {
		return err
	}

	return nil
}

// ExecuteCompose performs final video composition using video-audio-mcp
func ExecuteCompose(ctx context.Context, p *Pipeline, manifest *Manifest) error {
	log.Println("Composing final video with music...")

	// Determine video source
	videoSource := manifest.Result.MotionVideoPath
	if videoSource == "" {
		// No motion video, would need to convert image to video
		videoSource = manifest.Result.SegmentedImagePath
		if videoSource == "" {
			videoSource = manifest.Input.ImagePath
		}
	}

	outputPath := filepath.Join(manifest.Input.OutputDir, "final_output.mp4")

	// Check if we have music data from the search stage
	stageData := manifest.Stages[types.StageSearchMusic]
	if stageData != nil && len(stageData.Output) > 0 {
		// Parse the Output json.RawMessage into a map
		var stageOutput map[string]interface{}
		if err := json.Unmarshal(stageData.Output, &stageOutput); err != nil {
			log.Printf("Failed to parse stage output: %v", err)
		} else if musicDataStr, ok := stageOutput["data"].(string); ok && musicDataStr != "" {
			log.Println("Found music data, extracting track URL...")

			// Parse the JSON to extract the first track's audio URL
			var musicResp struct {
				Data struct {
					Recordings struct {
						Nodes []struct {
							Recording struct {
								Title     string `json:"title"`
								AudioFile struct {
									Lqmp3Url string `json:"lqmp3Url"`
								} `json:"audioFile"`
							} `json:"recording"`
						} `json:"nodes"`
					} `json:"recordings"`
				} `json:"data"`
			}

			if err := json.Unmarshal([]byte(musicDataStr), &musicResp); err != nil {
				log.Printf("Failed to parse music data: %v, continuing without music", err)
			} else if len(musicResp.Data.Recordings.Nodes) > 0 {
				// Get the first track (could filter for "happy" mood later)
				track := musicResp.Data.Recordings.Nodes[0].Recording
				musicURL := track.AudioFile.Lqmp3Url
				trackTitle := track.Title

				log.Printf("Selected track: '%s'", trackTitle)
				log.Printf("Downloading music from: %s", musicURL)

				// Download music file
				musicPath := "/tmp/temp_music.mp3"
				cmd := exec.CommandContext(ctx, "curl", "-L", "-o", musicPath, musicURL)
				if err := cmd.Run(); err != nil {
					log.Printf("Failed to download music: %v, continuing without music", err)
				} else {
					log.Println("Music downloaded successfully")

					// Use ffmpeg to add audio to video
					// -i video.mp4 -i audio.mp3 -c:v copy -c:a aac -shortest output.mp4
					log.Println("Adding music to video with ffmpeg...")
					cmd = exec.CommandContext(ctx, "ffmpeg", "-y",
						"-i", videoSource,
						"-i", musicPath,
						"-c:v", "copy",
						"-c:a", "aac",
						"-shortest",
						"-map", "0:v:0",
						"-map", "1:a:0",
						outputPath)

					output, err := cmd.CombinedOutput()
					if err != nil {
						log.Printf("ffmpeg failed: %v\nOutput: %s", err, string(output))
						log.Println("Falling back to video without audio")
						// Copy video without audio as fallback
						cmd = exec.CommandContext(ctx, "cp", videoSource, outputPath)
						if err := cmd.Run(); err != nil {
							return fmt.Errorf("failed to copy output: %w", err)
						}
					} else {
						log.Println("Successfully added music to video!")
					}

					// Clean up temp music file
					os.Remove(musicPath)
				}
			}
		}
	}

	// If no music was added, just copy the video
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		log.Println("No music added, using video without audio")
		cmd := exec.CommandContext(ctx, "cp", videoSource, outputPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to copy output: %w", err)
		}
	}

	if err := manifest.CompleteStage(types.StageCompose, map[string]string{
		"final_path": outputPath,
	}); err != nil {
		return err
	}

	manifest.Result.FinalOutputPath = outputPath
	return nil
}

// GetStepForStage returns the step function for a given stage
func GetStepForStage(stage types.PipelineStage) (StepFunc, error) {
	switch stage {
	case types.StageSegmentPerson:
		return ExecuteSegmentPerson, nil
	case types.StageLandmarks:
		return ExecuteEstimateLandmarks, nil
	case types.StageRenderMotion:
		return ExecuteRenderMotion, nil
	case types.StageSearchMusic:
		return ExecuteSearchMusic, nil
	case types.StageCompose:
		return ExecuteCompose, nil
	default:
		return nil, fmt.Errorf("unknown stage: %s", stage)
	}
}
