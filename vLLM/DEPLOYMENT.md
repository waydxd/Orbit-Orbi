# Message Broker Architecture - Deployment & Usage Guide

## Overview

This document describes the **GPU-backed LLM inference architecture** using Redis as a message broker. The system separates the API layer (Go web server) from GPU inference (vLLM workers).

### Architecture Diagram

```
┌──────────────┐
│   Clients    │
└──────┬───────┘
       │ HTTP /ask
       ▼
┌──────────────────────────────┐
│   Go Web Server              │
│  (Orbi LangChain Module)     │
│  - Accepts requests          │
│  - Enqueues to Redis         │
│  - Awaits results            │
└──────┬───────────────────────┘
       │ LPUSH request
       ▼
   ┌────────────┐
   │    Redis   │
   │   Broker   │
   │            │
   │  Queues:   │
   │  - Requests│
   │  - Results │
   └────┬───────┘
        │ BRPOP job
        ▼
┌──────────────────────────────┐
│  vLLM GPU Workers            │
│  (Docker containers)         │
│                              │
│  1. Pull job from queue      │
│  2. Call vLLM locally        │
│  3. Push result to Redis     │
└──────────────────────────────┘
```

---

## Components

### 1. **Redis Message Broker**
- **Role**: Central queue system
- **Image**: `redis:7-alpine`
- **Port**: `6379`
- **Queues**:
  - `llm:queue:requests`: Incoming inference requests
  - `llm:result:{job_id}`: Per-job result channel

### 2. **vLLM GPU Workers**
- **Role**: Perform LLM inference
- **Image**: `vllm/vllm-openai:latest` + custom worker script
- **Port**: `8000` (OpenAI-compatible API)
- **Components**:
  - **vLLM Server**: Runs inference on quantized Mistral-7B-AWQ
  - **Worker Process**: Pulls jobs from Redis, calls vLLM, pushes results

### 3. **Go Web Server** (Orbi)
- **Role**: Public API endpoint
- **Package**: `github.com/waydxd/Orbit-Orbi/pkg/llm`
- **Function**: Submits requests and retrieves results

---

## File Structure

```
vLLM/
├── common.py              # Job envelope helpers
├── worker.py              # GPU worker process
├── web_server.py          # Python FastAPI server (optional reference)
├── Dockerfile             # Multi-process container (vLLM + worker)
├── entrypoint.sh          # Start script for vLLM + worker
├── requirements.txt       # Python dependencies

pkg/llm/
├── client.go              # Go LLM client (uses Redis)

docker-compose.yml         # Orchestration (Redis + workers + Orbi)
```

---

## Quick Start

### Prerequisites
- Docker & Docker Compose
- GPU access (optional, but recommended for performance)

### 1. **Start the System**

```bash
# From project root
docker-compose up -d
```

This starts:
- Redis broker (`redis://localhost:6379`)
- vLLM worker (`http://localhost:8000`)
- Orbi chatbot (depends on workers)

### 2. **Verify Worker Health**

```bash
# Check vLLM API is responding
curl http://localhost:8000/health

# Check worker logs
docker logs orbi-vllm-worker
```

### 3. **Test via Python Client**

```python
import sys
sys.path.insert(0, '/path/to/Orbit-Orbi/vLLM')

from common import make_job, to_json, from_json
import redis
import time

r = redis.Redis(host='localhost', port=6379, decode_responses=True)

# Create and submit a job
job = make_job("What is 2+2?", model="mistral-7b-awq")
r.lpush("llm:queue:requests", to_json(job))

# Wait for result
result_key = f"llm:result:{job['job_id']}"
result = r.brpop(result_key, timeout=60)
if result:
    _, payload = result
    print(from_json(payload))
```

### 4. **Test via Go Client**

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/waydxd/Orbit-Orbi/pkg/llm"
)

func main() {
	// Create client
	client, err := llm.NewClient(
		"redis://localhost:6379/0",
		"llm:queue:requests",
		"mistral-7b-awq",
		30*time.Second,
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// Send request
	ctx := context.Background()
	resp, err := client.Ask(ctx, llm.AskRequest{
		Prompt: "What is 2+2?",
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Result: %s\n", resp.Text)
	fmt.Printf("Latency: %dms\n", resp.LatencyMS)
}
```

---

## Environment Variables

### Redis
- `REDIS_URL` (default: `redis://localhost:6379/0`)

### vLLM Worker
- `REDIS_URL` – Redis connection string
- `VLLM_URL` – Local vLLM API endpoint (default: `http://localhost:8000/v1/completions`)
- `WORKER_ID` – Unique identifier for this worker
- `MODEL` – Model to load (default: `TheBloke/Mistral-7B-Instruct-v0.2-AWQ`)
- `PORT` – vLLM server port (default: `8000`)

### Go Web Server
- `REDIS_URL` – Redis connection string
- `VLLM_API_URL` – vLLM API endpoint (for reference, Go clients use Redis)

---

## Job Format

### Request Job

```json
{
  "job_id": "uuid",
  "timestamp": "2024-11-02T10:30:00.000000Z",
  "priority": 0,
  "model": "mistral-7b-awq",
  "params": {
    "temperature": 0.2,
    "max_tokens": 512
  },
  "payload": {
    "prompt": "What is 2+2?"
  },
  "trace": {}
}
```

### Success Result

```json
{
  "job_id": "uuid",
  "status": "ok",
  "output": {
    "text": "2 + 2 = 4"
  },
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 8,
    "total_tokens": 18
  },
  "worker_meta": {
    "id": "worker-1"
  },
  "latency_ms": 1234
}
```

### Error Result

```json
{
  "job_id": "uuid",
  "status": "error",
  "error": {
    "code": "vllm_error",
    "message": "Connection refused",
    "details": {}
  },
  "worker_meta": {
    "id": "worker-1"
  }
}
```

---

## Scaling

### Add More GPU Workers

Update `docker-compose.yml`:

```yaml
vllm-worker-1:
  build: ./vLLM
  environment:
    - WORKER_ID=worker-1
  # ...

vllm-worker-2:
  build: ./vLLM
  environment:
    - WORKER_ID=worker-2
  # ...
```

Then:
```bash
docker-compose up -d --scale vllm-worker=3
```

All workers share the same Redis queue and will process jobs in parallel.

---

## Monitoring

### Check Redis Queue Depth

```bash
redis-cli LLEN llm:queue:requests
```

### Monitor Job Processing

```bash
# Watch Redis operations in real-time
docker exec orbi-redis redis-cli MONITOR

# Check worker logs
docker logs -f orbi-vllm-worker
```

### Performance Metrics

Each result includes `latency_ms` – time from worker start to inference completion.

---

## Troubleshooting

### vLLM Worker Not Processing Jobs

1. **Check worker is running**
   ```bash
   docker ps | grep vllm-worker
   ```

2. **Check vLLM server is healthy**
   ```bash
   docker exec orbi-vllm-worker curl http://localhost:8000/health
   ```

3. **Verify Redis connectivity**
   ```bash
   docker logs orbi-vllm-worker | grep "Redis\|Connected"
   ```

4. **Check queue for stuck jobs**
   ```bash
   redis-cli LRANGE llm:queue:requests 0 -1
   ```

### Timeout Errors

- Increase `timeout_ms` in client request
- Check vLLM model size vs available VRAM
- Monitor GPU utilization: `docker exec orbi-vllm-worker nvidia-smi`

### Memory Issues

- Use quantized models (AWQ, GPTQ)
- Reduce `max_tokens` in inference params
- Run multiple workers with smaller batch sizes

---

## GPU Support

### Enable GPU in Docker

Update `docker-compose.yml`:

```yaml
vllm-worker:
  deploy:
    resources:
      reservations:
        devices:
          - driver: nvidia
            count: all
            capabilities: [gpu]
```

Ensure:
- NVIDIA Docker runtime is installed
- `nvidia-docker` or Docker with GPU support is available

---

## References

- [vLLM OpenAI API](https://docs.vllm.ai/en/latest/serving/openai_compatible_server.html)
- [Redis Streams/Lists](https://redis.io/docs/data-types/lists/)
- [Go Redis Client](https://github.com/redis/go-redis)

---

## Next Steps

1. ✅ Deploy and test with `docker-compose`
2. 🔧 Integrate Go client into Orbi chatbot module
3. 📊 Add monitoring/metrics (Prometheus, Grafana)
4. 🔐 Secure Redis (TLS, auth tokens)
5. 🚀 Scale horizontally with multiple workers/regions

