# agent-funpic-act

A Go CLI agent that orchestrates image processing, animation, and music composition through Model Context Protocol (MCP) servers.

## Overview

This agent connects to four MCP servers to create animated videos with music:
- **ImageSorcery MCP**: Background removal using object detection and fill
- **YOLO Service**: Pose estimation and landmark detection
- **Video-Audio MCP**: Video composition and audio integration
- **Epidemic Sound MCP**: Music search and retrieval

The agent executes a stateful pipeline with automatic retry and resume capabilities.

## Features

- **Multi-Server MCP Client**: Connects to multiple MCP servers simultaneously
- **AI Agent Support**: Multi-provider LLM integration (Claude, Gemini, OpenAI, OpenRouter) for autonomous pipeline orchestration
- **Flexible LLM Providers**: Easy switching between Anthropic Claude, Google Gemini, OpenAI GPT, and OpenRouter models via configuration
- **Flexible Transport**: Supports stdio (subprocess) and HTTP (SSE) transports
- **State Persistence**: JSON manifest enables pipeline resume after failures
- **Idempotent Execution**: Automatically skips completed stages on resume
- **Capability Discovery**: Validates required tools are available before execution
- **Context-Aware**: Respects timeouts and cancellation signals
- **Comprehensive Testing**: Unit tests for timeouts, errors, and edge cases

## Architecture

```
┌──────────────────────────────────────────┐
│         CLI Application                  │
│       (cmd/agent/main.go)                │
└──────────────────────────────────────────┘
                  │
                  ▼
┌──────────────────────────────────────────┐
│        Pipeline Orchestrator             │
│     (internal/pipeline/pipeline.go)      │
└──────────────────────────────────────────┘
                  │
        ┌─────────┼─────────┐
        ▼         ▼         ▼         ▼
   ┌────────┐ ┌──────┐ ┌───────┐ ┌──────┐
   │ImageSor│ │ YOLO │ │ Video │ │Music │
   │  cery  │ │Client│ │Client │ │Client│
   └────────┘ └──────┘ └───────┘ └──────┘
        │         │         │         │
        ▼         ▼         ▼         ▼
   ┌────────┐ ┌──────┐ ┌───────┐ ┌──────┐
   │ImageSor│ │ YOLO │ │Video  │ │Epic  │
   │cery MCP│ │Service│ │Audio  │ │Sound │
   │(stdio) │ │(stdio)│ │ MCP   │ │ MCP  │
   │        │ │       │ │(stdio)│ │(HTTP)│
   └────────┘ └──────┘ └───────┘ └──────┘
```

## Prerequisites

- Go 1.21 or later
- Python 3.13+ for MCP servers
- FFmpeg (for video processing)
- Epidemic Sound API token (for music search)
- **Optional:** LLM API key (for AI agent features):
  - Google Gemini API key (recommended, default provider)
  - Anthropic Claude API key
  - OpenAI API key
  - OpenRouter API key (access 200+ models via unified API)
- MCP servers:
  - ImageSorcery MCP (Python)
  - YOLO Service (Python)
  - Video-Audio MCP (Python)
  - Epidemic Sound MCP (HTTP endpoint)

## Installation

```bash
# Clone repository
git clone <repository-url>
cd agent-funpic-act

# Install Go dependencies
make install-deps

# Install MCP servers (ImageSorcery, YOLO, Video-Audio)
make install-mcp-servers

# Test MCP server installations
make test-mcp-servers

# Configure environment variables
cp .env.template .env
# Edit .env and add your API credentials:
#   EPIDEMIC_SOUND_TOKEN=your_token_here
#   GOOGLE_API_KEY=your_gemini_key_here  (optional, for AI features)
# Get your tokens from:
#   Epidemic Sound: https://www.epidemicsound.com/
#   Google Gemini: https://makersuite.google.com/app/apikey

# Build agent
make build
```

### Environment Variables

The agent uses environment variables to securely store API credentials:

| Variable | Required | Description |
|----------|----------|-------------|
| `EPIDEMIC_SOUND_TOKEN` | Yes | Epidemic Sound API bearer token for music search |
| `GOOGLE_API_KEY` | Optional | Google Gemini API key (if using Gemini as LLM provider) |
| `ANTHROPIC_API_KEY` | Optional | Anthropic Claude API key (if using Claude as LLM provider) |
| `OPENAI_API_KEY` | Optional | OpenAI API key (if using OpenAI as LLM provider) |
| `OPENROUTER_API_KEY` | Optional | OpenRouter API key (if using OpenRouter as LLM provider) |

**Setup:**
1. Copy the template: `cp .env.template .env`
2. Edit `.env` and add your credentials
3. Never commit `.env` to version control (already in `.gitignore`)

**Get API Keys:**
- Epidemic Sound: https://www.epidemicsound.com/
- Google Gemini: https://makersuite.google.com/app/apikey
- Anthropic Claude: https://console.anthropic.com/
- OpenAI: https://platform.openai.com/api-keys
- OpenRouter: https://openrouter.ai/keys

The configuration file (`configs/agent.yaml`) uses `${VARIABLE_NAME}` syntax to reference environment variables, which are automatically expanded at runtime.

## Usage

### Cursor IDE Integration

This project includes full Cursor IDE integration with AI assistant support and VS Code tasks.

**Quick start in Cursor:**
1. Open project: `cursor /path/to/agent-funpic-act`
2. Press `Cmd+Shift+P` → "Tasks: Run Task" → "Build Agent"
3. Open an image file, then run task "Generate Video - Current File"

**Or use Cursor AI Chat:**
```
You: "Generate a video from image.jpg with 10 second duration"
AI: [Executes agent with correct parameters]
```

See [CURSOR_INTEGRATION.md](CURSOR_INTEGRATION.md) for complete guide.

### Command Line Usage

**Basic:**
```bash
./bin/agent --image path/to/image.jpg --duration 10.0
```

**With helper script:**
```bash
./scripts/generate-video.sh image.jpg 15.0
```

**Batch processing:**
```bash
./scripts/batch-process.sh "images/*.jpg" 10.0
```

**Advanced options:**
```bash
./bin/agent \
  --config configs/agent.yaml \
  --image path/to/image.jpg \
  --duration 15.0 \
  --manifest .pipeline_manifest.json \
  --id my-pipeline-001
```

### Flags

- `--config`: Path to configuration file (default: `configs/agent.yaml`)
- `--image`: Path to input image (required)
- `--duration`: Target duration in seconds (default: `10.0`)
- `--prompt`: User request for animation style
- `--manifest`: Path to state manifest file (default: from config)
- `--id`: Pipeline ID for resume (default: auto-generated)
- `--output`: Output directory (default: `output`)
- `--model`: Override LLM model (e.g., `gemini-1.5-flash`, `claude-3-5-sonnet-20241022`)

## Pipeline Stages

The agent executes the following stages in order:

1. **segment_person**: Remove background using ImageSorcery `detect` + `fill` tools
2. **estimate_landmarks**: Detect pose keypoints using YOLO `analyze_image_from_path` tool
3. **render_motion**: Generate head shake animation with FFmpeg rotation filter
4. **search_music**: Find music tracks using Epidemic Sound `SearchRecordings` tool
5. **compose**: Add audio to video using FFmpeg, creating final MP4 with music

Each stage saves its output to the manifest, enabling resume from any point.

## Configuration

The agent is configured via `configs/agent.yaml`. Environment variables can be referenced using `${VAR_NAME}` syntax:

```yaml
servers:
  imagesorcery:
    name: imagesorcery-mcp
    command: ["python", "-m", "mcp_servers.imagesorcery"]
    transport: stdio
    timeout: 60s
    capabilities:
      tools: [detect, fill, find, crop, resize]

  yolo:
    name: YOLO_Service
    command: ["python", "-m", "mcp_servers.yolo_service"]
    transport: stdio
    timeout: 60s
    capabilities:
      tools: [analyze_image_from_path, segment_objects, classify_image]

  video:
    name: VideoAudioServer
    command: ["python", "-m", "mcp_servers.video_audio"]
    transport: stdio
    timeout: 120s
    capabilities:
      tools: [concatenate_videos, extract_audio_from_video, add_text_overlay]

  music:
    name: Apollo MCP Server
    transport: http
    url: https://www.epidemicsound.com/a/mcp-service/mcp
    timeout: 30s
    headers:
      Authorization: "Bearer ${EPIDEMIC_SOUND_TOKEN}"
    capabilities:
      tools: [SearchRecordings, DownloadRecording]

pipeline:
  max_retries: 3
  manifest_path: .pipeline_manifest.json

# AI Agent configuration (optional)
llm:
  enabled: true
  provider: gemini  # Options: anthropic, google, openai, openrouter

  # Provider-specific configurations
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    model: claude-3-5-sonnet-20241022
    timeout: 30s

  google:
    api_key: "${GOOGLE_API_KEY}"
    model: gemini-2.0-flash-exp
    timeout: 30s

  openai:
    api_key: "${OPENAI_API_KEY}"
    model: gpt-4o
    timeout: 30s

  openrouter:
    api_key: "${OPENROUTER_API_KEY}"
    model: anthropic/claude-3.5-sonnet  # OpenRouter model format
    timeout: 30s

  # AI mode: "lightweight" or "full_ai"
  mode: lightweight
```

### Multi-Provider LLM Support

The agent supports four LLM providers for AI-assisted pipeline orchestration:

**Supported Providers:**
- **Google Gemini** (`gemini`): Fast, cost-effective multimodal model (default)
- **Anthropic Claude** (`anthropic` or `claude`): Advanced reasoning and vision capabilities
- **OpenAI GPT** (`openai`): Industry-standard multimodal model
- **OpenRouter** (`openrouter`): Unified API gateway providing access to 200+ models from multiple providers

**Switching Providers:**

Simply change the `provider` field in `configs/agent.yaml`:
```yaml
llm:
  enabled: true
  provider: gemini  # Change to: anthropic, claude, google, gemini, openai, or openrouter
```

Each provider has its own configuration section with model name, API key, and timeout settings. The agent automatically routes to the appropriate SDK implementation.

### Switching Models

The agent supports flexible model switching with three priority levels:

**Priority Order (highest to lowest):**
1. **Command-line flag** (`--model`)
2. **Environment variable** (`GEMINI_MODEL`)
3. **Configuration file** (`configs/agent.yaml`)

**Available Gemini Models:**

| Model | Speed | Cost | RPD Limit (Free) | Best For | Status |
|-------|-------|------|------------------|----------|--------|
| **Gemini 2.5 Series** |
| `gemini-2.5-pro` | ⚡⚡ | $$$ | 50/day | State-of-the-art reasoning | ✅ Stable |
| `gemini-2.5-flash` | ⚡⚡⚡ | $ | 1000/day | Best price-performance | ✅ Stable |
| `gemini-2.5-flash-lite` | ⚡⚡⚡⚡ | $ | High | Fastest, cost-efficient | ✅ Stable |
| `gemini-2.5-flash-preview-09-2025` | ⚡⚡⚡ | Free | ? | Preview features | ⚠️ Preview |
| `gemini-2.5-flash-lite-preview-09-2025` | ⚡⚡⚡⚡ | Free | ? | Preview lite version | ⚠️ Preview |
| **Gemini 2.0 Series** |
| `gemini-2.0-flash-exp` | ⚡⚡⚡ | Free | 50/day | Experimentation | 🧪 Experimental |
| `gemini-2.0-flash` | ⚡⚡⚡ | $ | 1000/day | Second generation | ✅ Stable |
| `gemini-2.0-flash-lite` | ⚡⚡⚡⚡ | $ | High | Cost-efficient 2.0 | ✅ Stable |
| **Gemini 1.5 Series** |
| `gemini-1.5-flash` | ⚡⚡⚡ | $ | 1500/day | Production workhorse | ✅ Stable |
| `gemini-1.5-pro` | ⚡⚡ | $$ | 50/day | Complex reasoning | ✅ Stable |
| `gemini-1.5-flash-8b` | ⚡⚡⚡⚡ | $ | 4000/day | High-frequency calls | ✅ Stable |

**Notes:**
- Speed: ⚡ (slower) to ⚡⚡⚡⚡ (fastest)
- RPD Limit: Requests Per Day for free tier
- Status: ✅ Stable | ⚠️ Preview (2-week deprecation notice) | 🧪 Experimental
- When hitting rate limits (Error 429), switch to models with higher RPD limits or stable versions

**Usage Examples:**

```bash
# 1. Quick switch via CLI flag (recommended for testing)
./bin/agent --model gemini-1.5-flash --image test.jpg --duration 10

# 2. Use environment variable (good for different environments)
export GEMINI_MODEL=gemini-1.5-pro
./bin/agent --image test.jpg --duration 10

# 3. Edit config file (permanent default)
# In configs/agent.yaml:
#   google:
#     model: gemini-1.5-flash-8b
```

**When to Switch Models:**

- **Hit RPD limit**: Switch from `gemini-2.0-flash-exp` (50/day) to `gemini-1.5-flash-8b` (4000/day)
- **Need better quality**: Switch from `flash` models to `gemini-1.5-pro`
- **Cost optimization**: Use `gemini-1.5-flash-8b` for lightweight tasks
- **Testing**: Use `--model` flag to quickly compare different models

## Development

### MCP Servers

```bash
# Install all MCP servers
make install-mcp-servers

# Install individual servers
make install-imagesorcery
make install-yolo
make install-video-audio

# Test installations
make test-mcp-servers

# Clean MCP servers (caution: removes all venvs)
make clean-mcp-servers
```

### Run Tests

```bash
make test               # All tests
make test-verbose       # Verbose output
make test-coverage      # With coverage report
make test-client        # Client tests only
make test-pipeline      # Pipeline tests only
```

### Code Quality

```bash
make fmt                # Format code
make vet                # Run go vet
make lint               # Run golangci-lint
make check              # All checks (fmt, vet, lint, test)
```

### Makefile Targets

Run `make help` to see all available targets.

## Testing

The project includes comprehensive unit tests using Go's built-in mock transport:

- **Timeout Tests** (`client_timeout_test.go`): Context deadline and cancellation ✅
- **Tool Error Tests** (`tool_error_test.go`): Tool execution errors with `isError=true` ✅
- **Tool Not Found Tests** (`tool_not_found_test.go`): MCP error codes -32000, -32601, -32603

Mock transport implementation (`internal/client/mock_transport_test.go`) enables isolated testing without actual MCP servers.

Run tests with:
```bash
go test ./internal/client/... -v
```

## Project Structure

```
agent-funpic-act/
├── cmd/agent/                      # CLI entry point
├── configs/agent.yaml              # Configuration file
├── scripts/                        # Helper scripts
│   ├── generate-video.sh           # Simple video generation wrapper
│   └── batch-process.sh            # Batch process multiple images
├── examples/
│   └── epidemic_search_example.go  # Epidemic Sound example
├── internal/
│   ├── client/                     # MCP client implementation
│   │   ├── client.go               # Core client and interfaces
│   │   ├── stdio.go                # Stdio transport
│   │   ├── mark3labs_transport.go  # HTTP transport (mark3labs/mcp-go)
│   │   └── discovery.go            # Capability discovery
│   ├── llm/                        # LLM integration
│   │   ├── provider.go             # Provider interface
│   │   ├── unified_types.go        # Provider-agnostic types
│   │   ├── tool_adapter.go         # MCP tool adapter
│   │   └── providers/              # Provider implementations
│   │       ├── claude/             # Anthropic Claude
│   │       ├── gemini/             # Google Gemini
│   │       ├── openai/             # OpenAI GPT
│   │       └── openrouter/         # OpenRouter Gateway
│   └── pipeline/                   # Pipeline orchestration
│       ├── pipeline.go             # Main orchestrator
│       ├── manifest.go             # State persistence
│       └── steps.go                # Stage implementations
├── mcp-servers/                    # MCP server implementations
│   ├── imagesorcery-env/           # ImageSorcery MCP
│   ├── yolo-service/               # YOLO Service
│   └── video-audio-mcp/            # Video-Audio MCP
├── pkg/types/                      # Shared types
├── .env                            # Environment variables (not in git)
├── .vscode/tasks.json              # VS Code/Cursor tasks
├── .cursorrules                    # Cursor AI assistant rules
├── Makefile                        # Build automation
├── CURSOR_INTEGRATION.md           # Cursor IDE integration guide
├── CLAUDE.md                       # Claude Code guidance
└── README.md                       # This file
```

## MCP Protocol

This agent implements the Model Context Protocol (MCP) version 2025-03-26:

- JSON-RPC 2.0 message format
- Protocol initialization with capability negotiation
- Tool discovery via `tools/list`
- Tool invocation via `tools/call`
- Support for stdio and HTTP (Server-Sent Events) transports
- HTTP transport uses `mark3labs/mcp-go` library for Streamable HTTP support

## Troubleshooting

### Gemini API Rate Limits

**Error 429: Resource exhausted**
```
Pipeline execution failed: Error 429, Message: Resource exhausted.
Please try again later.
```

This error means you've hit Gemini API's rate limits. Solutions:

1. **Switch to higher-limit models:**
   ```bash
   # From gemini-2.0-flash-exp (50/day) to:
   ./bin/agent --model gemini-1.5-flash-8b --image test.jpg   # 4000/day
   ./bin/agent --model gemini-1.5-flash --image test.jpg      # 1500/day
   ./bin/agent --model gemini-2.5-flash --image test.jpg      # 1000/day
   ```

2. **Wait for rate limit reset:** Free tier limits reset every 24 hours

3. **Use paid tier:** Upgrade to Google Cloud for higher quotas

4. **Switch LLM provider:** Try Claude or OpenAI (see Multi-Provider LLM Support)

**No text or tool calls in response**

This can also indicate rate limiting or safety filters. Try:
- Switching to a different model variant
- Waiting 5-10 minutes before retrying
- Using a stable release model instead of experimental/preview versions

### Environment Variable Issues

**Token not loading:**
```bash
# Verify .env file exists and has correct format
cat .env
# Should show: EPIDEMIC_SOUND_TOKEN=eyJhbGci...

# Test token expansion
grep "Authorization" configs/agent.yaml
# Should show: Authorization: "Bearer ${EPIDEMIC_SOUND_TOKEN}"
```

**Token not found error:**
- Ensure `.env` file is in the project root directory
- Check that the token variable name matches exactly: `EPIDEMIC_SOUND_TOKEN`
- Verify no extra spaces or quotes around the token value

### Connection Issues

If servers fail to connect:
1. Verify server commands in `configs/agent.yaml`
2. Check server dependencies are installed
3. Review stderr output for server errors
4. For Epidemic Sound: Verify your token is valid and not expired

### Timeout Errors

Increase timeout values in config:
```yaml
servers:
  yolo:
    timeout: 120s  # Increase for slower operations
```

### Resume Failed Pipeline

Use the same `--id` and `--manifest` flags to resume:
```bash
./bin/agent --image img.jpg --duration 10 --id pipeline-123 --manifest .pipeline_manifest.json
```

## IDE Integration

### Cursor IDE

Full integration with Cursor AI assistant and VS Code tasks. See [CURSOR_INTEGRATION.md](CURSOR_INTEGRATION.md) for:
- Using Cursor AI Chat to generate videos
- VS Code tasks (Cmd+Shift+P → Tasks)
- Helper scripts for batch processing
- Keyboard shortcuts and workflows

### Claude Code

This project is optimized for Claude Code. See [CLAUDE.md](CLAUDE.md) for:
- Project architecture and design decisions
- Common commands and development workflows
- Testing patterns and conventions
- How to add new pipeline stages

## License

[Specify your license]

## Contributing

[Contribution guidelines]
