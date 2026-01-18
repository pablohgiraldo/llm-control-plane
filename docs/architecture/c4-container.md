# C4 Model - Container

Containers
----------
- api-gateway: stateless HTTP service orchestrating the governance pipeline.
- postgres: state store for policies, audit events, API keys, quotas.
- redis: cache and rate limiter.
- provider adapters: outbound clients (not core logic).

Runtime Concerns
----------------
- Config via env and ConfigMaps; no mutable local state.
- Horizontal Pod Autoscaling; liveness/readiness probes.
- Structured JSON logs and Prometheus metrics endpoint.

