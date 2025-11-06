#!/usr/bin/env bash
# Batch process multiple images with agent-funpic-act
#
# Usage:
#   ./scripts/batch-process.sh <pattern> [duration]
#
# Examples:
#   ./scripts/batch-process.sh "images/*.jpg"
#   ./scripts/batch-process.sh "images/*.jpg" 15.0
#   ./scripts/batch-process.sh "image1.jpg image2.jpg image3.jpg" 10.0

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Change to project root
cd "$PROJECT_ROOT"

# Check arguments
if [ $# -lt 1 ]; then
    echo -e "${RED}Error: Missing file pattern${NC}"
    echo ""
    echo "Usage: $0 <pattern> [duration]"
    echo ""
    echo "Examples:"
    echo "  $0 'images/*.jpg'"
    echo "  $0 'images/*.jpg' 15.0"
    echo "  $0 'image1.jpg image2.jpg' 10.0"
    exit 1
fi

PATTERN="$1"
DURATION="${2:-10.0}"

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

# Expand pattern to files
FILES=($PATTERN)

# Check if any files found
if [ ${#FILES[@]} -eq 0 ]; then
    echo -e "${RED}Error: No files found matching pattern: $PATTERN${NC}"
    exit 1
fi

echo -e "${BLUE}Found ${#FILES[@]} file(s) to process${NC}"
echo "Duration: ${DURATION}s per video"
echo ""

# Create output directory with timestamp
OUTPUT_DIR="output/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$OUTPUT_DIR"

# Process each file
SUCCESS_COUNT=0
FAIL_COUNT=0

for i in "${!FILES[@]}"; do
    FILE="${FILES[$i]}"
    NUM=$((i + 1))

    echo -e "${BLUE}[$NUM/${#FILES[@]}] Processing: $FILE${NC}"

    # Skip if not a file
    if [ ! -f "$FILE" ]; then
        echo -e "${YELLOW}  Skipping (not a file)${NC}"
        continue
    fi

    # Get basename without extension
    BASENAME=$(basename "$FILE" | sed 's/\.[^.]*$//')
    PIPELINE_ID="batch_${BASENAME}_$(date +%s)"

    # Run agent
    if ./bin/agent --image "$FILE" --duration "$DURATION" --id "$PIPELINE_ID"; then
        echo -e "${GREEN}  ✓ Success${NC}"
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))

        # Copy output files to organized directory
        [ -f "/tmp/final_headshake_with_music.mp4" ] && \
            cp "/tmp/final_headshake_with_music.mp4" "$OUTPUT_DIR/${BASENAME}_final.mp4"
        [ -f "/tmp/headshake_animation.mp4" ] && \
            cp "/tmp/headshake_animation.mp4" "$OUTPUT_DIR/${BASENAME}_animation.mp4"
        [ -f "/tmp/segmented_person.png" ] && \
            cp "/tmp/segmented_person.png" "$OUTPUT_DIR/${BASENAME}_segmented.png"
    else
        echo -e "${RED}  ✗ Failed${NC}"
        FAIL_COUNT=$((FAIL_COUNT + 1))

        # Save manifest for debugging
        [ -f ".pipeline_manifest.json" ] && \
            cp ".pipeline_manifest.json" "$OUTPUT_DIR/${BASENAME}_manifest_error.json"
    fi

    echo ""
done

# Summary
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Batch Processing Complete${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "Total files: ${#FILES[@]}"
echo -e "${GREEN}Success: $SUCCESS_COUNT${NC}"
if [ $FAIL_COUNT -gt 0 ]; then
    echo -e "${RED}Failed: $FAIL_COUNT${NC}"
fi
echo ""
echo "Output directory: $OUTPUT_DIR"

# Open output directory (macOS)
if command -v open &> /dev/null; then
    echo ""
    read -p "Open output directory? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        open "$OUTPUT_DIR"
    fi
fi
