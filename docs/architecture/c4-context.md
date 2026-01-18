# C4 Model - Context

This control plane governs LLM usage across clients by providing a deterministic pipeline:
Authentication → Prompt Validation → Policy Evaluation → Model Routing → Audit & Metrics → Provider Call.

External Systems
---------------
- Identity Provider (JWT/OIDC) for authentication.
- PostgreSQL (authoritative data) and Redis (cache, rate limiting).
- LLM Providers (untrusted external systems).

Core Responsibilities
---------------------
- Enforce access control, quotas, cost limits.
- Validate and sanitize prompts for safety and compliance.
- Route to providers based on policy and availability.
- Emit structured logs, traces, and Prometheus metrics.

Quality Attributes
------------------
- Security-first, stateless, horizontally scalable, observable, and resilient.

