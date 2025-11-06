# Cursor IDE Integration Guide

This guide explains how to use agent-funpic-act seamlessly within Cursor IDE.

## Quick Start

1. **Open project in Cursor**
   ```bash
   cursor /path/to/agent-funpic-act
   ```

2. **Build the agent** (one-time setup)
   - Press `Cmd+Shift+P` (macOS) or `Ctrl+Shift+P` (Linux/Windows)
   - Type "Tasks: Run Task"
   - Select "Build Agent"

3. **Generate your first video**
   - Open an image file in Cursor
   - Press `Cmd+Shift+P`
   - Select "Tasks: Run Task" → "Generate Video - Current File"
   - Enter duration when prompted (default: 10.0 seconds)

## Using Cursor AI Chat

The `.cursorrules` file teaches Cursor AI about this tool. You can now interact naturally:

### Example Conversations

**Quick Generation:**
```
You: Generate a video from image.jpg
AI: I'll help you generate a video. Let me run the agent...
    [Executes: ./bin/agent --image image.jpg --duration 10]
```

**Custom Parameters:**
```
You: Create a 15-second animated video from portrait.png
AI: I'll create a 15-second video from portrait.png
    [Executes: ./bin/agent --image portrait.png --duration 15.0]
```

**Batch Processing:**
```
You: Process all JPG files in the images/ directory
AI: I'll use the batch processing script to handle all JPG files
    [Executes: ./scripts/batch-process.sh images/*.jpg]
```

**Debugging:**
```
You: The pipeline failed at the search_music stage
AI: Let me check the manifest file for details...
    [Reads: .pipeline_manifest.json]
    [Suggests: Resume with --id flag or check EPIDEMIC_SOUND_TOKEN]
```

## VS Code Tasks

Access via `Cmd+Shift+P` → "Tasks: Run Task":

### Available Tasks

| Task | Description | Use When |
|------|-------------|----------|
| **Generate Video - Quick Test** | 5s video from current file | Quick testing |
| **Generate Video - Current File** | Custom duration from current file | Primary use case |
| **Generate Video - Interactive** | Prompts for all parameters | Ad-hoc processing |
| **Batch Process Images** | Process multiple files | Bulk generation |
| **Build Agent** | Rebuild binary | After code changes |
| **Test MCP Servers** | Verify installations | Setup/debugging |
| **Install MCP Servers** | Install all servers | Initial setup |
| **Run Tests** | Run unit tests | Development |
| **View Pipeline Manifest** | Show current state | Debugging |
| **Resume Failed Pipeline** | Continue from failure | Error recovery |

### Task Shortcuts

You can assign keyboard shortcuts to frequently used tasks:

1. `Cmd+Shift+P` → "Preferences: Open Keyboard Shortcuts (JSON)"
2. Add shortcuts:

```json
[
  {
    "key": "cmd+shift+g",
    "command": "workbench.action.tasks.runTask",
    "args": "Generate Video - Current File"
  },
  {
    "key": "cmd+shift+b cmd+shift+g",
    "command": "workbench.action.tasks.runTask",
    "args": "Batch Process Images"
  }
]
```

## Helper Scripts

The project includes two helper scripts in `scripts/`:

### generate-video.sh

Simple wrapper around the agent for quick use:

```bash
# Basic usage
./scripts/generate-video.sh image.jpg

# With custom duration
./scripts/generate-video.sh image.jpg 15.0

# From Cursor terminal
./scripts/generate-video.sh ${file}
```

**Features:**
- Auto-builds agent if needed
- Validates .env configuration
- Shows output file locations
- Colored output for better readability

### batch-process.sh

Process multiple images in one go:

```bash
# Process all JPG files
./scripts/batch-process.sh "images/*.jpg"

# With custom duration
./scripts/batch-process.sh "images/*.jpg" 15.0

# Process specific files
./scripts/batch-process.sh "img1.jpg img2.jpg img3.jpg"
```

**Features:**
- Creates timestamped output directory
- Shows progress for each file
- Copies all outputs to organized structure
- Summary report with success/failure counts
- Option to open output directory (macOS)

**Output Structure:**
```
output/20231106_143022/
├── image1_final.mp4
├── image1_animation.mp4
├── image1_segmented.png
├── image2_final.mp4
├── image2_animation.mp4
└── image2_segmented.png
```

## AI Assistant Capabilities

Cursor AI can help you with:

### 1. Parameter Construction
```
You: I want a longer video
AI: I'll increase the duration to 15 seconds
    [Suggests: --duration 15.0]
```

### 2. Error Diagnosis
```
You: It says "MCP server connection failed"
AI: Let me check the server status
    [Runs: make test-mcp-servers]
    [Identifies: YOLO server not installed]
    [Suggests: make install-yolo]
```

### 3. Workflow Optimization
```
You: How can I process 100 images efficiently?
AI: Use batch processing with parallel execution:
    [Shows: modified batch script with parallel processing]
    [Explains: trade-offs and resource usage]
```

### 4. Resume Failed Pipelines
```
You: Pipeline pipeline-123 failed, how do I resume?
AI: I'll resume from where it failed
    [Reads manifest to find failure point]
    [Executes: ./bin/agent --image <path> --id pipeline-123]
```

## Tips and Best Practices

### 1. Use Relative Paths
When working in Cursor, use project-relative paths:
```bash
# Good
./bin/agent --image ./test-data/image.jpg

# Also good (Cursor variable)
./bin/agent --image ${workspaceFolder}/image.jpg
```

### 2. Check Prerequisites
Before generating videos, ensure:
- [ ] Agent is built (`make build`)
- [ ] `.env` file exists with `EPIDEMIC_SOUND_TOKEN`
- [ ] MCP servers are installed (`make test-mcp-servers`)

### 3. Monitor Resources
Video generation is resource-intensive:
- **CPU**: FFmpeg encoding, YOLO inference
- **Memory**: Image processing, model loading
- **Disk**: Temporary files in `/tmp/`

For batch processing, consider:
- Process in smaller batches
- Monitor system resources
- Use `--duration` wisely (shorter = faster)

### 4. Leverage Pipeline Resume
If a pipeline fails:
1. Check `.pipeline_manifest.json` for the error
2. Fix the issue (e.g., token expiry, network)
3. Resume with same `--id` to skip completed stages

### 5. Organize Output
For batch jobs, use the output directory feature:
```bash
# Creates organized output directory
./scripts/batch-process.sh "images/*.jpg"

# Custom organization
mkdir -p projects/client-demo/videos
./bin/agent --image client-photo.jpg --duration 10
mv /tmp/final_headshake_with_music.mp4 projects/client-demo/videos/
```

## Troubleshooting in Cursor

### Issue: "Agent not built"

**Solution:**
1. Open Cursor terminal (`Ctrl+` `)
2. Run: `make build`
3. Or use task: "Build Agent"

### Issue: "EPIDEMIC_SOUND_TOKEN not set"

**Solution:**
1. Check `.env` exists: `cat .env | grep EPIDEMIC`
2. If missing: `cp .env.template .env`
3. Edit `.env` in Cursor and add token
4. Restart task

### Issue: "MCP server failed"

**Solution via Cursor AI:**
```
You: MCP servers are failing
AI: Let me diagnose...
    [Runs: make test-mcp-servers]
    [Shows: which server failed]
    [Suggests: make install-<server-name>]
```

### Issue: Task not appearing

**Solution:**
1. Reload window: `Cmd+Shift+P` → "Developer: Reload Window"
2. Verify `.vscode/tasks.json` exists
3. Check tasks list: `Cmd+Shift+P` → "Tasks: Run Task"

## Advanced Integration

### Custom Task Variables

Edit `.vscode/tasks.json` to add custom variables:

```json
{
  "inputs": [
    {
      "id": "customDuration",
      "type": "pickString",
      "description": "Select duration",
      "options": ["5", "10", "15", "30"],
      "default": "10"
    }
  ]
}
```

### Workspace Settings

Add to `.vscode/settings.json`:

```json
{
  "terminal.integrated.env.osx": {
    "PATH": "${workspaceFolder}/bin:${env:PATH}"
  },
  "terminal.integrated.cwd": "${workspaceFolder}",
  "files.associations": {
    "*.yaml": "yaml",
    ".cursorrules": "plaintext"
  }
}
```

### Git Integration

The `.gitignore` is pre-configured to exclude:
- `.env` (contains secrets)
- `.pipeline_manifest.json` (temporary state)
- `bin/` (build artifacts)
- `/tmp/` (output files)

Safe to commit:
- `.cursorrules` ✓
- `.vscode/tasks.json` ✓
- `scripts/*.sh` ✓
- `.env.template` ✓

## Support

For issues or questions:
1. Check this guide
2. Ask Cursor AI (it knows about this tool from `.cursorrules`)
3. Read `README.md` for general usage
4. Check `INSTALL.md` for setup issues
5. File an issue on GitHub

## Quick Reference

| Action | Command |
|--------|---------|
| Generate video (current file) | `Cmd+Shift+P` → Task → "Generate Video - Current File" |
| Batch process | `./scripts/batch-process.sh "pattern"` |
| Build agent | `make build` or Run Task → "Build Agent" |
| Test servers | `make test-mcp-servers` |
| View manifest | `cat .pipeline_manifest.json` or Run Task → "View Pipeline Manifest" |
| Ask AI for help | Open Cursor Chat, describe what you want |

---

**Cursor AI Integration Version:** 1.0.0
**Last Updated:** 2025-11-06
