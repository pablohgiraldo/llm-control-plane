# C4 Model - Component (AI Gateway / Middleware Core)

Primary Components
------------------
- auth  
  Identity context resolution, JWT/API Key validation, tenant isolation, RBAC/ABAC enrichment.

- intent  
  Semantic intent extraction from prompts and structured inputs (JSON, events), used as input for policy and routing decisions.

- pre-rag  
  Pre-retrieval validation and enrichment layer: policy-aware query shaping, safety checks, and retrieval eligibility validation.

- rag  
  Retrieval orchestration against governed knowledge sources (vector stores, internal docs), with tenancy and policy constraints.

- post-rag  
  Post-retrieval validation and filtering: content safety, leakage prevention, and relevance enforcement before model execution.

- prompt  
  Prompt normalization, schema validation, PII detection, prompt injection guards, and system prompt composition.

- policy  
  Central policy engine for quota, cost, data access, compliance, model eligibility, and routing constraints.

- runtime-config  
  Low-latency access to feature flags, thresholds, and dynamic configuration used during request execution.

- router  
  Model routing and provider selection based on policy outcomes, intent, tenant rules, fallback strategies, and availability.

- providers  
  Adapters for external and internal LLM providers (OpenAI, Anthropic, Local LLM), enforcing a common execution contract.

- audit  
  Structured event capture for compliance, traceability, and forensic analysis.

- observability  
  Metrics, traces, and logs emission (Prometheus/OpenTelemetry), including policy decisions and routing outcomes.

- storage  
  - postgres: governance source of truth (policies, models, tenants, audit history).
  - dynamodb: runtime state, feature flags, thresholds, lightweight execution config.

- secrets  
  Secure access to provider credentials and sensitive configuration via Secrets Manager.

- shared  
  Cross-cutting utilities: config loaders, error models, request context, correlation IDs.

Flow
----
1) auth: identity and tenant context resolution  
2) intent: semantic intent extraction (prompt or structured input)  
3) pre-rag: pre-retrieval validation and policy checks  
4) rag: governed knowledge retrieval (if applicable)  
5) post-rag: post-retrieval validation and filtering  
6) prompt: prompt normalization and safety validation  
7) policy: full policy evaluation (quota, cost, compliance, model eligibility)  
8) runtime-config: resolve feature flags and execution thresholds  
9) router: model and provider selection with fallback strategies  
10) providers: LLM execution via adapter  
11) audit & observability: event logging, metrics, and traces
