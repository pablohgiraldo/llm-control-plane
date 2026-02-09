// Package auth provides authentication and authorization primitives
// for the LLM Control Plane.
//
// This package implements:
//   - JWT token validation (Cognito/OIDC)
//   - Role-Based Access Control (RBAC)
//   - Multi-tenancy context extraction
//   - Permission checking middleware
//
// All requests must pass through authentication before reaching
// the policy engine or routing layer.
package auth
