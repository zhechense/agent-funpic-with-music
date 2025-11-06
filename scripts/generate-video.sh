#!/usr/bin/env bash
# Generate video from image using agent-funpic-act
#
# Usage:
#   ./scripts/generate-video.sh <image-path> [duration]
#
# Examples:
#   ./scripts/generate-video.sh myimage.jpg
#   ./scripts/generate-video.sh myimage.jpg 15.0

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Change to project root
cd "$PROJECT_ROOT"

# Check arguments
if [ $# -lt 1 ]; then
    echo -e "${RED}Error: Missing image path${NC}"
    echo ""
    echo "Usage: $0 <image-path> [duration]"
    echo ""
    echo "Examples:"
    echo "  $0 myimage.jpg"
    echo "  $0 myimage.jpg 15.0"
    exit 1
fi

IMAGE_PATH="$1"
DURATION="${2:-10.0}"

# Validate image exists
if [ ! -f "$IMAGE_PATH" ]; then
    echo -e "${RED}Error: Image file not found: $IMAGE_PATH${NC}"
    exit 1
fi

# Check if agent is built
if [ ! -f "./bin/agent" ]; then
    echo -e "${YELLOW}Agent not built, building now...${NC}"
    make build
fi

# Check .env file
if [ ! -f ".env" ]; then
    echo -e "${RED}Error: .env file not found${NC}"
    echo "Please create .env from template:"
    echo "  cp .env.template .env"
    echo "  # Edit .env and add your EPIDEMIC_SOUND_TOKEN"
    exit 1
fi

# Run agent
echo -e "${GREEN}Generating video...${NC}"
echo "Image: $IMAGE_PATH"
echo "Duration: ${DURATION}s"
echo ""

./bin/agent --image "$IMAGE_PATH" --duration "$DURATION"

# Check if successful
if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✓ Video generation completed!${NC}"
    echo ""
    echo "Output files:"
    [ -f "/tmp/segmented_person.png" ] && echo "  - Segmented image: /tmp/segmented_person.png"
    [ -f "/tmp/headshake_animation.mp4" ] && echo "  - Animation: /tmp/headshake_animation.mp4"
    [ -f "/tmp/final_headshake_with_music.mp4" ] && echo "  - Final video: /tmp/final_headshake_with_music.mp4"
else
    echo -e "${RED}✗ Video generation failed${NC}"
    echo "Check the error messages above for details"
    exit 1
fi
