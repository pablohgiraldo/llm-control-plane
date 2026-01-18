LLM Control Plane / Governance Platform (Sprint 1 Foundation)
============================================================

Purpose
-------
Enterprise-grade, stateless control plane acting as deterministic middleware between clients and multiple LLM providers. It enforces governance through a pipeline:
Authentication → Prompt Validation → Policy Evaluation → Model Routing → Audit & Metrics → Provider Call.

Non-Goals
---------
- This is not an AI app or a RAG system.
- No UI in this repository.
- No provider-specific logic in core domains.

Key Tenets
----------
- Cloud-native, stateless services (12-factor).
- Deterministic pipeline orchestration.
- PostgreSQL as system of record; Redis for cache/rate limiting.
- Observability: structured logs and Prometheus-style metrics.
- Security-first: zero-trust to provider outputs; validate, govern, audit.

Repo Layout
-----------
- cmd/: service entrypoints (e.g., api-gateway)
- internal/: domain modules (auth, prompt, policy, router, audit, metrics, storage, shared)
- docs/: architecture, API, security, and roadmap
- deployments/: Kubernetes and Terraform scaffolding
- tests/: unit and integration scaffolding

Local Development (initial)
---------------------------
1) Start infra dependencies:
   - docker compose up -d
2) Build:
   - make build
3) Run gateway (stub):
   - make run

Security & Governance Notes
---------------------------
- Treat LLMs as untrusted external systems. Validate prompts, constrain policies, sanitize outputs, and record audit trails.
- RBAC and quota are enforced before any provider calls.
- Metrics and logs should never contain raw secrets or user PII.

Next
----
- Add go.mod and provider adapters as needed.
- Flesh out policies, quota, routing strategies, and audit schemas.
- Define OpenAPI endpoints and request/response contracts. 

