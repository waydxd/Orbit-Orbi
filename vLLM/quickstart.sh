#!/bin/bash
# Quick start script for the message broker LLM architecture

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "╔════════════════════════════════════════════════════════╗"
echo "║  Orbit-Orbi: Message Broker LLM Architecture          ║"
echo "║  Quick Start Script                                   ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}✗ Docker is not installed${NC}"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}✗ Docker Compose is not installed${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Docker and Docker Compose found${NC}"
echo ""

# Check Redis connectivity (if already running)
if command -v redis-cli &> /dev/null; then
    if redis-cli ping &> /dev/null; then
        echo -e "${YELLOW}⚠ Redis is already running on localhost:6379${NC}"
    fi
fi

echo ""
echo "Starting services..."
echo ""

cd "$PROJECT_ROOT"

# Start services
echo "Building and starting Docker containers..."
docker-compose up -d

echo ""
echo "Waiting for services to be ready..."
sleep 10

# Health checks
echo ""
echo "Performing health checks..."
echo ""

# Check Redis
echo -n "Redis: "
if docker exec orbi-redis redis-cli ping &> /dev/null; then
    echo -e "${GREEN}✓ Healthy${NC}"
else
    echo -e "${RED}✗ Not responding${NC}"
    exit 1
fi

# Check vLLM
echo -n "vLLM Server: "
if docker exec orbi-vllm-worker curl -s http://localhost:8000/health &> /dev/null; then
    echo -e "${GREEN}✓ Healthy${NC}"
else
    echo -e "${YELLOW}⚠ Still initializing (this is normal)${NC}"
fi

echo ""
echo "╔════════════════════════════════════════════════════════╗"
echo "║  Services Started Successfully                         ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""
echo "Available services:"
echo "  • Redis:        redis://localhost:6379"
echo "  • vLLM API:     http://localhost:8000"
echo "  • Orbi Chatbot: (check docker logs orbi-chatbot)"
echo ""
echo "Useful commands:"
echo "  • View logs:       docker logs -f orbi-vllm-worker"
echo "  • Redis CLI:       docker exec -it orbi-redis redis-cli"
echo "  • Run tests:       python vLLM/test_integration.py"
echo ""
echo "Next steps:"
echo "  1. Run integration tests: python vLLM/test_integration.py"
echo "  2. Submit a test job via Python or Go client"
echo "  3. Integrate the Go client into your Orbi module"
echo ""
echo -e "${GREEN}Setup complete!${NC}"
