// Package providers implements adapters for external LLM providers.
//
// This package provides:
//   - Unified interface for multiple LLM providers
//   - Provider-specific request/response translation
//   - Error handling and retry logic
//   - Circuit breaker patterns
//   - Cost calculation per provider
//
// Each provider adapter implements the Provider interface,
// enabling transparent switching and fallback.
package providers
