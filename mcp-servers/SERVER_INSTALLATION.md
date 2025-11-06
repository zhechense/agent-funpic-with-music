# MCP Servers Installation Summary

## Installation Location
`/Users/zhe.chen/workspace/hackweek/202511/agent-funpic-act/mcp-servers/`

## 1. ImageSorcery MCP
**Purpose**: Background removal, image processing, object detection
**Installation**: Python venv
**Location**: `./imagesorcery-env/`
**Command**: `./imagesorcery-env/bin/imagesorcery-mcp`
**Transport**: stdio
**Tools**:
- `detect` - Object detection using YOLO
- `fill` - Fill/mask areas (for background removal)
- `find` - Find objects by text description using CLIP
- `crop`, `resize`, `rotate`, `blur`, `overlay`, etc.

**Verification**:
```bash
source imagesorcery-env/bin/activate
imagesorcery-mcp --help
```

## 2. YOLO-MCP-Server
**Purpose**: Pose estimation, object detection, segmentation
**Installation**: Python venv
**Location**: `./yolo-service/`
**Command**: `./yolo-service/.venv/bin/python ./yolo-service/server.py`
**Transport**: stdio
**Models**: `./yolo-service/models/yolov8n-pose.pt`
**Tools**:
- `analyze_image_from_path` - Main image analysis (with pose model)
- `segment_objects` - Instance segmentation
- `classify_image` - Classification
- Camera detection tools

**Verification**:
```bash
cd yolo-service
source .venv/bin/activate
python server.py --help
```

## 3. video-audio-mcp
**Purpose**: Video/audio editing and composition using FFmpeg
**Installation**: uv (Python package manager)
**Location**: `./video-audio-mcp/`
**Command**: `uv --directory ./video-audio-mcp run server.py`
**Transport**: stdio
**Dependencies**: FFmpeg (installed via Homebrew)
**Tools**:
- `concatenate_videos` - Join videos with transitions
- `extract_audio_from_video` - Extract audio tracks
- `add_text_overlay`, `add_image_overlay` - Overlays
- `trim_video`, `convert_video_format` - Editing
- 30+ video/audio manipulation tools

**Verification**:
```bash
cd video-audio-mcp
uv run server.py --help
ffmpeg -version
```

## 4. Epidemic Sound MCP (Remote)
**Purpose**: Music search and royalty-free track access
**Type**: Remote HTTP service
**URL**: `https://www.epidemicsound.com/a/mcp-service/mcp`
**Transport**: HTTP + SSE
**Authentication**: Bearer token (JWT)
**Tools**:
- GraphQL queries for music catalog
- Track search by mood/scene
- Preview access

**No local installation required**

---

## Quick Start Commands

### Test ImageSorcery
```bash
cd /Users/zhe.chen/workspace/hackweek/202511/agent-funpic-act/mcp-servers
source imagesorcery-env/bin/activate
python -c "import imagesorcery_mcp; print('ImageSorcery OK')"
```

### Test YOLO
```bash
cd /Users/zhe.chen/workspace/hackweek/202511/agent-funpic-act/mcp-servers/yolo-service
source .venv/bin/activate
python -c "import ultralytics; print('YOLO OK')"
```

### Test video-audio-mcp
```bash
cd /Users/zhe.chen/workspace/hackweek/202511/agent-funpic-act/mcp-servers/video-audio-mcp
/Users/zhe.chen/.local/bin/uv run python -c "import ffmpeg; print('FFmpeg-python OK')"
```

---

## Integration Configuration

For Go agent integration, use these paths in `configs/agent-real.yaml`:

```yaml
servers:
  imagesorcery:
    command: ["/Users/zhe.chen/workspace/hackweek/202511/agent-funpic-act/mcp-servers/imagesorcery-env/bin/imagesorcery-mcp"]
    transport: stdio
    
  yolo:
    command: ["/Users/zhe.chen/workspace/hackweek/202511/agent-funpic-act/mcp-servers/yolo-service/.venv/bin/python", "/Users/zhe.chen/workspace/hackweek/202511/agent-funpic-act/mcp-servers/yolo-service/server.py"]
    transport: stdio
    
  video:
    command: ["/Users/zhe.chen/.local/bin/uv", "--directory", "/Users/zhe.chen/workspace/hackweek/202511/agent-funpic-act/mcp-servers/video-audio-mcp", "run", "server.py"]
    transport: stdio
    
  music:
    url: "https://www.epidemicsound.com/a/mcp-service/mcp"
    transport: http
    headers:
      Authorization: "Bearer eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSld..."
```
