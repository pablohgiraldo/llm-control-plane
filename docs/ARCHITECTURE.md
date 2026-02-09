# LLM Control Plane - AWS Serverless Architecture Guide

**Version:** 1.0  
**Date:** February 6, 2026  
**Based on:** PRD v1.0 + tsum-app Architecture Patterns

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Architecture Overview](#architecture-overview)
3. [AWS Services Selection](#aws-services-selection)
4. [Reusable Patterns from tsum-app](#reusable-patterns-from-tsum-app)
5. [Detailed Component Design](#detailed-component-design)
6. [Development Environment Setup](#development-environment-setup)
7. [Production Environment Setup](#production-environment-setup)
8. [GitHub Actions CI/CD Pipeline](#github-actions-cicd-pipeline)
9. [Multi-Tenancy Strategy](#multi-tenancy-strategy)
10. [Security Implementation](#security-implementation)
11. [Observability & Monitoring](#observability--monitoring)
12. [Cost Optimization](#cost-optimization)
13. [Migration Path](#migration-path)

---

## 1. Executive Summary

This document outlines a **serverless AWS architecture** for the LLM Control Plane, leveraging proven patterns from the tsum-app implementation. The architecture prioritizes:

- **Serverless-first**: Lambda, API Gateway, Aurora Serverless for auto-scaling and cost efficiency
- **Infrastructure as Code**: GitHub Actions with reusable Terraform-based actions
- **Development parity**: Local development environment mirrors production
- **Security by design**: Cognito for auth, Secrets Manager for credentials, WAF for protection
- **Observability**: Datadog APM for distributed tracing

### Key Differences from tsum-app

| Aspect | tsum-app | LLM Control Plane |
|--------|----------|-------------------|
| **Primary function** | CRUD application | Middleware/Gateway |
| **Request pattern** | Low-latency CRUD | Proxy to external LLMs |
| **Data model** | Relational entities | Request/response logging + policies |
| **Compute** | Single Lambda function | Single Lambda + async workers |
| **Scalability** | ~100 requests/sec | 1000+ requests/sec target |
| **Caching** | Not critical | Essential (Redis for rate limiting) |

---

## 2. Architecture Overview

### 2.1 High-Level Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                    CloudFront (Frontend)                       │
│                  https://llm-cp.yourdomain.com                 │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│              API Gateway + Custom Domain + WAF                 │
│                https://api.llm-cp.yourdomain.com               │
└────────────────────────┬───────────────────────────────────────┘
                         │
                         ▼
┌────────────────────────────────────────────────────────────────┐
│                    Lambda Function (Go)                        │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  Request Pipeline                                        │ │
│  │  1. Auth (Cognito JWT validation)                       │ │
│  │  2. Prompt Validation (PII/secrets detection)           │ │
│  │  3. Policy Evaluation (Redis rate limiting)             │ │
│  │  4. Model Routing                                       │ │
│  │  5. LLM Provider Call (OpenAI/Anthropic/Azure)          │ │
│  │  6. Audit Logging (async to PostgreSQL + S3)            │ │
│  └──────────────────────────────────────────────────────────┘ │
└─────────┬──────────────────────┬───────────────┬──────────────┘
          │                      │               │
          ▼                      ▼               ▼
┌─────────────────┐    ┌─────────────────┐    ┌──────────────┐
│  Aurora         │    │  ElastiCache    │    │  S3 Bucket   │
│  PostgreSQL     │    │  (Redis)        │    │  (Audit      │
│  Serverless v2  │    │  - Rate limits  │    │   Logs)      │
│  - Policies     │    │  - Cache        │    └──────────────┘
│  - Audit logs   │    │  - Session      │
│  - Users/orgs   │    └─────────────────┘
└─────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────┐
│              AWS Secrets Manager                            │
│  - Aurora credentials                                       │
│  - OpenAI API keys                                          │
│  - Anthropic API keys                                       │
│  - Azure OpenAI credentials                                 │
└─────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────┐
│              External LLM Providers                         │
│  - OpenAI (GPT-4, GPT-3.5)                                  │
│  - Anthropic (Claude)                                       │
│  - Azure OpenAI                                             │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Request Flow (Sequence)

```
Client → CloudFront (if web UI) → API Gateway → Lambda
                                                  │
                                                  ├──→ Cognito (JWT validation)
                                                  │
                                                  ├──→ Redis (rate limit check)
                                                  │
                                                  ├──→ Prompt validation (in-memory)
                                                  │
                                                  ├──→ Policy engine (PostgreSQL + Redis)
                                                  │
                                                  ├──→ LLM Provider (OpenAI/Anthropic)
                                                  │
                                                  ├──→ PostgreSQL (audit log - async)
                                                  │
                                                  └──→ S3 (long-term audit - async)
```

---

## 3. AWS Services Selection

### 3.1 Core Services (from tsum-app patterns)

| Service | Purpose | Configuration |
|---------|---------|---------------|
| **Lambda** | API Gateway handler | Go runtime, 512MB memory, 30s timeout, VPC-enabled |
| **API Gateway** | HTTP routing + custom domain | REST API with Lambda proxy integration |
| **Cognito** | User authentication | User Pool with JWT tokens, custom claims for org/app/role |
| **Aurora PostgreSQL Serverless v2** | Primary database | 0.5-4 ACU, multi-AZ, VPC-private |
| **ElastiCache (Redis)** | Rate limiting + caching | 1 node (dev), 2 nodes with replication (prod) |
| **S3** | Long-term audit log storage | Lifecycle policy: transition to Glacier after 90 days |
| **CloudFront** | Frontend hosting | SPA configuration with S3 origin |
| **Secrets Manager** | Credential storage | Auto-rotation for Aurora, manual for provider API keys |
| **CloudWatch** | Logs + metrics | Lambda logs, API Gateway logs, custom metrics |
| **WAF** | DDoS + OWASP protection | Rate limiting, geo-blocking, OWASP Top 10 rules |
| **IAM** | Service permissions | Least-privilege roles for Lambda, API Gateway, etc. |

### 3.2 Additional Services (specific to LLM Control Plane)

| Service | Purpose | Configuration |
|---------|---------|---------------|
| **EventBridge** | Async audit log processing | Trigger Lambda for S3 archival |
| **SQS** | Audit log queue | FIFO queue for ordered audit logs |
| **KMS** | Encryption keys | Data encryption at rest for Aurora + S3 |
| **VPC Endpoints** | Private AWS service access | S3, Secrets Manager, DynamoDB (no internet egress) |

### 3.3 Optional Services (future enhancements)

| Service | Purpose | When to Add |
|---------|---------|-------------|
| **SageMaker** | Local model hosting | Phase 2: Support self-hosted models |
| **ECS Fargate** | Long-running workers | Phase 2: Batch processing, analytics |
| **Step Functions** | Complex workflows | Phase 2: Multi-step policy evaluation |
| **DynamoDB** | Alternative for rate limiting | If Redis costs become prohibitive |

---

## 4. Reusable Patterns from tsum-app

### 4.1 Lambda Handler Pattern

**tsum-app pattern** (`backend/lambda.go`):
```go
// Unified handler supporting multiple event types
func handler(ctx context.Context, event json.RawMessage) (interface{}, error) {
    // Detect event type
    if isAPIGatewayV2(event) {
        return handleHTTPRequest(ctx, parseV2Event(event))
    } else if isFunctionURL(event) {
        return handleHTTPRequest(ctx, parseFunctionURLEvent(event))
    } else if isCustomEvent(event) {
        return handleCustomEvent(ctx, parseCustomEvent(event))
    }
}
```

**Adaptation for LLM Control Plane:**
```go
// backend/lambda.go
func handler(ctx context.Context, event json.RawMessage) (interface{}, error) {
    // HTTP requests (API Gateway)
    if isAPIGatewayEvent(event) {
        return handleInferenceRequest(ctx, event)
    }
    
    // Async workers (EventBridge/SQS)
    if isSQSEvent(event) {
        return handleAuditLogProcessing(ctx, event)
    }
    
    // Admin operations (direct invocation)
    if isCustomEvent(event) {
        switch getEventType(event) {
        case "migration":
            return runMigrations(ctx)
        case "policy-update":
            return refreshPolicyCache(ctx)
        }
    }
}
```

**Benefits:**
- Single deployment artifact
- Reduced cold start overhead
- Simplified CI/CD

### 4.2 Configuration Management

**tsum-app pattern** (`backend/config/config.go`):
```go
type Config struct {
    Environment     string
    DatabaseURL     string // from Secrets Manager
    CognitoUserPoolID string
    S3BucketName    string
    FrontEndURL     string
}

func LoadConfig() (*Config, error) {
    cfg := &Config{
        Environment: getEnvOrDefault("ENVIRONMENT", "dev"),
    }
    
    // Load database credentials from Secrets Manager
    if secretArn := os.Getenv("AURORA_SECRET_ARN"); secretArn != "" {
        secret, err := getSecretValue(secretArn)
        if err != nil {
            return nil, err
        }
        cfg.DatabaseURL = parseDBURL(secret)
    } else {
        // Local development: use DATABASE_URL env var
        cfg.DatabaseURL = os.Getenv("DATABASE_URL")
    }
    
    return cfg, nil
}
```

**Adaptation for LLM Control Plane:**
```go
// backend/config/config.go
type Config struct {
    Environment        string
    DatabaseURL        string
    RedisURL           string
    S3BucketName       string
    
    // Cognito
    CognitoUserPoolID  string
    CognitoClientID    string
    
    // LLM Provider credentials (from Secrets Manager)
    OpenAIAPIKey       string
    AnthropicAPIKey    string
    AzureOpenAIKey     string
    AzureOpenAIEndpoint string
    
    // Observability
    DatadogAPIKey      string
    DatadogSite        string
}

func LoadConfig() (*Config, error) {
    cfg := &Config{
        Environment: getEnvOrDefault("ENVIRONMENT", "dev"),
    }
    
    // Database
    if secretArn := os.Getenv("AURORA_SECRET_ARN"); secretArn != "" {
        cfg.DatabaseURL = getSecretValue(secretArn, "connection_string")
    } else {
        cfg.DatabaseURL = os.Getenv("DATABASE_URL")
    }
    
    // Redis
    cfg.RedisURL = getEnvOrDefault("REDIS_URL", "localhost:6379")
    
    // LLM provider credentials
    if secretArn := os.Getenv("PROVIDER_SECRETS_ARN"); secretArn != "" {
        secret := getSecretJSON(secretArn)
        cfg.OpenAIAPIKey = secret["openai_api_key"]
        cfg.AnthropicAPIKey = secret["anthropic_api_key"]
        cfg.AzureOpenAIKey = secret["azure_openai_key"]
        cfg.AzureOpenAIEndpoint = secret["azure_openai_endpoint"]
    }
    
    return cfg, nil
}
```

### 4.3 Router + Middleware Pattern

**tsum-app pattern** (`backend/routes/routes.go`):
```go
func SetupRoutes(r chi.Router, db *sql.DB, deps *app.Dependencies) {
    // Global middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(errorMiddleware)
    r.Use(corsMiddleware)
    
    // Public routes
    r.Get("/health", healthHandler)
    
    // Protected routes
    r.Group(func(r chi.Router) {
        r.Use(RequireAuthMiddleware)
        r.Use(TenantMiddleware)
        
        // Admin routes
        r.Route("/api/v1/admin", func(r chi.Router) {
            r.Use(RequireRoleMiddleware("admin"))
            r.Get("/users", listUsersHandler)
        })
        
        // Application routes
        r.Route("/api/v1", func(r chi.Router) {
            r.Post("/inrush", createInrushHandler)
        })
    })
}
```

**Adaptation for LLM Control Plane:**
```go
// backend/routes/routes.go
func SetupRoutes(r chi.Router, deps *Dependencies) {
    // Global middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(errorMiddleware)
    r.Use(corsMiddleware)
    
    // Public endpoints
    r.Get("/health", healthHandler)
    r.Get("/health/ready", readinessHandler)
    
    // Inference API (protected)
    r.Group(func(r chi.Router) {
        r.Use(RequireAuthMiddleware)        // JWT validation
        r.Use(ExtractTenantMiddleware)      // Org/App context
        r.Use(PromptValidationMiddleware)   // PII/secrets detection
        r.Use(PolicyEnforcementMiddleware)  // Rate limits, cost caps
        
        r.Post("/v1/chat/completions", chatCompletionHandler)
        r.Post("/v1/embeddings", embeddingsHandler)
    })
    
    // Admin API (superadmin only)
    r.Route("/v1/admin", func(r chi.Router) {
        r.Use(RequireAuthMiddleware)
        r.Use(RequireRoleMiddleware("superadmin"))
        
        r.Route("/organizations", func(r chi.Router) {
            r.Get("/", listOrganizationsHandler)
            r.Post("/", createOrganizationHandler)
            r.Put("/{id}", updateOrganizationHandler)
        })
        
        r.Route("/policies", func(r chi.Router) {
            r.Get("/", listPoliciesHandler)
            r.Post("/", createPolicyHandler)
        })
        
        r.Get("/audit-logs", queryAuditLogsHandler)
        r.Get("/metrics/usage", getUsageMetricsHandler)
    })
}
```

### 4.4 Database Migration Pattern

**tsum-app pattern** (Event-driven migrations):
```go
// backend/lambda.go (custom event handling)
type CustomEvent struct {
    Type string `json:"type"`
}

if event.Type == "user-migration" {
    return runMigrations(ctx, deps.DB)
}
```

**GitHub Actions invocation:**
```yaml
- name: Run database migrations
  run: |
    aws lambda invoke \
      --function-name ${{ steps.lambda.outputs.function_arn }} \
      --payload '{"type": "user-migration"}' \
      /tmp/migration-response.json
```

**Adaptation for LLM Control Plane:**
- Same pattern: invoke Lambda with `{"type": "user-migration"}`
- Migrations location: `backend/migrations/schema/`
- Up/down migrations for rollback support

### 4.5 Repository Pattern

**tsum-app pattern:**
```go
// internal/inrush/repository.go
type Repository struct {
    db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
    return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, entity *Entity) error {
    query := `INSERT INTO entities (id, name) VALUES ($1, $2)`
    _, err := r.db.ExecContext(ctx, query, entity.ID, entity.Name)
    return err
}
```

**Adaptation for LLM Control Plane:**
```go
// internal/policy/repository.go
type PolicyRepository struct {
    db *sql.DB
}

func (r *PolicyRepository) GetPoliciesForOrg(ctx context.Context, orgID string) ([]*Policy, error) {
    query := `
        SELECT id, org_id, policy_type, config, created_at
        FROM policies
        WHERE org_id = $1
    `
    rows, err := r.db.QueryContext(ctx, query, orgID)
    // ... parse rows
}

// internal/audit/repository.go
type AuditRepository struct {
    db *sql.DB
}

func (r *AuditRepository) LogRequest(ctx context.Context, log *RequestLog) error {
    query := `
        INSERT INTO request_logs (
            id, org_id, app_id, user_id, timestamp,
            model, provider, prompt_redacted, response_redacted,
            tokens_input, tokens_output, cost_usd, latency_ms, status
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
    `
    _, err := r.db.ExecContext(ctx, query, /* ... */)
    return err
}
```

### 4.6 Service Layer Pattern

**tsum-app pattern:**
```go
// internal/inrush/service.go
type Service struct {
    repo     *Repository
    fileService *filestorage.FileService
}

func (s *Service) CreateInrush(ctx context.Context, req *CreateRequest) (*Entity, error) {
    // Business logic
    entity := &Entity{
        ID: uuid.New(),
        Name: req.Name,
    }
    
    if err := s.repo.Create(ctx, entity); err != nil {
        return nil, err
    }
    
    return entity, nil
}
```

**Adaptation for LLM Control Plane:**
```go
// internal/gateway/service.go
type GatewayService struct {
    policyRepo    *policy.Repository
    auditRepo     *audit.Repository
    rateLimiter   *ratelimit.Service
    providers     map[string]Provider
}

func (s *GatewayService) ProcessInferenceRequest(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
    // 1. Extract tenant context
    tenant := getTenantFromContext(ctx)
    
    // 2. Check rate limits (Redis)
    if !s.rateLimiter.Allow(ctx, tenant.OrgID, tenant.AppID) {
        return nil, ErrRateLimitExceeded
    }
    
    // 3. Get policies (PostgreSQL with Redis cache)
    policies, err := s.policyRepo.GetPoliciesForOrg(ctx, tenant.OrgID)
    if err != nil {
        return nil, err
    }
    
    // 4. Evaluate policies
    if violation := evaluatePolicies(req, policies); violation != nil {
        return nil, violation
    }
    
    // 5. Route to provider
    provider := s.selectProvider(req.Model, policies)
    resp, err := provider.ChatCompletion(ctx, req)
    if err != nil {
        return nil, err
    }
    
    // 6. Async audit log (non-blocking)
    go s.auditRepo.LogRequest(context.Background(), buildAuditLog(req, resp))
    
    return resp, nil
}
```

---

## 5. Detailed Component Design

### 5.1 Lambda Function Structure

```
backend/
├── main.go                    # Lambda entrypoint
├── lambda.go                  # Event handler (unified)
├── config/
│   └── config.go              # Configuration loading
├── internal/
│   ├── auth/
│   │   ├── cognito.go         # JWT validation
│   │   └── claims.go          # Extract org/app/role from JWT
│   ├── validation/
│   │   ├── pii_detector.go    # Regex + ML-based PII detection
│   │   ├── secrets_detector.go # Detect API keys, passwords
│   │   └── injection_guard.go # Prompt injection patterns
│   ├── policy/
│   │   ├── engine.go          # Policy evaluation
│   │   ├── repository.go      # PostgreSQL access
│   │   └── cache.go           # Redis cache for policies
│   ├── ratelimit/
│   │   └── redis.go           # Sliding window rate limiter
│   ├── router/
│   │   ├── router.go          # Model → Provider mapping
│   │   └── failover.go        # Circuit breaker + fallback
│   ├── providers/
│   │   ├── interface.go       # Provider interface
│   │   ├── openai.go          # OpenAI adapter
│   │   ├── anthropic.go       # Anthropic adapter
│   │   └── azure.go           # Azure OpenAI adapter
│   ├── audit/
│   │   ├── logger.go          # Async audit logging
│   │   ├── repository.go      # PostgreSQL access
│   │   └── s3_archiver.go     # Long-term S3 storage
│   └── gateway/
│       └── service.go         # Main orchestration
├── routes/
│   ├── routes.go              # Chi router setup
│   └── handlers/
│       ├── inference.go       # /v1/chat/completions
│       ├── admin.go           # Admin CRUD APIs
│       └── health.go          # Health checks
└── migrations/
    └── schema/
        ├── 001_initial.up.sql
        ├── 001_initial.down.sql
        └── ...
```

### 5.2 Database Schema

```sql
-- backend/migrations/schema/001_initial.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Organizations
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Applications
CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    api_key_hash VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(org_id, name)
);

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    app_id UUID REFERENCES applications(id) ON DELETE SET NULL,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL, -- 'superadmin', 'admin', 'developer', 'auditor'
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(org_id, email)
);

-- Policies
CREATE TABLE policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    app_id UUID REFERENCES applications(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    policy_type VARCHAR(50) NOT NULL, -- 'rate_limit', 'cost_cap', 'model_restriction'
    config JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Request logs (hot storage: 30 days, then archive to S3)
CREATE TABLE request_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    app_id UUID REFERENCES applications(id),
    user_id UUID REFERENCES users(id),
    timestamp TIMESTAMP DEFAULT NOW(),
    model VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    prompt_redacted TEXT,
    response_redacted TEXT,
    tokens_input INT,
    tokens_output INT,
    cost_usd DECIMAL(10, 6),
    latency_ms INT,
    status VARCHAR(50), -- 'success', 'error', 'blocked'
    error_message TEXT
);

CREATE INDEX idx_request_logs_org_id ON request_logs(org_id);
CREATE INDEX idx_request_logs_timestamp ON request_logs(timestamp DESC);
CREATE INDEX idx_request_logs_status ON request_logs(status);

-- Provider credentials (encrypted by KMS)
CREATE TABLE provider_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL, -- 'openai', 'anthropic', 'azure'
    credential_encrypted TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(org_id, provider)
);
```

### 5.3 Redis Data Structures

```
# Rate limiting (sliding window)
Key: rate_limit:{org_id}:{app_id}:{window}
Type: Sorted Set (ZSET)
TTL: Window duration (e.g., 60s for per-minute)
Value: Request timestamps

Example:
ZADD rate_limit:org123:app456:minute 1707206400 req-1
ZCOUNT rate_limit:org123:app456:minute 1707206340 1707206400
ZREMRANGEBYSCORE rate_limit:org123:app456:minute -inf 1707206340

# Policy cache
Key: policy:{org_id}
Type: String (JSON)
TTL: 5 minutes
Value: Serialized policy list

Example:
GET policy:org123
SET policy:org123 '{"rate_limits":{"rpm":100},"cost_cap":5000}' EX 300

# Session cache (optional for JWT validation)
Key: session:{user_id}
Type: Hash
TTL: 1 hour
Value: User metadata

Example:
HGETALL session:user789
HSET session:user789 org_id org123 app_id app456 role developer
EXPIRE session:user789 3600
```

### 5.4 Provider Adapter Interface

```go
// internal/providers/interface.go
package providers

import (
    "context"
    "time"
)

type Provider interface {
    Name() string
    ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    IsAvailable(ctx context.Context) bool
    GetCost(req *ChatRequest, resp *ChatResponse) (float64, error)
}

type ChatRequest struct {
    Model       string
    Messages    []Message
    MaxTokens   int
    Temperature float64
    Metadata    map[string]string // User-provided metadata
}

type Message struct {
    Role    string // "system", "user", "assistant"
    Content string
}

type ChatResponse struct {
    ID      string
    Model   string
    Choices []Choice
    Usage   Usage
    Latency time.Duration
}

type Choice struct {
    Index        int
    Message      Message
    FinishReason string
}

type Usage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

**OpenAI Adapter:**
```go
// internal/providers/openai.go
package providers

import (
    "context"
    "github.com/openai/openai-go"
)

type OpenAIProvider struct {
    client *openai.Client
    apiKey string
}

func NewOpenAIProvider(apiKey string) *OpenAIProvider {
    return &OpenAIProvider{
        client: openai.NewClient(apiKey),
        apiKey: apiKey,
    }
}

func (p *OpenAIProvider) Name() string {
    return "openai"
}

func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    start := time.Now()
    
    // Convert to OpenAI request
    messages := make([]openai.ChatCompletionMessage, len(req.Messages))
    for i, msg := range req.Messages {
        messages[i] = openai.ChatCompletionMessage{
            Role:    msg.Role,
            Content: msg.Content,
        }
    }
    
    resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:       req.Model,
        Messages:    messages,
        MaxTokens:   req.MaxTokens,
        Temperature: float32(req.Temperature),
    })
    if err != nil {
        return nil, err
    }
    
    // Convert to unified response
    choices := make([]Choice, len(resp.Choices))
    for i, choice := range resp.Choices {
        choices[i] = Choice{
            Index: i,
            Message: Message{
                Role:    choice.Message.Role,
                Content: choice.Message.Content,
            },
            FinishReason: choice.FinishReason,
        }
    }
    
    return &ChatResponse{
        ID:      resp.ID,
        Model:   resp.Model,
        Choices: choices,
        Usage: Usage{
            PromptTokens:     resp.Usage.PromptTokens,
            CompletionTokens: resp.Usage.CompletionTokens,
            TotalTokens:      resp.Usage.TotalTokens,
        },
        Latency: time.Since(start),
    }, nil
}

func (p *OpenAIProvider) IsAvailable(ctx context.Context) bool {
    // Ping OpenAI API
    _, err := p.client.ListModels(ctx)
    return err == nil
}

func (p *OpenAIProvider) GetCost(req *ChatRequest, resp *ChatResponse) (float64, error) {
    // Pricing as of 2026-02-06
    pricePerMillionTokens := map[string]struct{ Input, Output float64 }{
        "gpt-4":          {Input: 30.0, Output: 60.0},
        "gpt-4-turbo":    {Input: 10.0, Output: 30.0},
        "gpt-3.5-turbo":  {Input: 0.5, Output: 1.5},
    }
    
    prices, ok := pricePerMillionTokens[resp.Model]
    if !ok {
        return 0, fmt.Errorf("unknown model: %s", resp.Model)
    }
    
    inputCost := float64(resp.Usage.PromptTokens) / 1_000_000 * prices.Input
    outputCost := float64(resp.Usage.CompletionTokens) / 1_000_000 * prices.Output
    
    return inputCost + outputCost, nil
}
```

---

## 6. Development Environment Setup

### 6.1 Prerequisites

- **Go 1.24+** (same as tsum-app)
- **Node.js 20+** (for frontend)
- **Docker Desktop** (for local PostgreSQL + Redis)
- **AWS CLI v2** (for Secrets Manager)
- **Make** (build automation)

### 6.2 Local Infrastructure

**docker-compose.yml:**
```yaml
# backend/docker-compose.yml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    container_name: llm-cp-postgres
    environment:
      POSTGRES_DB: llm_control_plane_dev
      POSTGRES_USER: dev
      POSTGRES_PASSWORD: dev
    ports:
      - "5432:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    container_name: llm-cp-redis
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes
    volumes:
      - redis-data:/data

volumes:
  postgres-data:
  redis-data:
```

**Start local services:**
```bash
cd backend
docker-compose up -d
```

### 6.3 Environment Variables

**backend/.env:**
```bash
# Environment
ENVIRONMENT=dev

# Database
DATABASE_URL=postgresql://dev:dev@localhost:5432/llm_control_plane_dev?sslmode=disable

# Redis
REDIS_URL=localhost:6379

# Cognito (for local testing, use sandbox user pool)
COGNITO_USER_POOL_ID=us-east-1_XXXXXXXXX
COGNITO_CLIENT_ID=xxxxxxxxxxxxxxxxxxxxxxxxxx
COGNITO_CLIENT_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
COGNITO_DOMAIN=llm-cp-sandbox.auth.us-east-1.amazoncognito.com
COGNITO_REDIRECT_URI=http://localhost:8080/oauth2/idpresponse

# Frontend URL
FRONT_END_URL=http://localhost:5173

# LLM Provider API Keys (for local testing)
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
AZURE_OPENAI_KEY=...
AZURE_OPENAI_ENDPOINT=https://....openai.azure.com

# Observability (optional for local)
DATADOG_API_KEY=
DATADOG_SITE=us5.datadoghq.com
```

**frontend/.env:**
```bash
VITE_BACKEND_URL=http://localhost:8080
VITE_DATADOG_APPLICATION_ID=
VITE_DATADOG_CLIENT_TOKEN=
VITE_DATADOG_SITE=us5.datadoghq.com
```

### 6.4 Makefile Setup (adapted from tsum-app)

**Makefile:**
```makefile
# Makefile
include make/config.mk
include make/backend.mk
include make/frontend.mk
include make/database.mk
include make/testing.mk

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: dev
dev: ## Run full development environment
	@make -j2 backend-dev frontend-dev

.PHONY: setup
setup: ## Initial setup (install dependencies, start Docker)
	@echo "Starting Docker services..."
	@cd backend && docker-compose up -d
	@echo "Installing backend dependencies..."
	@cd backend && go mod download
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install
	@echo "Running database migrations..."
	@make migrate-up
	@echo "Setup complete!"
```

**make/backend.mk:**
```makefile
# make/backend.mk
.PHONY: backend-dev
backend-dev: ## Run backend in development mode
	@cd backend && go run main.go

.PHONY: backend-build
backend-build: ## Build backend binary
	@cd backend && GOOS=linux GOARCH=amd64 go build -o bootstrap main.go

.PHONY: backend-test
backend-test: ## Run backend tests
	@cd backend && go test -v ./...

.PHONY: backend-lint
backend-lint: ## Run Go linter
	@cd backend && go vet ./...
	@cd backend && golangci-lint run
```

**make/database.mk:**
```makefile
# make/database.mk
.PHONY: migrate-up
migrate-up: ## Run database migrations
	@cd backend && go run cmd/migrate/main.go up

.PHONY: migrate-down
migrate-down: ## Rollback last migration
	@cd backend && go run cmd/migrate/main.go down

.PHONY: migrate-create
migrate-create: ## Create new migration (usage: make migrate-create NAME=add_users_table)
	@cd backend && go run cmd/migrate/main.go create $(NAME)

.PHONY: reset-db
reset-db: ## Drop all tables and re-run migrations
	@echo "Dropping all tables..."
	@docker exec -it llm-cp-postgres psql -U dev -d llm_control_plane_dev -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	@make migrate-up
	@echo "Running seeders..."
	@cd backend && go run cmd/seed/main.go
```

### 6.5 Running Locally

```bash
# 1. Start infrastructure
make setup

# 2. Run development servers (backend + frontend)
make dev

# Backend runs on http://localhost:8080
# Frontend runs on http://localhost:5173

# 3. Test API
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

---

## 7. Production Environment Setup

### 7.1 Infrastructure Configuration

**Directory structure** (adapted from tsum-app):
```
infra/
├── sandbox-llm-cp/          # Sandbox environment
│   ├── cors.json
│   └── assets/
│       ├── logo.png
│       ├── favicon.ico
│       └── background.png
├── demo-llm-cp/             # Demo/RC environment
│   ├── cors.json
│   └── assets/
└── prod-llm-cp/             # Production environment
    ├── cors.json
    └── assets/
```

**infra/sandbox-llm-cp/cors.json:**
```json
{
  "CORSRules": [
    {
      "AllowedOrigins": ["https://sandbox.llm-cp.yourdomain.com"],
      "AllowedMethods": ["GET", "POST", "PUT", "DELETE"],
      "AllowedHeaders": ["*"],
      "ExposeHeaders": ["ETag"],
      "MaxAgeSeconds": 3600
    }
  ]
}
```

### 7.2 AWS Account Setup

**Required AWS resources:**
1. **IAM Role for GitHub Actions** (OIDC federation)
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Principal": {
           "Federated": "arn:aws:iam::<account-id>:oidc-provider/token.actions.githubusercontent.com"
         },
         "Action": "sts:AssumeRoleWithWebIdentity",
         "Condition": {
           "StringLike": {
             "token.actions.githubusercontent.com:sub": "repo:<org>/<repo>:*"
           }
         }
       }
     ]
   }
   ```

2. **Permissions for deployment role:**
   - Lambda full access
   - API Gateway full access
   - Cognito full access
   - Aurora RDS full access
   - ElastiCache full access
   - S3 full access
   - CloudFront full access
   - Secrets Manager full access
   - CloudWatch Logs full access
   - IAM pass role

3. **Secrets in Secrets Manager:**
   - `/llm-cp/prod/aurora` - Database credentials
   - `/llm-cp/prod/providers` - LLM provider API keys
     ```json
     {
       "openai_api_key": "sk-...",
       "anthropic_api_key": "sk-ant-...",
       "azure_openai_key": "...",
       "azure_openai_endpoint": "https://....openai.azure.com"
     }
     ```
   - `/llm-cp/prod/datadog` - Datadog API key

### 7.3 Aurora Serverless Configuration

**Recommended settings:**

| Environment | Min ACU | Max ACU | Publicly Accessible | Multi-AZ | Backup Retention |
|-------------|---------|---------|---------------------|----------|------------------|
| **Sandbox** | 0.5 | 2 | Yes (for debugging) | No | 7 days |
| **Demo** | 0.5 | 4 | No | Yes | 7 days |
| **Production** | 1 | 8 | No | Yes | 30 days |

**Security:**
- VPC-private subnets only (production)
- Security group: allow Lambda SG + bastion host
- Encryption at rest: enabled (KMS)
- IAM authentication: optional (use for admin access)

### 7.4 ElastiCache Redis Configuration

**Recommended settings:**

| Environment | Node Type | Nodes | Cluster Mode | Backup | Encryption |
|-------------|-----------|-------|--------------|--------|------------|
| **Sandbox** | cache.t3.micro | 1 | Disabled | No | No |
| **Demo** | cache.t3.medium | 2 (primary + replica) | Disabled | Daily | In-transit + at-rest |
| **Production** | cache.r7g.large | 2 (primary + replica) | Disabled | Daily | In-transit + at-rest |

**Configuration:**
- Parameter group: `default.redis7`
- Max memory policy: `allkeys-lru` (evict least recently used)
- Persistence: AOF (append-only file) for durability

### 7.5 Lambda Configuration

**Settings:**

| Environment | Memory | Timeout | Concurrent Executions | Reserved Concurrency |
|-------------|--------|---------|----------------------|----------------------|
| **Sandbox** | 512 MB | 30s | 10 | 10 |
| **Demo** | 1024 MB | 30s | 50 | 50 |
| **Production** | 1024 MB | 30s | 1000 | 200 (reserved) |

**VPC Configuration:**
- Private subnets (2+ for high availability)
- Security group: outbound to Aurora, Redis, internet (for LLM providers)
- NAT Gateway for internet access

**Environment variables:**
```bash
ENVIRONMENT=prod
AURORA_SECRET_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:/llm-cp/prod/aurora
PROVIDER_SECRETS_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:/llm-cp/prod/providers
REDIS_URL=llm-cp-prod.cache.amazonaws.com:6379
S3_BUCKET_NAME=llm-cp-prod-file-storage
COGNITO_USER_POOL_ID=us-east-1_XXXXXXXXX
COGNITO_CLIENT_ID=xxxxxxxxxxxxxxxxxxxxxxxxxx
COGNITO_CLIENT_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
COGNITO_DOMAIN=llm-cp.auth.us-east-1.amazoncognito.com
COGNITO_REDIRECT_URI=https://api.llm-cp.yourdomain.com/oauth2/idpresponse
FRONT_END_URL=https://llm-cp.yourdomain.com
DD_SITE=us5.datadoghq.com
DD_SERVICE=llm-cp
DD_ENV=prod
DD_VERSION=$GITHUB_SHA
```

### 7.6 API Gateway Configuration

**Settings:**
- Type: REST API (not HTTP API, for WAF support)
- Integration: Lambda proxy
- Custom domain: `api.llm-cp.yourdomain.com`
- TLS: Minimum TLS 1.2
- CloudWatch logging: Full request/response (sandbox), Errors only (prod)
- Throttling: 1000 requests/sec (burst), 10000 requests/sec (steady-state)

**WAF Rules:**
- AWS Managed Rules: Core rule set (OWASP Top 10)
- Rate limiting: 100 requests per 5 minutes per IP
- Geo-blocking: Block countries if needed
- Custom rules: Block known malicious IPs

---

## 8. GitHub Actions CI/CD Pipeline

### 8.1 Reusable Workflow (adapted from tsum-app)

**.github/workflows/infra.yml:**
```yaml
name: Deploy Infrastructure

on:
  workflow_call:
    inputs:
      instance_name:
        required: true
        type: string
      web_domain:
        required: true
        type: string
      api_domain:
        required: true
        type: string
      environment:
        required: true
        type: string  # 'sandbox', 'demo', 'prod'
      publicly_accessible:
        required: false
        type: string
        default: 'false'
      reset_database:
        required: false
        type: string
        default: 'false'
      aurora_min_capacity:
        required: false
        type: string
        default: '0.5'
      aurora_max_capacity:
        required: false
        type: string
        default: '2'
    secrets:
      aws_role_arn:
        required: true
      datadog_api_key:
        required: true
      datadog_application_id:
        required: true
      datadog_client_token:
        required: true

jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          submodules: recursive

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.aws_role_arn }}
          aws-region: us-east-1

      - name: Setup Terraform backend
        id: backend
        uses: realsensesolutions/actions-aws-backend-setup@main
        with:
          project_name: llm-cp-${{ inputs.instance_name }}

      - name: Deploy network infrastructure
        id: network
        uses: realsensesolutions/actions-aws-network@main
        with:
          project_name: llm-cp-${{ inputs.instance_name }}

      - name: Deploy S3 bucket
        id: bucket
        uses: realsensesolutions/actions-aws-bucket@main
        with:
          bucket_name: llm-cp-${{ inputs.instance_name }}-files
          cors_file: infra/${{ inputs.instance_name }}/cors.json

      - name: Deploy Cognito
        id: auth
        uses: realsensesolutions/actions-aws-auth@main
        with:
          user_pool_name: llm-cp-${{ inputs.instance_name }}
          cognito_domain: llm-cp-${{ inputs.instance_name }}
          callback_urls: https://${{ inputs.api_domain }}/oauth2/idpresponse
          logout_urls: https://${{ inputs.web_domain }}
          logo_path: infra/${{ inputs.instance_name }}/assets/logo.png
          background_path: infra/${{ inputs.instance_name }}/assets/background.png
          favicon_path: infra/${{ inputs.instance_name }}/assets/favicon.ico

      - name: Deploy Aurora PostgreSQL
        id: aurora
        uses: realsensesolutions/actions-aws-postgres-aurora@main
        with:
          cluster_identifier: llm-cp-${{ inputs.instance_name }}
          database_name: llm_cp_${{ inputs.instance_name }}
          vpc_id: ${{ steps.network.outputs.vpc_id }}
          subnet_ids: ${{ steps.network.outputs.private_subnet_ids }}
          publicly_accessible: ${{ inputs.publicly_accessible }}
          min_capacity: ${{ inputs.aurora_min_capacity }}
          max_capacity: ${{ inputs.aurora_max_capacity }}
          deletion_protection: ${{ inputs.environment == 'prod' && 'true' || 'false' }}

      - name: Deploy ElastiCache Redis
        id: redis
        uses: realsensesolutions/actions-aws-elasticache@main
        with:
          cluster_id: llm-cp-${{ inputs.instance_name }}
          node_type: ${{ inputs.environment == 'prod' && 'cache.r7g.large' || 'cache.t3.medium' }}
          num_cache_nodes: ${{ inputs.environment == 'prod' && '2' || '1' }}
          vpc_id: ${{ steps.network.outputs.vpc_id }}
          subnet_ids: ${{ steps.network.outputs.private_subnet_ids }}

      - name: Build and deploy Lambda
        id: lambda
        uses: realsensesolutions/actions-aws-function-go@main
        with:
          function_name: llm-cp-${{ inputs.instance_name }}
          entry_point: backend/main.go
          memory_size: ${{ inputs.environment == 'prod' && '1024' || '512' }}
          timeout: 30
          vpc_id: ${{ steps.network.outputs.vpc_id }}
          subnet_ids: ${{ steps.network.outputs.private_subnet_ids }}
          environment_variables: |
            S3_BUCKET_NAME=${{ steps.bucket.outputs.name }}
            AURORA_SECRET_ARN=${{ steps.aurora.outputs.secret_arn }}
            REDIS_URL=${{ steps.redis.outputs.endpoint }}
            COGNITO_USER_POOL_ID=${{ steps.auth.outputs.user_pool_id }}
            COGNITO_CLIENT_ID=${{ steps.auth.outputs.client_id }}
            COGNITO_CLIENT_SECRET=${{ steps.auth.outputs.client_secret }}
            COGNITO_DOMAIN=${{ steps.auth.outputs.cognito_domain }}
            COGNITO_REDIRECT_URI=https://${{ inputs.api_domain }}/oauth2/idpresponse
            FRONT_END_URL=https://${{ inputs.web_domain }}
            ENVIRONMENT=${{ inputs.environment }}
            DD_SITE=us5.datadoghq.com
            DD_SERVICE=llm-cp
            DD_ENV=${{ inputs.environment }}
            DD_VERSION=${{ github.sha }}
          datadog_api_key: ${{ secrets.datadog_api_key }}

      - name: Reset database (sandbox only)
        if: inputs.reset_database == 'true' && inputs.environment == 'sandbox'
        run: |
          aws lambda invoke \
            --function-name ${{ steps.lambda.outputs.function_name }} \
            --payload '{"type": "database-reset"}' \
            /tmp/reset-response.json
          cat /tmp/reset-response.json

      - name: Run database migrations
        run: |
          aws lambda invoke \
            --function-name ${{ steps.lambda.outputs.function_name }} \
            --payload '{"type": "user-migration"}' \
            /tmp/migration-response.json
          cat /tmp/migration-response.json
          if grep -q "error" /tmp/migration-response.json; then
            echo "Migration failed"
            exit 1
          fi

      - name: Run database seeders
        run: |
          aws lambda invoke \
            --function-name ${{ steps.lambda.outputs.function_name }} \
            --payload '{"type": "seed-data", "environment": "${{ inputs.environment }}"}' \
            /tmp/seed-response.json
          cat /tmp/seed-response.json

      - name: Deploy API Gateway
        id: api-gateway
        uses: realsensesolutions/actions-aws-api-gateway@main
        with:
          api_name: llm-cp-${{ inputs.instance_name }}
          function_arn: ${{ steps.lambda.outputs.function_arn }}
          domain_name: ${{ inputs.api_domain }}

      - name: Build frontend
        working-directory: frontend
        env:
          VITE_BACKEND_URL: https://${{ inputs.api_domain }}
          VITE_DATADOG_APPLICATION_ID: ${{ secrets.datadog_application_id }}
          VITE_DATADOG_CLIENT_TOKEN: ${{ secrets.datadog_client_token }}
          VITE_DATADOG_SITE: us5.datadoghq.com
          VITE_DATADOG_SERVICE: llm-cp
          VITE_DATADOG_ENV: ${{ inputs.environment }}
          VITE_DATADOG_VERSION: ${{ github.sha }}
        run: |
          npm install
          cp -r ../infra/${{ inputs.instance_name }}/assets/* public/assets/images/
          npm run build

      - name: Deploy frontend
        id: website
        uses: realsensesolutions/actions-aws-website@main
        with:
          bucket_name: llm-cp-${{ inputs.instance_name }}-website
          source_dir: frontend/dist
          domain_name: ${{ inputs.web_domain }}
```

### 8.2 Environment-Specific Triggers

**.github/workflows/on-push.yml** (Sandbox):
```yaml
name: Deploy Sandbox
on:
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      reset_database:
        type: boolean
        default: false

jobs:
  deploy:
    uses: ./.github/workflows/infra.yml
    with:
      instance_name: sandbox-llm-cp
      web_domain: sandbox.llm-cp.yourdomain.com
      api_domain: api.sandbox.llm-cp.yourdomain.com
      environment: sandbox
      publicly_accessible: 'true'
      reset_database: ${{ inputs.reset_database == true && 'true' || 'false' }}
      aurora_min_capacity: '0.5'
      aurora_max_capacity: '2'
    secrets:
      aws_role_arn: ${{ secrets.AWS_ROLE_ARN }}
      datadog_api_key: ${{ secrets.DATADOG_API_KEY }}
      datadog_application_id: ${{ secrets.DATADOG_APPLICATION_ID }}
      datadog_client_token: ${{ secrets.DATADOG_CLIENT_TOKEN }}
```

**.github/workflows/on-tags.yml** (Production):
```yaml
name: Deploy Production
on:
  push:
    tags:
      - 'v*'
      - '!v*rc*'

jobs:
  deploy:
    uses: ./.github/workflows/infra.yml
    with:
      instance_name: prod-llm-cp
      web_domain: llm-cp.yourdomain.com
      api_domain: api.llm-cp.yourdomain.com
      environment: prod
      publicly_accessible: 'false'
      reset_database: 'false'
      aurora_min_capacity: '1'
      aurora_max_capacity: '8'
    secrets:
      aws_role_arn: ${{ secrets.AWS_ROLE_ARN_PROD }}
      datadog_api_key: ${{ secrets.DATADOG_API_KEY }}
      datadog_application_id: ${{ secrets.DATADOG_APPLICATION_ID }}
      datadog_client_token: ${{ secrets.DATADOG_CLIENT_TOKEN }}
```

**.github/workflows/on-tags-rc.yml** (Demo/RC):
```yaml
name: Deploy Demo
on:
  push:
    tags:
      - 'v*rc*'

jobs:
  deploy:
    uses: ./.github/workflows/infra.yml
    with:
      instance_name: demo-llm-cp
      web_domain: demo.llm-cp.yourdomain.com
      api_domain: api.demo.llm-cp.yourdomain.com
      environment: prod  # Use prod seeders
      publicly_accessible: 'false'
      reset_database: 'false'
      aurora_min_capacity: '0.5'
      aurora_max_capacity: '4'
    secrets:
      aws_role_arn: ${{ secrets.AWS_ROLE_ARN }}
      datadog_api_key: ${{ secrets.DATADOG_API_KEY }}
      datadog_application_id: ${{ secrets.DATADOG_APPLICATION_ID }}
      datadog_client_token: ${{ secrets.DATADOG_CLIENT_TOKEN }}
```

### 8.3 GitHub Secrets Configuration

**Required secrets:**
```
AWS_ROLE_ARN                  # Non-prod AWS account (sandbox, demo)
AWS_ROLE_ARN_PROD             # Production AWS account
DATADOG_API_KEY               # Datadog APM API key
DATADOG_APPLICATION_ID        # Datadog RUM Application ID
DATADOG_CLIENT_TOKEN          # Datadog RUM Client Token
```

---

## 9. Multi-Tenancy Strategy

### 9.1 Phase 1: Single-Tenant (MVP)

**Approach:**
- Single Aurora database for all organizations
- Row-level isolation via `org_id` foreign keys
- Shared Lambda function
- Shared Redis cache

**Advantages:**
- Simple architecture
- Lower operational complexity
- Cost-effective for MVP

**Schema:**
```sql
-- All tables include org_id for isolation
SELECT * FROM policies WHERE org_id = $1;
SELECT * FROM request_logs WHERE org_id = $1 AND timestamp > $2;
```

**Middleware:**
```go
// Extract org_id from JWT claims
func ExtractTenantMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := getClaimsFromContext(r.Context())
        orgID := claims["custom:orgId"].(string)
        
        ctx := context.WithValue(r.Context(), "org_id", orgID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 9.2 Phase 2: Database-Per-Tenant (Future)

**Approach** (similar to tsum-app's multi-tenant mode):
- Separate Aurora database per organization
- Tenant database name: `llm_cp_{org_id}`
- Connection pooling per tenant
- Metadata stored in central database

**Schema:**
```sql
-- Central metadata database
CREATE TABLE service_providers (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE
);

CREATE TABLE tenants (
    id UUID PRIMARY KEY,
    provider_id UUID REFERENCES service_providers(id),
    name VARCHAR(255) NOT NULL,
    database_name VARCHAR(255) GENERATED ALWAYS AS (
        'llm_cp_' || provider_id || '_' || id
    ) STORED,
    created_at TIMESTAMP DEFAULT NOW()
);
```

**Tenant connection manager:**
```go
// internal/tenant/manager.go
type Manager struct {
    connections map[string]*sql.DB
    mu          sync.RWMutex
}

func (m *Manager) GetConnection(ctx context.Context, orgID string) (*sql.DB, error) {
    m.mu.RLock()
    db, exists := m.connections[orgID]
    m.mu.RUnlock()
    
    if exists {
        return db, nil
    }
    
    // Create new connection
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Double-check (another goroutine may have created it)
    if db, exists := m.connections[orgID]; exists {
        return db, nil
    }
    
    // Fetch tenant metadata from central DB
    tenant, err := m.getTenantMetadata(orgID)
    if err != nil {
        return nil, err
    }
    
    // Create connection to tenant database
    db, err = sql.Open("postgres", fmt.Sprintf(
        "host=%s port=5432 user=%s password=%s dbname=%s sslmode=require",
        m.auroraHost, m.username, m.password, tenant.DatabaseName,
    ))
    if err != nil {
        return nil, err
    }
    
    m.connections[orgID] = db
    return db, nil
}
```

**Benefits:**
- Strong data isolation (compliance requirements)
- Independent backup/restore per tenant
- Easier to move large tenants to dedicated Aurora cluster

**Challenges:**
- Connection pool management
- Schema migration complexity (run per tenant)
- Increased operational complexity

---

## 10. Security Implementation

### 10.1 Authentication Flow (Cognito)

**OAuth2/OIDC flow:**
```
1. User clicks "Login" → Frontend redirects to:
   https://llm-cp.auth.us-east-1.amazoncognito.com/login?
     client_id=XXXX&
     response_type=code&
     redirect_uri=https://api.llm-cp.yourdomain.com/oauth2/idpresponse&
     scope=openid+email+profile

2. Cognito hosted UI → User enters credentials

3. Cognito redirects to callback URL with authorization code:
   https://api.llm-cp.yourdomain.com/oauth2/idpresponse?code=YYYY

4. Backend exchanges code for tokens:
   POST https://llm-cp.auth.us-east-1.amazoncognito.com/oauth2/token
   Body: {
     grant_type: "authorization_code",
     code: "YYYY",
     client_id: "XXXX",
     client_secret: "ZZZZ",
     redirect_uri: "https://api.llm-cp.yourdomain.com/oauth2/idpresponse"
   }

5. Cognito returns JWT tokens:
   {
     "id_token": "eyJhbGc...",      // User identity
     "access_token": "eyJhbGc...",  // API access
     "refresh_token": "eyJhbGc...", // Token refresh
     "expires_in": 3600
   }

6. Backend sets HTTP-only cookie:
   Set-Cookie: jwt=<id_token>; HttpOnly; Secure; SameSite=Strict; Path=/

7. Backend redirects to frontend:
   Location: https://llm-cp.yourdomain.com/dashboard
```

**JWT validation middleware:**
```go
// internal/auth/cognito.go
func ValidateJWT(tokenString string, userPoolID string) (*Claims, error) {
    // Fetch Cognito public keys (JWKS)
    jwksURL := fmt.Sprintf(
        "https://cognito-idp.us-east-1.amazonaws.com/%s/.well-known/jwks.json",
        userPoolID,
    )
    keySet, err := jwk.Fetch(context.Background(), jwksURL)
    if err != nil {
        return nil, err
    }
    
    // Parse and validate JWT
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        keyID, ok := token.Header["kid"].(string)
        if !ok {
            return nil, fmt.Errorf("missing kid in JWT header")
        }
        
        key, found := keySet.LookupKeyID(keyID)
        if !found {
            return nil, fmt.Errorf("key not found")
        }
        
        var rawKey interface{}
        if err := key.Raw(&rawKey); err != nil {
            return nil, err
        }
        
        return rawKey, nil
    })
    if err != nil {
        return nil, err
    }
    
    // Extract claims
    claims := &Claims{}
    if err := token.Claims(claims); err != nil {
        return nil, err
    }
    
    return claims, nil
}

type Claims struct {
    Sub      string `json:"sub"`
    Email    string `json:"email"`
    OrgID    string `json:"custom:orgId"`
    AppID    string `json:"custom:appId"`
    Role     string `json:"custom:userRole"`
    Exp      int64  `json:"exp"`
}
```

### 10.2 PII Detection

**Strategy:**
- **Regex patterns** (fast, low latency)
- **ML-based detection** (future, higher accuracy)

**Regex patterns:**
```go
// internal/validation/pii_detector.go
var piiPatterns = map[string]*regexp.Regexp{
    "email":       regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
    "phone":       regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`),
    "ssn":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
    "credit_card": regexp.MustCompile(`\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`),
    "ip_address":  regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
}

func DetectPII(text string) []PIIViolation {
    violations := []PIIViolation{}
    
    for piiType, pattern := range piiPatterns {
        matches := pattern.FindAllString(text, -1)
        for _, match := range matches {
            violations = append(violations, PIIViolation{
                Type:  piiType,
                Value: match,
            })
        }
    }
    
    return violations
}

func RedactPII(text string) string {
    for _, pattern := range piiPatterns {
        text = pattern.ReplaceAllString(text, "[REDACTED]")
    }
    return text
}
```

**Middleware:**
```go
func PromptValidationMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Parse request body
        var req InferenceRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request", http.StatusBadRequest)
            return
        }
        
        // Check for PII in prompt
        for _, msg := range req.Messages {
            violations := DetectPII(msg.Content)
            if len(violations) > 0 {
                w.WriteHeader(http.StatusBadRequest)
                json.NewEncoder(w).Encode(map[string]interface{}{
                    "error": map[string]interface{}{
                        "code":    "PII_DETECTED",
                        "message": "Prompt contains personally identifiable information",
                        "details": violations,
                    },
                })
                return
            }
        }
        
        // Store validated request in context
        ctx := context.WithValue(r.Context(), "validated_request", req)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 10.3 Rate Limiting (Redis)

**Sliding window algorithm:**
```go
// internal/ratelimit/redis.go
type Service struct {
    redis *redis.Client
}

func (s *Service) Allow(ctx context.Context, orgID, appID string, limit int, window time.Duration) (bool, error) {
    key := fmt.Sprintf("rate_limit:%s:%s:%s", orgID, appID, window.String())
    now := time.Now().Unix()
    windowStart := now - int64(window.Seconds())
    
    pipe := s.redis.Pipeline()
    
    // Remove old entries
    pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
    
    // Add current request
    pipe.ZAdd(ctx, key, &redis.Z{
        Score:  float64(now),
        Member: fmt.Sprintf("%d-%s", now, uuid.New()),
    })
    
    // Count requests in window
    pipe.ZCount(ctx, key, fmt.Sprintf("%d", windowStart), "+inf")
    
    // Set TTL
    pipe.Expire(ctx, key, window)
    
    _, err := pipe.Exec(ctx)
    if err != nil {
        return false, err
    }
    
    // Get count from pipeline result
    count, err := pipe.ZCount(ctx, key, fmt.Sprintf("%d", windowStart), "+inf").Result()
    if err != nil {
        return false, err
    }
    
    return count <= int64(limit), nil
}
```

**Middleware:**
```go
func PolicyEnforcementMiddleware(rateLimiter *ratelimit.Service) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            orgID := r.Context().Value("org_id").(string)
            appID := r.Context().Value("app_id").(string)
            
            // Check rate limit (100 requests per minute)
            allowed, err := rateLimiter.Allow(r.Context(), orgID, appID, 100, time.Minute)
            if err != nil {
                http.Error(w, "Internal server error", http.StatusInternalServerError)
                return
            }
            
            if !allowed {
                w.Header().Set("X-RateLimit-Limit", "100")
                w.Header().Set("X-RateLimit-Remaining", "0")
                w.Header().Set("Retry-After", "60")
                w.WriteHeader(http.StatusTooManyRequests)
                json.NewEncoder(w).Encode(map[string]interface{}{
                    "error": map[string]interface{}{
                        "code":    "RATE_LIMIT_EXCEEDED",
                        "message": "You have exceeded the rate limit of 100 requests per minute",
                    },
                })
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 11. Observability & Monitoring

### 11.1 Datadog APM Integration

**Lambda Layer** (added in GitHub Actions):
```yaml
- name: Build and deploy Lambda
  uses: realsensesolutions/actions-aws-function-go@main
  with:
    datadog_api_key: ${{ secrets.DATADOG_API_KEY }}
    # This automatically adds Datadog Lambda extension layer
```

**Environment variables:**
```bash
DD_SITE=us5.datadoghq.com
DD_SERVICE=llm-cp
DD_ENV=prod
DD_VERSION=$GITHUB_SHA
DD_TRACE_ENABLED=true
DD_LOGS_ENABLED=true
```

**Custom metrics:**
```go
// internal/observability/metrics.go
import "github.com/DataDog/datadog-go/v5/statsd"

type Metrics struct {
    client *statsd.Client
}

func (m *Metrics) RecordInferenceRequest(model, provider string, tokens int, cost float64, latency time.Duration, status string) {
    tags := []string{
        fmt.Sprintf("model:%s", model),
        fmt.Sprintf("provider:%s", provider),
        fmt.Sprintf("status:%s", status),
    }
    
    m.client.Count("llm_cp.requests", 1, tags, 1)
    m.client.Count("llm_cp.tokens", int64(tokens), tags, 1)
    m.client.Gauge("llm_cp.cost_usd", cost, tags, 1)
    m.client.Timing("llm_cp.latency", latency, tags, 1)
}
```

### 11.2 CloudWatch Dashboards

**Key metrics:**
- Lambda invocations, errors, duration, throttles
- API Gateway 4xx, 5xx, latency
- Aurora connections, CPU, memory
- Redis CPU, memory, evictions

**Alarms:**
```yaml
# CloudFormation/Terraform
LambdaErrorRateAlarm:
  Type: AWS::CloudWatch::Alarm
  Properties:
    AlarmName: llm-cp-lambda-error-rate
    MetricName: Errors
    Namespace: AWS/Lambda
    Statistic: Sum
    Period: 300
    EvaluationPeriods: 1
    Threshold: 10
    ComparisonOperator: GreaterThanThreshold
    AlarmActions:
      - !Ref SNSTopic

AuroraHighConnectionsAlarm:
  Type: AWS::CloudWatch::Alarm
  Properties:
    AlarmName: llm-cp-aurora-high-connections
    MetricName: DatabaseConnections
    Namespace: AWS/RDS
    Statistic: Average
    Period: 300
    EvaluationPeriods: 2
    Threshold: 80
    ComparisonOperator: GreaterThanThreshold
```

### 11.3 Structured Logging

**Log format:**
```go
// internal/observability/logger.go
import "go.uber.org/zap"

type Logger struct {
    *zap.Logger
}

func (l *Logger) LogInferenceRequest(ctx context.Context, req *InferenceRequest, resp *InferenceResponse, err error) {
    fields := []zap.Field{
        zap.String("request_id", getRequestID(ctx)),
        zap.String("org_id", getOrgID(ctx)),
        zap.String("app_id", getAppID(ctx)),
        zap.String("model", req.Model),
        zap.String("provider", resp.Provider),
        zap.Int("tokens_input", resp.Usage.PromptTokens),
        zap.Int("tokens_output", resp.Usage.CompletionTokens),
        zap.Float64("cost_usd", resp.Cost),
        zap.Duration("latency", resp.Latency),
    }
    
    if err != nil {
        fields = append(fields, zap.Error(err))
        l.Error("Inference request failed", fields...)
    } else {
        l.Info("Inference request succeeded", fields...)
    }
}
```

**Example log output:**
```json
{
  "level": "info",
  "timestamp": "2026-02-06T12:34:56.789Z",
  "message": "Inference request succeeded",
  "request_id": "req-xyz789",
  "org_id": "org-123",
  "app_id": "app-456",
  "model": "gpt-4",
  "provider": "openai",
  "tokens_input": 15,
  "tokens_output": 8,
  "cost_usd": 0.00046,
  "latency_ms": 342
}
```

---

## 12. Cost Optimization

### 12.1 Cost Breakdown (Monthly Estimates)

**Sandbox Environment:**
| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| Lambda | 10,000 requests/month, 512MB, 1s avg | $0.20 |
| API Gateway | 10,000 requests/month | $0.10 |
| Aurora Serverless v2 | 0.5-2 ACU, 10 hours/day | $15 |
| ElastiCache Redis | cache.t3.micro | $12 |
| S3 | 10 GB storage, 1000 requests | $0.25 |
| CloudFront | 10 GB transfer | $1.00 |
| Secrets Manager | 3 secrets | $1.20 |
| CloudWatch | 10 GB logs | $5.00 |
| **Total** | | **~$35/month** |

**Production Environment (1000 req/sec):**
| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| Lambda | 2.6B requests/month, 1024MB, 1s avg | $5,200 |
| API Gateway | 2.6B requests/month | $9,100 |
| Aurora Serverless v2 | 1-8 ACU, 24/7 | $200 |
| ElastiCache Redis | cache.r7g.large, 2 nodes | $300 |
| S3 | 1 TB storage, 10M requests | $25 |
| CloudFront | 10 TB transfer | $850 |
| Secrets Manager | 5 secrets | $2 |
| CloudWatch | 100 GB logs | $50 |
| WAF | 2.6B requests | $5,200 |
| **Total** | | **~$20,927/month** |

**Note:** LLM provider costs (OpenAI, Anthropic) are passed through to end users, not included above.

### 12.2 Optimization Strategies

1. **Lambda optimization:**
   - Use arm64 (Graviton2) for 20% cost savings
   - Optimize memory allocation (more memory = faster CPU)
   - Use provisioned concurrency sparingly (only for critical endpoints)

2. **Aurora optimization:**
   - Use Serverless v2 (auto-scales to zero)
   - Set appropriate min/max ACU based on load
   - Use Data API for occasional queries (no connection pooling)

3. **API Gateway optimization:**
   - Use HTTP API instead of REST API (cheaper, but no WAF support)
   - Cache responses where possible (cache policy)

4. **S3 optimization:**
   - Use S3 Intelligent-Tiering for audit logs
   - Lifecycle policy: transition to Glacier after 90 days
   - Delete old logs after 7 years (compliance requirement)

5. **CloudFront optimization:**
   - Use cache headers aggressively
   - Compress assets (gzip, brotli)
   - Use S3 Transfer Acceleration for uploads

---

## 13. Migration Path

### 13.1 Phase 1: MVP (Weeks 1-10)

**Objective:** Core inference pipeline

**Deliverables:**
- ✅ Lambda function with Go Chi router
- ✅ Cognito authentication
- ✅ Basic prompt validation (PII regex)
- ✅ Single LLM provider (OpenAI)
- ✅ PostgreSQL audit logs
- ✅ GitHub Actions deployment (sandbox)
- ✅ Minimal frontend (React + TailwindCSS)

**Testing:**
- End-to-end inference request
- PII detection blocks requests
- Audit logs persisted

### 13.2 Phase 2: Governance (Weeks 11-15)

**Objective:** Policy engine and rate limiting

**Deliverables:**
- ✅ Redis rate limiting
- ✅ Policy engine (rate limits, cost caps, model restrictions)
- ✅ Multiple LLM providers (OpenAI, Anthropic, Azure)
- ✅ Model routing logic
- ✅ Admin APIs (CRUD for policies)
- ✅ Datadog APM integration

**Testing:**
- Rate limiting works
- Cost cap enforcement
- Provider failover

### 13.3 Phase 3: Observability (Weeks 16-20)

**Objective:** Production-ready monitoring

**Deliverables:**
- ✅ CloudWatch dashboards
- ✅ Datadog RUM for frontend
- ✅ S3 archival for audit logs
- ✅ Alerting (PagerDuty/SNS)
- ✅ Load testing (1000 req/sec)
- ✅ Production deployment

**Testing:**
- Load test passes
- Alerts fire correctly
- Audit logs archived to S3

### 13.4 Phase 4: Advanced Features (Post-MVP)

**Objective:** Multi-tenancy, RAG, advanced routing

**Deliverables:**
- ⬜ Database-per-tenant multi-tenancy
- ⬜ RAG integration (vector database)
- ⬜ Cost optimization routing
- ⬜ Prompt template library
- ⬜ A/B testing framework

---

## Appendices

### A. Comparison: Kubernetes vs. Serverless

| Aspect | Kubernetes (PRD) | Serverless (This Doc) |
|--------|------------------|------------------------|
| **Ops complexity** | High | Low |
| **Scaling** | Manual HPA tuning | Automatic |
| **Cost** | Fixed (EKS ~$75/month + nodes) | Pay-per-use |
| **Cold starts** | None | ~100-300ms |
| **Suitability** | Large enterprises, high traffic | MVP, variable traffic |

**Recommendation:** Start with serverless for MVP, migrate to Kubernetes if sustained > 1000 req/sec.

### B. Useful Links

- **tsum-app repository:** (internal reference)
- **AWS Lambda Go:** https://github.com/aws/aws-lambda-go
- **Chi router:** https://github.com/go-chi/chi
- **Datadog Go:** https://github.com/DataDog/datadog-go
- **OpenAI Go SDK:** https://github.com/openai/openai-go
- **Anthropic Go SDK:** https://github.com/anthropics/anthropic-go

### C. Contact

**Project Owner:** [Your name]  
**AWS Account:** [Account ID]  
**GitHub Repository:** https://github.com/[org]/llm-control-plane

---

**End of Architecture Document**

**Version:** 1.0  
**Last Updated:** February 6, 2026  
**Next Review:** Phase 1 completion (Week 10)
