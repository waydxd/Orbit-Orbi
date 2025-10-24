#!/bin/bash

# Example script to run Orbi chatbot with configuration

# Set environment variables
export CALENDAR_SERVICE_ADDR="${CALENDAR_SERVICE_ADDR:-localhost:50051}"
export OPENAI_API_KEY="${OPENAI_API_KEY:-your-api-key-here}"
export OPENAI_MODEL="${OPENAI_MODEL:-gpt-3.5-turbo}"

echo "Starting Orbi with configuration:"
echo "  Calendar Service: $CALENDAR_SERVICE_ADDR"
echo "  OpenAI Model: $OPENAI_MODEL"
echo ""

# Run Orbi
./bin/orbi
