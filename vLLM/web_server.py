"""
FastAPI web server that interfaces with vLLM GPU workers via Redis message broker.

This demonstrates the architecture:
  Client -> /ask endpoint -> Redis queue -> vLLM Worker -> Result back to client
"""
import os
import time
import logging
import redis
from typing import Optional, Dict, Any

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

# Import helpers (adjust path based on your structure)
try:
    from common import make_job, to_json, from_json
except ImportError:
    # Fallback if common.py is not in the same directory
    import json
    import uuid
    from datetime import datetime

    def now_iso():
        return datetime.utcnow().isoformat() + "Z"

    def make_job(prompt, model="mistral-7b-awq", params=None, meta=None):
        return {
            "job_id": str(uuid.uuid4()),
            "timestamp": now_iso(),
            "priority": 0,
            "model": model,
            "params": params or {"temperature": 0.2, "max_tokens": 512},
            "payload": {"prompt": prompt},
            "trace": meta or {}
        }

    def to_json(obj):
        return json.dumps(obj, ensure_ascii=False)

    def from_json(s):
        return json.loads(s)


# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379/0")
REQUEST_QUEUE = "llm:queue:requests"

# Initialize FastAPI and Redis
app = FastAPI(
    title="Orbi LLM API",
    description="GPU-backed LLM inference API via Redis message broker",
    version="1.0.0"
)

try:
    r = redis.from_url(REDIS_URL, decode_responses=True)
    r.ping()
    logger.info(f"Connected to Redis at {REDIS_URL}")
except redis.ConnectionError as e:
    logger.error(f"Failed to connect to Redis: {e}")
    raise


# Request/Response models
class AskRequest(BaseModel):
    """Request body for /ask endpoint."""
    prompt: str
    model: Optional[str] = None
    temperature: Optional[float] = None
    max_tokens: Optional[int] = None
    timeout_ms: int = 30000  # Client-facing SLA in milliseconds


class AskResponse(BaseModel):
    """Response body for /ask endpoint."""
    job_id: str
    text: str
    usage: Dict[str, Any]
    latency_ms: Optional[int] = None


class ErrorResponse(BaseModel):
    """Error response."""
    job_id: str
    code: str
    message: str
    details: Optional[Dict[str, Any]] = None


# Routes
@app.get("/health")
def health():
    """Health check endpoint."""
    try:
        r.ping()
        return {"status": "healthy"}
    except redis.RedisError:
        return {"status": "unhealthy"}, 503


@app.post("/ask", response_model=AskResponse, responses={500: {"model": ErrorResponse}})
def ask(body: AskRequest):
    """
    Submit a prompt to the LLM inference pipeline.
    
    The request is enqueued to Redis, processed by a GPU worker,
    and the result is returned to the client.
    
    Args:
        body: Request containing prompt and optional parameters
        
    Returns:
        AskResponse with generated text and metadata
        
    Raises:
        HTTPException: If inference fails or times out
    """
    try:
        # Prepare job
        params = {}
        if body.temperature is not None:
            params["temperature"] = body.temperature
        if body.max_tokens is not None:
            params["max_tokens"] = body.max_tokens
        else:
            params["max_tokens"] = 512
        
        job = make_job(
            body.prompt,
            model=body.model or "mistral-7b-awq",
            params=params
        )
        job_id = job["job_id"]
        result_key = f"llm:result:{job_id}"
        
        logger.info(f"Submitting job {job_id} to queue")
        
        # Enqueue request
        r.lpush(REQUEST_QUEUE, to_json(job))
        
        # Wait for result with timeout
        timeout_sec = body.timeout_ms / 1000.0
        start = time.time()
        
        while True:
            elapsed = time.time() - start
            remaining = timeout_sec - elapsed
            
            if remaining <= 0:
                logger.warning(f"Job {job_id} timed out after {elapsed:.1f}s")
                raise HTTPException(
                    status_code=504,
                    detail=f"Inference timeout (>{body.timeout_ms}ms)"
                )
            
            # Block for up to 1s at a time
            block_timeout = min(1, remaining)
            res = r.brpop(result_key, timeout=int(block_timeout) + 1)
            
            if res:
                _, payload = res
                result = from_json(payload)
                
                if result.get("status") == "ok":
                    logger.info(f"Job {job_id} completed successfully in {elapsed:.1f}s")
                    return AskResponse(
                        job_id=job_id,
                        text=result["output"]["text"],
                        usage=result.get("usage", {}),
                        latency_ms=result.get("latency_ms")
                    )
                else:
                    # Error from worker
                    err = result.get("error", {})
                    error_code = err.get("code", "unknown_error")
                    error_msg = err.get("message", "Unknown error")
                    logger.error(f"Job {job_id} failed: {error_code}: {error_msg}")
                    raise HTTPException(
                        status_code=500,
                        detail=f"{error_code}: {error_msg}"
                    )
    
    except HTTPException:
        raise
    except redis.RedisError as e:
        logger.error(f"Redis error: {e}")
        raise HTTPException(status_code=500, detail="Redis error")
    except Exception as e:
        logger.error(f"Unexpected error in /ask: {e}")
        raise HTTPException(status_code=500, detail="Internal server error")


@app.get("/")
def root():
    """Root endpoint with API info."""
    return {
        "name": "Orbi LLM API",
        "version": "1.0.0",
        "endpoints": {
            "POST /ask": "Submit a prompt for inference",
            "GET /health": "Health check"
        }
    }


if __name__ == "__main__":
    import uvicorn
    
    port = int(os.getenv("API_PORT", 8080))
    host = os.getenv("API_HOST", "0.0.0.0")
    
    logger.info(f"Starting API server on {host}:{port}")
    uvicorn.run(app, host=host, port=port)
