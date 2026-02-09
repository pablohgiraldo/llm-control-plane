# C4 Model - Container (AI Governance Platform)

Containers
----------
- ai-gateway  
  Stateless HTTP service exposing a single governed API. Orchestrates the full execution pipeline including auth, intent analysis, pre/post RAG validation, policy enforcement, routing, and observability.

- middleware-sdk  
  Embeddable library used by internal services and legacy systems to access the same governance core without network hops. Shares policy, routing, and validation logic with the gateway.

- governance-core  
  Core governance engine used by both gateway and SDK. Contains policy evaluation, intent analysis, prompt validation, RAG controls, runtime configuration resolution, and routing decisions.

- admin-console  
  Serverless frontend for policy, model, tenant, and configuration management. Consumes the governance APIs.

- postgres  
  Governance source of truth. Stores policies, tenants, models, audit history, and compliance artifacts.

- dynamodb  
  Runtime configuration and state store. Holds feature flags, thresholds, quotas counters, and low-latency execution config.

- secrets-manager  
  Secure storage for provider credentials, API keys, and sensitive configuration.

- observability-stack  
  Centralized logging, metrics, and tracing (CloudWatch, Prometheus, OpenTelemetry).

- llm-providers  
  External and internal LLM providers (OpenAI, Anthropic, Local LLM) accessed through provider adapters.

Runtime Concerns
----------------
- Stateless containers with no mutable local state.
- Configuration split between:
  - PostgreSQL for authoritative governance data.
  - DynamoDB for fast-changing runtime config.
- Horizontal autoscaling (HPA) and multi-AZ deployment.
- Health checks, circuit breakers, and provider-level fallbacks.
- Structured logs, metrics, and traces emitted at each governance step.
