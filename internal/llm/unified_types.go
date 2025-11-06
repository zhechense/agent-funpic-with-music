package llm

// UnifiedMessage represents a provider-agnostic message in a conversation
type UnifiedMessage struct {
	Role    MessageRole
	Content []ContentPart
}

// MessageRole defines the role of a message sender
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

// ContentPart represents a single piece of content in a message
type ContentPart struct {
	Type ContentType

	// For text content
	Text string

	// For image content
	ImageData *ImageData

	// For tool use
	ToolCall *ToolCall

	// For tool result
	ToolResult *ToolResult
}

// ContentType defines the type of content in a message
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeImage      ContentType = "image"
	ContentTypeToolUse    ContentType = "tool_use"
	ContentTypeToolResult ContentType = "tool_result"
)

// ImageData represents an image in a message
type ImageData struct {
	// Base64-encoded image data
	Data string

	// Media type (image/jpeg, image/png, image/gif, image/webp)
	MediaType string

	// Optional: Image URL (for providers that support URL references)
	URL string
}

// ToolCall represents a request to call a tool
type ToolCall struct {
	// Unique ID for this tool call
	ID string

	// Tool name (e.g., "imagesorcery__detect")
	Name string

	// Tool arguments as a map
	Arguments map[string]interface{}
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	// ID of the tool call this result is for
	ToolCallID string

	// Result content (usually JSON or text)
	Content string

	// Whether the tool execution resulted in an error
	IsError bool
}

// UnifiedTool represents a provider-agnostic tool definition
type UnifiedTool struct {
	// Tool name
	Name string

	// Tool description
	Description string

	// Input parameters as JSON schema
	// This is a map representing a JSON Schema object
	Parameters map[string]interface{}
}

// NewTextMessage creates a simple text message
func NewTextMessage(role MessageRole, text string) UnifiedMessage {
	return UnifiedMessage{
		Role: role,
		Content: []ContentPart{
			{
				Type: ContentTypeText,
				Text: text,
			},
		},
	}
}

// NewVisionMessage creates a message with both image and text
func NewVisionMessage(imageData, mediaType, text string) UnifiedMessage {
	return UnifiedMessage{
		Role: RoleUser,
		Content: []ContentPart{
			{
				Type: ContentTypeImage,
				ImageData: &ImageData{
					Data:      imageData,
					MediaType: mediaType,
				},
			},
			{
				Type: ContentTypeText,
				Text: text,
			},
		},
	}
}

// NewToolCallMessage creates a message with tool call requests
func NewToolCallMessage(toolCalls []ToolCall) UnifiedMessage {
	content := make([]ContentPart, len(toolCalls))
	for i, tc := range toolCalls {
		content[i] = ContentPart{
			Type:     ContentTypeToolUse,
			ToolCall: &tc,
		}
	}

	return UnifiedMessage{
		Role:    RoleAssistant,
		Content: content,
	}
}

// NewToolResultMessage creates a message with tool execution results
func NewToolResultMessage(toolResults []ToolResult) UnifiedMessage {
	content := make([]ContentPart, len(toolResults))
	for i, tr := range toolResults {
		content[i] = ContentPart{
			Type:       ContentTypeToolResult,
			ToolResult: &tr,
		}
	}

	return UnifiedMessage{
		Role:    RoleUser,
		Content: content,
	}
}
