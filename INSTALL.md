# Installation Guide

## Quick Start

```bash
# 1. Clone and setup
git clone <repository-url>
cd agent-funpic-act

# 2. Install dependencies
make install-deps
make install-mcp-servers

# 3. Configure environment
cp .env.template .env
# Edit .env and add your token

# 4. Build and test
make build
make test-mcp-servers

# 5. Run
./bin/agent --image path/to/image.jpg --duration 10.0
```

## Detailed Setup

### 1. Go Dependencies

```bash
make install-deps
```

This installs:
- Go modules from `go.mod`
- `golangci-lint` for code quality checks

### 2. MCP Servers

```bash
make install-mcp-servers
```

This installs three local MCP servers:

**ImageSorcery MCP** (Background removal)
- Location: `mcp-servers/imagesorcery-env/`
- Command: `pip install imagesorcery-mcp`

**YOLO Service** (Pose detection)
- Location: `mcp-servers/yolo-service/.venv/`
- Downloads: YOLOv8 pose model (~6MB)

**Video-Audio MCP** (Video composition)
- Location: `mcp-servers/video-audio-mcp/.venv/`
- Requires: FFmpeg

### 3. Environment Variables

Create `.env` from template:
```bash
cp .env.template .env
```

Edit `.env` and add your Epidemic Sound token:
```
EPIDEMIC_SOUND_TOKEN=your_actual_token_here
```

Get your token from: https://www.epidemicsound.com/

**Security Notes:**
- `.env` is in `.gitignore` and should never be committed
- Only commit `.env.template` with placeholder values
- Use different tokens for development/production

### 4. Verify Installation

Test MCP servers:
```bash
make test-mcp-servers
```

Expected output:
```
✅ ImageSorcery OK
✅ YOLO OK
✅ Video-Audio OK
✅ FFmpeg ffmpeg version 6.x
```

### 5. Build

```bash
make build
```

Creates `bin/agent` binary.

## Uninstall

Remove MCP servers:
```bash
make clean-mcp-servers  # Prompts for confirmation
```

Remove build artifacts:
```bash
make clean
```
