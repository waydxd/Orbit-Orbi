````markdown
# Orbit-Orbi — Consolidated Documentation

This single file consolidates the repository's docs (architecture, implementation, deployment, examples, and quick references).

## What this project is

Orbit-Orbi is a Go-based intelligent agent/chatbot for smart calendar management. It uses a multi-agent LangChain-style architecture, gRPC for calendar service integration, and an optional vLLM/Redis-backed inference pipeline for LLM work.

Core capabilities:
- Natural-language calendar operations (create/update/delete/query events)
- Approval gates for destructive actions with audit logging
- Pluggable tools (gRPC, SQL, Redis-backed LLMs)
- Example in-memory calendar service for local testing

## High-level components

- cmd/orbi — CLI entrypoint
- pkg/orbi — Orbi agent, sub-agents, tools, memory and classifier
- pkg/grpcclient — gRPC client wrapper for calendar service
- proto — calendar.proto and generated Go files
- examples/calendar-service — in-memory example calendar service
- vLLM — optional worker & helper scripts for Redis + vLLM inference

## Quick start (local development)

1. Ensure Go 1.21+, protoc and Docker are installed.
2. Clone and fetch deps:
   - `make deps`
3. Generate protobufs:
   - `make proto`
4. Start the example calendar service (for development):
   - `cd examples/calendar-service && go run main.go`
5. Run Orbi (point to the calendar service):
   - `export CALENDAR_SERVICE_ADDR=localhost:50051`
   - `export OPENAI_API_KEY=your-api-key`
   - `make build && ./bin/orbi`

## Developer commands

- `make build` — build the binary
- `make fmt` — format code
- `make vet` — vet
- `make test` — run tests
- `make proto` — regenerate protos

## Deployment notes (condensed)

- Use Docker/Docker Compose for multi-service deployments (Redis, vLLM worker, Orbi, example calendar service).
- For production: enable TLS for gRPC, secure Redis, rotate API keys, enable authentication and audit logging.
- vLLM: use quantized models (AWQ) on GPUs; run multiple workers for throughput.

## Architecture summary

- PlannerChatbotAgent orchestrates sub-agents (RetrievalAgent, UpdateAgent, ConfirmationAgent).
- RetrievalAgent: read-only operations against calendar or DB.
- UpdateAgent: creates pending approvals and applies changes after user confirmation.
- ConfirmationAgent: validates changes and detects conflicts.
- Memory/Audit: conversation memory and audit logs for traceability.

## Where to find specifics

- Protobuf service: `proto/calendar.proto` — contains API contract for calendar services.
- gRPC client: `pkg/grpcclient` — wrapper for calendar RPCs used by tools.
- Agent implementation & examples: `pkg/orbi` — contains types, agents, tools, and usage samples.
- Example calendar service: `examples/calendar-service` — in-memory test service used for development.
- vLLM worker & scripts: `vLLM/` — optional Python workers, Dockerfile and integration scripts.

## What changed

This repository previously included many separate markdown files (architecture, flow, implementation guides, deployment checklists, quick references). These have been consolidated into this single canonical document `DOC_SUMMARY.md` to reduce duplication and to provide a single place to find essential information. Detailed historical docs were removed to avoid divergence; the repository's git history preserves the previous files if you need to restore sections.

## Restoring details

If you need to recover a removed doc with full diagrams or longer guides, use git history:

```bash
git log --name-only -- README.md
git checkout <commit> -- ARCHITECTURE_DIAGRAMS.md
```

## Feedback

If you'd like a different organization (e.g., keep a separate `DEPLOYMENT.md`), tell me which parts you want split back out and I will re-create them.

````
