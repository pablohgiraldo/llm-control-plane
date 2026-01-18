# Sequence Diagrams (High-Level)

## Inference Request
Client -> api-gateway: HTTP request
api-gateway -> auth: validate auth (JWT/API key/RBAC)
api-gateway -> prompt: validate prompt (length, content, PII, injection)
api-gateway -> policy: evaluate policies (quota/cost/content)
api-gateway -> router: select provider route (SLA, price, availability)
api-gateway -> audit: record request metadata
api-gateway -> metrics: increment counters/histograms
api-gateway -> provider: outbound call
provider -> api-gateway: response (stream/non-stream)
api-gateway -> audit: record outcome
api-gateway -> Client: HTTP response

