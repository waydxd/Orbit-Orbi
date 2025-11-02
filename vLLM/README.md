# Message Broker LLM Architecture

This directory contains the implementation of the **GPU-backed LLM inference system** using Redis as a message broker. The architecture decouples API requests from GPU inference, enabling scalable and efficient LLM serving.

## 📋 Overview

```
┌─────────────────────┐
│  Go Web Server      │  (Orbi LangChain)
│  - Accepts /ask     │
└──────────┬──────────┘
           │ LPUSH
           ▼
    ┌─────────────┐
    │    Redis    │  (Message Broker)
    │   Queues   │
    └─────────────┘
           │ BRPOP
           ▼
┌─────────────────────┐
│  vLLM GPU Workers   │  (Docker containers)
│  - Call vLLM API    │
│  - Push results     │
└─────────────────────┘
```

## 📦 Files

| File | Purpose |
|------|---------|
| `common.py` | Job envelope serialization helpers |
| `worker.py` | GPU worker process (pulls jobs, calls vLLM) |
| `entrypoint.sh` | Container startup script (runs vLLM + worker) |
| `Dockerfile` | Multi-process container image |
| `requirements.txt` | Python dependencies for worker |
| `web_server.py` | Reference FastAPI implementation (optional) |
| `test_integration.py` | Test suite for the architecture |
| `DEPLOYMENT.md` | Detailed deployment guide |

## 🚀 Quick Start

### 1. Start the System

```bash
bash vLLM/quickstart.sh
```

This starts:
- **Redis** (message broker)
- **vLLM Worker** (inference engine)
- **Orbi** (Go chatbot service)

### 2. Run Integration Tests

```bash
python vLLM/test_integration.py
```

### 3. Submit a Test Request (Python)

```python
import sys
sys.path.insert(0, 'vLLM')

from common import make_job, to_json, from_json
import redis

r = redis.Redis(host='localhost', port=6379, decode_responses=True)

# Create job
job = make_job("What is 2+2?")
r.lpush("llm:queue:requests", to_json(job))

# Wait for result
result = r.brpop(f"llm:result:{job['job_id']}", timeout=60)
if result:
    print(from_json(result[1]))
```

### 4. Submit a Test Request (Go)

```go
package main

import (
	"context"
	"fmt"
	"time"
	"github.com/waydxd/Orbit-Orbi/pkg/llm"
)

func main() {
	client, _ := llm.NewClient(
		"redis://localhost:6379/0",
		"llm:queue:requests",
		"mistral-7b-awq",
		30*time.Second,
	)
	defer client.Close()

	resp, _ := client.Ask(context.Background(), llm.AskRequest{
		Prompt: "What is 2+2?",
	})
	fmt.Println(resp.Text)
}
```

## 🔧 Configuration

### Environment Variables

**Worker Container:**
```bash
REDIS_URL=redis://redis:6379/0      # Redis connection
VLLM_URL=http://localhost:8000/v1/completions
WORKER_ID=worker-1                   # Unique worker ID
MODEL=TheBloke/Mistral-7B-Instruct-v0.2-AWQ
PORT=8000                            # vLLM port
```

**Go Client:**
```go
llm.NewClient(
    "redis://localhost:6379/0",     // Redis URL
    "llm:queue:requests",           // Request queue name
    "mistral-7b-awq",               // Default model
    30*time.Second,                 // Default timeout
)
```

## 📊 Job Format

### Request
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
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

### Response (Success)
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "ok",
  "output": {
    "text": "2 + 2 = 4"
  },
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 8,
    "total_tokens": 18
  },
  "worker_meta": {"id": "worker-1"},
  "latency_ms": 1234
}
```

### Response (Error)
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "error",
  "error": {
    "code": "vllm_error",
    "message": "Connection refused",
    "details": {}
  },
  "worker_meta": {"id": "worker-1"}
}
```

## 🔍 Monitoring

### View Worker Logs
```bash
docker logs -f orbi-vllm-worker
```

### Check Queue Depth
```bash
docker exec orbi-redis redis-cli LLEN llm:queue:requests
```

### Monitor Redis Operations
```bash
docker exec -it orbi-redis redis-cli MONITOR
```

### Check vLLM Health
```bash
curl http://localhost:8000/health
```

## 🐛 Troubleshooting

| Issue | Solution |
|-------|----------|
| Worker not processing jobs | Check `docker logs orbi-vllm-worker` for vLLM initialization errors |
| Timeout errors | Increase `timeout_ms` in client, or check vLLM model fits in VRAM |
| Redis connection errors | Verify `REDIS_URL` environment variable, check Redis is running |
| OOM errors | Use quantized models (AWQ), reduce `max_tokens` |

## 📚 Documentation

- **[DEPLOYMENT.md](./DEPLOYMENT.md)** - Comprehensive deployment guide with examples
- **[worker.py](./worker.py)** - Worker implementation with error handling
- **[common.py](./common.py)** - Job envelope helpers
- **[pkg/llm/client.go](../pkg/llm/client.go)** - Go client library

## 🔗 Related Components

- **Go LLM Client**: `pkg/llm/client.go` - Redis-based inference client for Orbi
- **Orbi Chatbot**: Main Go service that uses the LLM client
- **vLLM**: Inference engine running locally in each worker

## 🎯 Next Steps

1. ✅ Deploy and test with `docker-compose`
2. 🔧 Integrate Go client into Orbi's LangChain module
3. 📊 Add monitoring (Prometheus, Grafana)
4. 🔐 Secure Redis (TLS, authentication)
5. 🚀 Scale horizontally (multiple workers/regions)

## 📝 License

Part of the Orbit-Orbi project. See parent repository for license details.

---

**Questions?** Check [DEPLOYMENT.md](./DEPLOYMENT.md) for detailed documentation.
