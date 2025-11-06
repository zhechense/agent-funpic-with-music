# Agent-Funpic-Act 架构深度分析

## 核心发现：这不是一个 AI Agent！

这是一个**确定性工作流编排器** (Deterministic Workflow Orchestrator)，而非传统的 AI Agent。

```
传统 AI Agent:           这个项目:
┌──────────┐            ┌──────────┐
│  Model   │            │   无!     │  ← 没有 LLM 推理
│  (LLM)   │            │          │
└──────────┘            └──────────┘
     ↓                       ↓
┌──────────┐            ┌──────────┐
│ Planning │            │ 固定流程  │  ← 预定义的 5 个步骤
│ (动态)   │            │ (静态)   │
└──────────┘            └──────────┘
     ↓                       ↓
┌──────────┐            ┌──────────┐
│  Memory  │            │ Manifest │  ← JSON 文件持久化
└──────────┘            └──────────┘
     ↓                       ↓
┌──────────┐            ┌──────────┐
│  Tools   │            │ 4 MCP    │  ← MCP 工具调用
└──────────┘            └──────────┘
```

---

## Agent 三要素详解

### 1. Model (模型) - ❌ 不存在

**关键点：没有 LLM，没有 AI 决策！**

```go
// ❌ 不存在的代码
// llm.Decide("what should I do next?")
// llm.Reason("how to process this image?")

// ✅ 实际的代码 - 固定流程
stages := []types.PipelineStage{
    types.StageSegmentPerson,   // 1. 背景移除
    types.StageLandmarks,        // 2. 姿态检测
    types.StageRenderMotion,     // 3. 动画渲染
    types.StageSearchMusic,      // 4. 音乐搜索
    types.StageCompose,          // 5. 视频合成
}
```

**流程是硬编码的，不会根据情况改变！**

### 2. Memory (记忆) - ✅ Pipeline Manifest

**实现位置**: `internal/pipeline/manifest.go`

#### Memory 结构

```go
// Manifest 就是 Agent 的"大脑记忆"
type Manifest struct {
    // 元数据
    PipelineID string    // 唯一标识
    CreatedAt  time.Time // 开始时间
    UpdatedAt  time.Time // 最后更新

    // 输入（长期记忆）
    Input types.PipelineInput // image.jpg, duration: 10s

    // 当前状态（工作记忆）
    CurrentStage types.PipelineStage // 当前在哪一步？
    Stages       map[PipelineStage]*StageState // 每步的状态

    // 输出（结果记忆）
    Result *PipelineResult // 所有中间和最终产物
}
```

#### Memory 的三种类型

```go
// 1. 长期记忆 (Long-term Memory)
Input: {
    ImagePath: "image.jpg",
    Duration: 10.0
}

// 2. 工作记忆 (Working Memory)
CurrentStage: "render_motion"
Stages: {
    "segment_person": {
        Status: "completed",
        Output: {"segmented_path": "/tmp/seg.png"}
    },
    "estimate_landmarks": {
        Status: "completed",
        Output: {"landmarks": "[{x:100,y:50}...]"}
    },
    "render_motion": {
        Status: "running",  // ← 当前正在执行
        RetryCount: 0
    }
}

// 3. 结果记忆 (Result Memory)
Result: {
    SegmentedImagePath: "/tmp/seg.png",
    LandmarksData: "[{...}]",
    MotionVideoPath: "/tmp/motion.mp4",
    MusicTracks: ["track1.mp3"],
    FinalOutputPath: "/tmp/final.mp4"
}
```

#### Memory 的持久化

```go
// internal/pipeline/manifest.go:81-100

// 原子写入，防止数据损坏
func (m *Manifest) Save(path string) error {
    // 1. 序列化为 JSON
    data := json.MarshalIndent(m, "", "  ")

    // 2. 写入临时文件
    os.WriteFile(path + ".tmp", data)

    // 3. 原子重命名 (atomic rename)
    os.Rename(path + ".tmp", path)
}
```

**为什么需要 Memory?**
- **可恢复性**: Pipeline 失败后可以从中断处继续
- **幂等性**: 已完成的步骤不会重复执行
- **可追溯**: 知道每一步的输入输出和时间
- **调试**: 出错时可以查看完整的执行历史

#### Memory 的使用示例

```go
// 检查是否已完成
if manifest.IsStageCompleted(types.StageSegmentPerson) {
    log.Println("Stage already completed, skipping")
    continue
}

// 标记开始
manifest.StartStage(types.StageSegmentPerson)

// 执行步骤...

// 保存结果
manifest.CompleteStage(types.StageSegmentPerson, map[string]string{
    "segmented_path": "/tmp/segmented_person.png"
})

// 持久化到磁盘
manifest.Save(".pipeline_manifest.json")
```

### 3. Tools (工具) - ✅ 4 个 MCP Servers

#### Tool 结构

```
Agent 调用工具的流程:

cmd/agent/main.go                   MCP Server (Python)
      │                                   │
      │ 1. 创建 MCP 客户端                │
      ├─────────────────────────────────→│
      │                                   │
      │ 2. 初始化协议                     │
      │    initialize()                   │
      ├─────────────────────────────────→│
      │                                   │
      │ 3. 列出可用工具                   │
      │    tools/list                     │
      ├─────────────────────────────────→│
      │←─────────────────────────────────│
      │   [detect, fill, find, ...]      │
      │                                   │
internal/pipeline/steps.go              │
      │                                   │
      │ 4. 调用工具                       │
      │    tools/call "detect"            │
      ├─────────────────────────────────→│
      │                                   │
      │                              ┌────┴────┐
      │                              │ YOLO    │
      │                              │ 模型推理 │
      │                              └────┬────┘
      │                                   │
      │←─────────────────────────────────│
      │   {"detections": [...]}           │
```

#### 4 个 MCP Servers 的详细说明

**1. ImageSorcery MCP** (Python)
```yaml
位置: mcp-servers/imagesorcery-env/
传输: stdio (subprocess)
工具:
  - detect: 物体检测 (使用 YOLO)
  - fill: 区域填充/透明化
  - find: 文本描述查找物体
  - crop: 裁剪
  - resize: 调整大小
```

调用示例 (internal/pipeline/steps.go:37):
```go
detectResult, err := p.imagesorceryClient.CallTool(ctx, "detect", map[string]interface{}{
    "input_path":      "/path/to/image.jpg",
    "confidence":      0.3,
    "return_geometry": true,
    "geometry_format": "polygon",
})
// 返回: {"detections": [{"class": "person", "polygon": [[x,y],...]}]}
```

**2. YOLO Service MCP** (Python)
```yaml
位置: mcp-servers/yolo-service/
传输: stdio (subprocess)
工具:
  - analyze_image_from_path: 姿态估计
  - segment_objects: 物体分割
  - classify_image: 图像分类
模型: YOLOv8n-pose.pt (17 个 COCO 关键点)
```

调用示例 (internal/pipeline/steps.go:140):
```go
result, err := p.yoloClient.CallTool(ctx, "analyze_image_from_path", map[string]interface{}{
    "image_path": "/tmp/segmented_person.png",
    "model_name": "yolov8n-pose.pt",
    "confidence": 0.3,
})
// 返回: {"keypoints": [[x, y, confidence], ...], "bbox": [x,y,w,h]}
```

**3. Video-Audio MCP** (Python)
```yaml
位置: mcp-servers/video-audio-mcp/
传输: stdio (subprocess)
工具:
  - concatenate_videos: 拼接视频
  - extract_audio_from_video: 提取音频
  - add_text_overlay: 添加文字
  - add_image_overlay: 添加图片
  - convert_video_format: 格式转换
依赖: FFmpeg
```

**4. Epidemic Sound MCP** (HTTP)
```yaml
位置: 远程 HTTP 服务
URL: https://www.epidemicsound.com/a/mcp-service/mcp
传输: HTTP with SSE (Server-Sent Events)
工具:
  - SearchRecordings: 搜索音乐
  - DownloadRecording: 下载音乐
认证: Bearer Token (需要账号)
```

调用示例 (internal/pipeline/steps.go:218):
```go
result, err := p.musicClient.CallTool(ctx, "SearchRecordings", map[string]interface{}{
    "first": 5,  // 返回前 5 个结果
})
// 返回: {"data": {"recordings": {"nodes": [{"recording": {...}}]}}}
```

---

## 代码分层架构

### Framework 层 (可复用 ✅)

这些代码是**通用的 MCP 客户端框架**，可以用于任何 MCP 集成项目：

```
internal/client/           ← Framework: MCP 客户端实现
├── client.go              ✅ 通用: MCP 协议核心
├── stdio.go               ✅ 通用: Stdio 传输层
├── mark3labs_transport.go ✅ 通用: HTTP/SSE 传输层
└── discovery.go           ✅ 通用: 工具发现和验证

pkg/types/types.go         ✅ 通用: MCP 数据结构
├── Tool                   MCP 工具定义
├── ToolCallResult         工具调用结果
├── ContentBlock           内容块
└── ServerConfig           服务器配置
```

**可复用示例**:
```go
// 在任何项目中使用这个 MCP 客户端框架

import "github.com/zhe.chen/agent-funpic-act/internal/client"

// 1. 创建客户端
config := types.ServerConfig{
    Command:   []string{"python", "-m", "my_mcp_server"},
    Transport: "stdio",
    Timeout:   60 * time.Second,
}
mcpClient, _ := client.CreateClient(config)

// 2. 连接和初始化
mcpClient.Connect(ctx)
mcpClient.Initialize(ctx)

// 3. 调用工具
result, _ := mcpClient.CallTool(ctx, "my_tool", args)
```

### Application 层 (项目特定 ⚙️)

这些代码是**针对视频生成任务定制的**，不能直接复用：

```
internal/pipeline/         ⚙️ 自定义: 视频生成 Pipeline
├── pipeline.go            特定: 5 步工作流编排
├── manifest.go            可改: 状态管理 (可调整字段)
└── steps.go               特定: 各步骤实现

cmd/agent/main.go          ⚙️ 自定义: CLI 入口
configs/agent.yaml         ⚙️ 自定义: 4 个服务器配置
```

---

## 各组件的详细作用

### 1. Transport Layer (传输层)

**作用**: 在 Go 进程和 MCP Server 之间建立通信通道

#### Stdio Transport
```go
// internal/client/stdio.go:47-80

func (t *StdioTransport) Start(ctx context.Context) error {
    // 1. 启动子进程
    t.cmd = exec.Command(t.command[0], t.command[1:]...)

    // 2. 创建管道
    t.stdin, _ = t.cmd.StdinPipe()   // Go → Python
    t.stdout, _ = t.cmd.StdoutPipe() // Python → Go

    // 3. 启动进程
    t.cmd.Start()

    // 4. 启动后台读取线程
    go t.readLoop()  // 持续读取 stdout
}
```

**数据流**:
```
Go Process                Python Process
    │                          │
    │  JSON-RPC Request        │
    │  {"method":"detect"}     │
    ├──────stdin─────────────→ │
    │                          │
    │                     ┌────┴────┐
    │                     │ Process │
    │                     └────┬────┘
    │                          │
    │  JSON-RPC Response       │
    │  {"result":{...}}        │
    │←─────stdout───────────── │
    │                          │
```

#### HTTP Transport
```go
// internal/client/mark3labs_transport.go

func (t *Mark3LabsTransport) Start(ctx context.Context) error {
    // 使用 mark3labs/mcp-go 库
    // 支持 SSE (Server-Sent Events)
    t.mcpClient = client.NewClient(
        transport.NewStreamableHTTP(url, headers)
    )
}
```

**数据流**:
```
Go Process                Remote Server
    │                          │
    │  HTTP POST               │
    │  /mcp                    │
    ├──────────────────────────→│
    │                          │
    │  SSE Stream (持续)        │
    │  data: {"result":{...}}  │
    │←──────────────────────────│
    │←──────────────────────────│  多次推送
    │←──────────────────────────│
```

### 2. Protocol Layer (协议层)

**作用**: 实现 MCP 协议的初始化握手和 JSON-RPC 通信

```go
// internal/client/client.go:157-187

// MCP 初始化序列
func (c *Client) Initialize(ctx context.Context) error {
    // 1. 发送 initialize 请求
    req := InitializeRequest{
        ProtocolVersion: "2025-03-26",
        Capabilities:    {},
        ClientInfo:      {Name: "agent", Version: "1.0"},
    }
    resp := c.transport.SendRequest(ctx, "initialize", req)

    // 2. 解析服务器能力
    var initResp InitializeResponse
    json.Unmarshal(resp, &initResp)
    c.serverInfo = initResp.ServerInfo

    // 3. 发送 initialized 通知
    c.transport.SendNotification(ctx, "notifications/initialized", nil)
}
```

**握手序列**:
```
Client                  Server
  │                       │
  │ 1. initialize         │
  ├──────────────────────→│
  │                       │
  │←──────────────────────│
  │   serverInfo          │
  │   capabilities        │
  │                       │
  │ 2. notifications/     │
  │    initialized        │
  ├──────────────────────→│
  │                       │
  │ 3. tools/list         │
  ├──────────────────────→│
  │←──────────────────────│
  │   [tool1, tool2, ...]│
  │                       │
  │ Ready! 可以调用工具了  │
```

### 3. Tool Call Layer (工具调用层)

**作用**: 封装工具调用的 JSON-RPC 请求和响应处理

```go
// internal/client/client.go:213-240

func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*types.ToolCallResult, error) {
    // 1. 构建请求
    params := map[string]interface{}{
        "name":      name,
        "arguments": arguments,
    }

    // 2. 发送 JSON-RPC 请求
    resp, err := c.transport.SendRequest(ctx, "tools/call", params)

    // 3. 解析结果
    var result types.ToolCallResult
    json.Unmarshal(resp, &result)

    // 4. 检查是否有错误
    if result.IsError {
        return &result, fmt.Errorf("tool execution failed: %s", result.Content[0].Text)
    }

    return &result, nil
}
```

**请求格式**:
```json
{
  "jsonrpc": "2.0",
  "id": 123,
  "method": "tools/call",
  "params": {
    "name": "detect",
    "arguments": {
      "input_path": "/path/to/image.jpg",
      "confidence": 0.3
    }
  }
}
```

**响应格式**:
```json
{
  "jsonrpc": "2.0",
  "id": 123,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"detections\": [...]}"
      }
    ],
    "isError": false
  }
}
```

### 4. Pipeline Orchestration Layer (编排层)

**作用**: 按顺序执行各个步骤，管理状态，处理错误和重试

```go
// internal/pipeline/pipeline.go:45-107

func (p *Pipeline) Execute(ctx context.Context, input PipelineInput, id string) (*PipelineResult, error) {
    // 1. 加载或创建 manifest (memory)
    manifest, _ := LoadManifest(p.manifestPath)
    if manifest == nil {
        manifest = NewManifest(id, input)
    }

    // 2. 定义执行顺序
    stages := []PipelineStage{
        StageSegmentPerson,
        StageLandmarks,
        StageRenderMotion,
        StageSearchMusic,
        StageCompose,
    }

    // 3. 顺序执行各个 stage
    for _, stage := range stages {
        // 检查是否已完成 (幂等性)
        if manifest.IsStageCompleted(stage) {
            continue
        }

        // 执行 stage
        err := p.executeStageWithRetry(ctx, stage, manifest)
        if err != nil {
            manifest.FailStage(stage, err)
            manifest.Save(p.manifestPath)
            return nil, err
        }

        // 保存进度
        manifest.Save(p.manifestPath)
    }

    // 4. 返回结果
    return manifest.Result, nil
}
```

### 5. Step Implementation Layer (步骤实现层)

**作用**: 实现每个具体步骤的业务逻辑

```go
// internal/pipeline/steps.go

// 步骤 1: 背景移除
func ExecuteSegmentPerson(ctx, p *Pipeline, manifest *Manifest) error {
    // 1. 调用 detect 工具
    detectResult, _ := p.imagesorceryClient.CallTool(ctx, "detect", {...})

    // 2. 解析检测结果
    var response map[string]interface{}
    json.Unmarshal([]byte(detectResult.Content[0].Text), &response)

    // 3. 提取 person polygon
    personPolygon := extractPersonPolygon(response)

    // 4. 调用 fill 工具 (背景透明化)
    fillResult, _ := p.imagesorceryClient.CallTool(ctx, "fill", {
        "input_path": imagePath,
        "areas": [{"polygon": personPolygon, "opacity": 0.0}],
        "invert_areas": true,  // 填充背景而非人物
        "output_path": "/tmp/segmented_person.png",
    })

    // 5. 保存结果到 manifest (memory)
    manifest.CompleteStage(StageSegmentPerson, {
        "segmented_path": "/tmp/segmented_person.png"
    })
    manifest.Result.SegmentedImagePath = "/tmp/segmented_person.png"
}
```

---

## 如何集成新的 MCP Server

### 场景：添加一个 "文字识别 MCP"

假设你要添加一个 OCR MCP Server 来识别图片中的文字。

#### Step 1: 添加服务器配置

**文件**: `configs/agent.yaml`

```yaml
servers:
  # ... 现有的 4 个服务器 ...

  # 新增: OCR 服务器
  ocr:
    name: ocr-mcp-server
    command:
      - /path/to/ocr-mcp/.venv/bin/python
      - /path/to/ocr-mcp/server.py
    transport: stdio
    timeout: 30s
    capabilities:
      tools:
        - extract_text      # 提取文字
        - detect_language   # 检测语言
        - find_text        # 查找特定文字
```

#### Step 2: 创建 MCP 客户端

**文件**: `cmd/agent/main.go`

```go
// 现有代码:
imagesorceryClient, _ := createAndInitClient(ctx, config.Servers["imagesorcery"], "imagesorcery")
yoloClient, _ := createAndInitClient(ctx, config.Servers["yolo"], "yolo")
videoClient, _ := createAndInitClient(ctx, config.Servers["video"], "video")
musicClient, _ := createAndInitClient(ctx, config.Servers["music"], "music")

// 新增:
ocrClient, err := createAndInitClient(ctx, config.Servers["ocr"], "ocr")
if err != nil {
    log.Fatalf("Failed to initialize OCR client: %v", err)
}
defer ocrClient.Close()

// 验证工具可用性
if err := validateServerTools(ctx, ocrClient, config.Servers["ocr"]); err != nil {
    log.Fatalf("OCR server validation failed: %v", err)
}
```

#### Step 3: 添加到 Pipeline

**文件**: `internal/pipeline/pipeline.go`

```go
type Pipeline struct {
    imagesorceryClient client.MCPClient
    yoloClient         client.MCPClient
    videoClient        client.MCPClient
    musicClient        client.MCPClient
    ocrClient          client.MCPClient  // 新增
    // ...
}

func NewPipeline(
    imagesorceryClient, yoloClient, videoClient, musicClient, ocrClient client.MCPClient,  // 新增参数
    enableMotion bool, maxRetries int, manifestPath string,
) *Pipeline {
    return &Pipeline{
        imagesorceryClient: imagesorceryClient,
        yoloClient:         yoloClient,
        videoClient:        videoClient,
        musicClient:        musicClient,
        ocrClient:          ocrClient,  // 新增
        // ...
    }
}
```

#### Step 4: 定义新的 Stage

**文件**: `pkg/types/types.go`

```go
const (
    StageInit           PipelineStage = "init"
    StageSegmentPerson  PipelineStage = "segment_person"
    StageLandmarks      PipelineStage = "estimate_landmarks"
    StageExtractText    PipelineStage = "extract_text"        // 新增
    StageRenderMotion   PipelineStage = "render_motion"
    StageSearchMusic    PipelineStage = "search_music"
    StageCompose        PipelineStage = "compose"
    StageComplete       PipelineStage = "complete"
)
```

#### Step 5: 实现 Step 函数

**文件**: `internal/pipeline/steps.go`

```go
// 新增: OCR 文字提取
func ExecuteExtractText(ctx context.Context, p *Pipeline, manifest *Manifest) error {
    // 从 manifest 获取输入
    imagePath := manifest.Result.SegmentedImagePath
    if imagePath == "" {
        imagePath = manifest.Input.ImagePath
    }

    log.Println("Extracting text from image...")

    // 调用 OCR 工具
    result, err := p.ocrClient.CallTool(ctx, "extract_text", map[string]interface{}{
        "image_path": imagePath,
        "language":   "auto",  // 自动检测语言
    })
    if err != nil {
        return fmt.Errorf("extract_text tool failed: %w", err)
    }

    // 解析结果
    if len(result.Content) == 0 {
        return fmt.Errorf("OCR returned no content")
    }

    extractedText := result.Content[0].Text
    log.Printf("Extracted text: %s", extractedText)

    // 保存结果
    if err := manifest.CompleteStage(types.StageExtractText, map[string]string{
        "text": extractedText,
    }); err != nil {
        return err
    }

    // 更新 manifest
    manifest.Result.ExtractedText = extractedText  // 需要先在 PipelineResult 中添加这个字段

    return nil
}

// 注册 step 函数
func GetStepForStage(stage types.PipelineStage) (StepFunc, error) {
    switch stage {
    case types.StageSegmentPerson:
        return ExecuteSegmentPerson, nil
    case types.StageLandmarks:
        return ExecuteEstimateLandmarks, nil
    case types.StageExtractText:  // 新增
        return ExecuteExtractText, nil
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
```

#### Step 6: 添加到执行顺序

**文件**: `internal/pipeline/pipeline.go`

```go
func (p *Pipeline) Execute(ctx context.Context, input types.PipelineInput, pipelineID string) (*PipelineResult, error) {
    // ...

    stages := []types.PipelineStage{
        types.StageSegmentPerson,
        types.StageLandmarks,
        types.StageExtractText,     // 新增: 在这里插入
        types.StageRenderMotion,
        types.StageSearchMusic,
        types.StageCompose,
    }

    // ...
}
```

#### Step 7: 更新 PipelineResult

**文件**: `internal/pipeline/manifest.go`

```go
type PipelineResult struct {
    SegmentedImagePath string   `json:"segmented_image_path,omitempty"`
    LandmarksData      string   `json:"landmarks_data,omitempty"`
    ExtractedText      string   `json:"extracted_text,omitempty"`  // 新增
    MotionVideoPath    string   `json:"motion_video_path,omitempty"`
    MusicTracks        []string `json:"music_tracks,omitempty"`
    FinalOutputPath    string   `json:"final_output_path,omitempty"`
}
```

#### Step 8: 更新主函数

**文件**: `cmd/agent/main.go`

```go
// 创建 pipeline，传入新的客户端
pipe := pipeline.NewPipeline(
    imagesorceryClient,
    yoloClient,
    videoClient,
    musicClient,
    ocrClient,  // 新增
    config.Pipeline.EnableMotion,
    config.Pipeline.MaxRetries,
    *manifestPath,
)

// 执行后显示结果
result, err := pipe.Execute(ctx, input, *pipelineID)
if err != nil {
    log.Fatalf("Pipeline execution failed: %v", err)
}

log.Printf("Extracted Text: %s", result.ExtractedText)  // 新增
```

### 完整的集成 Checklist

```
□ 1. 配置文件 (configs/agent.yaml)
    └─ 添加服务器配置

□ 2. 类型定义 (pkg/types/types.go)
    └─ 添加新的 PipelineStage 常量

□ 3. Pipeline 结构 (internal/pipeline/pipeline.go)
    ├─ Pipeline struct 添加新客户端字段
    ├─ NewPipeline() 添加参数
    └─ Execute() 添加到 stages 数组

□ 4. Step 实现 (internal/pipeline/steps.go)
    ├─ 实现 ExecuteXXX() 函数
    └─ GetStepForStage() 添加 case

□ 5. Manifest (internal/pipeline/manifest.go)
    └─ PipelineResult 添加新字段

□ 6. 主程序 (cmd/agent/main.go)
    ├─ createAndInitClient() 创建客户端
    ├─ validateServerTools() 验证工具
    ├─ NewPipeline() 传入客户端
    └─ 显示结果

□ 7. 文档更新
    ├─ README.md 更新 pipeline 说明
    ├─ configs/agent.yaml 添加注释
    └─ CLAUDE.md 更新架构说明
```

---

## Framework vs Application 代码清单

### ✅ Framework (可提取复用)

这些代码构成了一个**通用的 MCP 客户端框架**:

```
通用 MCP Framework:
├── internal/client/
│   ├── client.go              # MCP 协议实现
│   ├── stdio.go               # Stdio 传输
│   ├── mark3labs_transport.go # HTTP/SSE 传输
│   └── discovery.go           # 工具发现
├── pkg/types/
│   └── types.go               # MCP 数据结构
│       ├── Tool
│       ├── ToolCallResult
│       ├── ContentBlock
│       └── ServerConfig
└── go.mod                     # 依赖: mark3labs/mcp-go
```

**如何提取为独立库**:

```go
// 创建新项目: github.com/yourname/go-mcp-client

package mcpclient

// 复制 internal/client/* 到这里
// 复制 pkg/types/types.go 到这里

// 使用方式:
import "github.com/yourname/go-mcp-client"

client := mcpclient.NewStdioClient([]string{"python", "-m", "server"})
client.Connect(ctx)
client.Initialize(ctx)
tools, _ := client.ListTools(ctx)
result, _ := client.CallTool(ctx, "my_tool", args)
```

### ⚙️ Application (项目特定)

这些代码是**视频生成任务专用的**:

```
视频生成 Application:
├── internal/pipeline/
│   ├── pipeline.go      # 5 步编排逻辑
│   ├── manifest.go      # 状态持久化 (可调整)
│   └── steps.go         # 各步骤实现 (特定任务)
│       ├── ExecuteSegmentPerson
│       ├── ExecuteEstimateLandmarks
│       ├── ExecuteRenderMotion
│       ├── ExecuteSearchMusic
│       └── ExecuteCompose
├── cmd/agent/main.go    # CLI 入口
├── configs/agent.yaml   # 4 个服务器配置
└── scripts/             # 辅助脚本
```

---

## 总结

### 这个项目的本质

```
不是: AI Agent (有推理和决策能力)
而是: Workflow Orchestrator (工作流编排器)

类比:
┌─────────────────┐     ┌─────────────────┐
│ AI Agent        │     │ This Project    │
├─────────────────┤     ├─────────────────┤
│ ChatGPT         │     │ Zapier          │
│ AutoGPT         │     │ Apache Airflow  │
│ LangChain Agent │     │ GitHub Actions  │
└─────────────────┘     └─────────────────┘
    动态决策                 固定流程
```

### 三要素对应

| 元素 | 传统 AI Agent | 这个项目 |
|------|--------------|---------|
| **Model** | LLM (GPT-4, Claude) | ❌ 无 (硬编码流程) |
| **Memory** | Vector DB, Chat History | ✅ JSON Manifest |
| **Tools** | Function Calling | ✅ 4 个 MCP Servers |

### 可复用部分

1. **MCP Client Framework** (80% 通用)
   - Transport 层 (stdio, HTTP)
   - Protocol 层 (initialize, tools/list, tools/call)
   - 可提取为独立 Go 库

2. **Manifest Pattern** (50% 通用)
   - 状态持久化模式可复用
   - 需要根据业务调整字段

3. **Pipeline Pattern** (20% 通用)
   - 顺序执行 + 重试 + 幂等性的模式可复用
   - 具体步骤实现是业务相关的

### 关键设计模式

1. **Strategy Pattern** (Transport)
   - Stdio, WebSocket, HTTP 实现同一接口
   - 可以轻松切换传输方式

2. **Template Method** (Pipeline)
   - Pipeline 定义骨架流程
   - 各 Step 实现具体逻辑

3. **Memento Pattern** (Manifest)
   - 保存和恢复 Pipeline 状态
   - 支持断点续传

4. **Factory Pattern** (CreateClient)
   - 根据配置创建不同 Transport
   - 统一的创建接口

---

想了解更多细节？可以问我：
- 某个具体函数的实现原理
- 如何优化某个部分
- 如何转换为真正的 AI Agent (加入 LLM)
- 如何提取 Framework 代码
