# Architecture Decision Records (ADRs)

ADR-0001: Stateless services and externalized state
---------------------------------------------------
Status: Accepted
We adopt stateless Pods, all state in Postgres/Redis to enable scale-out and resilience.

ADR-0002: Deterministic governance pipeline
-------------------------------------------
Status: Accepted
Requests pass through explicit stages: auth, prompt validation, policy evaluation, routing, audit/metrics, provider call.

