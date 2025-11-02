import os
import sys
import time
import logging
import requests
import redis
from common import from_json, to_json, make_result_ok, make_result_error

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    stream=sys.stdout
)
logger = logging.getLogger(__name__)

REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379/0")
REQUEST_QUEUE = "llm:queue:requests"
VLLM_URL = os.getenv("VLLM_URL", "http://localhost:8000/v1/completions")
WORKER_ID = os.getenv("WORKER_ID", "worker-1")

logger.info(f"Worker ID: {WORKER_ID}")
logger.info(f"Redis URL: {REDIS_URL}")
logger.info(f"vLLM URL: {VLLM_URL}")

r = redis.from_url(REDIS_URL, decode_responses=True)

def run_vllm(prompt, params):
    """Call vLLM OpenAI-compatible completion API."""
    payload = {
        "model": params.get("model", "mistral-7b-awq"),
        "prompt": prompt,
        "max_tokens": params.get("max_tokens", 512),
        "temperature": params.get("temperature", 0.2)
    }
    try:
        resp = requests.post(VLLM_URL, json=payload, timeout=30)
        resp.raise_for_status()
        j = resp.json()
        text = j["choices"][0]["text"]
        usage = j.get("usage", {})
        logger.info("vLLM inference successful for job")
        return text, usage
    except requests.exceptions.Timeout:
        logger.error("vLLM request timed out")
        raise
    except requests.exceptions.RequestException as e:
        logger.error("vLLM request failed: %s", str(e))
        raise

def main():
    """Main worker loop: pull jobs from Redis, process with vLLM, push results."""
    logger.info("Worker starting, listening on queue: %s", REQUEST_QUEUE)
    
    while True:
        try:
            item = r.brpop(REQUEST_QUEUE, timeout=5)
            if not item:
                continue  # idle, no job available
            
            _, raw = item
            job = from_json(raw)
            job_id = job["job_id"]
            result_key = f"llm:result:{job_id}"
            
            logger.info("Processing job: %s", job_id)
            t0 = time.time()
            
            try:
                prompt = job["payload"]["prompt"]
                params = job["params"]
                text, usage = run_vllm(prompt, params | {"model": job["model"]})
                latency = int((time.time() - t0) * 1000)
                result = make_result_ok(job_id, text, usage, worker_meta={"id": WORKER_ID}, latency_ms=latency)
                logger.info("Job completed: %s (latency: %dms)", job_id, latency)
            except requests.exceptions.RequestException as e:
                logger.error("vLLM error for job %s: %s", job_id, str(e))
                result = make_result_error(job_id, "vllm_error", str(e), worker_meta={"id": WORKER_ID})
            except (KeyError, ValueError) as e:
                logger.error("Job format error for job %s: %s", job_id, str(e))
                result = make_result_error(job_id, "job_format_error", str(e), worker_meta={"id": WORKER_ID})
            except Exception as e:  # pylint: disable=broad-except
                logger.error("Unexpected error for job %s: %s", job_id, str(e))
                result = make_result_error(job_id, "worker_exception", str(e), worker_meta={"id": WORKER_ID})
            
            # Publish result (single-consumer channel)
            r.lpush(result_key, to_json(result))
            # Set expiry to avoid stale keys
            r.expire(result_key, 60)
            
        except redis.RedisError as e:
            logger.error("Redis error: %s", str(e))
            time.sleep(1)  # back off briefly before retrying
        except KeyboardInterrupt:
            logger.info("Worker shutting down...")
            break
        except Exception as e:  # pylint: disable=broad-except
            logger.error("Unexpected error in main loop: %s", str(e))
            time.sleep(1)

if __name__ == "__main__":
    main()
