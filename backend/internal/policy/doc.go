// Package policy provides the policy evaluation engine for the LLM Control Plane.
//
// This package implements governance rules including:
//   - Rate limiting policies (requests per time window)
//   - Cost cap enforcement (spending limits per org/app)
//   - Model restrictions (allowed/denied models per tenant)
//   - Quota management (token limits, request budgets)
//
// The policy engine evaluates rules before routing requests to LLM providers,
// ensuring compliance with organizational governance requirements.
package policy
