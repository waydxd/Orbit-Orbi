# Message Broker Architecture - Quick Reference

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     ORBIT-ORBI LLM INFERENCE                    │
└─────────────────────────────────────────────────────────────────┘

                    ┌──────────────────────┐
                    │   Client (Browser)   │
                    │   or CLI Tool        │
                    └──────────┬───────────┘
                               │
                               │ HTTP Request
                               ▼
                    ┌──────────────────────┐
                    │  Go Web Server       │
                    │  (Orbi Chatbot)      │
                    │  :8080               │
                    └──────┬───────────────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
          │ Use Redis      │ Check          │
          │ Client Lib     │ Results        │
          ▼                ▼                ▼
    ┌───────────────────────────────────────────┐
    │         Redis Message Broker              │
    │         :6379                             │
    │                                           │
    │  ┌─────────────────────────────────────┐ │
    │  │ Queue: llm:queue:requests          │ │
    │  │ (Incoming jobs, FIFO)               │ │
    │  └─────────────────────────────────────┘ │
    │                                           │
    │  ┌─────────────────────────────────────┐ │
    │  │ Channels: llm:result:{job_id}      │ │
    │  │ (Per-job result channels)           │ │
    │  └─────────────────────────────────────┘ │
    └────────┬────────────────────────────────┘
             │
             │ Pull jobs
             │ Push results
             │
    ┌────────▼──────────────────────────────┐
    │  vLLM GPU Worker Container(s)         │
    │  (Docker - may have multiple)         │
    │                                       │
    │  ┌──────────────────────────────────┐│
    │  │ vLLM Server (:8000)              ││
    │  │ - Loads Mistral-7B-AWQ           ││
    │  │ - OpenAI-compatible API          ││
    │  └──────────────────────────────────┘│
    │                                       │
    │  ┌──────────────────────────────────┐│
    │  │ Worker Process                    ││
    │  │ - BRPOP from queue                ││
    │  │ - Call vLLM (/v1/completions)     ││
    │  │ - LPUSH results back to Redis     ││
    │  └──────────────────────────────────┘│
    │                                       │
    │  GPU: Available                       │
    │  VRAM: Monitor with nvidia-smi       │
    └───────────────────────────────────────┘
```

## Component Interactions

### 1. Request Flow

```
┌───────────────────────────────────────────────────────────┐
│ CLIENT SUBMISSION                                         │
├───────────────────────────────────────────────────────────┤
│                                                           │
│  Go Client creates AskRequest:                          │
│  ├─ prompt: "What is 2+2?"                              │
│  ├─ model: "mistral-7b-awq"                             │
│  ├─ temperature: 0.2                                    │
│  └─ timeout: 30 seconds                                 │
│                                                           │
│  Client.Ask(ctx, request):                              │
│  ├─ Generate job_id (UUID)                              │
│  ├─ Serialize to JSON                                   │
│  └─ LPUSH to llm:queue:requests                          │
│                                                           │
│  Begin polling for result:                              │
│  └─ BRPOP llm:result:{job_id} (blocks 1s at a time)     │
│                                                           │
└───────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────┐
│ WORKER PROCESSING                                         │
├───────────────────────────────────────────────────────────┤
│                                                           │
│  Worker.main() loop:                                     │
│  ├─ BRPOP llm:queue:requests (timeout=5s)               │
│  │                                                       │
│  ├─ Parse job envelope                                  │
│  ├─ Extract: prompt, params, model                      │
│  │                                                       │
│  ├─ Call vLLM API:                                       │
│  │  POST http://localhost:8000/v1/completions           │
│  │  ├─ model: "mistral-7b-awq"                          │
│  │  ├─ prompt: "What is 2+2?"                           │
│  │  ├─ max_tokens: 512                                  │
│  │  └─ temperature: 0.2                                 │
│  │                                                       │
│  ├─ Measure latency (time.time())                        │
│  │                                                       │
│  └─ Create result envelope:                              │
│     LPUSH llm:result:{job_id}                            │
│     SET expiry 60s                                       │
│                                                           │
└───────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────┐
│ CLIENT RECEIVES RESULT                                    │
├───────────────────────────────────────────────────────────┤
│                                                           │
│  BRPOP returns:                                          │
│  ├─ status: "ok"                                         │
│  ├─ output.text: "2 + 2 = 4"                             │
│  ├─ latency_ms: 1234                                     │
│  └─ usage: {tokens...}                                   │
│                                                           │
│  Return to caller:                                       │
│  └─ AskResponse{Text, JobID, LatencyMS, Usage}           │
│                                                           │
└───────────────────────────────────────────────────────────┘
```

## File Organization

```
project/
│
├── docker-compose.yml                    [Orchestration]
│   ├─ service: redis
│   ├─ service: vllm-worker
│   └─ service: orbi
│
├── vLLM/                                  [Worker & Utils]
│   ├─ worker.py                          [Main worker loop]
│   ├─ common.py                          [Job helpers]
│   ├─ entrypoint.sh                      [Container startup]
│   ├─ Dockerfile                         [Multi-process image]
│   ├─ requirements.txt                   [Python deps]
│   ├─ test_integration.py                [Test suite]
│   ├─ web_server.py                      [FastAPI reference]
│   ├─ README.md                          [Quick start]
│   ├─ DEPLOYMENT.md                      [Detailed guide]
│   └─ quickstart.sh                      [Setup script]
│
├── pkg/llm/                              [Go Client Library]
│   └─ client.go                          [Redis LLM client]
│
├── cmd/orbi/                             [Application]
│   ├─ main.go                            [Orbi chatbot]
│   └─ llm_example.go                     [Integration example]
│
├── go.mod                                [Go dependencies]
│
└── docs/                                 [Documentation]
    ├─ MESSAGE_BROKER_SUMMARY.md          [Implementation]
    └─ DEPLOYMENT_CHECKLIST.md            [Pre-flight]
```

## Key Concepts

### Job Envelope
```json
{
  "job_id": "unique-id",
  "timestamp": "ISO8601",
  "priority": 0,
  "model": "mistral-7b-awq",
  "params": {
    "temperature": 0.2,
    "max_tokens": 512
  },
  "payload": {
    "prompt": "user input"
  },
  "trace": {}
}
```

### Result Envelope (Success)
```json
{
  "job_id": "unique-id",
  "status": "ok",
  "output": {
    "text": "model response"
  },
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 5
  },
  "worker_meta": {
    "id": "worker-1"
  },
  "latency_ms": 1234
}
```

### Result Envelope (Error)
```json
{
  "job_id": "unique-id",
  "status": "error",
  "error": {
    "code": "vllm_error",
    "message": "timeout",
    "details": {}
  },
  "worker_meta": {
    "id": "worker-1"
  }
}
```

## Configuration Quick Reference

| Component | Host | Port | Env Var | Default |
|-----------|------|------|---------|---------|
| Redis | redis | 6379 | REDIS_URL | redis://localhost:6379/0 |
| vLLM | localhost | 8000 | VLLM_URL | http://localhost:8000/v1/completions |
| Worker ID | - | - | WORKER_ID | worker-1 |
| Model | - | - | MODEL | TheBloke/Mistral-7B-Instruct-v0.2-AWQ |

## Queue Names

| Queue | Purpose | Type |
|-------|---------|------|
| `llm:queue:requests` | Incoming jobs | LIST (FIFO) |
| `llm:result:{job_id}` | Per-job results | LIST (single-consumer) |

## Common Commands

### Python Client
```python
import sys; sys.path.insert(0, 'vLLM')
from common import make_job, to_json
import redis

r = redis.Redis(host='localhost', decode_responses=True)
job = make_job("prompt text")
r.lpush("llm:queue:requests", to_json(job))
```

### Go Client
```go
client, _ := llm.NewClient(
    "redis://localhost:6379/0",
    "llm:queue:requests",
    "mistral-7b-awq",
    30*time.Second,
)
resp, _ := client.Ask(ctx, llm.AskRequest{Prompt: "text"})
```

### Docker
```bash
# Start all
docker-compose up -d

# View logs
docker logs -f orbi-vllm-worker

# Redis CLI
docker exec -it orbi-redis redis-cli

# Check stats
docker stats orbi-vllm-worker
```

## Latency Expectations

- **Queue operation**: <5ms
- **Job serialization**: <5ms
- **vLLM inference (7B model, 50 tokens)**: 500-2000ms
- **Result serialization**: <5ms
- **Total latency**: 510-2010ms

## Scaling Patterns

### Horizontal (Multiple Workers)
```yaml
vllm-worker-1, vllm-worker-2, vllm-worker-3...
  ↓ All read from same Redis queue
  ↓ Process in parallel
  ↓ Push to separate result channels
```

### Vertical (Single Worker, More GPU)
```yaml
vllm-worker
  ├─ Larger model (13B, 70B)
  ├─ Higher batch size
  └─ More VRAM allocated
```

## Troubleshooting Quick Map

| Symptom | Check | Fix |
|---------|-------|-----|
| No processing | Worker logs | `docker logs orbi-vllm-worker` |
| Timeout | vLLM health | `curl localhost:8000/health` |
| Queue backlog | Queue depth | `redis-cli LLEN llm:queue:requests` |
| OOM | Model size | Use quantized model (AWQ) |
| Connection error | Redis | `redis-cli ping` |

## Performance Monitoring

```bash
# Queue depth
redis-cli LLEN llm:queue:requests

# Active workers
docker ps | grep vllm

# GPU utilization
nvidia-smi

# Network traffic
docker stats --no-stream
```

---

**For detailed information**, see:
- **Quick Start**: [vLLM/README.md](./vLLM/README.md)
- **Deployment**: [vLLM/DEPLOYMENT.md](./vLLM/DEPLOYMENT.md)
- **Implementation**: [MESSAGE_BROKER_SUMMARY.md](./MESSAGE_BROKER_SUMMARY.md)
- **Checklist**: [DEPLOYMENT_CHECKLIST.md](./DEPLOYMENT_CHECKLIST.md)

