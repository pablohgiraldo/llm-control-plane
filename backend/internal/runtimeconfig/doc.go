// Package runtimeconfig provides dynamic configuration management
// for the LLM Control Plane.
//
// This package enables:
//   - Hot-reloading of configuration without restarts
//   - Per-tenant configuration overrides
//   - Feature flags and gradual rollouts
//   - A/B testing configuration
//   - Provider credential rotation
//
// Configuration can be sourced from environment variables, files,
// or external configuration services (AWS Secrets Manager, etc.).
package runtimeconfig
