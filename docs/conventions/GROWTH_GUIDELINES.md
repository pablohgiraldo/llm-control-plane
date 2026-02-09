# Growth and Scalability Guidelines

**Version:** 1.0  
**Last Updated:** February 9, 2026  
**Purpose:** Define conventions for extending the LLM Control Plane as the system grows

---

## Table of Contents

1. [Feature Flags](#feature-flags)
2. [Runtime Configuration](#runtime-configuration)
3. [Adding New Providers](#adding-new-providers)
4. [Control Plane vs Runtime Plane](#control-plane-vs-runtime-plane)
5. [Scaling Considerations](#scaling-considerations)

---

## Feature Flags

### Location and Structure

**Primary Location:** `backend/internal/runtimeconfig/features.go`

```go
package runtimeconfig

// FeatureFlags contains all feature flag definitions
type FeatureFlags struct {
    // RAG Integration
    EnableRAG bool `env:"FEATURE_RAG_ENABLED" default:"false"`
    
    // Advanced Routing
    EnableCostOptimizedRouting bool `env:"FEATURE_COST_ROUTING" default:"false"`
    EnableABTesting            bool `env:"FEATURE_AB_TESTING" default:"false"`
    
    // Observability
    EnableDetailedMetrics bool `env:"FEATURE_DETAILED_METRICS" default:"true"`
    EnableDistributedTracing bool `env:"FEATURE_DISTRIBUTED_TRACING" default:"false"`
    
    // Security
    EnableAdvancedPIIDetection bool `env:"FEATURE_ADVANCED_PII" default:"false"`
    EnableMLBasedValidation    bool `env:"FEATURE_ML_VALIDATION" default:"false"`
}
```

### Storage Options

#### 1. Environment Variables (Simple, MVP)
- **Use for:** Binary on/off flags
- **Location:** Environment variables
- **Hot reload:** Requires restart
- **Example:** `FEATURE_RAG_ENABLED=true`

#### 2. Database (Per-Tenant)
- **Use for:** Tenant-specific feature rollouts
- **Location:** `feature_flags` table in PostgreSQL
- **Hot reload:** Yes, with cache invalidation
- **Schema:**

```sql
CREATE TABLE feature_flags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID REFERENCES organizations(id),
    feature_name VARCHAR(100) NOT NULL,
    enabled BOOLEAN DEFAULT false,
    rollout_percentage INT DEFAULT 0, -- 0-100 for gradual rollout
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(org_id, feature_name)
);
```

#### 3. External Service (Enterprise)
- **Use for:** Complex rollout strategies, A/B testing
- **Options:** LaunchDarkly, Unleash, ConfigCat
- **Integration:** `backend/internal/runtimeconfig/featureflag_client.go`

### Usage Pattern

```go
// In service layer
func (s *Service) ProcessRequest(ctx context.Context, req *Request) error {
    flags := runtimeconfig.GetFeatureFlags(ctx)
    
    if flags.EnableRAG {
        // Use RAG enhancement
        docs, err := s.ragRetriever.Retrieve(ctx, req.Prompt)
        if err != nil {
            // Graceful degradation
            log.Warn(ctx, "RAG retrieval failed, continuing without enhancement")
        } else {
            req.Prompt = enrichWithContext(req.Prompt, docs)
        }
    }
    
    return s.processCore(ctx, req)
}
```

### Naming Conventions

- **Prefix:** `FEATURE_` for environment variables
- **Format:** `FEATURE_<DOMAIN>_<ACTION>` (e.g., `FEATURE_RAG_ENABLED`)
- **Database:** `snake_case` (e.g., `rag_enabled`)
- **Code:** `PascalCase` with `Enable` prefix (e.g., `EnableRAG`)

### Lifecycle

1. **Development:** Flag off by default, enabled in dev environment
2. **Testing:** Enabled for specific test tenants
3. **Rollout:** Gradual percentage-based rollout (10% → 50% → 100%)
4. **Stable:** Remove flag, make feature permanent
5. **Deprecation:** Flag controls old behavior, default to new

---

## Runtime Configuration

### Location and Structure

**Primary Location:** `backend/internal/runtimeconfig/`

```
runtimeconfig/
├── doc.go              # Package documentation
├── types.go            # Config structs
├── manager.go          # Config manager implementation
├── loader.go           # Load from various sources
├── features.go         # Feature flags
└── watcher.go          # Hot-reload support
```

### Configuration Sources (Priority Order)

1. **Environment Variables** (Highest priority)
   - Use for: Deployment-specific settings
   - Example: `DATABASE_URL`, `REDIS_URL`

2. **AWS Secrets Manager** (Production)
   - Use for: Sensitive credentials
   - Example: LLM provider API keys, database passwords

3. **Configuration Files** (Local development)
   - Use for: Complex structured config
   - Location: `config/` directory (gitignored)
   - Format: YAML or JSON

4. **Database** (Per-tenant overrides)
   - Use for: Tenant-specific settings
   - Table: `tenant_config`

### Hot-Reload Support

**Implementation:** `backend/internal/runtimeconfig/watcher.go`

```go
type Watcher interface {
    Watch(ctx context.Context) error
    OnChange(callback func(Config))
}

// Usage
watcher := runtimeconfig.NewWatcher(configPath)
watcher.OnChange(func(cfg Config) {
    log.Info("Configuration reloaded")
    // Update in-memory config
})
go watcher.Watch(ctx)
```

### Configuration Categories

#### 1. Static (Requires Restart)
- Server port and address
- Database connection pool size
- TLS certificates

#### 2. Dynamic (Hot-Reloadable)
- Feature flags
- Rate limit thresholds
- Cost cap values
- Provider endpoints
- Routing weights

#### 3. Per-Tenant (Database)
- Model restrictions
- Cost caps
- Rate limits
- Preferred providers

### Access Pattern

```go
// Get configuration with context
cfg := runtimeconfig.FromContext(ctx)

// Get with default fallback
timeout := cfg.GetDurationWithDefault("provider.timeout", 30*time.Second)

// Get tenant-specific override
rateLimit := cfg.GetTenantInt(ctx, "rate_limit.rpm", 100)
```

### Validation

All configuration must be validated on load:

```go
func (c *Config) Validate() error {
    if c.HTTPAddr == "" {
        return errors.New("HTTP_ADDR is required")
    }
    if c.Database.MaxConnections < 1 {
        return errors.New("database max connections must be >= 1")
    }
    // ... more validations
    return nil
}
```

---

## Adding New Providers

### Step-by-Step Process

#### 1. Create Provider Adapter

**Location:** `backend/internal/providers/<provider_name>.go`

```go
package providers

import (
    "context"
    "time"
)

// NewGeminiProvider creates a Google Gemini provider adapter
func NewGeminiProvider(apiKey string) Provider {
    return &geminiProvider{
        apiKey: apiKey,
        client: gemini.NewClient(apiKey),
    }
}

type geminiProvider struct {
    apiKey string
    client *gemini.Client
}

func (p *geminiProvider) Name() string {
    return "gemini"
}

func (p *geminiProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 1. Convert unified request to provider-specific format
    geminiReq := p.convertRequest(req)
    
    // 2. Call provider API
    start := time.Now()
    geminiResp, err := p.client.GenerateContent(ctx, geminiReq)
    if err != nil {
        return nil, fmt.Errorf("gemini API call failed: %w", err)
    }
    
    // 3. Convert provider response to unified format
    resp := p.convertResponse(geminiResp)
    resp.Provider = p.Name()
    resp.Latency = time.Since(start)
    
    return resp, nil
}

func (p *geminiProvider) IsAvailable(ctx context.Context) bool {
    // Health check implementation
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    _, err := p.client.ListModels(ctx)
    return err == nil
}

func (p *geminiProvider) CalculateCost(req *ChatRequest, resp *ChatResponse) (float64, error) {
    // Provider-specific pricing logic
    pricePerMillionTokens := map[string]struct{ Input, Output float64 }{
        "gemini-pro":    {Input: 0.5, Output: 1.5},
        "gemini-ultra":  {Input: 10.0, Output: 30.0},
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

#### 2. Register Provider

**Location:** `backend/internal/providers/registry.go`

```go
package providers

type Registry struct {
    providers map[string]Provider
}

func NewRegistry(cfg *runtimeconfig.ProvidersConfig) *Registry {
    r := &Registry{
        providers: make(map[string]Provider),
    }
    
    // Register providers based on configuration
    if cfg.OpenAI.APIKey != "" {
        r.Register(NewOpenAIProvider(cfg.OpenAI.APIKey))
    }
    if cfg.Anthropic.APIKey != "" {
        r.Register(NewAnthropicProvider(cfg.Anthropic.APIKey))
    }
    if cfg.Gemini.APIKey != "" {
        r.Register(NewGeminiProvider(cfg.Gemini.APIKey))
    }
    
    return r
}

func (r *Registry) Register(p Provider) {
    r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (Provider, bool) {
    p, ok := r.providers[name]
    return p, ok
}
```

#### 3. Update Configuration

**Location:** `backend/internal/runtimeconfig/types.go`

```go
type ProvidersConfig struct {
    OpenAI    ProviderCredentials
    Anthropic ProviderCredentials
    Azure     AzureCredentials
    Gemini    ProviderCredentials  // Add new provider
}
```

#### 4. Update Routing Logic

**Location:** `backend/internal/routing/router.go`

```go
// Model-to-provider mapping
var modelProviderMap = map[string]string{
    "gpt-4":              "openai",
    "gpt-3.5-turbo":      "openai",
    "claude-3-opus":      "anthropic",
    "claude-3-sonnet":    "anthropic",
    "gemini-pro":         "gemini",     // Add new models
    "gemini-ultra":       "gemini",
}
```

#### 5. Add Tests

**Location:** `backend/internal/providers/gemini_test.go`

```go
func TestGeminiProvider_ChatCompletion(t *testing.T) {
    // Table-driven tests
}

func TestGeminiProvider_CalculateCost(t *testing.T) {
    // Cost calculation tests
}
```

#### 6. Update Documentation

- Add provider to README.md
- Document model names and pricing
- Add configuration example

### Provider Checklist

- [ ] Implement `Provider` interface
- [ ] Handle rate limiting and retries
- [ ] Implement circuit breaker
- [ ] Add cost calculation
- [ ] Add health check
- [ ] Write unit tests (80%+ coverage)
- [ ] Add integration tests
- [ ] Update configuration
- [ ] Update routing logic
- [ ] Document in README

---

## Control Plane vs Runtime Plane

### Definitions

#### Control Plane (Configuration & Governance)
**Purpose:** Manages policies, configuration, and governance rules

**Components:**
- Policy management (CRUD operations)
- User/organization management
- Configuration management
- Audit log queries
- Metrics dashboards
- Admin APIs

**Characteristics:**
- Low traffic volume
- High consistency requirements
- Can tolerate higher latency
- Requires strong authentication
- Typically synchronous

**Location:** `backend/internal/admin/` or separate service

#### Runtime Plane (Request Processing)
**Purpose:** Handles actual LLM requests with low latency

**Components:**
- Authentication middleware
- Prompt validation
- Policy evaluation (read-only)
- Model routing
- Provider calls
- Audit logging (async)
- Metrics collection

**Characteristics:**
- High traffic volume
- Low latency requirements (<500ms target)
- Read-heavy operations
- Caching critical
- Mostly stateless
- Async where possible

**Location:** `backend/internal/` (current structure)

### Separation Strategy

#### Phase 1: Monolithic (Current)
- Single service handles both planes
- Shared database and cache
- Simple deployment

```
┌─────────────────────────────────┐
│      API Gateway (Single)       │
│  ┌──────────────────────────┐   │
│  │   Control Plane APIs     │   │
│  │   /admin/*               │   │
│  └──────────────────────────┘   │
│  ┌──────────────────────────┐   │
│  │   Runtime Plane APIs     │   │
│  │   /v1/chat/completions   │   │
│  └──────────────────────────┘   │
└─────────────────────────────────┘
```

#### Phase 2: Logical Separation
- Same service, different routers
- Separate middleware stacks
- Different rate limits

```go
// Control plane routes
adminRouter := chi.NewRouter()
adminRouter.Use(RequireAdminAuth)
adminRouter.Use(AdminRateLimiter)
adminRouter.Post("/policies", createPolicyHandler)

// Runtime plane routes
runtimeRouter := chi.NewRouter()
runtimeRouter.Use(RequireAuth)
runtimeRouter.Use(RuntimeRateLimiter)
runtimeRouter.Use(PromptValidation)
runtimeRouter.Post("/v1/chat/completions", chatHandler)
```

#### Phase 3: Physical Separation (Scale)
- Separate services
- Independent scaling
- Shared data stores

```
┌──────────────────┐      ┌──────────────────┐
│  Control Plane   │      │  Runtime Plane   │
│  (Admin API)     │      │  (Inference API) │
│                  │      │                  │
│  - Low traffic   │      │  - High traffic  │
│  - CRUD ops      │      │  - Read-heavy    │
│  - Sync          │      │  - Low latency   │
└────────┬─────────┘      └────────┬─────────┘
         │                         │
         └────────┬────────────────┘
                  │
         ┌────────▼─────────┐
         │   PostgreSQL     │
         │   Redis          │
         └──────────────────┘
```

### Request Flow Comparison

#### Control Plane Request (Policy Creation)
```
Admin UI → Auth (Admin) → Validation → Database Write → Cache Invalidate → Response
Time: ~100-500ms (acceptable)
```

#### Runtime Plane Request (Inference)
```
Client → Auth → Prompt Validation → Policy Check (Cache) → Route → Provider → Audit (Async) → Response
Time: <500ms target (excluding provider latency)
```

### Data Access Patterns

#### Control Plane
- **Writes:** Frequent (policies, config, users)
- **Reads:** Moderate (admin dashboards, reports)
- **Consistency:** Strong (immediate consistency required)
- **Caching:** Minimal (data changes frequently)

#### Runtime Plane
- **Writes:** Minimal (audit logs only, async)
- **Reads:** Very frequent (policy checks, config)
- **Consistency:** Eventual (can tolerate slight staleness)
- **Caching:** Aggressive (Redis, in-memory)

### Scaling Considerations

#### Control Plane Scaling
- **Vertical:** Increase instance size
- **Horizontal:** 2-3 instances for HA
- **Database:** Single writer, read replicas optional

#### Runtime Plane Scaling
- **Vertical:** Start here (optimize first)
- **Horizontal:** Scale to 10s-100s of instances
- **Database:** Read replicas essential
- **Cache:** Redis cluster with replication

---

## Scaling Considerations

### When to Scale Each Component

#### Application (Runtime Plane)
**Trigger:** CPU > 70%, Latency > 500ms
**Action:** Add more Lambda functions or container instances

#### Database (PostgreSQL)
**Trigger:** Connection pool exhaustion, slow queries
**Action:** 
1. Add read replicas
2. Optimize queries
3. Increase instance size
4. Consider Aurora Serverless v2

#### Cache (Redis)
**Trigger:** High eviction rate, memory pressure
**Action:**
1. Increase instance size
2. Add replica for read scaling
3. Implement cluster mode

### Caching Strategy

#### L1: In-Memory (Application)
- **TTL:** 1-5 minutes
- **Use for:** Feature flags, static config
- **Invalidation:** Time-based

#### L2: Redis (Distributed)
- **TTL:** 5-15 minutes
- **Use for:** Policies, tenant config, rate limits
- **Invalidation:** Event-based (pub/sub)

#### L3: Database (Source of Truth)
- **TTL:** Infinite
- **Use for:** All persistent data
- **Invalidation:** Never (update in place)

### Multi-Tenancy Scaling

#### Shared (Phase 1)
- Single database for all tenants
- Row-level isolation with `org_id`
- Shared cache with tenant-prefixed keys

#### Database-Per-Tenant (Phase 2)
- Separate database per large tenant
- Connection pooling per tenant
- Metadata in central database

---

## Migration Patterns

### Adding New Features

1. **Add feature flag** (default: off)
2. **Implement behind flag**
3. **Test with dev tenant**
4. **Gradual rollout** (10% → 50% → 100%)
5. **Monitor metrics**
6. **Remove flag** (make permanent)

### Deprecating Features

1. **Add deprecation warning** (logs, headers)
2. **Announce timeline** (90 days notice)
3. **Add feature flag** (default: new behavior)
4. **Monitor usage** (track old behavior usage)
5. **Remove old code** (after timeline)

### Breaking Changes

1. **Version API** (`/v1/`, `/v2/`)
2. **Support both versions** (6-12 months)
3. **Migrate tenants** (gradual, tenant-by-tenant)
4. **Deprecate old version**
5. **Remove old version**

---

## Summary

### Key Principles

1. **Feature Flags:** Use for gradual rollouts and A/B testing
2. **Runtime Config:** Hot-reload for operational flexibility
3. **Provider Adapters:** Follow interface pattern, add tests
4. **Plane Separation:** Start monolithic, separate as needed
5. **Caching:** Aggressive for runtime, minimal for control
6. **Scaling:** Horizontal for runtime, vertical for control

### Decision Matrix

| Concern | Solution | Location |
|---------|----------|----------|
| New feature rollout | Feature flag | `runtimeconfig/features.go` |
| Provider API key | Runtime config | AWS Secrets Manager |
| New LLM provider | Provider adapter | `providers/<name>.go` |
| Admin operations | Control plane | Separate router/service |
| Request processing | Runtime plane | Current structure |
| Tenant-specific settings | Database config | `tenant_config` table |

---

## References

- [12-Factor App](https://12factor.net/) - Configuration and deployment principles
- [Feature Toggles](https://martinfowler.com/articles/feature-toggles.html) - Martin Fowler's guide
- [Circuit Breaker Pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/circuit-breaker) - Resilience patterns
- [Caching Strategies](https://aws.amazon.com/caching/best-practices/) - AWS best practices

---

**Next Steps:**
1. Implement feature flag system
2. Add hot-reload for runtime config
3. Create provider registry
4. Separate control/runtime routers
5. Add caching layer
