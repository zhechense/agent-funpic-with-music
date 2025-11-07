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
	return fmt.Sprintf(`You are a video generation assistant. Your task is to analyze the provided image and **ACTUALLY GENERATE** a %.1f-second animated video file with background music.

**CRITICAL REQUIREMENTS**:
1. Segment person from background (remove background)
2. Generate animation from the segmented image
3. Search and add background music to the video
4. Return the path to the final video file with music

## Input Image Information
- **Image Path**: %s
- **IMPORTANT**: For all tool calls requiring an image path, use the complete absolute path above

%s

## Required Workflow (Execute ALL steps)

### Step 0: Segment Person (Background Removal)
- Use imagesorcery__find to detect the person and get their polygon coordinates
- Parameters:
  - input_path: Use the absolute path above
  - description: "person"
  - model: "yoloe-11s-seg.pt"
  - confidence: 0.25
  - return_geometry: true
  - geometry_format: "polygon"
- Extract the polygon from the result
- Use imagesorcery__fill to remove the background
- Parameters:
  - input_path: Use the absolute path above
  - areas: [{"polygon": <polygon from find>, "opacity": 0.0}]
  - invert_areas: true
  - output_path: Create unique filename (e.g., "segmented_person_<timestamp>.png")
- Save the segmented image path for use in Step 1
- **IMPORTANT**: Use the segmented image (not the original) in subsequent steps

### Step 1: Generate Animation
- Use video__generate_animation_from_image to create the animation
- Parameters:
  - image_path: Use the SEGMENTED image path from Step 0 (not the original image)
  - output_video_path: Create a unique filename in the working directory (e.g., "animation_nod_<timestamp>.mp4")
  - duration: %.1f
  - animation_type: Choose appropriate camera effect:
    * "rotate": Rotates entire image left-right (simulates head shake), intensity in degrees
    * "shake": Moves entire image left-right horizontally, intensity in pixels
    * "nod": Moves entire image up-down vertically (simulates nodding), intensity in pixels
    * "zoom": Zooms image in/out, intensity as scale factor (0.1 = 10 percent)
  - intensity:
    * For sad/slow effects: use LOW values (rotate: 3-5 degrees, nod/shake: 3-5 pixels)
    * For happy/energetic: use HIGHER values (rotate: 10-15, nod/shake: 10-15)
    * For zoom: always use 0.05-0.15
- Save the output video path for the next step

### Step 2: Search Background Music
- Use music__SearchRecordings to find suitable music
- **IMPORTANT**: Only pass {"first": 3} - do NOT pass "query" parameter
- Select the first track from the results

### Step 3: Compose Final Video with Music
- Extract the music track's download URL from the search results
- Use video__add_audio_to_video or similar tool to combine:
  - Video: The animation video from Step 1
  - Audio: The downloaded music track
  - Output: A final video file with music (e.g., "final_video_with_music_<timestamp>.mp4")

## Important Notes

- **File Paths**: All tool calls MUST use complete absolute paths
- **Do NOT skip steps**: Music is REQUIRED, not optional
- **Output**: Return the path to the final video file that includes both animation and music
- **Error Handling**: If music search fails, try again once before giving up

Now, please begin executing ALL THREE STEPS in order.`, duration, imagePath, toolsDescription, duration)
}
