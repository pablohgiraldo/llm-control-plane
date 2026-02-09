// Package routing implements intelligent model routing for the LLM Control Plane.
//
// This package provides:
//   - Model-to-provider mapping
//   - Fallback and circuit breaker patterns
//   - Load balancing across providers
//   - Cost-optimized routing strategies
//   - A/B testing and canary deployments
//
// The router selects the optimal provider based on policies, availability,
// and performance characteristics.
package routing
