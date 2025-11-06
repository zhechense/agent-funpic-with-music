.PHONY: help build test lint clean run install-deps install-mcp-servers

# Default target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build & Run:"
	@echo "  build                - Build the agent binary"
	@echo "  run                  - Run agent with sample config"
	@echo "  clean                - Remove build artifacts"
	@echo ""
	@echo "Testing:"
	@echo "  test                 - Run all tests"
	@echo "  test-verbose         - Run tests with verbose output"
	@echo "  test-coverage        - Run tests with coverage report"
	@echo "  test-mcp-servers     - Test MCP server installations"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt                  - Format code with gofmt"
	@echo "  vet                  - Run go vet"
	@echo "  lint                 - Run golangci-lint"
	@echo "  check                - Run all checks (fmt, vet, lint, test)"
	@echo ""
	@echo "Dependencies:"
	@echo "  install-deps         - Install Go development dependencies"
	@echo "  install-mcp-servers  - Install all MCP servers (Python venvs)"
	@echo "  install-imagesorcery - Install ImageSorcery MCP"
	@echo "  install-yolo         - Install YOLO Service"
	@echo "  install-video-audio  - Install Video-Audio MCP"

# Build the agent binary
build:
	@echo "Building agent..."
	go build -o bin/agent cmd/agent/main.go
	@echo "Build complete: bin/agent"

# Run all tests
test:
	@echo "Running tests..."
	go test ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run a single test package
test-client:
	@echo "Running client tests..."
	go test -v ./internal/client/...

test-pipeline:
	@echo "Running pipeline tests..."
	go test -v ./internal/pipeline/...

# Run golangci-lint
lint:
	@echo "Running golangci-lint..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed, run: make install-deps"; exit 1)
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	gofmt -w -s .

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f .pipeline_manifest.json
	@echo "Clean complete"

# Run agent with sample configuration
run:
	@echo "Running agent..."
	@if [ ! -f bin/agent ]; then \
		$(MAKE) build; \
	fi
	./bin/agent --config configs/agent.yaml --image image/image.png --duration 3

# Run with custom parameters
run-custom:
	@echo "Running agent with custom parameters..."
	@if [ ! -f bin/agent ]; then \
		$(MAKE) build; \
	fi
	@read -p "Enter image path: " img; \
	read -p "Enter duration (seconds): " dur; \
	./bin/agent --config configs/agent.yaml --image $$img --duration $$dur

# Install development dependencies
install-deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "Installing golangci-lint..."
	@which golangci-lint > /dev/null || \
		(curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin)
	@echo "Dependencies installed"

# Download go modules
mod-download:
	@echo "Downloading Go modules..."
	go mod download

# Tidy go modules
mod-tidy:
	@echo "Tidying Go modules..."
	go mod tidy

# Verify go modules
mod-verify:
	@echo "Verifying Go modules..."
	go mod verify

# Development: watch and rebuild on changes (requires fswatch)
watch:
	@echo "Watching for changes (requires fswatch)..."
	@which fswatch > /dev/null || (echo "fswatch not installed"; exit 1)
	fswatch -o . | xargs -n1 -I{} make build

# Check everything before commit
check: fmt vet lint test
	@echo "All checks passed!"

# ============================================================================
# MCP Server Installation
# ============================================================================

# Install all MCP servers
install-mcp-servers: install-imagesorcery install-yolo install-video-audio
	@echo ""
	@echo "✅ All MCP servers installed!"
	@echo ""
	@echo "To test the servers:"
	@echo "  make test-mcp-servers"

# Install ImageSorcery MCP
install-imagesorcery:
	@echo "📦 Installing ImageSorcery MCP..."
	@if [ -d "mcp-servers/imagesorcery-env" ]; then \
		echo "ImageSorcery already installed, skipping..."; \
	else \
		echo "Creating Python venv for ImageSorcery..."; \
		cd mcp-servers && python3 -m venv imagesorcery-env; \
		echo "Installing ImageSorcery..."; \
		./mcp-servers/imagesorcery-env/bin/pip install --upgrade pip; \
		./mcp-servers/imagesorcery-env/bin/pip install imagesorcery-mcp; \
		./mcp-servers/imagesorcery-env/bin/imagesorcery-mcp --post-install; \
		echo "✅ ImageSorcery installed at mcp-servers/imagesorcery-env/"; \
	fi

# Install YOLO Service
install-yolo:
	@echo "📦 Installing YOLO Service..."
	@if [ -d "mcp-servers/yolo-service/.venv" ]; then \
		echo "YOLO Service already installed, skipping..."; \
	else \
		echo "Creating Python venv for YOLO..."; \
		cd mcp-servers/yolo-service && python3 -m venv .venv; \
		echo "Installing YOLO dependencies..."; \
		./mcp-servers/yolo-service/.venv/bin/pip install --upgrade pip; \
		./mcp-servers/yolo-service/.venv/bin/pip install -r mcp-servers/yolo-service/requirements.txt; \
		echo "Downloading YOLO pose model..."; \
		mkdir -p mcp-servers/yolo-service/models; \
		cd mcp-servers/yolo-service && .venv/bin/python -c "from ultralytics import YOLO; YOLO('yolov8n-pose.pt')"; \
		echo "✅ YOLO Service installed at mcp-servers/yolo-service/"; \
	fi

# Install Video-Audio MCP
install-video-audio:
	@echo "📦 Installing Video-Audio MCP..."
	@which uv > /dev/null || (echo "❌ uv not found. Install it with: curl -LsSf https://astral.sh/uv/install.sh | sh"; exit 1)
	@which ffmpeg > /dev/null || (echo "⚠️  ffmpeg not found. Install it with: brew install ffmpeg" && exit 1)
	@if [ -d "mcp-servers/video-audio-mcp/.venv" ]; then \
		echo "Video-Audio MCP already installed, skipping..."; \
	else \
		echo "Installing Video-Audio MCP with uv..."; \
		cd mcp-servers/video-audio-mcp && uv sync; \
		echo "✅ Video-Audio MCP installed at mcp-servers/video-audio-mcp/"; \
	fi

# Test MCP server installations
test-mcp-servers:
	@echo "🧪 Testing MCP server installations..."
	@echo ""
	@echo "1. Testing ImageSorcery..."
	@./mcp-servers/imagesorcery-env/bin/python -c "import imagesorcery_mcp; print('   ✅ ImageSorcery OK')" || echo "   ❌ ImageSorcery FAILED"
	@echo ""
	@echo "2. Testing YOLO Service..."
	@./mcp-servers/yolo-service/.venv/bin/python -c "import ultralytics; print('   ✅ YOLO OK')" || echo "   ❌ YOLO FAILED"
	@echo ""
	@echo "3. Testing Video-Audio MCP..."
	@cd mcp-servers/video-audio-mcp && uv run python -c "import ffmpeg; print('   ✅ Video-Audio OK')" || echo "   ❌ Video-Audio FAILED"
	@echo ""
	@which ffmpeg > /dev/null && echo "4. FFmpeg: ✅ $(shell ffmpeg -version | head -1)" || echo "4. FFmpeg: ❌ NOT INSTALLED"
	@echo ""
	@echo "Note: Epidemic Sound MCP is a remote service (no local installation needed)"

# Clean MCP server installations (use with caution)
clean-mcp-servers:
	@echo "⚠️  This will remove all MCP server installations!"
	@read -p "Are you sure? (yes/no): " confirm && [ "$$confirm" = "yes" ] || (echo "Cancelled"; exit 1)
	@echo "Removing MCP servers..."
	@rm -rf mcp-servers/imagesorcery-env
	@rm -rf mcp-servers/yolo-service/.venv
	@rm -rf mcp-servers/video-audio-mcp/.venv
	@echo "✅ MCP servers removed"
