# Dockerfile for vLLM GPU Worker
# This container runs both vLLM inference server and the job worker process
FROM vllm/vllm-openai:latest

# Install Python dependencies for worker
WORKDIR /app

# Copy worker code
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY common.py .
COPY worker.py .
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

# Environment variables (can be overridden at runtime)
ENV REDIS_URL=redis://redis:6379/0
ENV VLLM_URL=http://localhost:8000/v1/completions
ENV WORKER_ID=worker-1
ENV MODEL=TheBloke/Mistral-7B-Instruct-v0.2-AWQ
ENV PORT=8000

EXPOSE 8000

# Start both vLLM server and worker process
ENTRYPOINT ["/app/entrypoint.sh"]
