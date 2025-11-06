package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	"github.com/zhe.chen/agent-funpic-act/internal/client"
	"github.com/zhe.chen/agent-funpic-act/internal/llm"
	"github.com/zhe.chen/agent-funpic-act/internal/llm/providers/claude"
	"github.com/zhe.chen/agent-funpic-act/internal/llm/providers/gemini"
	"github.com/zhe.chen/agent-funpic-act/internal/llm/providers/openai"
	"github.com/zhe.chen/agent-funpic-act/internal/pipeline"
	"github.com/zhe.chen/agent-funpic-act/pkg/types"
)

// createLLMProvider creates the appropriate LLM provider based on configuration
func createLLMProvider(config types.LLMConfig) (llm.Provider, error) {
	switch config.Provider {
	case "anthropic", "claude":
		return claude.NewProvider(config.Anthropic)

	case "google", "gemini":
		return gemini.NewProvider(config.Google)

	case "openai":
		return openai.NewProvider(config.OpenAI)

	case "":
		return nil, fmt.Errorf("llm.provider not specified in config")

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: anthropic, google, openai)", config.Provider)
	}
}

func main() {
	// Load .env file (ignore error if file doesn't exist)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Parse command-line flags
	var (
		configPath   = flag.String("config", "configs/agent.yaml", "Path to configuration file")
		imagePath    = flag.String("image", "", "Path to input image (required)")
		duration     = flag.Float64("duration", 10.0, "Target duration in seconds")
		userPrompt   = flag.String("prompt", "", "Your request (e.g., 'make a shake animation')")
		manifestPath = flag.String("manifest", "", "Path to pipeline manifest (default: from config)")
		pipelineID   = flag.String("id", "", "Pipeline ID for resume (default: auto-generate)")
		outputDir    = flag.String("output", "output", "Output directory for generated files")
	)
	flag.Parse()

	// Validate required flags
	if *imagePath == "" {
		log.Fatal("Error: --image flag is required")
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()

	// Load configuration
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate prompt requirement for Full AI mode
	if config.LLM.Mode == "full_ai" && *userPrompt == "" {
		log.Fatal("Error: --prompt flag is required in Full AI mode.\nExample: --prompt \"Generate a shake animation with the character's head moving left and right\"")
	}

	// Set manifest path
	if *manifestPath == "" {
		*manifestPath = config.Pipeline.ManifestPath
	}

	// Generate pipeline ID if not provided
	if *pipelineID == "" {
		*pipelineID = fmt.Sprintf("pipeline-%d", time.Now().Unix())
	}

	log.Printf("Starting agent-funpic-act")
	log.Printf("Pipeline ID: %s", *pipelineID)
	log.Printf("Image: %s", *imagePath)
	log.Printf("Duration: %.1fs", *duration)
	log.Printf("Output Directory: %s", *outputDir)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Create temporary directory for intermediate files
	tempDir := fmt.Sprintf(".pipeline_tmp/%s", *pipelineID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Fatalf("Failed to create temporary directory: %v", err)
	}
	log.Printf("Temporary Directory: %s", tempDir)

	// Create and initialize MCP clients
	imagesorceryClient, err := createAndInitClient(ctx, config.Servers["imagesorcery"], "imagesorcery")
	if err != nil {
		log.Fatalf("Failed to initialize imagesorcery client: %v", err)
	}
	defer imagesorceryClient.Close()

	yoloClient, err := createAndInitClient(ctx, config.Servers["yolo"], "yolo")
	if err != nil {
		log.Fatalf("Failed to initialize yolo client: %v", err)
	}
	defer yoloClient.Close()

	videoClient, err := createAndInitClient(ctx, config.Servers["video"], "video")
	if err != nil {
		log.Fatalf("Failed to initialize video client: %v", err)
	}
	defer videoClient.Close()

	musicClient, err := createAndInitClient(ctx, config.Servers["music"], "music")
	if err != nil {
		log.Fatalf("Failed to initialize music client: %v", err)
	}
	defer musicClient.Close()

	// Validate tools availability
	if err := validateServerTools(ctx, imagesorceryClient, config.Servers["imagesorcery"]); err != nil {
		log.Fatalf("ImageSorcery server validation failed: %v", err)
	}

	if err := validateServerTools(ctx, yoloClient, config.Servers["yolo"]); err != nil {
		log.Fatalf("YOLO server validation failed: %v", err)
	}

	if err := validateServerTools(ctx, videoClient, config.Servers["video"]); err != nil {
		log.Fatalf("Video server validation failed: %v", err)
	}

	if err := validateServerTools(ctx, musicClient, config.Servers["music"]); err != nil {
		log.Fatalf("Music server validation failed: %v", err)
	}

	// Initialize LLM provider (AI Agent feature)
	var llmProvider llm.Provider
	if config.LLM.Enabled {
		log.Printf("[AI Agent] Initializing LLM provider: %s...", config.LLM.Provider)
		provider, err := createLLMProvider(config.LLM)
		if err != nil {
			log.Fatalf("Failed to create LLM provider: %v", err)
		}
		llmProvider = provider
		if llmProvider.IsEnabled() {
			log.Printf("[AI Agent] %s enabled (mode: %s)", llmProvider.Name(), config.LLM.Mode)
		} else {
			log.Printf("[AI Agent] %s disabled (no API key)", llmProvider.Name())
		}
	} else {
		log.Println("[AI Agent] LLM features disabled in config")
		// Create disabled Claude provider as fallback
		llmProvider, _ = createLLMProvider(types.LLMConfig{
			Provider:  "anthropic",
			Anthropic: types.AnthropicConfig{APIKey: ""},
		})
	}

	// Determine AI mode (default to "lightweight" if not specified)
	aiMode := config.LLM.Mode
	if aiMode == "" {
		aiMode = "lightweight"
	}

	// Create pipeline with all 4 MCP clients + LLM provider
	pipe := pipeline.NewPipeline(
		imagesorceryClient,
		yoloClient,
		videoClient,
		musicClient,
		llmProvider,
		config.Pipeline.EnableMotion,
		config.Pipeline.MaxRetries,
		*manifestPath,
		aiMode,
	)

	// Prepare input
	input := types.PipelineInput{
		ImagePath:  *imagePath,
		Duration:   *duration,
		UserPrompt: *userPrompt,
		OutputDir:  *outputDir,
		TempDir:    tempDir,
	}

	// Validate input
	if err := pipeline.ValidateInput(input); err != nil {
		log.Fatalf("Invalid input: %v", err)
	}

	// Execute pipeline
	log.Println("Starting pipeline execution...")
	result, err := pipe.Execute(ctx, input, *pipelineID)
	if err != nil {
		log.Fatalf("Pipeline execution failed: %v", err)
	}

	// Display results
	log.Println("\n=== Pipeline Completed Successfully ===")
	log.Printf("Segmented Image: %s", result.SegmentedImagePath)
	log.Printf("Landmarks Data: %s", result.LandmarksData)
	if result.MotionVideoPath != "" {
		log.Printf("Motion Video: %s", result.MotionVideoPath)
	}
	log.Printf("Music Tracks: %v", result.MusicTracks)
	log.Printf("Final Output: %s", result.FinalOutputPath)
	log.Println("=======================================")
}

// loadConfig reads and parses the YAML configuration file
func loadConfig(path string) (*types.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the config file
	expandedData := os.ExpandEnv(string(data))

	var config types.Config
	if err := yaml.Unmarshal([]byte(expandedData), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// createAndInitClient creates an MCP client, connects, and initializes
func createAndInitClient(ctx context.Context, config types.ServerConfig, name string) (client.MCPClient, error) {
	log.Printf("Connecting to %s server...", name)

	mcpClient, err := client.CreateClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Connect to server
	if err := mcpClient.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	// Initialize MCP protocol
	if err := mcpClient.Initialize(ctx); err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	serverName, serverVersion := mcpClient.GetServerInfo()
	log.Printf("Connected to %s v%s", serverName, serverVersion)

	return mcpClient, nil
}

// validateServerTools checks if required tools are available
func validateServerTools(ctx context.Context, mcpClient client.MCPClient, config types.ServerConfig) error {
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	log.Printf("Server provides %d tools", len(tools))

	// Validate required tools
	if err := client.ValidateTools(tools, config.Capabilities.Tools); err != nil {
		return err
	}

	log.Printf("All required tools available: %v", config.Capabilities.Tools)
	return nil
}
