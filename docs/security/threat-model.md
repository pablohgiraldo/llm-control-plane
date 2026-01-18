# Threat Model (Initial)

Assumptions
-----------
- LLM providers are external and untrusted.
- Clients may be malicious or compromised.
- Secrets are not embedded in code; rotated via secret manager.

Threats
-------
- Prompt injection and data exfiltration.
- Abuse of API keys; elevation of privileges.
- Quota evasion and cost overruns.
- Model routing tampering.

Mitigations
-----------
- Strict auth (JWT/API keys) and RBAC.
- Prompt validators (PII detection, injection guard).
- Policy engine (quota, cost controls).
- Structured audit logs; immutable records.
- Defense-in-depth on network and runtime.

