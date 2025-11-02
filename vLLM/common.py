import json
import uuid
from datetime import datetime

def now_iso():
    return datetime.utcnow().isoformat() + "Z"

def make_job(prompt, model="mistral-7b-awq", params=None, meta=None):
    """Create a job envelope for LLM inference request."""
    return {
        "job_id": str(uuid.uuid4()),
        "timestamp": now_iso(),
        "priority": 0,
        "model": model,
        "params": params or {"temperature": 0.2, "max_tokens": 512},
        "payload": {"prompt": prompt},
        "trace": meta or {}
    }

def make_result_ok(job_id, text, usage=None, worker_meta=None, latency_ms=None):
    """Create a successful result envelope."""
    return {
        "job_id": job_id,
        "status": "ok",
        "output": {"text": text},
        "usage": usage or {},
        "worker_meta": worker_meta or {},
        "latency_ms": latency_ms or 0
    }

def make_result_error(job_id, code, message, details=None, worker_meta=None):
    """Create an error result envelope."""
    return {
        "job_id": job_id,
        "status": "error",
        "error": {"code": code, "message": message, "details": details or {}},
        "worker_meta": worker_meta or {}
    }

def to_json(obj): 
    """Serialize object to JSON string."""
    return json.dumps(obj, ensure_ascii=False)

def from_json(s): 
    """Deserialize JSON string to object."""
    return json.loads(s)
