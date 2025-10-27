# Dockerfile
FROM vllm/vllm-openai:latest

# Default environment (can override at runtime)
ENV MODEL=TheBloke/Mistral-7B-Instruct-v0.2-AWQ
ENV PORT=8000

EXPOSE 8000

# Start vLLM OpenAI server
CMD ["--model", "TheBloke/Mistral-7B-Instruct-v0.2-AWQ", "--host", "0.0.0.0", "--port", "8000"]
