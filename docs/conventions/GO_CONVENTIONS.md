# Go Conventions and Standards

**Project:** LLM Control Plane  
**Version:** 1.0  
**Last Updated:** February 9, 2026

---

## Table of Contents

1. [Module Structure](#module-structure)
2. [Package Organization](#package-organization)
3. [Naming Conventions](#naming-conventions)
4. [Code Style](#code-style)
5. [Error Handling](#error-handling)
6. [Testing Standards](#testing-standards)
7. [Dependency Management](#dependency-management)
8. [Configuration Management](#configuration-management)
9. [Logging and Observability](#logging-and-observability)
10. [Security Best Practices](#security-best-practices)

---

## Module Structure

### Go Module Definition

**Module Path:** `github.com/upb/llm-control-plane`

**Go Version:** `1.24` (latest stable as of Feb 2026)

### Directory Layout

```
llm-control-plane/
├── go.mod                          # Go module definition
├── go.sum                          # Dependency checksums
├── Makefile                        # Build automation
├── docker-compose.yml              # Local infrastructure
├── env.example                     # Environment template
│
├── backend/
│   ├── cmd/                        # Application entrypoints
│   │   └── api-gateway/
│   │       └── main.go             # Main application entry
│   │
│   └── internal/                   # Private application code
│       ├── auth/                   # Authentication & authorization
│       │   ├── jwt.go
│       │   ├── middleware.go
│       │   └── rbac.go
│       │
│       ├── audit/                  # Audit logging
│       │   ├── events.go
│       │   └── logger.go
│       │
│       ├── metrics/                # Metrics collection
│       │   └── prometheus.go
│       │
│       ├── policy/                 # Policy engine
│       │   ├── engine.go
│       │   ├── cost.go
│       │   └── quota.go
│       │
│       ├── prompt/                 # Prompt validation
│       │   ├── validator.go
│       │   ├── pii_detector.go
│       │   └── injection_guard.go
│       │
│       ├── router/                 # Model routing
│       │   ├── router.go
│       │   ├── fallback.go
│       │   └── providers/
│       │       ├── provider.go     # Interface definition
│       │       ├── openai.go
│       │       ├── anthropic.go
│       │       └── azure.go
│       │
│       ├── storage/                # Data persistence
│       │   ├── postgres.go
│       │   └── redis.go
│       │
│       └── shared/                 # Shared utilities
│           ├── config.go
│           ├── context.go
│           └── errors.go
│
├── docs/                           # Documentation
├── frontend/                       # React frontend (separate)
└── tests/                          # Integration tests
```

### Rationale

- **`cmd/`**: Contains application entrypoints. Each subdirectory is a buildable binary.
- **`internal/`**: Private code that cannot be imported by external projects (Go enforced).
- **Domain-driven packages**: Each package represents a bounded context (auth, policy, router, etc.).
- **Flat hierarchy**: Avoid deep nesting; prefer composition over inheritance.

---

## Package Organization

### Package Naming

- **Lowercase, single-word names**: `auth`, `policy`, `router` (not `authService`, `policyEngine`)
- **Avoid generic names**: Don't use `util`, `common`, `helpers` (be specific)
- **Domain-aligned**: Package names should reflect business domains

### Package Structure

Each domain package should follow this pattern:

```
package_name/
├── types.go          # Domain types and interfaces
├── service.go        # Business logic
├── repository.go     # Data access (if applicable)
├── handler.go        # HTTP handlers (if applicable)
└── *_test.go         # Tests
```

### Example: Policy Package

```go
// backend/internal/policy/types.go
package policy

type Policy struct {
    ID       string
    OrgID    string
    Type     PolicyType
    Config   PolicyConfig
}

type PolicyType string

const (
    PolicyTypeRateLimit PolicyType = "rate_limit"
    PolicyTypeCostCap   PolicyType = "cost_cap"
)

type Engine interface {
    Evaluate(ctx context.Context, req *Request) (*Decision, error)
}
```

```go
// backend/internal/policy/engine.go
package policy

type engine struct {
    repo Repository
    cache Cache
}

func NewEngine(repo Repository, cache Cache) Engine {
    return &engine{repo: repo, cache: cache}
}

func (e *engine) Evaluate(ctx context.Context, req *Request) (*Decision, error) {
    // Implementation
}
```

---

## Naming Conventions

### Variables

- **camelCase** for local variables: `userID`, `requestCount`
- **PascalCase** for exported: `UserID`, `RequestCount`
- **Descriptive names**: Avoid single-letter except for loops (`i`, `j`) and common idioms (`w`, `r` for HTTP)

```go
// Good
userID := "user-123"
requestCount := 0

// Bad
u := "user-123"
rc := 0
```

### Functions

- **Verbs for actions**: `GetUser()`, `CreatePolicy()`, `ValidatePrompt()`
- **Predicates return bool**: `IsValid()`, `HasPermission()`, `CanAccess()`
- **Constructors**: `New()` or `NewTypeName()`

```go
// Good
func NewEngine(repo Repository) Engine { ... }
func ValidatePrompt(prompt string) error { ... }
func IsRateLimited(ctx context.Context) bool { ... }

// Bad
func Engine(repo Repository) Engine { ... }  // Missing 'New'
func Prompt(prompt string) error { ... }     // Not descriptive
```

### Interfaces

- **Single-method interfaces**: Name with `-er` suffix: `Reader`, `Writer`, `Validator`
- **Multi-method interfaces**: Use domain names: `PolicyEngine`, `ProviderClient`

```go
// Good
type Validator interface {
    Validate(ctx context.Context, input string) error
}

type PolicyEngine interface {
    Evaluate(ctx context.Context, req *Request) (*Decision, error)
    GetPolicies(ctx context.Context, orgID string) ([]*Policy, error)
}

// Bad
type IValidator interface { ... }  // Don't use 'I' prefix
type ValidatorInterface interface { ... }  // Redundant
```

### Constants

- **PascalCase** for exported: `MaxRetries`, `DefaultTimeout`
- **Group related constants** in blocks with `iota`

```go
const (
    // HTTP timeouts
    ReadTimeout  = 5 * time.Second
    WriteTimeout = 10 * time.Second
    IdleTimeout  = 120 * time.Second
)

type Status int

const (
    StatusPending Status = iota
    StatusApproved
    StatusRejected
)
```

---

## Code Style

### Formatting

- **Use `gofmt`**: Always format code with `gofmt -s -w .`
- **Line length**: Aim for 100 characters; hard limit at 120
- **Imports**: Group standard library, external, and internal packages

```go
import (
    // Standard library
    "context"
    "fmt"
    "time"

    // External dependencies
    "github.com/go-chi/chi/v5"
    "go.uber.org/zap"

    // Internal packages
    "github.com/upb/llm-control-plane/backend/internal/auth"
    "github.com/upb/llm-control-plane/backend/internal/policy"
)
```

### Comments

- **Package comments**: Every package should have a doc comment
- **Exported symbols**: Must have doc comments starting with the symbol name
- **TODO comments**: Use `// TODO(username): description`

```go
// Package auth provides authentication and authorization primitives
// for the LLM Control Plane. It supports JWT validation, RBAC, and
// multi-tenancy context extraction.
package auth

// Principal represents an authenticated user with associated roles
// and tenant context.
type Principal struct {
    Subject string
    Roles   []string
    Tenant  string
}

// ValidateJWT verifies a JWT token and extracts the principal.
// It returns an error if the token is invalid, expired, or malformed.
func ValidateJWT(token string) (*Principal, error) {
    // TODO(pablo): Add JWKS caching for performance
    // Implementation
}
```

### Function Length

- **Keep functions small**: Aim for < 50 lines
- **Single responsibility**: Each function should do one thing well
- **Extract helpers**: If a function is too long, extract sub-functions

---

## Error Handling

### Error Creation

- **Use `fmt.Errorf` with `%w`** for wrapping errors
- **Custom errors**: Define sentinel errors for known cases
- **Context in errors**: Include relevant context (IDs, values)

```go
// Sentinel errors
var (
    ErrNotFound       = errors.New("resource not found")
    ErrUnauthorized   = errors.New("unauthorized")
    ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// Wrapping errors
func GetUser(ctx context.Context, id string) (*User, error) {
    user, err := repo.FindByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get user %s: %w", id, err)
    }
    return user, nil
}
```

### Error Handling Patterns

```go
// Check and return early
func ProcessRequest(ctx context.Context, req *Request) error {
    if err := validateRequest(req); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    if err := checkPermissions(ctx, req); err != nil {
        return fmt.Errorf("permission denied: %w", err)
    }

    // Main logic
    return nil
}

// Defer cleanup with error handling
func ProcessFile(path string) (err error) {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("failed to open file: %w", err)
    }
    defer func() {
        if closeErr := f.Close(); closeErr != nil && err == nil {
            err = fmt.Errorf("failed to close file: %w", closeErr)
        }
    }()

    // Process file
    return nil
}
```

### Never Panic in Production

- **Avoid `panic()`**: Use errors instead
- **Recover only at boundaries**: HTTP handlers, goroutines
- **Log and return 500**: Don't expose internal errors to clients

```go
// Good: Return error
func ValidateInput(input string) error {
    if input == "" {
        return errors.New("input cannot be empty")
    }
    return nil
}

// Bad: Panic
func ValidateInput(input string) {
    if input == "" {
        panic("input cannot be empty")  // DON'T DO THIS
    }
}
```

---

## Testing Standards

### Test File Naming

- **Same package**: `package_test.go` for black-box tests
- **Internal tests**: `package.go` for white-box tests

### Test Function Naming

```go
func TestFunctionName(t *testing.T) { ... }
func TestFunctionName_EdgeCase(t *testing.T) { ... }
func BenchmarkFunctionName(b *testing.B) { ... }
```

### Table-Driven Tests

```go
func TestValidatePrompt(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {
            name:    "valid prompt",
            input:   "Hello, world!",
            wantErr: false,
        },
        {
            name:    "contains PII",
            input:   "My email is user@example.com",
            wantErr: true,
        },
        {
            name:    "empty prompt",
            input:   "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePrompt(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidatePrompt() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Test Coverage

- **Aim for 80%+ coverage** for critical paths
- **Focus on business logic**: Don't test trivial getters/setters
- **Integration tests**: Use `//go:build integration` tag

```go
//go:build integration

package policy_test

func TestPolicyEngine_Integration(t *testing.T) {
    // Requires real database
}
```

---

## Dependency Management

### Adding Dependencies

```bash
# Add a specific version
go get github.com/go-chi/chi/v5@v5.2.0

# Add latest version
go get github.com/go-chi/chi/v5@latest

# Tidy up
go mod tidy
```

### Dependency Principles

1. **Minimize dependencies**: Only add what you need
2. **Pin versions**: Use exact versions in `go.mod`
3. **Review licenses**: Ensure compatibility with project license
4. **Security audits**: Run `go list -m -json all | nancy sleuth` periodically

### Vendoring

- **Not used by default**: Go modules handle dependencies
- **Use only if required**: For air-gapped deployments

---

## Configuration Management

### Environment Variables

- **Use `godotenv`** for local development
- **Explicit parsing**: Don't rely on defaults silently

```go
package config

import (
    "fmt"
    "os"
    "strconv"
    "time"

    "github.com/joho/godotenv"
)

type Config struct {
    Environment string
    HTTPAddr    string
    DatabaseURL string
    RedisURL    string
    LogLevel    string
}

func Load() (*Config, error) {
    // Load .env file (ignore error in production)
    _ = godotenv.Load()

    cfg := &Config{
        Environment: getEnv("ENVIRONMENT", "dev"),
        HTTPAddr:    getEnv("HTTP_ADDR", ":8080"),
        DatabaseURL: mustGetEnv("DATABASE_URL"),
        RedisURL:    mustGetEnv("REDIS_URL"),
        LogLevel:    getEnv("LOG_LEVEL", "info"),
    }

    return cfg, nil
}

func getEnv(key, fallback string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return fallback
}

func mustGetEnv(key string) string {
    value := os.Getenv(key)
    if value == "" {
        panic(fmt.Sprintf("required environment variable %s is not set", key))
    }
    return value
}
```

---

## Logging and Observability

### Structured Logging

- **Use `zap`** for structured logging
- **Log levels**: DEBUG, INFO, WARN, ERROR
- **Context propagation**: Pass `context.Context` for request IDs

```go
package main

import (
    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    logger.Info("server starting",
        zap.String("addr", ":8080"),
        zap.String("environment", "production"),
    )
}
```

### Metrics

- **Use Prometheus client**: `github.com/prometheus/client_golang`
- **Metric types**: Counter, Gauge, Histogram, Summary
- **Naming**: `namespace_subsystem_metric_unit`

```go
var (
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "llm_cp_requests_total",
            Help: "Total number of requests",
        },
        []string{"method", "path", "status"},
    )

    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "llm_cp_request_duration_seconds",
            Help:    "Request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )
)
```

---

## Security Best Practices

### Input Validation

- **Validate all inputs**: Never trust external data
- **Use allowlists**: Prefer allowlists over denylists
- **Sanitize outputs**: Prevent injection attacks

### Secrets Management

- **Never hardcode secrets**: Use environment variables or secret managers
- **Rotate credentials**: Implement automatic rotation
- **Audit access**: Log all secret accesses

### Context Timeouts

- **Always set timeouts**: Prevent resource exhaustion
- **Propagate context**: Pass `context.Context` through call chains

```go
func MakeRequest(ctx context.Context, url string) error {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    return nil
}
```

---

## Summary

This document establishes the Go conventions for the LLM Control Plane project. All contributors must adhere to these standards to ensure code quality, maintainability, and security.

**Key Principles:**
- **Clarity over cleverness**: Write code that is easy to understand
- **Explicit over implicit**: Make dependencies and behavior obvious
- **Fail fast**: Validate early and return errors immediately
- **Composability**: Build small, reusable components
- **Observability**: Log, measure, and trace everything

For questions or clarifications, refer to:
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
