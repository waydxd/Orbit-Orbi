#!/bin/bash
# entrypoint.sh: Start vLLM server and worker process in parallel

set -e

# Extract model from environment or use default
MODEL="${MODEL:-TheBloke/Mistral-7B-Instruct-v0.2-AWQ}"
PORT="${PORT:-8000}"

echo "Starting vLLM server with model: $MODEL on port $PORT"

# Start vLLM in the background
python -m vllm.entrypoints.openai.api_server \
  --model "$MODEL" \
  --host 0.0.0.0 \
  --port "$PORT" &

VLLM_PID=$!

# Give vLLM some time to start up
echo "Waiting for vLLM to initialize..."
sleep 10

# Check if vLLM is healthy
for i in {1..30}; do
  if curl -s "http://localhost:$PORT/health" > /dev/null 2>&1; then
    echo "vLLM is healthy"
    break
  fi
  echo "Attempt $i: vLLM not ready yet, waiting..."
  sleep 2
done

echo "Starting worker process..."

# Start worker in the foreground (main process)
python /app/worker.py

# If worker exits, kill vLLM
kill $VLLM_PID 2>/dev/null || true
