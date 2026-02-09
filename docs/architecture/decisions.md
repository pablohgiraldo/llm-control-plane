# Architecture Decision Records (ADRs)

ADR-0001: Stateless services with externalized state
---------------------------------------------------
Status: Accepted

All runtime services are designed to be stateless. Persistent and authoritative
state is externalized to managed data stores to enable horizontal scalability,
fault tolerance, and predictable behavior.

- PostgreSQL is used as the authoritative source of truth for governance data
  (policies, tenants, audit history).
- DynamoDB is used for runtime configuration, feature flags, thresholds, and
  low-latency operational state.
- No mutable local state is stored in application instances.


ADR-0002: Deterministic governance pipeline
-------------------------------------------
Status: Accepted

All AI requests must traverse a deterministic, explicit governance pipeline to
ensure security, compliance, and observability.

The pipeline stages are:
Authentication & Context Resolution →  
Intent & Prompt Validation →  
Pre-RAG Controls →  
Policy Evaluation →  
Model Routing & Provider Selection →  
Provider Execution →  
Post-RAG Validation →  
Audit, Metrics & Tracing.

This guarantees consistent enforcement regardless of provider, adoption mode,
or deployment environment.


ADR-0003: Centralized policy engine with dynamic runtime configuration
----------------------------------------------------------------------
Status: Accepted

Governance rules (quotas, budgets, access controls, content restrictions, routing
constraints) are centralized in a dedicated Policy Engine.

Policies can be updated dynamically at runtime without redeployments, enabling:
- Fast operational changes
- Cost and risk control
- Tenant- and application-level isolation


ADR-0004: RAG with dual verification (pre and post inference)
-------------------------------------------------------------
Status: Accepted

Retrieval-Augmented Generation (RAG) is treated as a governed operation rather
than a trusted data source.

- Pre-RAG controls validate retrieval intent, scope, and data access.
- Post-RAG validation ensures generated outputs do not leak sensitive or
  unauthorized information.

This ensures safe and compliant enterprise knowledge injection.


ADR-0005: Cloud-agnostic core with cloud-native deployment
----------------------------------------------------------
Status: Accepted

The core governance logic is infrastructure-agnostic and can be deployed in
cloud, on-premise, or hybrid environments.

AWS-managed services (EKS, ALB, DynamoDB, CloudWatch) are used for the reference
deployment to maximize resilience and reduce operational overhead, without
coupling core logic to a single provider.
