package llm

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// ReadAndEncodeImage reads an image file and converts it to base64
func ReadAndEncodeImage(imagePath string) (string, string, error) {
	// Read image file
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read image: %w", err)
	}

	// Detect media type
	mediaType := detectMediaType(imagePath)

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(data)

	return encoded, mediaType, nil
}

// CreateVisionMessage creates a Claude message with image and text
func CreateVisionMessage(imageBase64, mediaType, prompt string) anthropic.MessageParam {
	return anthropic.NewUserMessage(
		anthropic.NewImageBlockBase64(mediaType, imageBase64),
		anthropic.NewTextBlock(prompt),
	)
}

// detectMediaType returns the media type based on file extension
func detectMediaType(path string) string {
	lower := strings.ToLower(path)

	if strings.HasSuffix(lower, ".png") {
		return "image/png"
	}
	if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
		return "image/jpeg"
	}
	if strings.HasSuffix(lower, ".gif") {
		return "image/gif"
	}
	if strings.HasSuffix(lower, ".webp") {
		return "image/webp"
	}

	// Default to JPEG
	return "image/jpeg"
}

// CreateVideoGenerationPrompt creates a prompt for video generation task
func CreateVideoGenerationPrompt(duration float64, imagePath string, toolsDescription string) string {
	return fmt.Sprintf(`You are a video generation assistant. Your task is to analyze the provided image and **ACTUALLY GENERATE** a %.1f-second animated video file using available tools.

**IMPORTANT**: Your goal is to produce an actual video file, not just return commands or suggestions. Use the available MCP tools to complete the video generation.

## Input Image Information
- **Image Path**: %s
- **IMPORTANT**: For all tool calls requiring an image path, use the complete absolute path above

%s

## Suggested Workflow

1. **Analyze Image**
   - Identify image content (people, background, scene, mood)
   - Assess image quality (clarity, lighting)
   - Determine if there are people in the image

2. **Background Processing** (optional)
   - If the background is complex and you need to highlight people, use imagesorcery__detect and imagesorcery__fill
   - If the background is simple or solid color, skip this step
   - Note: imagesorcery tools require complete absolute paths

3. **Pose Estimation** (if people detected)
   - Use yolo__analyze_image_from_path to detect facial keypoints
   - This is important for generating natural animations

4. **Animation Generation**
   - Based on pose information and user's request, create the animation
   - You can either:
     a) Use available video tools to generate the animation
     b) If no suitable tool exists, return FFmpeg parameters as JSON for manual execution
   - Prefer calling tools over returning commands

5. **Music Search** (optional)
   - Based on image content and mood, use music__SearchRecordings to find suitable music
   - **IMPORTANT**: Only pass the "first" parameter (e.g., {"first": 5}) - do NOT pass "query" parameter
   - The "first" parameter specifies how many music tracks to return (typically 3-5)
   - Example: music__SearchRecordings with {"first": 5}
   - If music search fails, skip this step and continue with video generation

6. **Final Composition**
   - Compose animation and music into final video

## Important Notes

- **File Path**: All tool calls MUST use the complete absolute path provided above: %s
- **Efficiency First**: Skip unnecessary steps (e.g., no need to remove solid color backgrounds)
- **Explain Decisions**: Briefly explain your reasoning for each step
- **Error Handling**: If a tool fails (e.g., music search), skip it and continue with other steps
- **Step-by-step Execution**: Call one tool at a time, wait for results before continuing

Now, please begin analyzing the image and executing the task.`, duration, imagePath, toolsDescription, imagePath)
}
