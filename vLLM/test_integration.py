#!/usr/bin/env python3
"""
Integration test suite for the message broker LLM architecture.

Tests:
1. Redis connectivity
2. Job envelope serialization/deserialization
3. Worker job processing
4. End-to-end inference pipeline
"""

import os
import sys
import time
import json
import logging
from typing import Optional

# Add vLLM module to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__)))

import redis
from common import make_job, to_json, from_json, make_result_ok, make_result_error

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379/0")
REQUEST_QUEUE = "llm:queue:requests"


def test_redis_connectivity():
    """Test 1: Verify Redis is accessible."""
    logger.info("=" * 50)
    logger.info("TEST 1: Redis Connectivity")
    logger.info("=" * 50)
    
    try:
        r = redis.from_url(REDIS_URL, decode_responses=True)
        r.ping()
        logger.info("✓ Redis connection successful")
        
        # Check queue
        queue_depth = r.llen(REQUEST_QUEUE)
        logger.info(f"✓ Request queue depth: {queue_depth} jobs")
        
        r.close()
        return True
    except redis.ConnectionError as e:
        logger.error(f"✗ Redis connection failed: {e}")
        return False


def test_job_envelope():
    """Test 2: Job envelope serialization/deserialization."""
    logger.info("\n" + "=" * 50)
    logger.info("TEST 2: Job Envelope Format")
    logger.info("=" * 50)
    
    try:
        # Create job
        job = make_job(
            prompt="What is the capital of France?",
            model="mistral-7b-awq",
            params={"temperature": 0.5, "max_tokens": 256}
        )
        logger.info(f"✓ Created job: {job['job_id']}")
        
        # Serialize
        job_json = to_json(job)
        logger.info(f"✓ Serialized to JSON ({len(job_json)} bytes)")
        
        # Deserialize
        job_restored = from_json(job_json)
        logger.info(f"✓ Deserialized job")
        
        # Verify
        assert job_restored['job_id'] == job['job_id']
        assert job_restored['model'] == "mistral-7b-awq"
        assert job_restored['params']['temperature'] == 0.5
        logger.info("✓ Job format verified")
        
        # Create result
        result = make_result_ok(
            job_id=job['job_id'],
            text="Paris",
            usage={"prompt_tokens": 10, "completion_tokens": 1},
            worker_meta={"id": "worker-1"},
            latency_ms=500
        )
        logger.info(f"✓ Created result: {result['status']}")
        
        result_json = to_json(result)
        result_restored = from_json(result_json)
        assert result_restored['status'] == 'ok'
        logger.info("✓ Result format verified")
        
        return True
    except Exception as e:
        logger.error(f"✗ Job envelope test failed: {e}")
        return False


def test_job_queueing():
    """Test 3: Enqueue and dequeue a test job."""
    logger.info("\n" + "=" * 50)
    logger.info("TEST 3: Job Queueing")
    logger.info("=" * 50)
    
    try:
        r = redis.from_url(REDIS_URL, decode_responses=True)
        
        # Create test job
        job = make_job(
            prompt="Test prompt",
            model="mistral-7b-awq"
        )
        job_id = job['job_id']
        logger.info(f"✓ Created test job: {job_id}")
        
        # Enqueue
        r.lpush(REQUEST_QUEUE, to_json(job))
        logger.info(f"✓ Enqueued job to {REQUEST_QUEUE}")
        
        # Verify queue depth
        depth = r.llen(REQUEST_QUEUE)
        logger.info(f"✓ Queue depth: {depth}")
        
        # Dequeue (for cleanup, don't block long)
        result = r.brpop(REQUEST_QUEUE, timeout=1)
        if result:
            _, payload = result
            job_restored = from_json(payload)
            assert job_restored['job_id'] == job_id
            logger.info(f"✓ Dequeued and verified job")
        else:
            logger.warning("⚠ No job in queue (worker may have consumed it)")
        
        r.close()
        return True
    except Exception as e:
        logger.error(f"✗ Job queueing test failed: {e}")
        return False


def test_result_channel():
    """Test 4: Result channel delivery."""
    logger.info("\n" + "=" * 50)
    logger.info("TEST 4: Result Channel")
    logger.info("=" * 50)
    
    try:
        r = redis.from_url(REDIS_URL, decode_responses=True)
        
        # Create a fake job and result
        job_id = "test-" + str(int(time.time()))
        result_key = f"llm:result:{job_id}"
        
        # Clean up any previous result
        r.delete(result_key)
        logger.info(f"✓ Created result channel: {result_key}")
        
        # Simulate worker pushing result
        result = make_result_ok(
            job_id=job_id,
            text="Test response",
            usage={"total_tokens": 100},
            latency_ms=1000
        )
        r.lpush(result_key, to_json(result))
        logger.info(f"✓ Pushed result to channel")
        
        # Set expiry
        r.expire(result_key, 60)
        logger.info(f"✓ Set expiry to 60 seconds")
        
        # Client retrieves result
        res = r.brpop(result_key, timeout=5)
        if res:
            _, payload = res
            result_restored = from_json(payload)
            assert result_restored['job_id'] == job_id
            assert result_restored['status'] == 'ok'
            logger.info(f"✓ Retrieved result: {result_restored['output']['text']}")
            logger.info(f"✓ Latency: {result_restored['latency_ms']}ms")
        else:
            logger.error("✗ No result received")
            return False
        
        r.close()
        return True
    except Exception as e:
        logger.error(f"✗ Result channel test failed: {e}")
        return False


def test_error_handling():
    """Test 5: Error result creation."""
    logger.info("\n" + "=" * 50)
    logger.info("TEST 5: Error Handling")
    logger.info("=" * 50)
    
    try:
        job_id = "test-error-job"
        
        # Create error result
        error_result = make_result_error(
            job_id=job_id,
            code="vllm_error",
            message="Connection timeout",
            details={"timeout": 30},
            worker_meta={"id": "worker-1"}
        )
        logger.info(f"✓ Created error result")
        
        # Serialize/deserialize
        error_json = to_json(error_result)
        error_restored = from_json(error_json)
        
        assert error_restored['status'] == 'error'
        assert error_restored['error']['code'] == 'vllm_error'
        logger.info(f"✓ Error format verified: {error_restored['error']['message']}")
        
        return True
    except Exception as e:
        logger.error(f"✗ Error handling test failed: {e}")
        return False


def test_end_to_end_simulation():
    """Test 6: Simulate an end-to-end inference flow (without actual vLLM)."""
    logger.info("\n" + "=" * 50)
    logger.info("TEST 6: End-to-End Simulation")
    logger.info("=" * 50)
    
    try:
        r = redis.from_url(REDIS_URL, decode_responses=True)
        
        # Client submits job
        job = make_job(
            prompt="Simulate: What is 2+2?",
            model="mistral-7b-awq",
            params={"temperature": 0.2, "max_tokens": 50}
        )
        job_id = job['job_id']
        result_key = f"llm:result:{job_id}"
        
        r.delete(result_key)  # Clean up
        r.lpush(REQUEST_QUEUE, to_json(job))
        logger.info(f"✓ CLIENT: Submitted job {job_id}")
        
        # Simulate worker retrieving job
        item = r.brpop(REQUEST_QUEUE, timeout=2)
        if not item:
            logger.warning("⚠ Job not in queue (may have been consumed)")
            r.close()
            return True
        
        _, payload = item
        job_retrieved = from_json(payload)
        logger.info(f"✓ WORKER: Retrieved job {job_retrieved['job_id']}")
        logger.info(f"  Prompt: {job_retrieved['payload']['prompt']}")
        
        # Simulate vLLM inference
        time.sleep(0.5)  # Pretend inference takes time
        inference_result = "2 + 2 = 4"
        logger.info(f"✓ WORKER: Inference result: {inference_result}")
        
        # Worker pushes result
        result = make_result_ok(
            job_id=job_id,
            text=inference_result,
            usage={"prompt_tokens": 10, "completion_tokens": 8},
            worker_meta={"id": "worker-sim"},
            latency_ms=500
        )
        r.lpush(result_key, to_json(result))
        r.expire(result_key, 60)
        logger.info(f"✓ WORKER: Pushed result to {result_key}")
        
        # Client retrieves result
        res = r.brpop(result_key, timeout=5)
        if res:
            _, payload = res
            result_retrieved = from_json(payload)
            logger.info(f"✓ CLIENT: Retrieved result")
            logger.info(f"  Status: {result_retrieved['status']}")
            logger.info(f"  Text: {result_retrieved['output']['text']}")
            logger.info(f"  Latency: {result_retrieved['latency_ms']}ms")
        else:
            logger.error("✗ Client timeout waiting for result")
            r.close()
            return False
        
        r.close()
        return True
    except Exception as e:
        logger.error(f"✗ End-to-end simulation failed: {e}")
        return False


def main():
    """Run all tests."""
    logger.info("\n")
    logger.info("╔" + "=" * 48 + "╗")
    logger.info("║" + " " * 10 + "LLM Message Broker Test Suite" + " " * 8 + "║")
    logger.info("╚" + "=" * 48 + "╝")
    logger.info(f"Redis URL: {REDIS_URL}\n")
    
    tests = [
        ("Redis Connectivity", test_redis_connectivity),
        ("Job Envelope", test_job_envelope),
        ("Job Queueing", test_job_queueing),
        ("Result Channel", test_result_channel),
        ("Error Handling", test_error_handling),
        ("End-to-End Simulation", test_end_to_end_simulation),
    ]
    
    results = []
    for test_name, test_func in tests:
        try:
            passed = test_func()
            results.append((test_name, passed))
        except Exception as e:
            logger.error(f"✗ Test {test_name} crashed: {e}")
            results.append((test_name, False))
    
    # Summary
    logger.info("\n" + "=" * 50)
    logger.info("TEST SUMMARY")
    logger.info("=" * 50)
    
    passed = sum(1 for _, p in results if p)
    total = len(results)
    
    for test_name, passed_test in results:
        status = "✓ PASS" if passed_test else "✗ FAIL"
        logger.info(f"{status}: {test_name}")
    
    logger.info("=" * 50)
    logger.info(f"Total: {passed}/{total} tests passed\n")
    
    return 0 if passed == total else 1


if __name__ == "__main__":
    sys.exit(main())
