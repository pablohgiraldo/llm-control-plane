# Sequence Diagrams (High-Level)

## Inference / Non-Prompt Request

Client -> API Gateway: HTTP request (text, JSON, event, task)
API Gateway -> Auth Module: authenticate (API Key / JWT / RBAC)
Auth Module -> API Gateway: identity + org/app context

API Gateway -> Intent Analyzer: infer semantic intent (prompt / structured data)
Intent Analyzer -> API Gateway: intent classification

API Gateway -> Prompt & Payload Validator: sanitize input (PII, injection, schema)
Prompt & Payload Validator -> API Gateway: validated request

API Gateway -> Pre-RAG Controls: validate retrieval scope & permissions
Pre-RAG Controls -> Knowledge Store: retrieve governed context
Knowledge Store -> Pre-RAG Controls: contextual data
Pre-RAG Controls -> API Gateway: enriched request

API Gateway -> Policy Engine: evaluate policies (quota, cost, routing, compliance)
Policy Engine -> Runtime Config (DynamoDB): fetch flags, thresholds
Policy Engine -> Source of Truth (PostgreSQL): validate constraints
Policy Engine -> API Gateway: policy decision

API Gateway -> Model Router: select provider/model (rules, SLA, cost, availability)
Model Router -> Provider Adapter: execute request
Provider Adapter -> LLM Provider: outbound inference
LLM Provider -> Provider Adapter: raw response

Provider Adapter -> Post-RAG Validation: output sanitization & leakage checks
Post-RAG Validation -> API Gateway: approved response

API Gateway -> Audit Log (PostgreSQL): persist request/response trace
API Gateway -> Metrics & ML Pipeline: emit metrics, usage signals, categorizations

API Gateway -> Client: HTTP response (sync / stream)
