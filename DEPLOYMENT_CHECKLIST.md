# Message Broker Architecture - Deployment Checklist

Use this checklist to ensure all components are properly configured and ready for deployment.

## ✅ Pre-Deployment Checklist

### 1. Environment & Dependencies
- [ ] Docker is installed (`docker --version`)
- [ ] Docker Compose is installed (`docker-compose --version`)
- [ ] Python 3.9+ is available for testing
- [ ] Git is configured and ready for commits

### 2. Project Structure
- [ ] `/vLLM/common.py` exists and contains job envelope helpers
- [ ] `/vLLM/worker.py` exists and has proper error handling
- [ ] `/vLLM/entrypoint.sh` exists and is executable
- [ ] `/vLLM/Dockerfile` updated with multi-process setup
- [ ] `/vLLM/requirements.txt` has latest dependencies
- [ ] `/pkg/llm/client.go` exists with Redis client implementation
- [ ] `/docker-compose.yml` includes Redis, vLLM, and Orbi services
- [ ] `/go.mod` includes redis/go-redis dependency

### 3. Configuration Files
- [ ] `docker-compose.yml` has correct environment variables
- [ ] `vLLM/Dockerfile` sets proper defaults (MODEL, PORT, etc.)
- [ ] `vLLM/entrypoint.sh` waits for vLLM to initialize (sleep/health check)
- [ ] `vLLM/requirements.txt` includes: `redis`, `requests`

### 4. Documentation
- [ ] `vLLM/README.md` - Quick start guide exists
- [ ] `vLLM/DEPLOYMENT.md` - Detailed guide exists
- [ ] `MESSAGE_BROKER_SUMMARY.md` - Implementation summary exists
- [ ] Code comments explain key functions

### 5. Testing
- [ ] `vLLM/test_integration.py` can run locally
- [ ] Redis connectivity can be tested
- [ ] Job serialization/deserialization verified

## 🚀 Deployment Steps

### Step 1: Verify All Files
```bash
ls -la vLLM/{common.py,worker.py,entrypoint.sh,Dockerfile,requirements.txt,test_integration.py}
ls -la pkg/llm/client.go
ls -a docker-compose.yml
```

### Step 2: Run Tests (Pre-deployment)
```bash
# Test job envelope format (no Redis required)
python3 -c "import sys; sys.path.insert(0, 'vLLM'); from common import make_job, to_json, from_json; print(to_json(make_job('test')))"

# Should output a JSON string
```

### Step 3: Build Docker Images
```bash
# Build vLLM worker image
docker build -t orbi-vllm-worker:latest ./vLLM

# Verify image was created
docker images | grep orbi-vllm-worker
```

### Step 4: Start Services
```bash
# Option A: Automated startup (recommended)
bash vLLM/quickstart.sh

# Option B: Manual startup
docker-compose up -d

# Option C: Manual with build
docker-compose up -d --build
```

### Step 5: Verify Services are Running
```bash
# Check all containers
docker ps

# Expected output should show:
# - orbi-redis
# - orbi-vllm-worker
# - orbi-chatbot (or equivalent)

# Specific health checks
docker exec orbi-redis redis-cli ping
docker exec orbi-vllm-worker curl http://localhost:8000/health
```

### Step 6: Run Integration Tests
```bash
# Install test dependencies if needed
pip install redis requests

# Run full test suite
python vLLM/test_integration.py

# Expected output: All tests pass (6/6)
```

### Step 7: Test End-to-End Flow
```python
# Python test
import sys
sys.path.insert(0, 'vLLM')
from common import make_job, to_json, from_json
import redis
import time

r = redis.Redis(host='localhost', port=6379, decode_responses=True)
job = make_job("What is 2+2?")
r.lpush("llm:queue:requests", to_json(job))

# Wait a bit for processing
time.sleep(5)

result_key = f"llm:result:{job['job_id']}"
result = r.brpop(result_key, timeout=5)
if result:
    print("SUCCESS:", from_json(result[1])['output']['text'])
else:
    print("TIMEOUT - Check if worker is processing jobs")
```

### Step 8: Integration with Orbi
```bash
# Update Orbi's main.go to use the LLM client
# Reference: cmd/orbi/llm_example.go

# Build Orbi with new dependencies
go mod tidy
go build ./cmd/orbi

# Test integration
# (Start the full system and verify LLM calls work)
```

## 🔍 Post-Deployment Verification

### Check Redis Queue
```bash
# View request queue depth
docker exec orbi-redis redis-cli LLEN llm:queue:requests

# View specific job in queue
docker exec orbi-redis redis-cli LRANGE llm:queue:requests 0 0

# Monitor all Redis operations
docker exec orbi-redis redis-cli MONITOR
```

### Monitor Worker
```bash
# View worker logs in real-time
docker logs -f orbi-vllm-worker

# Check worker resource usage
docker stats orbi-vllm-worker

# Expected logs should show:
# - vLLM server startup
# - Worker initialization
# - Job processing: "Processing job: {job_id}"
# - Result completion: "Job completed: {job_id} (latency: XXXms)"
```

### Verify vLLM API
```bash
# Check vLLM is responding
curl http://localhost:8000/health

# List available models
curl http://localhost:8000/v1/models

# Test inference directly (optional)
curl -X POST http://localhost:8000/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mistral-7b-awq",
    "prompt": "What is 2+2?",
    "max_tokens": 50
  }'
```

## 🐛 Troubleshooting Checklist

### If Services Don't Start
- [ ] Check Docker daemon is running: `docker ps`
- [ ] Check port conflicts: `lsof -i :6379` (Redis), `lsof -i :8000` (vLLM)
- [ ] View error logs: `docker logs orbi-vllm-worker`
- [ ] Check disk space: `df -h`

### If Worker Doesn't Process Jobs
- [ ] Verify Redis connectivity: `docker exec orbi-redis redis-cli ping`
- [ ] Check vLLM server: `docker exec orbi-vllm-worker curl http://localhost:8000/health`
- [ ] Monitor logs: `docker logs -f orbi-vllm-worker`
- [ ] Check environment variables: `docker inspect orbi-vllm-worker | grep Env`

### If Results Timeout
- [ ] Increase timeout_ms in client request
- [ ] Check vLLM model size vs VRAM: `docker exec orbi-vllm-worker nvidia-smi`
- [ ] Monitor queue depth: `docker exec orbi-redis redis-cli LLEN llm:queue:requests`
- [ ] Check for errors in worker logs

### If Memory is Full
- [ ] Use quantized models (AWQ) - already configured
- [ ] Reduce max_tokens parameter
- [ ] Run multiple workers with smaller batches
- [ ] Check model cache: `du -sh ~/.cache/` inside container

## 📊 Performance Baselines

Once deployed, these are expected performance metrics:

| Metric | Expected Value | Threshold |
|--------|---|---|
| Redis ping latency | <1ms | >10ms = issue |
| vLLM server startup | 10-30s | >60s = issue |
| Job enqueue time | <5ms | >50ms = issue |
| Inference latency (7B model) | 500-2000ms | >5000ms = timeout |
| Worker throughput | 1-3 jobs/sec | depends on model |
| Queue depth (idle) | 0 | >100 = bottleneck |

## 🔐 Security Verification

- [ ] Redis requires authentication (production)
- [ ] TLS enabled for Redis communication (production)
- [ ] Worker validates job format before processing
- [ ] No sensitive data logged
- [ ] Container images are scanned for vulnerabilities

## 📝 Sign-Off

When everything passes, confirm:

- [ ] All health checks pass
- [ ] Integration tests complete successfully (6/6 pass)
- [ ] End-to-end test works (job → inference → result)
- [ ] Worker can handle multiple concurrent jobs
- [ ] Documentation is current and accurate
- [ ] Team members trained on deployment procedure

**Deployment Status:** __ Ready / __ In Progress / __ Blocked

**Date:** _______________

**Signed By:** _______________

---

## 📞 Support

For issues or questions:
1. Check [DEPLOYMENT.md](./DEPLOYMENT.md) for detailed solutions
2. Review [worker.py](./worker.py) logs for worker-specific issues
3. Check Redis MONITOR output for message flow
4. Consult [MESSAGE_BROKER_SUMMARY.md](../MESSAGE_BROKER_SUMMARY.md) for architecture overview

---

*Last Updated: November 2, 2024*
