# LLM Control Plane - Pipeline Specification

**Version:** 1.0  
**Date:** February 6, 2026  
**Status:** Technical Specification

---

## Table of Contents

1. [Pipeline Overview](#pipeline-overview)
2. [Data Flow Architecture](#data-flow-architecture)
3. [Stage-by-Stage Specification](#stage-by-stage-specification)
4. [Error Handling Contracts](#error-handling-contracts)
5. [AWS Service Integration Map](#aws-service-integration-map)
6. [Performance Budgets](#performance-budgets)
7. [Implementation Guidelines](#implementation-guidelines)

---

## 1. Pipeline Overview

### 1.1 Complete Pipeline Order

```
┌────────────────────────────────────────────────────────────────┐
│                    REQUEST INGRESS                             │
│  CloudFront → API Gateway → WAF → Lambda Function             │
└────────────────────┬───────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│               STAGE 1: AUTHENTICATION                           │
│  AWS: Cognito User Pools                                        │
│  Input: HTTP Request (headers, body)                            │
│  Output: Validated JWT Claims (user_id, org_id, app_id, role)  │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│        STAGE 2: APPLICATION & CALLER IDENTIFICATION             │
│  AWS: N/A (in-memory processing)                                │
│  Input: JWT Claims                                              │
│  Output: TenantContext (org_id, app_id, user_id, role)         │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│   STAGE 3: REQUEST NORMALIZATION & INTENT CLASSIFICATION        │
│  AWS: N/A (in-memory processing)                                │
│  Input: Raw request body + TenantContext                        │
│  Output: NormalizedRequest + IntentMetadata                     │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│         STAGE 4: POLICY RESOLUTION & AUTHORIZATION              │
│  AWS: Aurora PostgreSQL (policies) + ElastiCache (cache)        │
│  Input: TenantContext + IntentMetadata                          │
│  Output: ResolvedPolicies + AuthorizationDecision               │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                STAGE 5: CACHE CHECK (OPTIONAL)                  │
│  AWS: ElastiCache Redis                                         │
│  Input: NormalizedRequest + TenantContext                       │
│  Output: CachedResponse OR cache_miss                           │
│  [If cache hit] ──────────────────────┐                         │
└────────────────────┬──────────────────┼─────────────────────────┘
                     │                  │
                     │                  │
                     ▼                  │
┌─────────────────────────────────────────────────────────────────┐
│              STAGE 6: BUDGET & RATE PRE-CHECK                   │
│  AWS: ElastiCache Redis (rate limiting counters)                │
│       Aurora PostgreSQL (cost tracking)                         │
│  Input: TenantContext + ResolvedPolicies + EstimatedCost        │
│  Output: QuotaCheckResult (allowed/denied + remaining)          │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│            STAGE 7: PRE-PROCESSING VALIDATION                   │
│  AWS: N/A (in-memory regex + optional Comprehend)               │
│  Input: NormalizedRequest (messages, prompt)                    │
│  Output: ValidationResult + RedactedPrompt                      │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│          STAGE 8: POLICY-DRIVEN CONTENT FILTERING               │
│  AWS: N/A (in-memory policy evaluation)                         │
│  Input: ValidationResult + ResolvedPolicies + RedactedPrompt    │
│  Output: FilteredRequest                                        │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STAGE 9: PROVIDER ROUTING                      │
│  AWS: N/A (in-memory routing logic)                             │
│       Secrets Manager (provider credentials)                    │
│  Input: FilteredRequest + ResolvedPolicies + IntentMetadata     │
│  Output: SelectedProvider + ProviderConfig                      │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STAGE 11: LLM INVOCATION                       │
│  AWS: Lambda (execution) + VPC NAT Gateway (outbound)           │
│  External: OpenAI/Anthropic/Azure APIs                          │
│  Input: FilteredRequest + HealthyProvider + ProviderConfig      │
│  Output: RawLLMResponse + UsageMetrics                          │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                STAGE 12: RESPONSE INSPECTION                    │
│  AWS: N/A (in-memory validation + optional Comprehend)          │
│  Input: RawLLMResponse + ResolvedPolicies                       │
│  Output: InspectedResponse + ValidationReport                   │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│       STAGE 14: AUDIT, METRICS & TRACE CORRELATION              │
│  AWS: Aurora PostgreSQL (audit logs)                            │
│       S3 (long-term archive via EventBridge)                    │
│       CloudWatch (metrics, logs)                                │
│       SQS (async processing queue)                              │
│  Input: Full request/response + all stage decisions             │
│  Output: audit_logged (non-blocking)                            │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                 STAGE 15: RESPONSE DELIVERY                     │
│  AWS: API Gateway → CloudFront (if web client)                  │
│  Input: InspectedResponse + ResponseMetadata                    │
│  Output: HTTP Response (JSON)                                   │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 Critical Path vs. Async Stages

**Synchronous (blocks response):**

- Stages 1-12: Must complete before response

**Asynchronous (non-blocking):**

- Stage 13: Cache Store
- Stage 14: Audit Logging

**Early Exit Points:**

- Stage 1: Authentication failure → 401 Unauthorized
- Stage 4: Authorization denied → 403 Forbidden
- Stage 5: Cache hit → Skip to Stage 15
- Stage 6: Rate limit exceeded → 429 Too Many Requests
- Stage 7: PII detected → 400 Bad Request
- Stage 10: All providers down → 503 Service Unavailable

---

## 2. Data Flow Architecture

### 2.1 Context Object (passed through pipeline)

```go
// backend/internal/pipeline/context.go
package pipeline

import (
    "context"
    "time"
)

// RequestContext is the unified context passed through all stages
type RequestContext struct {
    // Request metadata
    RequestID       string
    Timestamp       time.Time
    ClientIP        string
    UserAgent       string

    // Authentication (Stage 1)
    JWTClaims       *JWTClaims

    // Tenant identification (Stage 2)
    TenantContext   *TenantContext

    // Normalization (Stage 3)
    NormalizedRequest *NormalizedRequest
    IntentMetadata    *IntentMetadata

    // Policy resolution (Stage 4)
    ResolvedPolicies  *ResolvedPolicies
    AuthzDecision     *AuthorizationDecision

    // Cache check (Stage 5)
    CacheResult       *CacheResult

    // Budget & rate check (Stage 6)
    QuotaCheck        *QuotaCheckResult

    // Validation (Stage 7)
    ValidationResult  *ValidationResult

    // Content filtering (Stage 8)
    FilteredRequest   *FilteredRequest

    // Provider routing (Stage 9)
    SelectedProvider  *ProviderSelection

    // Circuit breaker (Stage 10)
    ProviderHealth    *ProviderHealthStatus

    // LLM invocation (Stage 11)
    LLMResponse       *LLMResponse
    UsageMetrics      *UsageMetrics

    // Response inspection (Stage 12)
    InspectedResponse *InspectedResponse

    // Audit trail
    StageTimings      map[string]time.Duration
    StageDecisions    map[string]interface{}
    Errors            []StageError
}

// GetContext extracts RequestContext from Go context
func GetContext(ctx context.Context) *RequestContext {
    if rc, ok := ctx.Value(requestContextKey).(*RequestContext); ok {
        return rc
    }
    return nil
}

// WithContext adds RequestContext to Go context
func WithContext(ctx context.Context, rc *RequestContext) context.Context {
    return context.WithValue(ctx, requestContextKey, rc)
}
```

---

## 3. Stage-by-Stage Specification

### STAGE 1: AUTHENTICATION

**AWS Services:** Cognito User Pools

**Responsibilities:**

- Validate JWT token signature using Cognito JWKS
- Extract and verify JWT claims (issuer, audience, expiration)
- Ensure token has not been revoked
- Extract user identity and role

**Entry Contract:**

```go
type Stage1Input struct {
    HTTPRequest *http.Request
    Headers     map[string]string
    Body        []byte
}
```

**Exit Contract:**

```go
type Stage1Output struct {
    JWTClaims *JWTClaims
    Error     *StageError
}

type JWTClaims struct {
    Subject      string            // "sub" - Cognito user ID
    Email        string            // "email"
    Username     string            // "cognito:username"
    OrgID        string            // "custom:orgId"
    AppID        string            // "custom:appId"
    UserRole     string            // "custom:userRole" (superadmin, admin, developer, auditor)
    IssuedAt     int64             // "iat"
    ExpiresAt    int64             // "exp"
    Issuer       string            // "iss"
    Audience     []string          // "aud"
    CustomClaims map[string]string // Additional custom attributes
}
```

**AWS Integration:**

```go
// Fetch Cognito JWKS for JWT validation
jwksURL := fmt.Sprintf(
    "https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json",
    region, userPoolID,
)
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `AUTH_MISSING_TOKEN` | 401 | No Authorization header | No |
| `AUTH_INVALID_TOKEN` | 401 | JWT signature invalid | No |
| `AUTH_EXPIRED_TOKEN` | 401 | Token expired | Yes (refresh token) |
| `AUTH_INVALID_ISSUER` | 401 | Token from wrong Cognito pool | No |
| `AUTH_JWKS_FETCH_FAILED` | 503 | Cannot fetch Cognito public keys | Yes (retry) |

**Boundary:**

- **IN:** Raw HTTP request from API Gateway
- **OUT:** Validated JWT claims
- **Does NOT:** Authorize actions (that's Stage 4)
- **Does NOT:** Resolve policies (that's Stage 4)

---

### STAGE 2: APPLICATION & CALLER IDENTIFICATION

**AWS Services:** None (in-memory processing)

**Responsibilities:**

- Extract organization, application, and user identifiers from JWT
- Build tenant context for downstream stages
- Validate tenant identifiers exist

**Entry Contract:**

```go
type Stage2Input struct {
    JWTClaims *JWTClaims
}
```

**Exit Contract:**

```go
type Stage2Output struct {
    TenantContext *TenantContext
    Error         *StageError
}

type TenantContext struct {
    OrganizationID   string
    ApplicationID    string
    UserID           string
    UserRole         string
    UserEmail        string

    // Optional metadata
    ServiceProviderID string // For multi-tenant mode
    TenantDatabaseName string // For database-per-tenant mode
}
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `TENANT_MISSING_ORG_ID` | 400 | JWT missing custom:orgId | No |
| `TENANT_MISSING_APP_ID` | 400 | JWT missing custom:appId | No |
| `TENANT_INVALID_FORMAT` | 400 | Tenant ID format invalid | No |

**Boundary:**

- **IN:** JWT claims
- **OUT:** Structured tenant context
- **Does NOT:** Validate tenant exists in database (assumed valid if JWT is valid)
- **Does NOT:** Fetch tenant metadata from database

---

### STAGE 3: REQUEST NORMALIZATION & INTENT CLASSIFICATION

**AWS Services:** None (in-memory processing)

**Responsibilities:**

- Parse and validate request body structure
- Normalize different API formats (chat, embeddings, completions)
- Classify request intent (conversational, retrieval, code generation, etc.)
- Determine data sensitivity level
- Extract key request parameters

**Entry Contract:**

```go
type Stage3Input struct {
    RawRequestBody []byte
    TenantContext  *TenantContext
    ContentType    string
}
```

**Exit Contract:**

```go
type Stage3Output struct {
    NormalizedRequest *NormalizedRequest
    IntentMetadata    *IntentMetadata
    Error             *StageError
}

type NormalizedRequest struct {
    // Unified request format
    RequestType      string                 // "chat", "embedding", "completion"
    Model            string                 // "gpt-4", "claude-3-opus", etc.
    Messages         []Message              // For chat requests
    Prompt           string                 // For completion requests
    Input            interface{}            // For embedding requests
    Parameters       *RequestParameters
    Metadata         map[string]interface{} // User-provided metadata
}

type Message struct {
    Role    string // "system", "user", "assistant"
    Content string
}

type RequestParameters struct {
    MaxTokens        int
    Temperature      float64
    TopP             float64
    FrequencyPenalty float64
    PresencePenalty  float64
    Stop             []string
    Stream           bool
}

type IntentMetadata struct {
    Classification   string   // "conversational", "retrieval", "code_generation", "data_analysis"
    SensitivityLevel string   // "public", "internal", "confidential", "restricted"
    EstimatedTokens  int      // Rough estimate for cost prediction
    RequiresContext  bool     // Does this need RAG/external context?
    Language         string   // Detected language (optional)
    Topics           []string // Detected topics (optional)
}
```

**Classification Logic:**

```go
func ClassifyIntent(request *NormalizedRequest) string {
    // Keyword-based classification
    prompt := extractPromptText(request)

    if containsCodeKeywords(prompt) {
        return "code_generation"
    } else if containsDataKeywords(prompt) {
        return "data_analysis"
    } else if containsQuestionKeywords(prompt) {
        return "retrieval"
    } else {
        return "conversational"
    }
}

func DetermineSensitivity(request *NormalizedRequest) string {
    // Pattern-based sensitivity detection
    prompt := extractPromptText(request)

    if containsPII(prompt) || containsFinancialData(prompt) {
        return "restricted"
    } else if containsInternalTerms(prompt) {
        return "internal"
    } else {
        return "public"
    }
}
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `NORM_INVALID_JSON` | 400 | Request body not valid JSON | No |
| `NORM_MISSING_MODEL` | 400 | No model specified | No |
| `NORM_INVALID_MESSAGES` | 400 | Messages array malformed | No |
| `NORM_EMPTY_PROMPT` | 400 | Prompt is empty or whitespace | No |
| `NORM_TOKEN_LIMIT_EXCEEDED` | 400 | Estimated tokens > max allowed | No |

**Boundary:**

- **IN:** Raw request body
- **OUT:** Normalized, structured request + metadata
- **Does NOT:** Modify prompt content (that's Stage 7-8)
- **Does NOT:** Enforce policies (that's Stage 4)

---

### STAGE 4: POLICY RESOLUTION & AUTHORIZATION

**AWS Services:**

- Aurora PostgreSQL (policy storage)
- ElastiCache Redis (policy caching)

**Responsibilities:**

- Fetch applicable policies for tenant (org → app → user hierarchy)
- Resolve policy conflicts (most restrictive wins)
- Check RBAC permissions for requested operation
- Determine if operation is authorized
- Calculate estimated cost for quota checks

**Entry Contract:**

```go
type Stage4Input struct {
    TenantContext     *TenantContext
    NormalizedRequest *NormalizedRequest
    IntentMetadata    *IntentMetadata
}
```

**Exit Contract:**

```go
type Stage4Output struct {
    ResolvedPolicies *ResolvedPolicies
    AuthzDecision    *AuthorizationDecision
    Error            *StageError
}

type ResolvedPolicies struct {
    // Rate limiting policies
    RateLimits struct {
        RequestsPerMinute int
        RequestsPerHour   int
        RequestsPerDay    int
        TokensPerMinute   int
        TokensPerDay      int
    }

    // Cost control policies
    CostCaps struct {
        MonthlyBudgetUSD  float64
        DailyBudgetUSD    float64
        PerRequestMaxUSD  float64
        AlertThreshold    float64 // e.g., 0.8 = 80% of budget
    }

    // Model restrictions
    ModelRestrictions struct {
        AllowedModels   []string // e.g., ["gpt-4", "claude-3-opus"]
        BlockedModels   []string // e.g., ["gpt-3.5-turbo"]
        AllowedProviders []string // e.g., ["openai", "anthropic"]
        BlockedProviders []string
    }

    // Content policies
    ContentPolicies struct {
        BlockPII            bool
        BlockSecrets        bool
        BlockPromptInjection bool
        AllowedTopics       []string
        BlockedTopics       []string
        MaxPromptLength     int
        MaxResponseLength   int
    }

    // Time-based restrictions
    TimeRestrictions struct {
        AllowedHours []int // 0-23, empty = always allowed
        AllowedDays  []string // "monday", "tuesday", etc.
        Timezone     string
    }

    // Priority/QoS
    Priority int // 0 = low, 1 = normal, 2 = high, 3 = critical

    // Metadata
    PolicySources []string // Which policies were applied (for audit)
}

type AuthorizationDecision struct {
    Allowed        bool
    DenialReason   string
    RequiredRole   string
    RequiredPermissions []string
}
```

**AWS Integration:**

```go
// PostgreSQL query (with Redis caching)
func (r *PolicyRepository) GetPoliciesForTenant(
    ctx context.Context,
    orgID, appID, userID string,
) (*ResolvedPolicies, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("policy:%s:%s:%s", orgID, appID, userID)
    if cached := redis.Get(ctx, cacheKey); cached != nil {
        return parseCachedPolicies(cached), nil
    }

    // Fetch from PostgreSQL (hierarchy: org > app > user)
    query := `
        SELECT policy_type, config
        FROM policies
        WHERE (org_id = $1 AND app_id IS NULL AND user_id IS NULL)
           OR (org_id = $1 AND app_id = $2 AND user_id IS NULL)
           OR (org_id = $1 AND app_id = $2 AND user_id = $3)
        ORDER BY
            CASE
                WHEN user_id IS NOT NULL THEN 3
                WHEN app_id IS NOT NULL THEN 2
                ELSE 1
            END
    `

    rows, err := r.db.QueryContext(ctx, query, orgID, appID, userID)
    // ... merge policies, cache for 5 minutes

    return policies, nil
}
```

**Policy Resolution Logic:**

```
1. Fetch org-level policies (baseline)
2. Fetch app-level policies (override org)
3. Fetch user-level policies (override app)
4. Merge policies (most restrictive wins for limits)
5. Cache merged policies in Redis (TTL: 5 minutes)
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `AUTHZ_DENIED` | 403 | User role lacks permission | No |
| `AUTHZ_MODEL_BLOCKED` | 403 | Requested model not allowed | No |
| `AUTHZ_PROVIDER_BLOCKED` | 403 | Requested provider not allowed | No |
| `AUTHZ_TIME_RESTRICTED` | 403 | Request outside allowed hours | Yes (retry later) |
| `POLICY_FETCH_FAILED` | 503 | Database/cache error | Yes (retry) |

**Boundary:**

- **IN:** Tenant context + normalized request
- **OUT:** Resolved policies + authorization decision
- **Does NOT:** Enforce quotas (that's Stage 6)
- **Does NOT:** Modify request (that's Stage 7-8)

---

### STAGE 5: CACHE CHECK

**AWS Services:** ElastiCache Redis

**Responsibilities:**

- Generate cache key from request parameters
- Check if identical request has been cached
- Return cached response if valid (TTL not expired)
- Skip to Stage 15 if cache hit

**Entry Contract:**

```go
type Stage5Input struct {
    TenantContext     *TenantContext
    NormalizedRequest *NormalizedRequest
}
```

**Exit Contract:**

```go
type Stage5Output struct {
    CacheResult *CacheResult
    Error       *StageError
}

type CacheResult struct {
    Hit            bool
    CachedResponse *InspectedResponse // If hit
    CacheKey       string
    TTL            time.Duration
}
```

**Cache Key Generation:**

```go
func GenerateCacheKey(tenant *TenantContext, req *NormalizedRequest) string {
    // Include all parameters that affect response
    keyData := struct {
        OrgID       string
        AppID       string
        Model       string
        Messages    []Message
        Temperature float64
        MaxTokens   int
    }{
        OrgID:       tenant.OrganizationID,
        AppID:       tenant.ApplicationID,
        Model:       req.Model,
        Messages:    req.Messages,
        Temperature: req.Parameters.Temperature,
        MaxTokens:   req.Parameters.MaxTokens,
    }

    // Hash to 64-character key
    hash := sha256.Sum256([]byte(json.Marshal(keyData)))
    return fmt.Sprintf("cache:response:%x", hash)
}
```

**AWS Integration:**

```go
// Redis GET
cacheKey := GenerateCacheKey(tenant, request)
cachedData, err := redis.Get(ctx, cacheKey)
if err == redis.Nil {
    // Cache miss - continue pipeline
    return &CacheResult{Hit: false}, nil
} else if err != nil {
    // Redis error - continue pipeline (log error)
    logError("cache_check_failed", err)
    return &CacheResult{Hit: false}, nil
}

// Cache hit - deserialize and return
var cachedResponse InspectedResponse
json.Unmarshal(cachedData, &cachedResponse)
return &CacheResult{
    Hit:            true,
    CachedResponse: &cachedResponse,
}, nil
```

**Cache Policy:**

```
TTL: 1 hour (configurable per org)
Eviction: LRU (least recently used)
Max memory: 2GB per Redis node
Invalidation: On policy change (via pub/sub)
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `CACHE_READ_ERROR` | (none) | Redis connection failed | Yes (continue without cache) |

**Boundary:**

- **IN:** Normalized request
- **OUT:** Cached response OR cache miss
- **Does NOT:** Call LLM if cache hit
- **Does NOT:** Validate cache contents (assume valid)

---

### STAGE 6: BUDGET & RATE PRE-CHECK

**AWS Services:**

- ElastiCache Redis (rate limiting counters, sliding window)
- Aurora PostgreSQL (cost tracking, budget queries)

**Responsibilities:**

- Check rate limits (requests per minute/hour/day)
- Check token limits
- Estimate request cost
- Check if request would exceed budget
- Increment rate limit counters (pre-commit)

**Entry Contract:**

```go
type Stage6Input struct {
    TenantContext     *TenantContext
    ResolvedPolicies  *ResolvedPolicies
    NormalizedRequest *NormalizedRequest
    IntentMetadata    *IntentMetadata
}
```

**Exit Contract:**

```go
type Stage6Output struct {
    QuotaCheck *QuotaCheckResult
    Error      *StageError
}

type QuotaCheckResult struct {
    // Rate limiting
    RateLimitStatus struct {
        RequestsRemaining int
        TokensRemaining   int
        ResetAt           time.Time
        WindowType        string // "minute", "hour", "day"
    }

    // Cost estimation
    CostEstimate struct {
        EstimatedCostUSD  float64
        CurrentMonthSpend float64
        RemainingBudget   float64
        WouldExceedBudget bool
    }

    // Quota decisions
    Allowed          bool
    DenialReason     string
    RetryAfter       *time.Duration
}
```

**AWS Integration (Rate Limiting with Redis):**

```go
// Sliding window rate limiting using Redis ZSET
func (s *RateLimitService) CheckRateLimit(
    ctx context.Context,
    orgID, appID string,
    limit int,
    window time.Duration,
) (bool, int, error) {
    key := fmt.Sprintf("rate_limit:%s:%s:%s", orgID, appID, window.String())
    now := time.Now().Unix()
    windowStart := now - int64(window.Seconds())

    // Redis pipeline for atomicity
    pipe := s.redis.Pipeline()

    // Remove old entries outside window
    pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))

    // Add current request
    requestID := fmt.Sprintf("%d-%s", now, uuid.New().String())
    pipe.ZAdd(ctx, key, &redis.Z{
        Score:  float64(now),
        Member: requestID,
    })

    // Count requests in window
    countCmd := pipe.ZCount(ctx, key, strconv.FormatInt(windowStart, 10), "+inf")

    // Set TTL
    pipe.Expire(ctx, key, window)

    _, err := pipe.Exec(ctx)
    if err != nil {
        return false, 0, err
    }

    count, _ := countCmd.Result()
    remaining := limit - int(count)

    return count <= int64(limit), remaining, nil
}
```

**AWS Integration (Cost Tracking with Aurora):**

```go
func (s *BudgetService) CheckBudget(
    ctx context.Context,
    orgID string,
    estimatedCost float64,
) (bool, float64, error) {
    // Query current month spending
    query := `
        SELECT COALESCE(SUM(cost_usd), 0) as month_spend
        FROM request_logs
        WHERE org_id = $1
          AND timestamp >= date_trunc('month', NOW())
    `

    var monthSpend float64
    err := s.db.QueryRowContext(ctx, query, orgID).Scan(&monthSpend)
    if err != nil {
        return false, 0, err
    }

    // Get monthly budget from policies
    budget := s.policies.CostCaps.MonthlyBudgetUSD

    // Check if request would exceed budget
    projectedSpend := monthSpend + estimatedCost
    allowed := projectedSpend <= budget
    remaining := budget - monthSpend

    return allowed, remaining, nil
}
```

**Cost Estimation:**

```go
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
    pricing := map[string]struct{ Input, Output float64 }{
        "gpt-4":         {Input: 30.0, Output: 60.0},   // per 1M tokens
        "gpt-4-turbo":   {Input: 10.0, Output: 30.0},
        "gpt-3.5-turbo": {Input: 0.5, Output: 1.5},
        "claude-3-opus": {Input: 15.0, Output: 75.0},
    }

    price := pricing[model]
    inputCost := float64(inputTokens) / 1_000_000 * price.Input
    outputCost := float64(outputTokens) / 1_000_000 * price.Output

    return inputCost + outputCost
}
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `QUOTA_RATE_LIMIT_EXCEEDED` | 429 | Too many requests | Yes (retry after window) |
| `QUOTA_TOKEN_LIMIT_EXCEEDED` | 429 | Too many tokens | Yes (reduce input) |
| `QUOTA_BUDGET_EXCEEDED` | 402 | Monthly budget exhausted | No (until next month) |
| `QUOTA_SERVICE_ERROR` | 503 | Redis/DB connection failed | Yes (retry) |

**Boundary:**

- **IN:** Tenant context + policies + request
- **OUT:** Quota check result (allowed/denied)
- **Does NOT:** Charge actual cost (that's Stage 14)
- **Does NOT:** Call LLM (that's Stage 11)

---

### STAGE 7: PRE-PROCESSING VALIDATION

**AWS Services:**

- None (in-memory regex)
- Optional: Amazon Comprehend (for ML-based PII detection)

**Responsibilities:**

- Detect PII (names, emails, SSNs, phone numbers, addresses)
- Detect secrets (API keys, passwords, connection strings)
- Detect prompt injection patterns
- Redact or block based on policy
- Generate validation report

**Entry Contract:**

```go
type Stage7Input struct {
    NormalizedRequest *NormalizedRequest
    ResolvedPolicies  *ResolvedPolicies
}
```

**Exit Contract:**

```go
type Stage7Output struct {
    ValidationResult *ValidationResult
    RedactedPrompt   *NormalizedRequest // Redacted version
    Error            *StageError
}

type ValidationResult struct {
    Valid            bool
    Violations       []Violation
    Action           string // "allow", "redact", "block"
    ConfidenceScore  float64
}

type Violation struct {
    Type         string   // "pii", "secret", "injection"
    Subtype      string   // "email", "ssn", "api_key", "sql_injection"
    Value        string   // Detected value (may be partially masked)
    Position     int      // Character position in prompt
    Severity     string   // "low", "medium", "high", "critical"
    Confidence   float64  // 0.0-1.0
    Remediation  string   // "redact", "block", "warn"
}
```

**Detection Patterns:**

```go
var piiPatterns = map[string]*regexp.Regexp{
    "email":        regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
    "phone_us":     regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`),
    "ssn":          regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
    "credit_card":  regexp.MustCompile(`\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`),
    "ip_address":   regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
    "address":      regexp.MustCompile(`\b\d+\s+[A-Za-z]+\s+(Street|St|Avenue|Ave|Road|Rd|Boulevard|Blvd)\b`),
}

var secretPatterns = map[string]*regexp.Regexp{
    "api_key":      regexp.MustCompile(`\b[A-Za-z0-9]{32,}\b`),
    "aws_key":      regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
    "github_token": regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
    "password":     regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*\S+`),
    "jwt":          regexp.MustCompile(`eyJ[A-Za-z0-9-_]+\.eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+`),
}

var injectionPatterns = map[string]*regexp.Regexp{
    "sql_injection":    regexp.MustCompile(`(?i)(union\s+select|drop\s+table|delete\s+from)`),
    "command_injection": regexp.MustCompile(`[;&|]\s*(rm|cat|wget|curl)`),
    "prompt_jailbreak": regexp.MustCompile(`(?i)(ignore\s+previous|disregard\s+all|system\s+prompt)`),
}
```

**Validation Logic:**

```go
func ValidatePrompt(prompt string, policies *ResolvedPolicies) *ValidationResult {
    violations := []Violation{}

    // Check PII
    if policies.ContentPolicies.BlockPII {
        for piiType, pattern := range piiPatterns {
            matches := pattern.FindAllString(prompt, -1)
            for _, match := range matches {
                violations = append(violations, Violation{
                    Type:     "pii",
                    Subtype:  piiType,
                    Value:    maskValue(match),
                    Severity: "high",
                    Remediation: "redact",
                })
            }
        }
    }

    // Check secrets
    if policies.ContentPolicies.BlockSecrets {
        for secretType, pattern := range secretPatterns {
            matches := pattern.FindAllString(prompt, -1)
            for _, match := range matches {
                violations = append(violations, Violation{
                    Type:     "secret",
                    Subtype:  secretType,
                    Value:    maskValue(match),
                    Severity: "critical",
                    Remediation: "block",
                })
            }
        }
    }

    // Check prompt injection
    if policies.ContentPolicies.BlockPromptInjection {
        for injectionType, pattern := range injectionPatterns {
            matches := pattern.FindAllString(prompt, -1)
            for _, match := range matches {
                violations = append(violations, Violation{
                    Type:     "injection",
                    Subtype:  injectionType,
                    Value:    maskValue(match),
                    Severity: "critical",
                    Remediation: "block",
                })
            }
        }
    }

    // Determine action
    action := "allow"
    if hasBlockableViolation(violations) {
        action = "block"
    } else if hasRedactableViolation(violations) {
        action = "redact"
    }

    return &ValidationResult{
        Valid:      len(violations) == 0 || action == "redact",
        Violations: violations,
        Action:     action,
    }
}
```

**Redaction:**

```go
func RedactPrompt(prompt string, violations []Violation) string {
    redacted := prompt
    for _, v := range violations {
        if v.Remediation == "redact" {
            redacted = strings.ReplaceAll(redacted, v.Value, "[REDACTED]")
        }
    }
    return redacted
}
```

**Optional: AWS Comprehend Integration:**

```go
func DetectPIIWithComprehend(ctx context.Context, text string) ([]Violation, error) {
    client := comprehend.NewFromConfig(cfg)

    result, err := client.DetectPiiEntities(ctx, &comprehend.DetectPiiEntitiesInput{
        Text:         aws.String(text),
        LanguageCode: types.LanguageCodeEn,
    })
    if err != nil {
        return nil, err
    }

    violations := []Violation{}
    for _, entity := range result.Entities {
        violations = append(violations, Violation{
            Type:       "pii",
            Subtype:    string(entity.Type),
            Confidence: float64(*entity.Score),
            Severity:   "high",
        })
    }

    return violations, nil
}
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `VALIDATE_PII_DETECTED` | 400 | PII found and blocking enabled | No |
| `VALIDATE_SECRET_DETECTED` | 400 | Secret found and blocking enabled | No |
| `VALIDATE_INJECTION_DETECTED` | 400 | Prompt injection detected | No |
| `VALIDATE_PROMPT_TOO_LONG` | 400 | Exceeds max prompt length | No |

**Boundary:**

- **IN:** Normalized request + policies
- **OUT:** Validation result + redacted prompt
- **Does NOT:** Apply business rules (that's Stage 8)
- **Does NOT:** Modify prompt beyond redaction

---

### STAGE 8: POLICY-DRIVEN CONTENT FILTERING

**AWS Services:** None (in-memory policy evaluation)

**Responsibilities:**

- Apply business-specific content policies
- Filter blocked topics/keywords
- Enforce allowed use cases
- Apply time-based restrictions
- Generate filtered request

**Entry Contract:**

```go
type Stage8Input struct {
    ValidationResult  *ValidationResult
    RedactedPrompt    *NormalizedRequest
    ResolvedPolicies  *ResolvedPolicies
}
```

**Exit Contract:**

```go
type Stage8Output struct {
    FilteredRequest *FilteredRequest
    Error           *StageError
}

type FilteredRequest struct {
    // Final request to send to LLM
    Request          *NormalizedRequest
    FilterActions    []FilterAction
    PolicyViolations []string
}

type FilterAction struct {
    Action      string // "allow", "modify", "block"
    PolicyRule  string // Which policy triggered this
    Reason      string
}
```

**Filtering Logic:**

```go
func ApplyContentFilters(
    request *NormalizedRequest,
    policies *ResolvedPolicies,
) (*FilteredRequest, error) {
    actions := []FilterAction{}

    // Check allowed topics
    if len(policies.ContentPolicies.AllowedTopics) > 0 {
        if !containsAllowedTopic(request, policies.ContentPolicies.AllowedTopics) {
            return nil, &StageError{
                Code:    "FILTER_TOPIC_NOT_ALLOWED",
                Message: "Request topic not in allowed list",
            }
        }
    }

    // Check blocked topics
    if len(policies.ContentPolicies.BlockedTopics) > 0 {
        if containsBlockedTopic(request, policies.ContentPolicies.BlockedTopics) {
            return nil, &StageError{
                Code:    "FILTER_TOPIC_BLOCKED",
                Message: "Request contains blocked topic",
            }
        }
    }

    // Check time restrictions
    if !isWithinAllowedTime(policies.TimeRestrictions) {
        return nil, &StageError{
            Code:    "FILTER_TIME_RESTRICTED",
            Message: "Requests not allowed during this time",
        }
    }

    return &FilteredRequest{
        Request:       request,
        FilterActions: actions,
    }, nil
}
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `FILTER_TOPIC_BLOCKED` | 403 | Request contains blocked topic | No |
| `FILTER_TOPIC_NOT_ALLOWED` | 403 | Topic not in allowed list | No |
| `FILTER_TIME_RESTRICTED` | 403 | Outside allowed time window | Yes (retry later) |

**Boundary:**

- **IN:** Validated, redacted request + policies
- **OUT:** Filtered request ready for LLM
- **Does NOT:** Call LLM (that's Stage 11)
- **Does NOT:** Modify response (that's Stage 12)

---

### STAGE 9: PROVIDER ROUTING

**AWS Services:**

- Secrets Manager (provider credentials)
- None (in-memory routing logic)

**Responsibilities:**

- Select appropriate LLM provider based on model request
- Apply routing policies (cost optimization, latency, availability)
- Fetch provider credentials from Secrets Manager
- Prepare provider-specific request format
- Select fallback providers

**Entry Contract:**

```go
type Stage9Input struct {
    FilteredRequest  *FilteredRequest
    ResolvedPolicies *ResolvedPolicies
    IntentMetadata   *IntentMetadata
}
```

**Exit Contract:**

```go
type Stage9Output struct {
    SelectedProvider *ProviderSelection
    Error            *StageError
}

type ProviderSelection struct {
    ProviderName     string   // "openai", "anthropic", "azure"
    Model            string   // Actual model name
    Endpoint         string   // API endpoint
    Credentials      *ProviderCredentials
    FallbackProviders []string // Ordered list of fallbacks
    RoutingReason    string   // Why this provider was selected
}

type ProviderCredentials struct {
    APIKey      string
    OrgID       string // For OpenAI org-specific keys
    ProjectID   string // For Anthropic/Azure projects
    Endpoint    string // For Azure custom endpoints
    Region      string // For regional routing
}
```

**Routing Logic:**

```go
func SelectProvider(
    request *FilteredRequest,
    policies *ResolvedPolicies,
) (*ProviderSelection, error) {
    // 1. Determine which providers support the requested model
    candidates := []string{}
    for _, provider := range []string{"openai", "anthropic", "azure"} {
        if supportsModel(provider, request.Request.Model) {
            if !isProviderBlocked(provider, policies) {
                candidates = append(candidates, provider)
            }
        }
    }

    if len(candidates) == 0 {
        return nil, &StageError{
            Code:    "ROUTE_NO_PROVIDER",
            Message: fmt.Sprintf("No provider supports model: %s", request.Request.Model),
        }
    }

    // 2. Apply routing strategy
    var selected string
    switch getRoutingStrategy(policies) {
    case "cost-optimized":
        selected = selectCheapest(candidates, request.Request.Model)
    case "latency-optimized":
        selected = selectFastest(candidates)
    case "balanced":
        selected = selectBalanced(candidates, request.Request.Model)
    default:
        selected = candidates[0] // First available
    }

    // 3. Fetch credentials from Secrets Manager
    creds, err := fetchProviderCredentials(selected)
    if err != nil {
        return nil, err
    }

    // 4. Prepare fallbacks
    fallbacks := []string{}
    for _, c := range candidates {
        if c != selected {
            fallbacks = append(fallbacks, c)
        }
    }

    return &ProviderSelection{
        ProviderName:      selected,
        Model:             request.Request.Model,
        Endpoint:          getProviderEndpoint(selected),
        Credentials:       creds,
        FallbackProviders: fallbacks,
        RoutingReason:     fmt.Sprintf("Selected %s (strategy: %s)", selected, getRoutingStrategy(policies)),
    }, nil
}
```

**AWS Integration (Secrets Manager):**

```go
func fetchProviderCredentials(provider string) (*ProviderCredentials, error) {
    client := secretsmanager.NewFromConfig(cfg)

    secretName := fmt.Sprintf("/llm-cp/%s/providers", os.Getenv("ENVIRONMENT"))
    result, err := client.GetSecretValue(context.Background(), &secretsmanager.GetSecretValueInput{
        SecretId: aws.String(secretName),
    })
    if err != nil {
        return nil, err
    }

    // Parse JSON secret
    var secrets map[string]interface{}
    json.Unmarshal([]byte(*result.SecretString), &secrets)

    switch provider {
    case "openai":
        return &ProviderCredentials{
            APIKey: secrets["openai_api_key"].(string),
        }, nil
    case "anthropic":
        return &ProviderCredentials{
            APIKey: secrets["anthropic_api_key"].(string),
        }, nil
    case "azure":
        return &ProviderCredentials{
            APIKey:   secrets["azure_openai_key"].(string),
            Endpoint: secrets["azure_openai_endpoint"].(string),
        }, nil
    }

    return nil, fmt.Errorf("unknown provider: %s", provider)
}
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `ROUTE_NO_PROVIDER` | 400 | No provider supports requested model | No |
| `ROUTE_ALL_BLOCKED` | 403 | All providers blocked by policy | No |
| `ROUTE_CREDS_FETCH_FAILED` | 503 | Cannot fetch credentials | Yes (retry) |

**Boundary:**

- **IN:** Filtered request + policies
- **OUT:** Selected provider + credentials
- **Does NOT:** Call LLM (that's Stage 11)
- **Does NOT:** Handle provider failures (that's Stage 10)

---

### STAGE 10: CIRCUIT BREAKER

**AWS Services:** ElastiCache Redis (circuit breaker state)

**Responsibilities:**

- Check provider health status
- Implement circuit breaker pattern (Open/Half-Open/Closed)
- Select fallback provider if primary is unhealthy
- Update provider health metrics

**Entry Contract:**

```go
type Stage10Input struct {
    SelectedProvider *ProviderSelection
}
```

**Exit Contract:**

```go
type Stage10Output struct {
    HealthyProvider *ProviderSelection
    CircuitState    string // "closed", "open", "half-open"
    Error           *StageError
}

type ProviderHealthStatus struct {
    ProviderName    string
    CircuitState    string
    FailureCount    int
    LastFailureTime time.Time
    LastSuccessTime time.Time
}
```

**Circuit Breaker Logic:**

```go
// Circuit breaker state machine
type CircuitState int

const (
    Closed CircuitState = iota // Normal operation
    Open                        // Provider failing, block requests
    HalfOpen                    // Testing if provider recovered
)

func (cb *CircuitBreaker) Allow(provider string) (bool, string) {
    state := cb.GetState(provider)

    switch state {
    case Closed:
        // Normal operation, allow request
        return true, "closed"

    case Open:
        // Check if timeout expired to try again
        if time.Since(cb.GetLastFailure(provider)) > cb.OpenTimeout {
            cb.SetState(provider, HalfOpen)
            return true, "half-open"
        }
        // Still open, deny request
        return false, "open"

    case HalfOpen:
        // Allow limited requests to test recovery
        return true, "half-open"
    }

    return false, "unknown"
}

func (cb *CircuitBreaker) RecordSuccess(provider string) {
    state := cb.GetState(provider)
    if state == HalfOpen {
        // Provider recovered, close circuit
        cb.SetState(provider, Closed)
        cb.ResetFailureCount(provider)
    }
    cb.RecordLastSuccess(provider, time.Now())
}

func (cb *CircuitBreaker) RecordFailure(provider string) {
    cb.IncrementFailureCount(provider)
    cb.RecordLastFailure(provider, time.Now())

    if cb.GetFailureCount(provider) >= cb.FailureThreshold {
        // Open circuit after N consecutive failures
        cb.SetState(provider, Open)
    }
}
```

**AWS Integration (Redis):**

```go
// Store circuit breaker state in Redis
type RedisCircuitBreaker struct {
    redis *redis.Client
}

func (cb *RedisCircuitBreaker) GetState(provider string) CircuitState {
    key := fmt.Sprintf("circuit:%s:state", provider)
    val, err := cb.redis.Get(context.Background(), key).Result()
    if err == redis.Nil {
        return Closed // Default to closed
    }

    switch val {
    case "open":
        return Open
    case "half-open":
        return HalfOpen
    default:
        return Closed
    }
}

func (cb *RedisCircuitBreaker) SetState(provider string, state CircuitState) {
    key := fmt.Sprintf("circuit:%s:state", provider)
    var val string
    switch state {
    case Open:
        val = "open"
    case HalfOpen:
        val = "half-open"
    default:
        val = "closed"
    }
    cb.redis.Set(context.Background(), key, val, 5*time.Minute)
}

func (cb *RedisCircuitBreaker) GetFailureCount(provider string) int {
    key := fmt.Sprintf("circuit:%s:failures", provider)
    count, _ := cb.redis.Get(context.Background(), key).Int()
    return count
}

func (cb *RedisCircuitBreaker) IncrementFailureCount(provider string) {
    key := fmt.Sprintf("circuit:%s:failures", provider)
    cb.redis.Incr(context.Background(), key)
    cb.redis.Expire(context.Background(), key, 5*time.Minute)
}
```

**Fallback Selection:**

```go
func SelectFallbackProvider(
    selected *ProviderSelection,
    cb *CircuitBreaker,
) (*ProviderSelection, error) {
    // Try fallback providers in order
    for _, fallback := range selected.FallbackProviders {
        allowed, _ := cb.Allow(fallback)
        if allowed {
            creds, err := fetchProviderCredentials(fallback)
            if err != nil {
                continue
            }

            return &ProviderSelection{
                ProviderName: fallback,
                Model:        selected.Model,
                Endpoint:     getProviderEndpoint(fallback),
                Credentials:  creds,
                RoutingReason: fmt.Sprintf("Fallback from %s (circuit open)", selected.ProviderName),
            }, nil
        }
    }

    return nil, &StageError{
        Code:    "CIRCUIT_ALL_OPEN",
        Message: "All providers are currently unavailable",
    }
}
```

**Configuration:**

```
FailureThreshold: 5    # Open circuit after 5 failures
OpenTimeout: 30s       # Try half-open after 30 seconds
HalfOpenRequests: 3    # Allow 3 test requests in half-open
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `CIRCUIT_ALL_OPEN` | 503 | All providers unhealthy | Yes (retry after timeout) |
| `CIRCUIT_STATE_ERROR` | 503 | Cannot read circuit state | Yes (retry) |

**Boundary:**

- **IN:** Selected provider
- **OUT:** Healthy provider (may be fallback)
- **Does NOT:** Call LLM (that's Stage 11)
- **Does NOT:** Retry failed requests (that's caller's responsibility)

---

### STAGE 11: LLM INVOCATION

**AWS Services:**

- Lambda (execution context)
- VPC NAT Gateway (outbound internet access)
- CloudWatch (logs)

**External Services:** OpenAI, Anthropic, Azure OpenAI APIs

**Responsibilities:**

- Call LLM provider API with filtered request
- Handle API-specific request/response formats
- Measure latency and token usage
- Handle provider errors
- Update circuit breaker on success/failure

**Entry Contract:**

```go
type Stage11Input struct {
    FilteredRequest *FilteredRequest
    HealthyProvider *ProviderSelection
}
```

**Exit Contract:**

```go
type Stage11Output struct {
    RawLLMResponse *LLMResponse
    UsageMetrics   *UsageMetrics
    Error          *StageError
}

type LLMResponse struct {
    ID               string
    Object           string
    Created          int64
    Model            string
    Choices          []Choice
    Usage            Usage
    ProviderMetadata map[string]interface{}
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

type UsageMetrics struct {
    Provider       string
    Model          string
    Latency        time.Duration
    InputTokens    int
    OutputTokens   int
    TotalTokens    int
    EstimatedCost  float64
    RetryCount     int
    ProviderStatus int // HTTP status code
}
```

**Provider Adapters:**

```go
// Common interface for all providers
type LLMProvider interface {
    Name() string
    ChatCompletion(ctx context.Context, req *FilteredRequest) (*LLMResponse, error)
    IsAvailable(ctx context.Context) bool
}

// OpenAI implementation
type OpenAIProvider struct {
    client *openai.Client
    apiKey string
}

func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req *FilteredRequest) (*LLMResponse, error) {
    start := time.Now()

    // Convert to OpenAI format
    messages := make([]openai.ChatCompletionMessage, len(req.Request.Messages))
    for i, msg := range req.Request.Messages {
        messages[i] = openai.ChatCompletionMessage{
            Role:    msg.Role,
            Content: msg.Content,
        }
    }

    // Call OpenAI API
    resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:       req.Request.Model,
        Messages:    messages,
        MaxTokens:   req.Request.Parameters.MaxTokens,
        Temperature: float32(req.Request.Parameters.Temperature),
        TopP:        float32(req.Request.Parameters.TopP),
        Stream:      req.Request.Parameters.Stream,
    })
    if err != nil {
        return nil, err
    }

    // Convert to unified format
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

    return &LLMResponse{
        ID:      resp.ID,
        Object:  resp.Object,
        Created: resp.Created,
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
```

**Retry Logic:**

```go
func InvokeWithRetry(
    ctx context.Context,
    provider LLMProvider,
    request *FilteredRequest,
    maxRetries int,
) (*LLMResponse, error) {
    var lastErr error

    for attempt := 0; attempt <= maxRetries; attempt++ {
        if attempt > 0 {
            // Exponential backoff
            backoff := time.Duration(1<<uint(attempt-1)) * time.Second
            time.Sleep(backoff)
        }

        resp, err := provider.ChatCompletion(ctx, request)
        if err == nil {
            return resp, nil
        }

        // Check if error is retryable
        if !isRetryableError(err) {
            return nil, err
        }

        lastErr = err
    }

    return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func isRetryableError(err error) bool {
    // Retry on rate limits, timeouts, 5xx errors
    if strings.Contains(err.Error(), "rate limit") {
        return true
    }
    if strings.Contains(err.Error(), "timeout") {
        return true
    }
    if strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), "503") {
        return true
    }
    return false
}
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `LLM_TIMEOUT` | 504 | Provider API timeout | Yes (retry) |
| `LLM_RATE_LIMITED` | 429 | Provider rate limit | Yes (retry with backoff) |
| `LLM_INVALID_REQUEST` | 400 | Malformed request to provider | No |
| `LLM_AUTH_FAILED` | 401 | Invalid API key | No |
| `LLM_MODEL_NOT_FOUND` | 404 | Model does not exist | No |
| `LLM_PROVIDER_ERROR` | 502 | Provider returned 5xx | Yes (retry or fallback) |
| `LLM_CONTENT_FILTERED` | 451 | Provider content filter triggered | No |

**Boundary:**

- **IN:** Filtered request + healthy provider
- **OUT:** Raw LLM response + metrics
- **Does NOT:** Validate response content (that's Stage 12)
- **Does NOT:** Store audit logs (that's Stage 14)

---

### STAGE 12: RESPONSE INSPECTION

**AWS Services:**

- None (in-memory validation)
- Optional: Amazon Comprehend (ML-based content moderation)

**Responsibilities:**

- Validate response structure
- Detect PII in response
- Detect harmful content (violence, illegal advice)
- Apply output filters based on policies
- Redact sensitive information
- Generate inspection report

**Entry Contract:**

```go
type Stage12Input struct {
    RawLLMResponse   *LLMResponse
    ResolvedPolicies *ResolvedPolicies
}
```

**Exit Contract:**

```go
type Stage12Output struct {
    InspectedResponse *InspectedResponse
    ValidationReport  *ResponseValidationReport
    Error             *StageError
}

type InspectedResponse struct {
    // Final response to return to client
    Response         *LLMResponse
    FilterActions    []string
    RedactionApplied bool
}

type ResponseValidationReport struct {
    Valid           bool
    Violations      []Violation
    Action          string // "allow", "redact", "block"
    ContentSafe     bool
    PIIDetected     bool
    HarmfulContent  bool
}
```

**Inspection Logic:**

```go
func InspectResponse(
    response *LLMResponse,
    policies *ResolvedPolicies,
) (*InspectedResponse, *ResponseValidationReport, error) {
    report := &ResponseValidationReport{
        Valid:       true,
        Violations:  []Violation{},
        ContentSafe: true,
    }

    // Extract response text
    responseText := extractResponseText(response)

    // Check for PII in response
    if policies.ContentPolicies.BlockPII {
        piiViolations := detectPII(responseText)
        if len(piiViolations) > 0 {
            report.PIIDetected = true
            report.Violations = append(report.Violations, piiViolations...)
        }
    }

    // Check for harmful content
    harmfulViolations := detectHarmfulContent(responseText)
    if len(harmfulViolations) > 0 {
        report.HarmfulContent = true
        report.Violations = append(report.Violations, harmfulViolations...)
    }

    // Determine action
    if report.HarmfulContent {
        report.Action = "block"
        report.Valid = false
        return nil, report, &StageError{
            Code:    "INSPECT_HARMFUL_CONTENT",
            Message: "Response contains harmful content",
        }
    } else if report.PIIDetected {
        report.Action = "redact"
        response = redactPIIFromResponse(response)
    } else {
        report.Action = "allow"
    }

    return &InspectedResponse{
        Response:         response,
        RedactionApplied: report.Action == "redact",
    }, report, nil
}
```

**Harmful Content Detection:**

```go
var harmfulPatterns = map[string]*regexp.Regexp{
    "violence":        regexp.MustCompile(`(?i)(kill|murder|assault|attack|hurt|harm)`),
    "illegal_drugs":   regexp.MustCompile(`(?i)(cocaine|heroin|methamphetamine|how to make)`),
    "self_harm":       regexp.MustCompile(`(?i)(suicide|self-harm|cutting|overdose)`),
    "hate_speech":     regexp.MustCompile(`(?i)(racial slur|hate speech patterns)`),
}

func detectHarmfulContent(text string) []Violation {
    violations := []Violation{}

    for category, pattern := range harmfulPatterns {
        if pattern.MatchString(text) {
            violations = append(violations, Violation{
                Type:     "harmful_content",
                Subtype:  category,
                Severity: "critical",
                Remediation: "block",
            })
        }
    }

    return violations
}
```

**Optional: AWS Comprehend Content Moderation:**

```go
func ModerateContentWithComprehend(ctx context.Context, text string) ([]Violation, error) {
    client := comprehend.NewFromConfig(cfg)

    result, err := client.DetectToxicContent(ctx, &comprehend.DetectToxicContentInput{
        TextSegments: []types.TextSegment{
            {Text: aws.String(text)},
        },
        LanguageCode: types.LanguageCodeEn,
    })
    if err != nil {
        return nil, err
    }

    violations := []Violation{}
    for _, segment := range result.ResultList {
        for _, label := range segment.Labels {
            if *label.Score > 0.7 { // High confidence
                violations = append(violations, Violation{
                    Type:       "harmful_content",
                    Subtype:    string(label.Name),
                    Confidence: float64(*label.Score),
                    Severity:   "high",
                    Remediation: "block",
                })
            }
        }
    }

    return violations, nil
}
```

**Error Scenarios:**
| Error Code | HTTP Status | Reason | Recoverable |
|------------|-------------|--------|-------------|
| `INSPECT_HARMFUL_CONTENT` | 451 | Response contains harmful content | No |
| `INSPECT_POLICY_VIOLATION` | 451 | Response violates content policy | No |
| `INSPECT_SERVICE_ERROR` | 503 | Comprehend API failed | Yes (allow response) |

**Boundary:**

- **IN:** Raw LLM response + policies
- **OUT:** Inspected, filtered response
- **Does NOT:** Cache response (that's Stage 13)
- **Does NOT:** Log audit trail (that's Stage 14)

---

### STAGE 13: CACHE STORE (ASYNC)

**AWS Services:**

- ElastiCache Redis (response cache)
- SQS (async queue for cache writes)

**Responsibilities:**

- Store inspected response in cache
- Set appropriate TTL based on request type
- Handle cache write failures gracefully (non-blocking)
- Invalidate cache on policy changes

**Entry Contract:**

```go
type Stage13Input struct {
    NormalizedRequest *NormalizedRequest
    InspectedResponse *InspectedResponse
    TenantContext     *TenantContext
}
```

**Exit Contract:**

```go
type Stage13Output struct {
    CacheStored bool
    Error       *StageError // Non-blocking, logged but not returned
}
```

**Async Cache Store:**

```go
func StoreCacheAsync(
    ctx context.Context,
    request *NormalizedRequest,
    response *InspectedResponse,
    tenant *TenantContext,
) {
    // Run in goroutine (non-blocking)
    go func() {
        cacheKey := GenerateCacheKey(tenant, request)

        // Serialize response
        data, err := json.Marshal(response.Response)
        if err != nil {
            logError("cache_serialize_failed", err)
            return
        }

        // Store in Redis with TTL
        ttl := determineCacheTTL(request)
        err = redis.Set(context.Background(), cacheKey, data, ttl)
        if err != nil {
            logError("cache_store_failed", err)
            return
        }

        logInfo("cache_stored", map[string]interface{}{
            "cache_key": cacheKey,
            "ttl":       ttl,
        })
    }()
}

func determineCacheTTL(request *NormalizedRequest) time.Duration {
    // Shorter TTL for high-temperature (creative) requests
    if request.Parameters.Temperature > 0.7 {
        return 5 * time.Minute
    }

    // Longer TTL for deterministic requests
    if request.Parameters.Temperature == 0 {
        return 24 * time.Hour
    }

    // Default TTL
    return 1 * time.Hour
}
```

**Cache Invalidation:**

```go
// Invalidate cache on policy changes (via Redis pub/sub)
func InvalidateCacheOnPolicyChange(orgID string) {
    pattern := fmt.Sprintf("cache:response:*:%s:*", orgID)

    // Use Redis SCAN to find all matching keys
    iter := redis.Scan(context.Background(), 0, pattern, 100)
    for iter.Next(context.Background()) {
        redis.Del(context.Background(), iter.Val())
    }
}
```

**Boundary:**

- **IN:** Request + response + tenant context
- **OUT:** Cache stored confirmation (async)
- **Does NOT:** Block response delivery
- **Does NOT:** Retry on failure (best-effort)

---

### STAGE 14: AUDIT, METRICS & TRACE CORRELATION

**AWS Services:**

- Aurora PostgreSQL (audit logs - 30 days hot storage)
- S3 (long-term archive via EventBridge)
- CloudWatch (metrics, logs)
- SQS (async audit queue)
- EventBridge (scheduled archival to S3)

**Responsibilities:**

- Log complete request/response with all stage decisions
- Store in PostgreSQL for 30 days
- Archive to S3 for long-term retention (7 years)
- Emit metrics to CloudWatch/Datadog
- Correlate with distributed traces
- Non-blocking (async via SQS)

**Entry Contract:**

```go
type Stage14Input struct {
    RequestContext *RequestContext // Contains all stage data
}
```

**Exit Contract:**

```go
type Stage14Output struct {
    AuditLogID string
    Logged     bool
    Error      *StageError // Non-blocking, logged but not returned
}
```

**Audit Log Structure:**

```go
type AuditLog struct {
    // Identifiers
    ID                string
    RequestID         string
    Timestamp         time.Time

    // Tenant
    OrganizationID    string
    ApplicationID     string
    UserID            string

    // Request
    Model             string
    Provider          string
    PromptRedacted    string // PII redacted
    Messages          []Message

    // Response
    ResponseRedacted  string // PII redacted
    CompletionText    string

    // Metrics
    TokensInput       int
    TokensOutput      int
    TotalTokens       int
    CostUSD           float64
    LatencyMS         int

    // Status
    Status            string // "success", "error", "blocked"
    ErrorMessage      string

    // Stage decisions
    AuthResult        string
    PolicyViolations  []string
    ValidationResult  string
    ProviderSelected  string
    CircuitState      string

    // Metadata
    ClientIP          string
    UserAgent         string
    StageTimings      map[string]int // Stage name -> duration in ms
}
```

**Async Logging via SQS:**

```go
func LogAuditAsync(ctx context.Context, rc *RequestContext) {
    // Build audit log from request context
    auditLog := buildAuditLog(rc)

    // Serialize to JSON
    data, err := json.Marshal(auditLog)
    if err != nil {
        logError("audit_serialize_failed", err)
        return
    }

    // Send to SQS queue (non-blocking)
    go func() {
        sqsClient := sqs.NewFromConfig(cfg)

        _, err := sqsClient.SendMessage(context.Background(), &sqs.SendMessageInput{
            QueueUrl:    aws.String(os.Getenv("AUDIT_QUEUE_URL")),
            MessageBody: aws.String(string(data)),
        })
        if err != nil {
            logError("audit_queue_failed", err)
            return
        }
    }()
}
```

**SQS Consumer (separate Lambda):**

```go
// Lambda function triggered by SQS
func HandleAuditMessage(ctx context.Context, event events.SQSEvent) error {
    for _, record := range event.Records {
        var auditLog AuditLog
        json.Unmarshal([]byte(record.Body), &auditLog)

        // Insert into PostgreSQL
        err := insertAuditLog(ctx, &auditLog)
        if err != nil {
            return err // Will retry via SQS
        }

        // Emit metrics to CloudWatch
        emitMetrics(ctx, &auditLog)
    }

    return nil
}

func insertAuditLog(ctx context.Context, log *AuditLog) error {
    query := `
        INSERT INTO request_logs (
            id, request_id, timestamp,
            org_id, app_id, user_id,
            model, provider,
            prompt_redacted, response_redacted,
            tokens_input, tokens_output, total_tokens,
            cost_usd, latency_ms,
            status, error_message,
            client_ip, user_agent,
            stage_timings
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
            $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
        )
    `

    _, err := db.ExecContext(ctx, query,
        log.ID, log.RequestID, log.Timestamp,
        log.OrganizationID, log.ApplicationID, log.UserID,
        log.Model, log.Provider,
        log.PromptRedacted, log.ResponseRedacted,
        log.TokensInput, log.TokensOutput, log.TotalTokens,
        log.CostUSD, log.LatencyMS,
        log.Status, log.ErrorMessage,
        log.ClientIP, log.UserAgent,
        log.StageTimings,
    )

    return err
}
```

**S3 Archival (EventBridge scheduled):**

```go
// Lambda function triggered daily by EventBridge
func ArchiveOldLogsToS3(ctx context.Context, event events.CloudWatchEvent) error {
    // Query logs older than 30 days
    query := `
        SELECT *
        FROM request_logs
        WHERE timestamp < NOW() - INTERVAL '30 days'
        ORDER BY timestamp
        LIMIT 1000
    `

    rows, err := db.QueryContext(ctx, query)
    if err != nil {
        return err
    }
    defer rows.Close()

    // Batch logs by date
    batches := make(map[string][]AuditLog)
    for rows.Next() {
        var log AuditLog
        // Scan row...

        date := log.Timestamp.Format("2006-01-02")
        batches[date] = append(batches[date], log)
    }

    // Upload to S3
    s3Client := s3.NewFromConfig(cfg)
    for date, logs := range batches {
        data, _ := json.Marshal(logs)

        key := fmt.Sprintf("audit-logs/%s/%s.json", date[:7], date)
        _, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
            Bucket: aws.String(os.Getenv("AUDIT_BUCKET")),
            Key:    aws.String(key),
            Body:   bytes.NewReader(data),
        })
        if err != nil {
            logError("s3_upload_failed", err)
            continue
        }

        // Delete from PostgreSQL
        deleteQuery := `
            DELETE FROM request_logs
            WHERE timestamp::date = $1
        `
        db.ExecContext(ctx, deleteQuery, date)
    }

    return nil
}
```

**CloudWatch Metrics:**

```go
func emitMetrics(ctx context.Context, log *AuditLog) {
    cw := cloudwatch.NewFromConfig(cfg)

    metrics := []types.MetricDatum{
        {
            MetricName: aws.String("RequestCount"),
            Value:      aws.Float64(1),
            Unit:       types.StandardUnitCount,
            Dimensions: []types.Dimension{
                {Name: aws.String("OrgID"), Value: aws.String(log.OrganizationID)},
                {Name: aws.String("Model"), Value: aws.String(log.Model)},
                {Name: aws.String("Status"), Value: aws.String(log.Status)},
            },
        },
        {
            MetricName: aws.String("TokensConsumed"),
            Value:      aws.Float64(float64(log.TotalTokens)),
            Unit:       types.StandardUnitCount,
            Dimensions: []types.Dimension{
                {Name: aws.String("OrgID"), Value: aws.String(log.OrganizationID)},
                {Name: aws.String("Model"), Value: aws.String(log.Model)},
            },
        },
        {
            MetricName: aws.String("CostUSD"),
            Value:      aws.Float64(log.CostUSD),
            Unit:       types.StandardUnitNone,
            Dimensions: []types.Dimension{
                {Name: aws.String("OrgID"), Value: aws.String(log.OrganizationID)},
            },
        },
        {
            MetricName: aws.String("Latency"),
            Value:      aws.Float64(float64(log.LatencyMS)),
            Unit:       types.StandardUnitMilliseconds,
            Dimensions: []types.Dimension{
                {Name: aws.String("Provider"), Value: aws.String(log.Provider)},
            },
        },
    }

    cw.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
        Namespace:  aws.String("LLMControlPlane"),
        MetricData: metrics,
    })
}
```

**Boundary:**

- **IN:** Complete request context (all stages)
- **OUT:** Audit logged confirmation (async)
- **Does NOT:** Block response delivery
- **Does NOT:** Retry indefinitely (SQS DLQ after 3 attempts)

---

### STAGE 15: RESPONSE DELIVERY

**AWS Services:**

- API Gateway (response formatting)
- CloudFront (if web client)

**Responsibilities:**

- Format final HTTP response
- Add response headers (rate limit info, trace IDs)
- Set appropriate HTTP status codes
- Compress response if requested
- Return to client

**Entry Contract:**

```go
type Stage15Input struct {
    InspectedResponse *InspectedResponse
    RequestContext    *RequestContext
}
```

**Exit Contract:**

```go
type Stage15Output struct {
    HTTPResponse *http.Response
}

type FinalResponse struct {
    // LLM response
    ID      string                 `json:"id"`
    Object  string                 `json:"object"`
    Created int64                  `json:"created"`
    Model   string                 `json:"model"`
    Choices []Choice               `json:"choices"`
    Usage   Usage                  `json:"usage"`

    // Governance metadata
    Metadata GovernanceMetadata `json:"metadata"`
}

type GovernanceMetadata struct {
    Provider      string  `json:"provider"`
    CostUSD       float64 `json:"cost_usd"`
    LatencyMS     int     `json:"latency_ms"`
    CacheHit      bool    `json:"cache_hit"`
    RequestID     string  `json:"request_id"`
    PolicyVersion string  `json:"policy_version"`
}
```

**Response Formatting:**

```go
func DeliverResponse(
    w http.ResponseWriter,
    r *http.Request,
    rc *RequestContext,
) {
    // Build final response
    finalResp := &FinalResponse{
        ID:      rc.LLMResponse.ID,
        Object:  rc.LLMResponse.Object,
        Created: rc.LLMResponse.Created,
        Model:   rc.LLMResponse.Model,
        Choices: rc.InspectedResponse.Response.Choices,
        Usage:   rc.LLMResponse.Usage,
        Metadata: GovernanceMetadata{
            Provider:  rc.SelectedProvider.ProviderName,
            CostUSD:   rc.UsageMetrics.EstimatedCost,
            LatencyMS: int(rc.UsageMetrics.Latency.Milliseconds()),
            CacheHit:  rc.CacheResult != nil && rc.CacheResult.Hit,
            RequestID: rc.RequestID,
        },
    }

    // Add response headers
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Request-ID", rc.RequestID)
    w.Header().Set("X-Provider", rc.SelectedProvider.ProviderName)

    // Rate limit headers
    if rc.QuotaCheck != nil {
        w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rc.ResolvedPolicies.RateLimits.RequestsPerMinute))
        w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(rc.QuotaCheck.RateLimitStatus.RequestsRemaining))
        w.Header().Set("X-RateLimit-Reset", rc.QuotaCheck.RateLimitStatus.ResetAt.Format(time.RFC3339))
    }

    // Trace correlation headers
    w.Header().Set("X-Trace-ID", getTraceID(r.Context()))

    // Write response
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(finalResp)
}
```

**Error Response Formatting:**

```go
func DeliverErrorResponse(
    w http.ResponseWriter,
    err *StageError,
    requestID string,
) {
    errorResp := map[string]interface{}{
        "error": map[string]interface{}{
            "code":       err.Code,
            "message":    err.Message,
            "details":    err.Details,
            "request_id": requestID,
        },
    }

    // Add retry-after header if applicable
    if err.RetryAfter != nil {
        w.Header().Set("Retry-After", strconv.Itoa(int(err.RetryAfter.Seconds())))
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Request-ID", requestID)
    w.WriteHeader(err.HTTPStatus)
    json.NewEncoder(w).Encode(errorResp)
}
```

**Boundary:**

- **IN:** Inspected response + request context
- **OUT:** HTTP response to client
- **Does NOT:** Modify response content
- **Does NOT:** Log (already logged in Stage 14)

---

## 4. Error Handling Contracts

### 4.1 Error Structure

```go
type StageError struct {
    Stage      string                 // Which stage produced the error
    Code       string                 // Machine-readable error code
    Message    string                 // Human-readable message
    HTTPStatus int                    // HTTP status code
    Details    map[string]interface{} // Additional context
    Recoverable bool                   // Can this be retried?
    RetryAfter  *time.Duration        // How long to wait before retry
    Cause      error                  // Underlying error
}

func (e *StageError) Error() string {
    return fmt.Sprintf("[%s] %s: %s", e.Stage, e.Code, e.Message)
}
```

### 4.2 Error Handling Flow

```go
func HandlePipelineError(err *StageError, rc *RequestContext) *http.Response {
    // Log error with full context
    logError(err, rc)

    // Update circuit breaker if provider error
    if err.Stage == "LLM_INVOCATION" && err.Recoverable {
        circuitBreaker.RecordFailure(rc.SelectedProvider.ProviderName)
    }

    // Emit error metrics
    metrics.RecordError(err.Stage, err.Code)

    // Return appropriate HTTP response
    return &http.Response{
        StatusCode: err.HTTPStatus,
        Body:       buildErrorBody(err),
        Header:     buildErrorHeaders(err, rc),
    }
}
```

### 4.3 Error Code Taxonomy

| Prefix       | Category                  | Examples                                             |
| ------------ | ------------------------- | ---------------------------------------------------- |
| `AUTH_*`     | Authentication            | `AUTH_MISSING_TOKEN`, `AUTH_EXPIRED_TOKEN`           |
| `AUTHZ_*`    | Authorization             | `AUTHZ_DENIED`, `AUTHZ_MODEL_BLOCKED`                |
| `TENANT_*`   | Tenant identification     | `TENANT_MISSING_ORG_ID`                              |
| `NORM_*`     | Request normalization     | `NORM_INVALID_JSON`, `NORM_EMPTY_PROMPT`             |
| `POLICY_*`   | Policy resolution         | `POLICY_FETCH_FAILED`                                |
| `QUOTA_*`    | Budget & rate limits      | `QUOTA_RATE_LIMIT_EXCEEDED`, `QUOTA_BUDGET_EXCEEDED` |
| `VALIDATE_*` | Pre-processing validation | `VALIDATE_PII_DETECTED`, `VALIDATE_SECRET_DETECTED`  |
| `FILTER_*`   | Content filtering         | `FILTER_TOPIC_BLOCKED`                               |
| `ROUTE_*`    | Provider routing          | `ROUTE_NO_PROVIDER`, `ROUTE_ALL_BLOCKED`             |
| `CIRCUIT_*`  | Circuit breaker           | `CIRCUIT_ALL_OPEN`                                   |
| `LLM_*`      | LLM invocation            | `LLM_TIMEOUT`, `LLM_PROVIDER_ERROR`                  |
| `INSPECT_*`  | Response inspection       | `INSPECT_HARMFUL_CONTENT`                            |
| `CACHE_*`    | Caching                   | `CACHE_READ_ERROR`                                   |

---

## 5. AWS Service Integration Map

| Stage                       | AWS Service           | Purpose                        | Critical?                |
| --------------------------- | --------------------- | ------------------------------ | ------------------------ |
| **0. Ingress**              | CloudFront            | Frontend CDN                   | No (direct to ALB works) |
| **0. Ingress**              | API Gateway           | HTTP routing, custom domain    | Yes                      |
| **0. Ingress**              | WAF                   | DDoS protection, rate limiting | Recommended              |
| **0. Ingress**              | Lambda                | Compute (runs all stages)      | Yes                      |
| **1. Authentication**       | Cognito               | User authentication, JWT       | Yes                      |
| **2. Identification**       | (none)                | In-memory processing           | N/A                      |
| **3. Normalization**        | (none)                | In-memory processing           | N/A                      |
| **4. Policy Resolution**    | Aurora PostgreSQL     | Policy storage                 | Yes                      |
| **4. Policy Resolution**    | ElastiCache Redis     | Policy caching                 | Yes (performance)        |
| **5. Cache Check**          | ElastiCache Redis     | Response caching               | No (optimization)        |
| **6. Budget & Rate**        | ElastiCache Redis     | Rate limit counters            | Yes                      |
| **6. Budget & Rate**        | Aurora PostgreSQL     | Cost tracking                  | Yes                      |
| **7. Validation**           | (none)                | Regex in-memory                | N/A                      |
| **7. Validation**           | Comprehend (optional) | ML-based PII detection         | No (enhancement)         |
| **8. Content Filter**       | (none)                | In-memory processing           | N/A                      |
| **9. Provider Routing**     | Secrets Manager       | Provider credentials           | Yes                      |
| **10. Circuit Breaker**     | ElastiCache Redis     | Circuit state                  | Yes                      |
| **11. LLM Invocation**      | VPC NAT Gateway       | Outbound internet              | Yes (if VPC-enabled)     |
| **12. Response Inspection** | (none)                | In-memory processing           | N/A                      |
| **12. Response Inspection** | Comprehend (optional) | Content moderation             | No (enhancement)         |
| **13. Cache Store**         | ElastiCache Redis     | Response storage               | No (optimization)        |
| **13. Cache Store**         | SQS                   | Async queue                    | No (optimization)        |
| **14. Audit Logging**       | SQS                   | Async queue                    | Yes                      |
| **14. Audit Logging**       | Aurora PostgreSQL     | Hot storage (30 days)          | Yes                      |
| **14. Audit Logging**       | S3                    | Cold storage (7 years)         | Yes (compliance)         |
| **14. Audit Logging**       | EventBridge           | Scheduled archival             | Yes                      |
| **14. Audit Logging**       | CloudWatch            | Metrics, logs                  | Yes                      |
| **15. Response Delivery**   | API Gateway           | HTTP response                  | Yes                      |
| **15. Response Delivery**   | CloudFront (optional) | Response caching               | No                       |

### 5.1 AWS Service Configuration Summary

```yaml
# Infrastructure overview
VPC:
  CIDR: 10.0.0.0/16
  Subnets:
    - Public: 10.0.1.0/24, 10.0.2.0/24 (API Gateway, NAT)
    - Private: 10.0.10.0/24, 10.0.11.0/24 (Lambda, Aurora, Redis)

Lambda:
  Runtime: Go 1.24
  Memory: 1024 MB (prod), 512 MB (sandbox)
  Timeout: 30 seconds
  Concurrency: 1000 (prod), 10 (sandbox)
  VPC: Enabled (private subnets)

Aurora PostgreSQL Serverless v2:
  MinCapacity: 0.5 ACU (sandbox), 1 ACU (prod)
  MaxCapacity: 2 ACU (sandbox), 8 ACU (prod)
  MultiAZ: Yes (prod)
  BackupRetention: 30 days

ElastiCache Redis:
  NodeType: cache.t3.medium (sandbox), cache.r7g.large (prod)
  Nodes: 1 (sandbox), 2 with replica (prod)
  EngineVersion: 7.x
  Persistence: AOF enabled

API Gateway:
  Type: REST API
  Throttling: 1000 req/sec (burst), 10000 req/sec (steady)
  CustomDomain: api.llm-cp.yourdomain.com
  WAF: Enabled (prod)

Cognito User Pool:
  MFA: Optional
  PasswordPolicy: Strong
  CustomAttributes: orgId, appId, userRole

Secrets Manager:
  Secrets:
    - /llm-cp/{env}/providers (LLM API keys)
    - /llm-cp/{env}/aurora (DB credentials)
    - /llm-cp/{env}/datadog (APM key)

CloudWatch:
  LogRetention: 30 days
  MetricNamespace: LLMControlPlane
  Alarms: Lambda errors, Aurora connections, Redis CPU

S3:
  AuditBucket: llm-cp-{env}-audit-logs
  Lifecycle:
    - Transition to Glacier: 90 days
    - Expiration: 7 years
  Encryption: AES-256 (SSE-S3)

SQS:
  AuditQueue:
    Type: Standard
    VisibilityTimeout: 60 seconds
    MessageRetention: 14 days
    DLQ: Enabled (3 retries)

EventBridge:
  Rules:
    - AuditArchival: Daily at 2 AM UTC
    - PolicyCacheRefresh: Every 5 minutes
```

---

## 6. Performance Budgets

### 6.1 Latency Targets (P95)

| Stage                        | Target         | Critical Path | AWS Service Overhead                      |
| ---------------------------- | -------------- | ------------- | ----------------------------------------- |
| 1. Authentication            | 20ms           | Yes           | Cognito JWKS fetch (cached)               |
| 2. Identification            | 2ms            | Yes           | In-memory                                 |
| 3. Normalization             | 5ms            | Yes           | In-memory                                 |
| 4. Policy Resolution         | 50ms           | Yes           | PostgreSQL (10ms) + Redis (5ms)           |
| 5. Cache Check               | 10ms           | Yes           | Redis GET                                 |
| 6. Budget & Rate Check       | 30ms           | Yes           | Redis ZSET ops (10ms) + PostgreSQL (15ms) |
| 7. Pre-Processing Validation | 20ms           | Yes           | Regex in-memory                           |
| 8. Content Filtering         | 10ms           | Yes           | In-memory                                 |
| 9. Provider Routing          | 20ms           | Yes           | Secrets Manager (15ms)                    |
| 10. Circuit Breaker          | 5ms            | Yes           | Redis GET                                 |
| 11. LLM Invocation           | 500-2000ms     | Yes           | External API (dominant factor)            |
| 12. Response Inspection      | 30ms           | Yes           | In-memory regex                           |
| 13. Cache Store              | N/A            | No            | Async (Redis PUT)                         |
| 14. Audit Logging            | N/A            | No            | Async (SQS)                               |
| 15. Response Delivery        | 10ms           | Yes           | API Gateway formatting                    |
| **Total (synchronous)**      | **712-2212ms** |               |                                           |

### 6.2 Throughput Targets

| Environment | Requests/Second | Concurrent Lambda            | Aurora ACU | Redis Nodes      |
| ----------- | --------------- | ---------------------------- | ---------- | ---------------- |
| Sandbox     | 10              | 10                           | 0.5-2      | 1                |
| Demo        | 50              | 50                           | 0.5-4      | 2                |
| Production  | 1000            | 200 (reserved) + 800 (burst) | 1-8        | 2 (with replica) |

### 6.3 Cost Targets

| Environment               | Monthly Budget | Lambda | Aurora | Redis | Other   |
| ------------------------- | -------------- | ------ | ------ | ----- | ------- |
| Sandbox                   | $35            | $0.20  | $15    | $12   | $8      |
| Demo                      | $150           | $50    | $50    | $30   | $20     |
| Production (1000 req/sec) | $20,927        | $5,200 | $200   | $300  | $15,227 |

---

## 7. Implementation Guidelines

### 7.1 Development Phases

**Phase 1: Core Pipeline (Weeks 1-4)**

- Stages 1-2: Authentication + Identification
- Stage 11: LLM Invocation (OpenAI only)
- Stage 15: Response Delivery
- **Goal:** End-to-end request flow

**Phase 2: Governance (Weeks 5-8)**

- Stage 4: Policy Resolution
- Stage 6: Budget & Rate Limiting
- Stage 7-8: Validation + Filtering
- **Goal:** Policy enforcement

**Phase 3: Reliability (Weeks 9-12)**

- Stage 9-10: Provider Routing + Circuit Breaker
- Stage 5, 13: Caching
- Stage 12: Response Inspection
- **Goal:** Production-ready

**Phase 4: Observability (Weeks 13-16)**

- Stage 14: Audit Logging + Metrics
- CloudWatch dashboards
- Datadog APM integration
- **Goal:** Full visibility

### 7.2 Testing Strategy

**Unit Tests:**

- Each stage in isolation
- Mock dependencies (Redis, PostgreSQL, Cognito)
- Test all error paths

**Integration Tests:**

- Full pipeline with real AWS services
- Test containerized (Docker Compose for local services)

**Load Tests:**

- k6 or Apache Bench
- Target: 1000 req/sec sustained for 5 minutes
- Monitor Aurora, Redis, Lambda metrics

**Security Tests:**

- OWASP ZAP for API Gateway
- PII detection accuracy tests
- Prompt injection tests

### 7.3 Monitoring & Alerting

**Critical Alerts (PagerDuty):**

- Lambda error rate > 5%
- Aurora connections exhausted
- Redis CPU > 80%
- Any stage consistently timing out

**Warning Alerts (Slack):**

- Budget threshold exceeded (80%)
- Circuit breaker opened
- Cache hit rate < 20%

---

**End of Pipeline Specification**

**Version:** 1.0  
**Last Updated:** February 6, 2026  
**Next Review:** After Phase 1 implementation
