# LLM Control Plane - Implementation Summary

**Version:** 1.0  
**Date:** February 6, 2026

---

## Executive Summary

This document summarizes the analysis of the **tsum-app** architecture and provides recommendations for implementing the **LLM Control Plane** project using proven AWS serverless patterns.

---

## Key Findings from tsum-app Analysis

### Architecture Strengths

1. **Serverless-First Design**
   - Single Lambda function handles all HTTP requests
   - Aurora Serverless v2 auto-scales based on demand
   - ElastiCache Redis for rate limiting and caching
   - **Cost:** ~$35/month for sandbox, scales to production needs

2. **Event-Driven Migrations**
   - Database migrations run via Lambda invocation
   - Eliminates need for manual database access
   - Integrated into CI/CD pipeline
   - **Pattern:** `{"type": "user-migration"}` event

3. **GitHub Actions-Based Infrastructure**
   - Reusable workflows for infrastructure provisioning
   - OIDC-based AWS authentication (no long-lived credentials)
   - Environment-specific deployments (sandbox, demo, prod)
   - **Deployment time:** ~15-20 minutes per environment

4. **Multi-Tenant Architecture**
   - Database-per-tenant strategy for strong isolation
   - Connection pooling per tenant
   - Metadata stored in central database
   - **Use case:** Compliance-heavy industries

5. **Observability Integration**
   - Datadog APM for Lambda tracing
   - CloudWatch for logs and metrics
   - Structured JSON logging
   - **Retention:** 30 days in CloudWatch, 7 years in S3

---

## Architectural Comparison

| Aspect | PRD (Kubernetes) | Recommended (Serverless) | Rationale |
|--------|------------------|--------------------------|-----------|
| **Compute** | EKS + EC2 nodes | Lambda (Go) | Lower ops complexity, auto-scaling |
| **Database** | RDS Multi-AZ | Aurora Serverless v2 | Cost-effective, scales to zero |
| **Caching** | ElastiCache | ElastiCache (same) | Proven pattern for rate limiting |
| **Load Balancer** | ALB | API Gateway | Native Lambda integration |
| **Deployment** | Helm + ArgoCD | GitHub Actions | Simpler CI/CD, less tooling |
| **Scaling** | HPA + Cluster Autoscaler | Automatic | No manual configuration |
| **Cost (Sandbox)** | ~$100-150/month | ~$35/month | 60-70% savings |
| **Cold Start** | None | ~100-300ms | Trade-off for cost savings |
| **Maturity** | Production-ready | Production-ready | Both proven at scale |

### Recommendation: Start with Serverless

**Why:**
- 60-70% cost savings for MVP
- Simpler operations (no Kubernetes cluster management)
- Faster time to market (proven patterns from tsum-app)
- Can migrate to Kubernetes later if sustained high traffic (>1000 req/sec)

---

## Recommended Architecture

### High-Level Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     Client Applications                     │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  CloudFront (Frontend) + API Gateway (Backend) + WAF       │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                Lambda Function (Go) - 1024MB, 30s           │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ 1. Auth (Cognito JWT)                                  │ │
│  │ 2. Prompt Validation (PII/secrets)                     │ │
│  │ 3. Policy Evaluation (Redis rate limiting)             │ │
│  │ 4. Model Routing (OpenAI/Anthropic/Azure)              │ │
│  │ 5. Inference (LLM Provider API call)                   │ │
│  │ 6. Audit Logging (async to PostgreSQL + S3)            │ │
│  └────────────────────────────────────────────────────────┘ │
└──────────┬──────────────────┬────────────────┬─────────────┘
           │                  │                │
           ▼                  ▼                ▼
    ┌──────────────┐   ┌─────────────┐   ┌──────────┐
    │   Aurora     │   │ ElastiCache │   │    S3    │
    │ PostgreSQL   │   │   (Redis)   │   │  Bucket  │
    │ Serverless   │   │             │   │ (Audit)  │
    └──────────────┘   └─────────────┘   └──────────┘
           │
           ▼
    ┌──────────────────────────────────────────────┐
    │          AWS Secrets Manager                 │
    │  - Aurora credentials                        │
    │  - LLM provider API keys                     │
    └──────────────────────────────────────────────┘
           │
           ▼
    ┌──────────────────────────────────────────────┐
    │       External LLM Providers                 │
    │  - OpenAI (GPT-4, GPT-3.5)                   │
    │  - Anthropic (Claude)                        │
    │  - Azure OpenAI                              │
    └──────────────────────────────────────────────┘
```

### Request Flow (Latency Breakdown)

```
Client Request
  ↓ (10-50ms) - CloudFront/API Gateway
Lambda Cold Start (first request only)
  ↓ (100-300ms) - Lambda initialization
Lambda Handler
  ↓ (5-10ms) - JWT validation (Cognito)
  ↓ (2-5ms) - PII detection (regex)
  ↓ (5-10ms) - Redis rate limit check
  ↓ (10-20ms) - PostgreSQL policy lookup (cached in Redis)
LLM Provider
  ↓ (300-2000ms) - OpenAI/Anthropic API call
Lambda Response
  ↓ (2-5ms) - Async audit log queue (SQS)
  ↓ (10-50ms) - API Gateway/CloudFront
Client Response
───────────────────────────────────────
Total (warm): ~350-2100ms
Total (cold): ~450-2400ms
```

**Optimization opportunities:**
- Provisioned concurrency for Lambda (eliminates cold starts)
- Redis caching for policy lookups (reduces PostgreSQL queries)
- Async audit logging via SQS (non-blocking)

---

## Reusable Patterns from tsum-app

### 1. Unified Lambda Handler

**Source:** `backend/lambda.go`

**Pattern:**
```go
func handler(ctx context.Context, event json.RawMessage) (interface{}, error) {
    // Single Lambda handles multiple event types
    if isHTTPRequest(event) {
        return handleHTTPRequest(ctx, event)
    } else if isCustomEvent(event) {
        return handleCustomEvent(ctx, event) // Migrations, seeders
    }
}
```

**Benefits:**
- Single deployment artifact
- Reduced cold start overhead (one function stays warm)
- Simpler CI/CD

**Adaptation for LLM Control Plane:**
- HTTP requests → Inference API
- Custom events → Migrations, policy cache refresh

---

### 2. Configuration Management

**Source:** `backend/config/config.go`

**Pattern:**
```go
type Config struct {
    Environment     string
    DatabaseURL     string // From Secrets Manager in prod
    CognitoUserPoolID string
    S3BucketName    string
}

func LoadConfig() (*Config, error) {
    if secretArn := os.Getenv("AURORA_SECRET_ARN"); secretArn != "" {
        // Production: Load from Secrets Manager
        cfg.DatabaseURL = getSecretValue(secretArn)
    } else {
        // Development: Load from .env
        cfg.DatabaseURL = os.Getenv("DATABASE_URL")
    }
    return cfg, nil
}
```

**Benefits:**
- Environment parity (dev uses .env, prod uses Secrets Manager)
- No hardcoded credentials
- Easy to test locally

---

### 3. Chi Router + Middleware

**Source:** `backend/routes/routes.go`

**Pattern:**
```go
func SetupRoutes(r chi.Router, deps *Dependencies) {
    // Global middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.Logger)
    r.Use(errorMiddleware)
    r.Use(corsMiddleware)
    
    // Protected routes
    r.Group(func(r chi.Router) {
        r.Use(RequireAuthMiddleware)
        r.Use(TenantMiddleware)
        r.Post("/api/v1/resource", handler)
    })
}
```

**Benefits:**
- Clean separation of concerns
- Easy to add new middleware (PII detection, rate limiting)
- Standard HTTP patterns

**Adaptation for LLM Control Plane:**
```go
r.Group(func(r chi.Router) {
    r.Use(RequireAuthMiddleware)        // JWT validation
    r.Use(ExtractTenantMiddleware)      // Org/App context
    r.Use(PromptValidationMiddleware)   // PII/secrets detection
    r.Use(PolicyEnforcementMiddleware)  // Rate limits, cost caps
    
    r.Post("/v1/chat/completions", chatCompletionHandler)
})
```

---

### 4. Event-Driven Migrations

**Source:** `.github/workflows/infra.yml` + `backend/lambda.go`

**Pattern:**
```yaml
# GitHub Actions
- name: Run database migrations
  run: |
    aws lambda invoke \
      --function-name ${{ steps.lambda.outputs.function_arn }} \
      --payload '{"type": "user-migration"}' \
      /tmp/migration-response.json
```

```go
// Lambda handler
if event.Type == "user-migration" {
    return runMigrations(ctx, deps.DB)
}
```

**Benefits:**
- No direct database access needed
- Migrations run in same environment as application
- Traceable in CloudWatch/Datadog

**Adaptation for LLM Control Plane:**
- Same pattern, migrations in `backend/migrations/schema/`
- Add `{"type": "policy-cache-refresh"}` event for policy updates

---

### 5. Repository Pattern

**Source:** `internal/inrush/repository.go`

**Pattern:**
```go
type Repository struct {
    db *sql.DB
}

func (r *Repository) Create(ctx context.Context, entity *Entity) error {
    query := `INSERT INTO entities (id, name) VALUES ($1, $2)`
    _, err := r.db.ExecContext(ctx, query, entity.ID, entity.Name)
    return err
}
```

**Benefits:**
- Testable (mock repository for unit tests)
- Centralized data access logic
- Easy to swap database (PostgreSQL → DynamoDB)

**Adaptation for LLM Control Plane:**
```go
// internal/policy/repository.go
func (r *PolicyRepository) GetPoliciesForOrg(ctx context.Context, orgID string) ([]*Policy, error)

// internal/audit/repository.go
func (r *AuditRepository) LogRequest(ctx context.Context, log *RequestLog) error
```

---

### 6. Multi-Tenant Connection Manager

**Source:** `backend/app/tenant_connections.go`

**Pattern:**
```go
type Manager struct {
    connections map[string]*sql.DB
    mu          sync.RWMutex
}

func (m *Manager) GetConnection(ctx context.Context, orgID string) (*sql.DB, error) {
    // Check if connection exists
    m.mu.RLock()
    db, exists := m.connections[orgID]
    m.mu.RUnlock()
    if exists {
        return db, nil
    }
    
    // Create new connection
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Fetch tenant metadata, create connection to tenant DB
    db = createConnectionToTenantDB(orgID)
    m.connections[orgID] = db
    return db, nil
}
```

**Benefits:**
- Strong data isolation (database-per-tenant)
- Connection pooling per tenant
- Independent backup/restore

**Adaptation for LLM Control Plane:**
- **Phase 1 (MVP):** Single database with `org_id` foreign keys
- **Phase 2:** Database-per-tenant using this pattern

---

## GitHub Actions CI/CD

### Workflow Structure (from tsum-app)

```
.github/workflows/
├── infra.yml          # Reusable workflow (infrastructure + deployment)
├── on-push.yml        # Sandbox deployment (triggers on push to main)
├── on-tags.yml        # Production deployment (triggers on version tags)
└── on-tags-rc.yml     # Demo/RC deployment (triggers on RC tags)
```

**Key features:**
1. **OIDC authentication:** No long-lived AWS credentials
2. **Reusable actions:** Custom actions for network, Aurora, Lambda, etc.
3. **Environment-specific:** Different configs per environment
4. **Secrets management:** GitHub Secrets + AWS Secrets Manager

### Deployment Flow

```
1. Code push/tag → GitHub Actions triggered
2. Checkout code + authenticate to AWS (OIDC)
3. Terraform backend setup (S3 + DynamoDB)
4. Network infrastructure (VPC, subnets, security groups)
5. Aurora PostgreSQL deployment (Serverless v2)
6. ElastiCache Redis deployment
7. Lambda function build + deployment
8. Database migrations (Lambda invocation)
9. Database seeders (Lambda invocation)
10. API Gateway deployment (custom domain)
11. Frontend build (React + Vite)
12. Frontend deployment (S3 + CloudFront)
```

**Deployment time:** ~15-20 minutes per environment

---

## Cost Optimization

### Monthly Cost Estimates

**Sandbox Environment (10,000 requests/month):**
- Lambda: $0.20
- API Gateway: $0.10
- Aurora Serverless v2: $15
- ElastiCache Redis: $12
- S3: $0.25
- CloudFront: $1.00
- Secrets Manager: $1.20
- CloudWatch: $5.00
- **Total: ~$35/month**

**Production Environment (1000 req/sec = 2.6B requests/month):**
- Lambda: $5,200
- API Gateway: $9,100
- Aurora Serverless v2: $200
- ElastiCache Redis: $300
- S3: $25
- CloudFront: $850
- Secrets Manager: $2
- CloudWatch: $50
- WAF: $5,200
- **Total: ~$20,927/month**

**Note:** LLM provider costs (OpenAI, Anthropic) are passed through to end users.

### Optimization Strategies

1. **Use Lambda arm64** (Graviton2): 20% cost savings
2. **Aurora Serverless v2:** Scales to zero in sandbox
3. **S3 Intelligent-Tiering:** Automatic cost optimization
4. **CloudFront caching:** Reduce origin requests
5. **Provisioned concurrency:** Only for critical endpoints

---

## Development Environment Setup

### Quick Start (5 minutes)

```powershell
# 1. Clone repository
git clone https://github.com/<org>/llm-control-plane.git
cd llm-control-plane

# 2. Start Docker services
cd backend
docker-compose up -d

# 3. Configure environment
cp .env.example .env
# Edit .env with your LLM API keys

# 4. Install dependencies
go mod download
cd ../frontend && npm install

# 5. Run migrations
cd ../backend
make migrate-up

# 6. Start development servers (2 terminals)
# Terminal 1: make backend-dev
# Terminal 2: cd frontend && npm run dev

# 7. Test API
curl http://localhost:8080/health
```

### Docker Services

```yaml
services:
  postgres:
    image: postgres:16-alpine
    ports: ["5432:5432"]
    environment:
      POSTGRES_DB: audit
      POSTGRES_USER: dev
      POSTGRES_PASSWORD: audit_password
  
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
```

---

## Production Deployment

### Prerequisites

1. **AWS Account** with IAM role for GitHub Actions (OIDC)
2. **GitHub Secrets** configured:
   - `AWS_ROLE_ARN`
   - `DATADOG_API_KEY`
   - `DATADOG_APPLICATION_ID`
   - `DATADOG_CLIENT_TOKEN`
3. **Secrets Manager** secrets:
   - `/llm-cp/prod/providers` (LLM API keys)
   - `/llm-cp/prod/datadog` (Datadog API key)

### Deployment Steps

```powershell
# 1. Create production infrastructure config
mkdir infra/prod-llm-cp
cp infra/sandbox-llm-cp/cors.json infra/prod-llm-cp/cors.json
# Edit CORS for production domain

# 2. Commit and push
git add infra/prod-llm-cp/
git commit -m "Add production configuration"
git push origin main

# 3. Tag release
git tag -a v1.0.0 -m "Release v1.0.0 - MVP"
git push origin v1.0.0

# 4. Monitor deployment
gh run watch

# 5. Test production
curl https://api.llm-cp.yourdomain.com/health
```

---

## Security Implementation

### Authentication Flow (Cognito)

1. User clicks "Login" → Redirect to Cognito hosted UI
2. Cognito authenticates user
3. Cognito redirects to `/oauth2/idpresponse` with authorization code
4. Backend exchanges code for JWT
5. Backend sets HTTP-only cookie with JWT
6. Frontend reads JWT from cookie for API calls

### JWT Claims

```json
{
  "sub": "user-id",
  "email": "user@example.com",
  "custom:orgId": "org-123",
  "custom:appId": "app-456",
  "custom:userRole": "developer"
}
```

### PII Detection

**Regex patterns:**
- Email: `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`
- Phone: `\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`
- SSN: `\b\d{3}-\d{2}-\d{4}\b`
- Credit card: `\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`

**Action:** Block request with 400 Bad Request if PII detected

### Rate Limiting (Redis)

**Algorithm:** Sliding window

**Implementation:**
```go
// Redis ZSET: sorted set of request timestamps
key := fmt.Sprintf("rate_limit:%s:%s:minute", orgID, appID)
now := time.Now().Unix()

// Add current request
redis.ZAdd(key, now, requestID)

// Count requests in last 60 seconds
count := redis.ZCount(key, now-60, now)

// Check limit
if count > 100 {
    return ErrRateLimitExceeded
}
```

---

## Observability

### Datadog APM Integration

**Lambda Layer:** Automatically added by GitHub Actions

**Custom Metrics:**
```go
m.client.Count("llm_cp.requests", 1, tags, 1)
m.client.Count("llm_cp.tokens", int64(tokens), tags, 1)
m.client.Gauge("llm_cp.cost_usd", cost, tags, 1)
m.client.Timing("llm_cp.latency", latency, tags, 1)
```

**Tags:**
- `model:gpt-4`
- `provider:openai`
- `org_id:org-123`
- `status:success`

### CloudWatch Dashboards

**Key Metrics:**
- Lambda invocations, errors, duration, throttles
- API Gateway 4xx, 5xx, latency
- Aurora connections, CPU, memory
- Redis CPU, memory, evictions

### Structured Logging

**Format:**
```json
{
  "level": "info",
  "timestamp": "2026-02-06T12:34:56.789Z",
  "request_id": "req-xyz789",
  "org_id": "org-123",
  "model": "gpt-4",
  "provider": "openai",
  "tokens_input": 15,
  "tokens_output": 8,
  "cost_usd": 0.00046,
  "latency_ms": 342
}
```

---

## Migration Path

### Phase 1: MVP (Weeks 1-10)

**Objective:** Core inference pipeline

**Deliverables:**
- ✅ Lambda function with Go Chi router
- ✅ Cognito authentication
- ✅ Basic prompt validation (PII regex)
- ✅ Single LLM provider (OpenAI)
- ✅ PostgreSQL audit logs
- ✅ GitHub Actions deployment (sandbox)

**Testing:**
- End-to-end inference request
- PII detection blocks requests
- Audit logs persisted

---

### Phase 2: Governance (Weeks 11-15)

**Objective:** Policy engine and rate limiting

**Deliverables:**
- ✅ Redis rate limiting
- ✅ Policy engine (rate limits, cost caps)
- ✅ Multiple LLM providers (OpenAI, Anthropic, Azure)
- ✅ Model routing logic
- ✅ Admin APIs

**Testing:**
- Rate limiting works
- Cost cap enforcement
- Provider failover

---

### Phase 3: Observability (Weeks 16-20)

**Objective:** Production-ready monitoring

**Deliverables:**
- ✅ CloudWatch dashboards
- ✅ Datadog RUM for frontend
- ✅ S3 archival for audit logs
- ✅ Alerting
- ✅ Load testing (1000 req/sec)

**Testing:**
- Load test passes
- Alerts fire correctly
- Audit logs archived to S3

---

## Key Differences from PRD

| PRD | This Implementation | Reasoning |
|-----|---------------------|-----------|
| **Kubernetes (EKS)** | Lambda | Lower ops complexity, cost-effective for MVP |
| **RDS Multi-AZ** | Aurora Serverless v2 | Auto-scaling, scales to zero |
| **Helm/ArgoCD** | GitHub Actions | Simpler CI/CD, fewer dependencies |
| **Prometheus/Grafana** | CloudWatch + Datadog | Managed services, less maintenance |
| **Manual HPA tuning** | Automatic Lambda scaling | No configuration needed |

**When to migrate to Kubernetes:**
- Sustained traffic > 1000 req/sec
- Cold starts become unacceptable (< 100ms required)
- Need for long-running connections (WebSockets, gRPC)
- Complex multi-service architecture

---

## Useful Resources

### Documentation
- **Architecture Guide:** `docs/ARCHITECTURE.md`
- **Setup Guide:** `docs/SETUP_GUIDE.md`
- **PRD:** `docs/PRD.md`

### External Resources
- **tsum-app repository:** (internal reference)
- **AWS Lambda Go:** https://github.com/aws/aws-lambda-go
- **Chi router:** https://github.com/go-chi/chi
- **Datadog Go:** https://github.com/DataDog/datadog-go
- **OpenAI Go SDK:** https://github.com/openai/openai-go

### Tools
- **AWS CLI:** https://aws.amazon.com/cli/
- **GitHub CLI:** https://cli.github.com/
- **Datadog:** https://app.datadoghq.com

---

## Conclusion

The **tsum-app** architecture provides a proven foundation for building the **LLM Control Plane** using AWS serverless services. Key takeaways:

1. **Serverless is sufficient for MVP** - Lower cost, simpler operations
2. **GitHub Actions patterns are reusable** - Adapt workflows for new project
3. **Multi-tenancy can be phased** - Start with single-tenant, migrate to database-per-tenant later
4. **Event-driven migrations are powerful** - Eliminates manual database access
5. **Observability is built-in** - Datadog APM + CloudWatch from day 1

**Next Steps:**
1. Set up local development environment (1 day)
2. Create initial Lambda function structure (2-3 days)
3. Deploy sandbox environment (1 day)
4. Implement core inference pipeline (1-2 weeks)
5. Add governance features (2-3 weeks)
6. Production deployment (1 week)

**Total estimated time:** 6-8 weeks for MVP (Phase 1)

---

**End of Implementation Summary**

**Version:** 1.0  
**Last Updated:** February 6, 2026  
**Authors:** Analysis of tsum-app + LLM Control Plane PRD
