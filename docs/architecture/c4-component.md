# C4 Model - Component (api-gateway)

Primary Components
------------------
- auth: JWT validation, API key auth, RBAC middleware.
- prompt: validation, PII detection, injection guard.
- policy: engine for quota, cost, content policies.
- router: model routing, fallback strategies, provider selection.
- audit: event capture, structured logging.
- metrics: Prometheus metrics registration and emission.
- storage: postgres and redis access layers.
- shared: config, errors, context keys.

Flow
----
1) auth middleware
2) prompt validation
3) policy evaluation (including quota/cost)
4) routing decision
5) audit/metrics wrap
6) provider call (adapter)

