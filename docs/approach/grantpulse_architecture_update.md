# GrantPulse Architecture Update for LLM Control Plane

## Executive Summary

Based on the GrantPulse backend refactor analysis, the following architectural patterns should be applied to the LLM Control Plane project:

### Key Patterns from GrantPulse Refactor:

1. **Dependency Injection Container** (`app/dependencies.go`)
2. **Repository Pattern with Interfaces** (`repositories/interfaces.go`)
3. **Domain-Driven Service Organization** (services organized by feature domain)
4. **Layered Architecture** (routes → handlers → services → repositories)
5. **Transaction Management** (`services/transaction.go`)
6. **Domain Errors** (`services/errors.go` with type checking helpers)
7. **Thin Handlers** (only parse, validate, delegate, format)
8. **Middleware for Cross-Cutting Concerns**
9. **Testing with Mocks** (`repositories/mocks/`)

## Updated Backend Structure

```
backend/
├── app/
│   └── dependencies.go               # Dependency injection container
├── config/
│   └── config.go                     # Configuration management
├── db/
│   └── connection.go                 # Database connection setup
├── models/                           # Domain models (entities)
│   ├── organization.go
│   ├── application.go
│   ├── user.go
│   ├── policy.go
│   ├── audit_log.go
│   └── inference_request.go
├── repositories/                     # Data access layer
│   ├── interfaces.go                 # Repository interfaces
│   ├── postgres/                     # PostgreSQL implementations
│   │   ├── organization_repository.go
│   │   ├── policy_repository.go
│   │   ├── audit_repository.go
│   │   └── transaction.go            # Transaction support
│   └── mocks/                        # Mock implementations for testing
├── services/                         # Business logic layer (domain-driven)
│   ├── inference/                    # LLM inference domain
│   │   ├── service.go
│   │   └── service_test.go
│   ├── policy/                       # Policy management domain
│   │   ├── service.go
│   │   ├── cache.go                  # Redis caching
│   │   └── service_test.go
│   ├── ratelimit/                    # Rate limiting domain
│   │   ├── service.go
│   │   └── limiter.go
│   ├── budget/                       # Budget tracking domain
│   │   ├── service.go
│   │   └── tracker.go
│   ├── audit/                        # Audit logging domain
│   │   ├── service.go
│   │   └── async_logger.go
│   ├── prompt/                       # Prompt validation domain
│   │   ├── service.go
│   │   ├── pii_detector.go
│   │   ├── secrets_detector.go
│   │   └── injection_guard.go
│   ├── providers/                    # LLM provider integrations
│   │   ├── openai/
│   │   │   ├── adapter.go
│   │   │   └── client.go
│   │   ├── interface.go              # Provider interface
│   │   └── registry.go               # Provider registry
│   ├── routing/                      # Model routing logic
│   │   └── service.go
│   ├── transaction.go                # Transaction helpers
│   └── errors.go                     # Domain errors
├── handlers/                         # HTTP handlers (thin layer)
│   ├── inference_handler.go          # LLM inference endpoints
│   ├── policy_handler.go             # Policy management endpoints
│   ├── audit_handler.go              # Audit query endpoints
│   ├── health_handler.go             # Health checks
│   ├── dtos.go                       # Data Transfer Objects
│   ├── validation.go                 # Request validation
│   └── service_errors.go             # Error mapping
├── routes/                           # Route definitions
│   ├── routes.go                     # Main router setup
│   ├── inference_routes.go           # Inference-specific routes
│   ├── policy_routes.go              # Policy-specific routes
│   └── admin_routes.go               # Admin routes
├── middleware/                       # HTTP middleware
│   ├── middleware.go                 # Core middleware (auth, logging, errors)
│   ├── context.go                    # Context helpers
│   ├── auth_middleware.go            # Authentication/authorization
│   └── policy_middleware.go          # Policy enforcement middleware
├── cognito/                          # AWS Cognito integration
│   └── validator.go                  # JWT validation
├── utils/                            # Utility functions
│   ├── http_utils.go                 # HTTP response helpers
│   └── validation.go                 # Validation helpers
├── migrations/                       # Database migrations
│   └── schema/
│       ├── 001_initial.up.sql
│       ├── 001_initial.down.sql
│       ├── 002_policies.up.sql
│       └── 002_policies.down.sql
├── lambda.go                         # Lambda handler (for AWS deployment)
└── main.go                           # Application entry point
```

## Detailed Implementation Patterns

### 1. Dependency Injection Container

**File: `backend/app/dependencies.go`**

```go
package app

import (
    "database/sql"
    "github.com/redis/go-redis/v9"
    "github.com/upb/llm-control-plane/backend/config"
    "github.com/upb/llm-control-plane/backend/repositories"
    "github.com/upb/llm-control-plane/backend/services"
)

type Dependencies struct {
    // Infrastructure
    DB          *sql.DB
    RedisClient *redis.Client
    Config      *config.Config
    
    // Repositories (interfaces - testable!)
    OrgRepository    repositories.OrganizationRepository
    PolicyRepository repositories.PolicyRepository
    AuditRepository  repositories.AuditRepository
    
    // Services (business logic)
    InferenceService *inference.Service
    PolicyService    *policy.Service
    RateLimitService *ratelimit.Service
    BudgetService    *budget.Service
    AuditService     *audit.Service
    PromptService    *prompt.Service
    RoutingService   *routing.Service
    
    // External integrations
    CognitoValidator *cognito.Validator
}

func NewDependencies(cfg *config.Config, db *sql.DB, redisClient *redis.Client) (*Dependencies, error) {
    // Initialize repositories
    orgRepo := postgres.NewOrganizationRepository(db)
    policyRepo := postgres.NewPolicyRepository(db)
    auditRepo := postgres.NewAuditRepository(db)
    txManager := postgres.NewTransactionManager(db)
    
    // Initialize services with dependencies
    policyCache := policy.NewRedisCache(redisClient)
    policyService := policy.NewService(policyRepo, policyCache)
    
    rateLimitService := ratelimit.NewService(redisClient)
    budgetService := budget.NewService(redisClient, txManager)
    auditService := audit.NewService(auditRepo)
    promptService := prompt.NewService()
    
    // Initialize provider registry
    providerRegistry := providers.NewRegistry()
    providerRegistry.RegisterProvider("openai", openai.NewAdapter(cfg.Providers.OpenAI.APIKey))
    routingService := routing.NewService(providerRegistry)
    
    // Initialize main inference service (orchestrates pipeline)
    inferenceService := inference.NewService(
        policyService,
        rateLimitService,
        budgetService,
        promptService,
        routingService,
        auditService,
        txManager,
    )
    
    // Initialize Cognito validator
    cognitoValidator := cognito.NewValidator(cfg.Cognito.Region, cfg.Cognito.UserPoolID)
    
    return &Dependencies{
        DB:                db,
        RedisClient:       redisClient,
        Config:            cfg,
        OrgRepository:     orgRepo,
        PolicyRepository:  policyRepo,
        AuditRepository:   auditRepo,
        InferenceService:  inferenceService,
        PolicyService:     policyService,
        RateLimitService:  rateLimitService,
        BudgetService:     budgetService,
        AuditService:      auditService,
        PromptService:     promptService,
        RoutingService:    routingService,
        CognitoValidator:  cognitoValidator,
    }, nil
}
```

### 2. Repository Pattern with Interfaces

**File: `backend/repositories/interfaces.go`**

```go
package repositories

import (
    "context"
    "time"
    "github.com/upb/llm-control-plane/backend/models"
)

// TransactionManager manages database transactions
type TransactionManager interface {
    BeginTx(ctx context.Context) (Transaction, error)
}

// Transaction represents a database transaction
type Transaction interface {
    Commit() error
    Rollback() error
}

// PolicyRepository handles policy data access
type PolicyRepository interface {
    Create(ctx context.Context, policy *models.Policy) error
    GetByOrgID(ctx context.Context, orgID string) ([]*models.Policy, error)
    GetByAppID(ctx context.Context, orgID, appID string) ([]*models.Policy, error)
    GetByUserID(ctx context.Context, orgID, appID, userID string) ([]*models.Policy, error)
    Update(ctx context.Context, policy *models.Policy) error
    Delete(ctx context.Context, id string) error
    WithTx(tx Transaction) PolicyRepository  // Transaction support
}

// AuditRepository handles audit log data access
type AuditRepository interface {
    Insert(ctx context.Context, log *models.AuditLog) error
    GetByOrgID(ctx context.Context, orgID string, limit, offset int) ([]*models.AuditLog, error)
    GetByDateRange(ctx context.Context, orgID string, start, end time.Time) ([]*models.AuditLog, error)
    WithTx(tx Transaction) AuditRepository
}

// OrganizationRepository handles organization data access
type OrganizationRepository interface {
    Create(ctx context.Context, org *models.Organization) error
    GetByID(ctx context.Context, id string) (*models.Organization, error)
    Update(ctx context.Context, org *models.Organization) error
    WithTx(tx Transaction) OrganizationRepository
}
```

### 3. Domain Errors

**File: `backend/services/errors.go`**

```go
package services

import "errors"

// Domain errors (GrantPulse pattern)
var (
    // Policy errors
    ErrPolicyNotFound         = errors.New("policy not found")
    ErrPolicyDenied           = errors.New("request denied by policy")
    ErrInvalidPolicyConfig    = errors.New("invalid policy configuration")
    
    // Rate limiting errors
    ErrRateLimitExceeded      = errors.New("rate limit exceeded")
    ErrTokenLimitExceeded     = errors.New("token limit exceeded")
    
    // Budget errors
    ErrBudgetExceeded         = errors.New("budget limit exceeded")
    ErrInsufficientFunds      = errors.New("insufficient funds")
    
    // Prompt validation errors
    ErrPIIDetected            = errors.New("PII detected in prompt")
    ErrSecretsDetected        = errors.New("secrets detected in prompt")
    ErrInjectionDetected      = errors.New("injection attempt detected")
    
    // Model/provider errors
    ErrModelNotSupported      = errors.New("model not supported")
    ErrProviderUnavailable    = errors.New("provider unavailable")
    ErrProviderRateLimited    = errors.New("provider rate limited")
    
    // Auth errors
    ErrUnauthorized           = errors.New("unauthorized")
    ErrForbidden              = errors.New("forbidden")
    ErrInvalidToken           = errors.New("invalid token")
)

// Error type checking helpers
func IsNotFoundError(err error) bool {
    return errors.Is(err, ErrPolicyNotFound)
}

func IsValidationError(err error) bool {
    return errors.Is(err, ErrPIIDetected) || 
           errors.Is(err, ErrSecretsDetected) ||
           errors.Is(err, ErrInjectionDetected)
}

func IsRateLimitError(err error) bool {
    return errors.Is(err, ErrRateLimitExceeded) ||
           errors.Is(err, ErrTokenLimitExceeded) ||
           errors.Is(err, ErrProviderRateLimited)
}

func IsBudgetError(err error) bool {
    return errors.Is(err, ErrBudgetExceeded) ||
           errors.Is(err, ErrInsufficientFunds)
}

func IsAuthError(err error) bool {
    return errors.Is(err, ErrUnauthorized) ||
           errors.Is(err, ErrForbidden) ||
           errors.Is(err, ErrInvalidToken)
}
```

### 4. Transaction Management Helper

**File: `backend/services/transaction.go`**

```go
package services

import (
    "context"
    "github.com/upb/llm-control-plane/backend/repositories"
)

// WithTransaction executes a function within a transaction
func WithTransaction(ctx context.Context, txManager repositories.TransactionManager, 
    fn func(tx repositories.Transaction) error) error {
    
    tx, err := txManager.BeginTx(ctx)
    if err != nil {
        return err
    }
    
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            panic(r)
        }
    }()
    
    if err := fn(tx); err != nil {
        tx.Rollback()
        return err
    }
    
    return tx.Commit()
}

// WithTransactionResult executes a function that returns a value and error within a transaction
func WithTransactionResult[T any](ctx context.Context, txManager repositories.TransactionManager,
    fn func(tx repositories.Transaction) (T, error)) (T, error) {
    
    var result T
    tx, err := txManager.BeginTx(ctx)
    if err != nil {
        return result, err
    }
    
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            panic(r)
        }
    }()
    
    result, err = fn(tx)
    if err != nil {
        tx.Rollback()
        return result, err
    }
    
    return result, tx.Commit()
}
```

### 5. Thin HTTP Handlers with Service Error Mapping

**File: `backend/handlers/service_errors.go`**

```go
package handlers

import (
    "net/http"
    "github.com/upb/llm-control-plane/backend/services"
    "github.com/upb/llm-control-plane/backend/utils"
)

// handleServiceError maps service layer errors to HTTP responses
func handleServiceError(w http.ResponseWriter, err error) {
    switch {
    case services.IsNotFoundError(err):
        utils.WriteNotFound(w, err.Error())
    
    case services.IsValidationError(err):
        utils.WriteBadRequest(w, err.Error())
    
    case services.IsRateLimitError(err):
        utils.WriteTooManyRequests(w, err.Error())
    
    case services.IsBudgetError(err):
        utils.WritePaymentRequired(w, err.Error())
    
    case services.IsAuthError(err):
        if errors.Is(err, services.ErrUnauthorized) || errors.Is(err, services.ErrInvalidToken) {
            utils.WriteUnauthorized(w, err.Error())
        } else {
            utils.WriteForbidden(w, err.Error())
        }
    
    default:
        log.Error("unhandled service error", zap.Error(err))
        utils.WriteInternalServerError(w, "An unexpected error occurred")
    }
}
```

**File: `backend/handlers/inference_handler.go`** (Example Handler)

```go
package handlers

import (
    "encoding/json"
    "net/http"
    "github.com/upb/llm-control-plane/backend/middleware"
    "github.com/upb/llm-control-plane/backend/services/inference"
)

// HandleChatCompletion handles LLM chat completion requests (thin handler pattern)
func HandleChatCompletion(inferenceService *inference.Service) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Extract context (user, org from middleware)
        claims := middleware.MustGetClaimsFromContext(r.Context())
        
        // 2. Parse request
        var req ChatCompletionRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            utils.WriteBadRequest(w, "Invalid JSON")
            return
        }
        
        // 3. Validate request
        if err := ValidateStruct(&req); err != nil {
            utils.WriteBadRequest(w, err.Error())
            return
        }
        
        // 4. Convert to service request DTO
        serviceReq := &inference.CompletionRequest{
            OrgID:    claims.OrgID,
            AppID:    claims.AppID,
            UserID:   claims.Sub,
            Model:    req.Model,
            Messages: req.Messages,
            MaxTokens: req.MaxTokens,
        }
        
        // 5. Call service (all business logic is in the service layer)
        resp, err := inferenceService.ProcessChatCompletion(r.Context(), serviceReq)
        if err != nil {
            handleServiceError(w, err)
            return
        }
        
        // 6. Format response
        utils.WriteOK(w, resp, "")
    }
}
```

### 6. Domain-Driven Service Organization

**File: `backend/services/inference/service.go`**

```go
package inference

import (
    "context"
    "github.com/upb/llm-control-plane/backend/services"
    "github.com/upb/llm-control-plane/backend/services/policy"
    "github.com/upb/llm-control-plane/backend/services/ratelimit"
    "github.com/upb/llm-control-plane/backend/services/budget"
    "github.com/upb/llm-control-plane/backend/services/prompt"
    "github.com/upb/llm-control-plane/backend/services/routing"
    "github.com/upb/llm-control-plane/backend/services/audit"
    "github.com/upb/llm-control-plane/backend/repositories"
)

// Service orchestrates the LLM inference pipeline
type Service struct {
    policyService    *policy.Service
    rateLimitService *ratelimit.Service
    budgetService    *budget.Service
    promptService    *prompt.Service
    routingService   *routing.Service
    auditService     *audit.Service
    txManager        repositories.TransactionManager
}

// NewService creates a new inference service
func NewService(
    policyService *policy.Service,
    rateLimitService *ratelimit.Service,
    budgetService *budget.Service,
    promptService *prompt.Service,
    routingService *routing.Service,
    auditService *audit.Service,
    txManager repositories.TransactionManager,
) *Service {
    return &Service{
        policyService:    policyService,
        rateLimitService: rateLimitService,
        budgetService:    budgetService,
        promptService:    promptService,
        routingService:   routingService,
        auditService:     auditService,
        txManager:        txManager,
    }
}

// ProcessChatCompletion orchestrates the complete inference pipeline
func (s *Service) ProcessChatCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    startTime := time.Now()
    
    // 1. Rate limiting check
    if err := s.rateLimitService.CheckLimit(ctx, req.OrgID, req.AppID); err != nil {
        return nil, err // Returns ErrRateLimitExceeded
    }
    
    // 2. Policy evaluation
    decision, err := s.policyService.Evaluate(ctx, &policy.EvaluationRequest{
        OrgID:  req.OrgID,
        AppID:  req.AppID,
        UserID: req.UserID,
        Model:  req.Model,
    })
    if err != nil {
        return nil, err
    }
    if !decision.Allow {
        return nil, services.ErrPolicyDenied
    }
    
    // 3. Budget pre-check
    if err := s.budgetService.CheckBudget(ctx, req.OrgID, req.AppID, decision.DailyCostCap); err != nil {
        return nil, err // Returns ErrBudgetExceeded
    }
    
    // 4. Prompt validation
    if err := s.promptService.Validate(ctx, req.Messages); err != nil {
        return nil, err // Returns ErrPIIDetected, ErrSecretsDetected, or ErrInjectionDetected
    }
    
    // 5. Route to provider
    provider, err := s.routingService.RouteRequest(ctx, req.Model)
    if err != nil {
        return nil, err
    }
    
    // 6. Invoke LLM
    resp, err := provider.Complete(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("provider request failed: %w", err)
    }
    
    // 7. Response validation
    if err := s.promptService.ValidateResponse(ctx, resp.Content); err != nil {
        return nil, err
    }
    
    // 8. Calculate cost and update budget
    cost := provider.EstimateCost(req.Model, resp.Usage.TotalTokens)
    s.budgetService.RecordCost(ctx, req.OrgID, req.AppID, cost)
    
    // 9. Async audit logging
    s.auditService.LogEvent(ctx, &audit.AuditEvent{
        RequestID:        middleware.GetRequestID(ctx),
        Timestamp:        time.Now(),
        OrgID:            req.OrgID,
        AppID:            req.AppID,
        UserID:           req.UserID,
        Model:            req.Model,
        PromptTokens:     resp.Usage.PromptTokens,
        CompletionTokens: resp.Usage.CompletionTokens,
        TotalCost:        cost,
        LatencyMs:        int(time.Since(startTime).Milliseconds()),
        ClientIP:         middleware.GetClientIP(ctx),
    })
    
    return resp, nil
}
```

## Benefits of This Architecture

1. **Testability**: Repository interfaces enable easy mocking
2. **Maintainability**: Clear separation of concerns across layers
3. **Scalability**: Domain-driven organization supports growth
4. **Flexibility**: Easy to swap implementations (e.g., PostgreSQL → MySQL)
5. **Transaction Support**: Consistent transaction handling via interfaces
6. **Error Handling**: Type-safe domain errors with HTTP mapping
7. **Dependency Management**: Centralized via `app.Dependencies`
8. **Code Reusability**: Middleware, utils, and service patterns

## Migration Steps

1. Create `app/dependencies.go` for centralized DI
2. Create `repositories/interfaces.go` with all repository interfaces
3. Move current `internal/*` packages to appropriate locations:
   - `internal/auth` → Keep, enhance with context helpers
   - `internal/policy` → Move to `services/policy/`
   - `internal/audit` → Move to `services/audit/`
   - `internal/providers` → Move to `services/providers/`
4. Create `services/errors.go` with domain errors
5. Create `services/transaction.go` with transaction helpers
6. Create `handlers/` with thin handlers
7. Create `routes/` with route definitions
8. Create `middleware/` with auth and policy middleware
9. Update `main.go` to use `app.NewDependencies()`

## Summary

The GrantPulse refactor provides a proven blueprint for organizing the LLM Control Plane backend with:
- Clear layered architecture
- Domain-driven service organization
- Repository pattern for data access
- Dependency injection for testability
- Thin handlers that delegate to services
- Domain errors with type checking
- Transaction management helpers

This architecture has been battle-tested in production and should be applied to ensure long-term maintainability and scalability.
