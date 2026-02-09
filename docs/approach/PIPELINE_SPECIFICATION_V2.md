# LLM Control Plane - Pipeline Specification v2.0

**Version:** 2.0  
**Date:** February 6, 2026  
**Status:** Technical Specification  
**Changes:** Added RAG (Stage 3b), Removed Circuit Breaker, Removed Caching

---

## Table of Contents

1. [Pipeline Overview](#1-pipeline-overview)
2. [Stage-by-Stage Specification](#2-stage-by-stage-specification)
3. [Error Handling Contracts](#3-error-handling-contracts)
4. [AWS Service Integration Map](#4-aws-service-integration-map)
5. [Performance Budgets](#5-performance-budgets)

---

## 1. Pipeline Overview

### 1.1 Complete Pipeline (11 Stages)

```
┌────────────────────────────────────────────────────────────────┐
│                    REQUEST INGRESS                             │
│  CloudFront → API Gateway → WAF → Lambda Function             │
└────────────────────────┬───────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│               STAGE 1: AUTHENTICATION                           │
│  AWS: Cognito User Pools                                        │
│  Input: HTTP Request (headers, body)                            │
│  Output: Validated JWT Claims                                   │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│        STAGE 2: APPLICATION & CALLER IDENTIFICATION             │
│  AWS: N/A (in-memory)                                           │
│  Input: JWT Claims                                              │
│  Output: TenantContext (org_id, app_id, user_id, role)         │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│   STAGE 3: REQUEST NORMALIZATION & INTENT CLASSIFICATION        │
│  AWS: N/A (in-memory)                                           │
│  Input: Raw request + TenantContext                             │
│  Output: NormalizedRequest + IntentMetadata                     │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│        STAGE 3b: RAG CONTEXT RETRIEVAL (CONDITIONAL)            │
│  AWS: Aurora PostgreSQL (full-text search)                      │
│       OR OpenSearch Serverless (vector search - Phase 2)        │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │ PRE-RAG VALIDATION                                        │ │
│  │ • Validate user query is safe for search                 │ │
│  │ • Check query doesn't contain secrets                    │ │
│  │ • Ensure query is well-formed                            │ │
│  └───────────────────────────────────────────────────────────┘ │
│                     │                                           │
│                     ▼                                           │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │ CONTEXT RETRIEVAL                                         │ │
│  │ • Query vector/full-text database                        │ │
│  │ • Filter by org_id + access_level                        │ │
│  │ • Rank by relevance                                      │ │
│  └───────────────────────────────────────────────────────────┘ │
│                     │                                           │
│                     ▼                                           │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │ POST-RAG VALIDATION                                       │ │
│  │ • Validate retrieved documents for PII                   │ │
│  │ • Check documents match access level                     │ │
│  │ • Ensure no sensitive data in context                    │ │
│  └───────────────────────────────────────────────────────────┘ │
│                                                                 │
│  Input: NormalizedRequest + TenantContext + Policies            │
│  Output: AugmentedRequest + RetrievedContexts                   │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│         STAGE 4: POLICY RESOLUTION & AUTHORIZATION              │
│  AWS: Aurora PostgreSQL + ElastiCache Redis (cache)             │
│  Input: TenantContext + AugmentedRequest                        │
│  Output: ResolvedPolicies + AuthorizationDecision               │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│              STAGE 5: BUDGET & RATE PRE-CHECK                   │
│  AWS: ElastiCache Redis + Aurora PostgreSQL                     │
│  Input: TenantContext + Policies + EstimatedCost                │
│  Output: QuotaCheckResult                                       │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│            STAGE 6: PRE-PROCESSING VALIDATION                   │
│  AWS: N/A (in-memory regex + optional Comprehend)               │
│  Input: AugmentedRequest (user prompt + RAG context)            │
│  Output: ValidationResult + RedactedPrompt                      │
│  Validates: User input + Retrieved context (full prompt)        │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│          STAGE 7: POLICY-DRIVEN CONTENT FILTERING               │
│  AWS: N/A (in-memory)                                           │
│  Input: ValidationResult + Policies + RedactedPrompt            │
│  Output: FilteredRequest                                        │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STAGE 8: PROVIDER ROUTING                      │
│  AWS: Secrets Manager (credentials)                             │
│  Input: FilteredRequest + Policies                              │
│  Output: SelectedProvider + ProviderConfig                      │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STAGE 9: LLM INVOCATION                        │
│  AWS: Lambda + VPC NAT Gateway                                  │
│  External: OpenAI/Anthropic/Azure APIs                          │
│  Input: FilteredRequest + SelectedProvider                      │
│  Output: RawLLMResponse + UsageMetrics                          │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                STAGE 10: RESPONSE INSPECTION                    │
│  AWS: N/A (in-memory + optional Comprehend)                     │
│  Input: RawLLMResponse + Policies                               │
│  Output: InspectedResponse + ValidationReport                   │
│                                                                 │
│  POST-RAG VERIFICATION:                                         │
│  • Ensures RAG context didn't leak into response                │
│  • Validates LLM didn't expose sensitive retrieved docs         │
│  • Checks response for PII from RAG sources                     │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│       STAGE 11: AUDIT, METRICS & TRACE CORRELATION (ASYNC)      │
│  AWS: SQS → Lambda → Aurora PostgreSQL + S3                     │
│  Input: Full request/response + all stage decisions             │
│  Output: audit_logged                                           │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                 STAGE 12: RESPONSE DELIVERY                     │
│  AWS: API Gateway → CloudFront                                  │
│  Input: InspectedResponse + Metadata                            │
│  Output: HTTP Response (JSON)                                   │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 Pipeline Characteristics

**Total stages:** 12 (11 synchronous + 1 async)

**Critical path (synchronous):**
- Stages 1-10: Must complete before response

**Asynchronous (non-blocking):**
- Stage 11: Audit logging

**Early exit points:**
| Stage | Exit Condition | HTTP Status |
|-------|----------------|-------------|
| Stage 1 | Auth failure | 401 Unauthorized |
| Stage 3b | Pre-RAG validation failure | 400 Bad Request |
| Stage 3b | Post-RAG validation failure | 400 Bad Request |
| Stage 4 | Authorization denied | 403 Forbidden |
| Stage 5 | Rate limit exceeded | 429 Too Many Requests |
| Stage 6 | PII detected in final prompt | 400 Bad Request |
| Stage 7 | Content policy violation | 403 Forbidden |
| Stage 8 | No provider available | 503 Service Unavailable |
| Stage 9 | LLM provider error | 502 Bad Gateway |
| Stage 10 | Harmful content in response | 451 Unavailable For Legal Reasons |

---

## 2. Stage-by-Stage Specification

### STAGE 1: AUTHENTICATION

**AWS Services:** Cognito User Pools

**Responsibilities:**
- Validate JWT token signature using Cognito JWKS
- Extract and verify JWT claims (issuer, audience, expiration)
- Extract user identity and role

**Entry Contract:**
```go
type Stage1Input struct {
    HTTPRequest *http.Request
    Headers     map[string]string
}
```

**Exit Contract:**
```go
type Stage1Output struct {
    JWTClaims *JWTClaims
    Error     *StageError
}

type JWTClaims struct {
    Subject      string  // "sub" - Cognito user ID
    Email        string  // "email"
    OrgID        string  // "custom:orgId"
    AppID        string  // "custom:appId"
    UserRole     string  // "custom:userRole"
    ExpiresAt    int64   // "exp"
}
```

**Performance:** 10-20ms (JWKS cached)

**Error Codes:**
- `AUTH_MISSING_TOKEN` (401)
- `AUTH_INVALID_TOKEN` (401)
- `AUTH_EXPIRED_TOKEN` (401)

---

### STAGE 2: APPLICATION & CALLER IDENTIFICATION

**AWS Services:** None (in-memory)

**Responsibilities:**
- Extract org/app/user from JWT claims
- Build tenant context

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
    OrganizationID string
    ApplicationID  string
    UserID         string
    UserRole       string
    UserEmail      string
}
```

**Performance:** 1-2ms

**Error Codes:**
- `TENANT_MISSING_ORG_ID` (400)
- `TENANT_MISSING_APP_ID` (400)

---

### STAGE 3: REQUEST NORMALIZATION & INTENT CLASSIFICATION

**AWS Services:** None (in-memory)

**Responsibilities:**
- Parse request body
- Normalize to unified format
- Classify intent (conversational, retrieval, code generation)
- Determine if RAG is needed

**Entry Contract:**
```go
type Stage3Input struct {
    RawBody       []byte
    TenantContext *TenantContext
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
    RequestType string      // "chat", "embedding", "completion"
    Model       string      // "gpt-4", "claude-3-opus"
    Messages    []Message
    Parameters  *RequestParameters
}

type Message struct {
    Role    string // "system", "user", "assistant"
    Content string
}

type IntentMetadata struct {
    Classification   string // "conversational", "retrieval", "code_generation"
    RequiresRAG      bool   // Should RAG be used?
    SensitivityLevel string // "public", "internal", "confidential"
    EstimatedTokens  int
}
```

**Intent Classification Logic:**
```go
func ClassifyIntent(messages []Message) *IntentMetadata {
    lastMessage := messages[len(messages)-1].Content
    
    // Keywords that indicate retrieval/RAG is needed
    retrievalKeywords := []string{
        "what is", "tell me about", "explain", "documentation",
        "policy", "procedure", "guideline", "how to",
    }
    
    requiresRAG := false
    for _, keyword := range retrievalKeywords {
        if strings.Contains(strings.ToLower(lastMessage), keyword) {
            requiresRAG = true
            break
        }
    }
    
    classification := "conversational"
    if requiresRAG {
        classification = "retrieval"
    } else if containsCodeKeywords(lastMessage) {
        classification = "code_generation"
    }
    
    return &IntentMetadata{
        Classification: classification,
        RequiresRAG:    requiresRAG,
    }
}
```

**Performance:** 3-5ms

**Error Codes:**
- `NORM_INVALID_JSON` (400)
- `NORM_MISSING_MODEL` (400)
- `NORM_EMPTY_PROMPT` (400)

---

### STAGE 3b: RAG CONTEXT RETRIEVAL (CONDITIONAL)

**AWS Services:** 
- Aurora PostgreSQL (full-text search - MVP)
- OpenSearch Serverless (vector search - Phase 2)
- S3 (document storage)

**Responsibilities:**
- **Pre-RAG Validation:** Validate user query before retrieval
- **Context Retrieval:** Search knowledge base for relevant documents
- **Post-RAG Validation:** Validate retrieved documents for sensitive data
- **Prompt Augmentation:** Combine user query with retrieved context

**Entry Contract:**
```go
type Stage3bInput struct {
    NormalizedRequest *NormalizedRequest
    IntentMetadata    *IntentMetadata
    TenantContext     *TenantContext
    ResolvedPolicies  *ResolvedPolicies // For access control
}
```

**Exit Contract:**
```go
type Stage3bOutput struct {
    AugmentedRequest *AugmentedRequest
    RetrievalMetrics *RetrievalMetrics
    Error            *StageError
}

type AugmentedRequest struct {
    OriginalRequest   *NormalizedRequest
    RetrievedContexts []RetrievedContext
    AugmentedMessages []Message // Original + RAG context
    RAGApplied        bool
    ContextTokenCount int
}

type RetrievedContext struct {
    DocumentID    string
    Title         string
    Content       string
    Snippet       string  // Relevant excerpt (max 500 chars)
    Score         float64 // Relevance score 0-1
    Source        string  // "knowledge_base", "documentation"
    AccessLevel   string  // "public", "internal", "confidential"
    OrganizationID string
    
    // Validation flags (Post-RAG)
    ContainsPII       bool
    ContainsSecrets   bool
    AccessGranted     bool
}

type RetrievalMetrics struct {
    QueryLatency       time.Duration
    DocumentsSearched  int
    DocumentsRetrieved int
    DocumentsFiltered  int // Filtered due to access/PII
    TopScore           float64
    SearchMethod       string // "postgres_fts", "opensearch_vector"
}
```

### Sub-Stage 3b.1: Pre-RAG Validation

**Purpose:** Validate user query is safe for knowledge base search

**Validation checks:**
```go
func PreRAGValidation(query string) error {
    // 1. Check query isn't empty
    if strings.TrimSpace(query) == "" {
        return &StageError{Code: "RAG_EMPTY_QUERY"}
    }
    
    // 2. Check query doesn't contain secrets (prevent secret leakage in search)
    if containsSecrets(query) {
        return &StageError{Code: "RAG_QUERY_CONTAINS_SECRETS"}
    }
    
    // 3. Check query length (prevent DoS via very long queries)
    if len(query) > 1000 {
        return &StageError{Code: "RAG_QUERY_TOO_LONG"}
    }
    
    // 4. Check for injection attempts in search query
    if containsSQLInjection(query) {
        return &StageError{Code: "RAG_QUERY_INJECTION_DETECTED"}
    }
    
    return nil
}
```

### Sub-Stage 3b.2: Context Retrieval

**PostgreSQL Full-Text Search (MVP):**
```go
func RetrieveContext_PostgreSQL(
    ctx context.Context,
    query string,
    tenant *TenantContext,
) ([]RetrievedContext, error) {
    // Full-text search with access control
    sql := `
        SELECT 
            id,
            title,
            content,
            access_level,
            substring(content from 1 for 500) as snippet,
            ts_rank(search_vector, plainto_tsquery('english', $1)) as relevance
        FROM documents
        WHERE org_id = $2
          AND search_vector @@ plainto_tsquery('english', $1)
          AND access_level = ANY($3)
        ORDER BY relevance DESC
        LIMIT 5
    `
    
    allowedLevels := getAllowedAccessLevels(tenant.UserRole)
    rows, err := db.QueryContext(ctx, sql, query, tenant.OrganizationID, allowedLevels)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    contexts := []RetrievedContext{}
    for rows.Next() {
        var ctx RetrievedContext
        err := rows.Scan(
            &ctx.DocumentID,
            &ctx.Title,
            &ctx.Content,
            &ctx.AccessLevel,
            &ctx.Snippet,
            &ctx.Score,
        )
        if err != nil {
            continue
        }
        
        ctx.OrganizationID = tenant.OrganizationID
        ctx.Source = "knowledge_base"
        ctx.AccessGranted = true // Already filtered by SQL
        
        contexts = append(contexts, ctx)
    }
    
    return contexts, nil
}

func getAllowedAccessLevels(role string) []string {
    switch role {
    case "superadmin", "admin":
        return []string{"public", "internal", "confidential"}
    case "developer":
        return []string{"public", "internal"}
    case "auditor":
        return []string{"public", "internal"}
    default:
        return []string{"public"}
    }
}
```

**OpenSearch Vector Search (Phase 2):**
```go
func RetrieveContext_OpenSearch(
    ctx context.Context,
    query string,
    tenant *TenantContext,
) ([]RetrievedContext, error) {
    // 1. Generate embedding for query (using Bedrock or OpenAI)
    embedding, err := generateEmbedding(ctx, query)
    if err != nil {
        return nil, err
    }
    
    // 2. Vector similarity search
    searchRequest := map[string]interface{}{
        "size": 5,
        "query": map[string]interface{}{
            "bool": map[string]interface{}{
                "must": []map[string]interface{}{
                    {
                        "knn": map[string]interface{}{
                            "embedding_vector": map[string]interface{}{
                                "vector": embedding,
                                "k":      5,
                            },
                        },
                    },
                },
                "filter": []map[string]interface{}{
                    {"term": map[string]interface{}{"org_id": tenant.OrganizationID}},
                    {"terms": map[string]interface{}{"access_level": getAllowedAccessLevels(tenant.UserRole)}},
                },
            },
        },
    }
    
    results, err := osClient.Search(
        osClient.Search.WithContext(ctx),
        osClient.Search.WithIndex("knowledge-base"),
        osClient.Search.WithBody(buildSearchBody(searchRequest)),
    )
    
    return parseOpenSearchResults(results), nil
}
```

### Sub-Stage 3b.3: Post-RAG Validation

**Purpose:** Validate retrieved documents don't contain sensitive data

**Validation checks:**
```go
func PostRAGValidation(contexts []RetrievedContext) ([]RetrievedContext, error) {
    validated := []RetrievedContext{}
    violations := []string{}
    
    for i, ctx := range contexts {
        // 1. Check for PII in retrieved content
        piiFound := detectPII(ctx.Snippet)
        if len(piiFound) > 0 {
            ctx.ContainsPII = true
            logWarning("PII detected in retrieved document", map[string]interface{}{
                "document_id": ctx.DocumentID,
                "violations":  piiFound,
            })
            
            // Redact PII from snippet
            ctx.Snippet = redactPII(ctx.Snippet)
        }
        
        // 2. Check for secrets in retrieved content
        secretsFound := detectSecrets(ctx.Snippet)
        if len(secretsFound) > 0 {
            ctx.ContainsSecrets = true
            
            // BLOCK this document (secrets are critical)
            violations = append(violations, fmt.Sprintf(
                "Document %s contains secrets: %v",
                ctx.DocumentID,
                secretsFound,
            ))
            continue // Skip this document
        }
        
        // 3. Verify access control (double-check)
        if !hasAccess(ctx.AccessLevel, ctx.OrganizationID) {
            violations = append(violations, fmt.Sprintf(
                "Document %s access denied",
                ctx.DocumentID,
            ))
            continue
        }
        
        validated = append(validated, ctx)
    }
    
    // If all documents were filtered out, that's okay
    // Just means no safe context available
    if len(validated) == 0 && len(contexts) > 0 {
        logWarning("All retrieved documents filtered", map[string]interface{}{
            "total_retrieved": len(contexts),
            "violations":      violations,
        })
    }
    
    return validated, nil
}
```

### Prompt Augmentation

**Strategy:** Prepend retrieved context to user message

```go
func AugmentPrompt(
    request *NormalizedRequest,
    contexts []RetrievedContext,
) *AugmentedRequest {
    if len(contexts) == 0 {
        // No RAG context available, use original request
        return &AugmentedRequest{
            OriginalRequest:   request,
            RetrievedContexts: []RetrievedContext{},
            AugmentedMessages: request.Messages,
            RAGApplied:        false,
        }
    }
    
    // Build context section
    contextText := "Relevant context from knowledge base:\n\n"
    for i, ctx := range contexts {
        contextText += fmt.Sprintf(
            "Document %d (from %s, relevance: %.2f):\n%s\n\n",
            i+1,
            ctx.Title,
            ctx.Score,
            ctx.Snippet,
        )
    }
    
    // Prepend context to last user message
    augmentedMessages := make([]Message, len(request.Messages))
    copy(augmentedMessages, request.Messages)
    
    lastIdx := len(augmentedMessages) - 1
    augmentedMessages[lastIdx].Content = fmt.Sprintf(
        "%s\nUser question: %s\n\nPlease answer based on the provided context.",
        contextText,
        augmentedMessages[lastIdx].Content,
    )
    
    return &AugmentedRequest{
        OriginalRequest:   request,
        RetrievedContexts: contexts,
        AugmentedMessages: augmentedMessages,
        RAGApplied:        true,
        ContextTokenCount: countTokens(contextText),
    }
}
```

**Complete Stage 3b Flow:**
```go
func Stage3b_RAG(ctx context.Context, input *Stage3bInput) (*Stage3bOutput, error) {
    start := time.Now()
    
    // Skip RAG if not required
    if !input.IntentMetadata.RequiresRAG {
        return &Stage3bOutput{
            AugmentedRequest: &AugmentedRequest{
                OriginalRequest:   input.NormalizedRequest,
                RetrievedContexts: []RetrievedContext{},
                AugmentedMessages: input.NormalizedRequest.Messages,
                RAGApplied:        false,
            },
        }, nil
    }
    
    // Extract user query
    userQuery := extractLastUserMessage(input.NormalizedRequest.Messages)
    
    // ─────────────────────────────────────────────
    // PRE-RAG VALIDATION
    // ─────────────────────────────────────────────
    if err := PreRAGValidation(userQuery); err != nil {
        return nil, &StageError{
            Stage:      "STAGE_3b_PRE_RAG",
            Code:       err.Code,
            Message:    "Query validation failed before RAG retrieval",
            HTTPStatus: 400,
        }
    }
    
    // ─────────────────────────────────────────────
    // CONTEXT RETRIEVAL
    // ─────────────────────────────────────────────
    retrievedContexts, err := RetrieveContext_PostgreSQL(ctx, userQuery, input.TenantContext)
    if err != nil {
        // Graceful degradation: continue without RAG
        logWarning("RAG retrieval failed, continuing without context", err)
        return &Stage3bOutput{
            AugmentedRequest: &AugmentedRequest{
                OriginalRequest:   input.NormalizedRequest,
                AugmentedMessages: input.NormalizedRequest.Messages,
                RAGApplied:        false,
            },
        }, nil
    }
    
    // ─────────────────────────────────────────────
    // POST-RAG VALIDATION
    // ─────────────────────────────────────────────
    validatedContexts, err := PostRAGValidation(retrievedContexts)
    if err != nil {
        return nil, &StageError{
            Stage:      "STAGE_3b_POST_RAG",
            Code:       "RAG_VALIDATION_FAILED",
            Message:    "Retrieved documents failed validation",
            HTTPStatus: 400,
        }
    }
    
    // If all documents filtered, continue without RAG
    if len(validatedContexts) == 0 {
        logInfo("No valid RAG context after validation, continuing without RAG")
        return &Stage3bOutput{
            AugmentedRequest: &AugmentedRequest{
                OriginalRequest:   input.NormalizedRequest,
                AugmentedMessages: input.NormalizedRequest.Messages,
                RAGApplied:        false,
            },
        }, nil
    }
    
    // ─────────────────────────────────────────────
    // PROMPT AUGMENTATION
    // ─────────────────────────────────────────────
    augmented := AugmentPrompt(input.NormalizedRequest, validatedContexts)
    
    metrics := &RetrievalMetrics{
        QueryLatency:       time.Since(start),
        DocumentsSearched:  len(retrievedContexts),
        DocumentsRetrieved: len(validatedContexts),
        DocumentsFiltered:  len(retrievedContexts) - len(validatedContexts),
        TopScore:           validatedContexts[0].Score,
        SearchMethod:       "postgres_fts",
    }
    
    return &Stage3bOutput{
        AugmentedRequest: augmented,
        RetrievalMetrics: metrics,
    }, nil
}
```

**Performance:** 
- PostgreSQL full-text: 30-80ms
- OpenSearch vector: 50-200ms

**Error Codes:**
- `RAG_EMPTY_QUERY` (400) - Pre-RAG
- `RAG_QUERY_CONTAINS_SECRETS` (400) - Pre-RAG
- `RAG_QUERY_TOO_LONG` (400) - Pre-RAG
- `RAG_QUERY_INJECTION_DETECTED` (400) - Pre-RAG
- `RAG_ALL_DOCS_FILTERED` (warning, not error) - Post-RAG
- `RAG_RETRIEVAL_FAILED` (graceful degradation, continue without RAG)

**Boundary:**
- **IN:** Normalized request + tenant context
- **OUT:** Augmented request with validated RAG context
- **Does NOT:** Enforce policies (Stage 4 does that)
- **Does NOT:** Call LLM (Stage 9 does that)
- **CAN SKIP:** If intent doesn't require RAG or if RAG fails gracefully

---

### STAGE 4: POLICY RESOLUTION & AUTHORIZATION

**AWS Services:** Aurora PostgreSQL + ElastiCache Redis

**Responsibilities:**
- Fetch policies for tenant
- Resolve policy hierarchy (org → app → user)
- Check RBAC permissions
- Authorize operation

**Entry Contract:**
```go
type Stage4Input struct {
    TenantContext    *TenantContext
    AugmentedRequest *AugmentedRequest // May include RAG context
    IntentMetadata   *IntentMetadata
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
    RateLimits struct {
        RequestsPerMinute int
        RequestsPerDay    int
        TokensPerDay      int
    }
    
    CostCaps struct {
        MonthlyBudgetUSD float64
        DailyBudgetUSD   float64
    }
    
    ModelRestrictions struct {
        AllowedModels []string
        BlockedModels []string
    }
    
    ContentPolicies struct {
        BlockPII            bool
        BlockSecrets        bool
        BlockPromptInjection bool
        MaxPromptLength     int
    }
    
    // RAG-specific policies
    RAGPolicies struct {
        Enabled           bool
        MaxContexts       int
        MaxContextTokens  int
        MinRelevanceScore float64
    }
}
```

**Performance:** 20-50ms (cached)

**Error Codes:**
- `AUTHZ_DENIED` (403)
- `AUTHZ_MODEL_BLOCKED` (403)
- `POLICY_FETCH_FAILED` (503)

---

### STAGE 5: BUDGET & RATE PRE-CHECK

**AWS Services:** ElastiCache Redis + Aurora PostgreSQL

**Responsibilities:**
- Check rate limits
- Estimate request cost (including RAG tokens!)
- Validate budget

**Entry Contract:**
```go
type Stage5Input struct {
    TenantContext    *TenantContext
    ResolvedPolicies *ResolvedPolicies
    AugmentedRequest *AugmentedRequest // Includes RAG context tokens
}
```

**Exit Contract:**
```go
type Stage5Output struct {
    QuotaCheck *QuotaCheckResult
    Error      *StageError
}

type QuotaCheckResult struct {
    RateLimitAllowed  bool
    RequestsRemaining int
    ResetAt           time.Time
    
    BudgetAllowed     bool
    EstimatedCostUSD  float64
    RemainingBudget   float64
}
```

**Cost Estimation (with RAG):**
```go
func EstimateCost(request *AugmentedRequest, model string) float64 {
    // Count tokens in augmented prompt (includes RAG context)
    inputTokens := countTokens(request.AugmentedMessages)
    
    // Estimate output tokens (conservative)
    outputTokens := request.OriginalRequest.Parameters.MaxTokens
    
    // Get pricing
    pricing := getModelPricing(model)
    
    inputCost := float64(inputTokens) / 1_000_000 * pricing.Input
    outputCost := float64(outputTokens) / 1_000_000 * pricing.Output
    
    return inputCost + outputCost
}
```

**Performance:** 20-30ms

**Error Codes:**
- `QUOTA_RATE_LIMIT_EXCEEDED` (429)
- `QUOTA_BUDGET_EXCEEDED` (402)

---

### STAGE 6: PRE-PROCESSING VALIDATION

**AWS Services:** None (in-memory), Optional: Amazon Comprehend

**Responsibilities:**
- Validate FULL augmented prompt (user input + RAG context)
- Detect PII in combined prompt
- Detect secrets
- Detect prompt injection

**Entry Contract:**
```go
type Stage6Input struct {
    AugmentedRequest *AugmentedRequest // Contains original + RAG context
    ResolvedPolicies *ResolvedPolicies
}
```

**Exit Contract:**
```go
type Stage6Output struct {
    ValidationResult *ValidationResult
    RedactedRequest  *AugmentedRequest
    Error            *StageError
}

type ValidationResult struct {
    Valid      bool
    Violations []Violation
    Action     string // "allow", "redact", "block"
}

type Violation struct {
    Type       string  // "pii", "secret", "injection"
    Subtype    string  // "email", "ssn", "api_key"
    Source     string  // "user_input", "rag_context"
    Value      string  // Masked value
    Severity   string  // "low", "medium", "high", "critical"
}
```

**Validation Logic (User Input + RAG Context):**
```go
func ValidateAugmentedPrompt(request *AugmentedRequest, policies *ResolvedPolicies) (*ValidationResult, error) {
    violations := []Violation{}
    
    // 1. Validate original user input
    for _, msg := range request.OriginalRequest.Messages {
        userViolations := detectPII(msg.Content)
        for _, v := range userViolations {
            v.Source = "user_input"
            violations = append(violations, v)
        }
    }
    
    // 2. Validate RAG context (should have been validated in 3b, but double-check)
    if request.RAGApplied {
        for _, ctx := range request.RetrievedContexts {
            contextViolations := detectPII(ctx.Snippet)
            for _, v := range contextViolations {
                v.Source = "rag_context"
                violations = append(violations, v)
            }
        }
    }
    
    // 3. Determine action
    hasBlockableViolation := false
    for _, v := range violations {
        if v.Severity == "critical" || (v.Type == "secret") {
            hasBlockableViolation = true
            break
        }
    }
    
    action := "allow"
    if hasBlockableViolation {
        action = "block"
    } else if len(violations) > 0 {
        action = "redact"
    }
    
    return &ValidationResult{
        Valid:      action != "block",
        Violations: violations,
        Action:     action,
    }, nil
}
```

**Performance:** 15-30ms

**Error Codes:**
- `VALIDATE_PII_DETECTED` (400)
- `VALIDATE_SECRET_DETECTED` (400)
- `VALIDATE_INJECTION_DETECTED` (400)

---

### STAGE 7: POLICY-DRIVEN CONTENT FILTERING

**AWS Services:** None (in-memory)

**Responsibilities:**
- Apply content policies
- Filter blocked topics
- Enforce time restrictions

**Entry Contract:**
```go
type Stage7Input struct {
    ValidationResult *ValidationResult
    RedactedRequest  *AugmentedRequest
    ResolvedPolicies *ResolvedPolicies
}
```

**Exit Contract:**
```go
type Stage7Output struct {
    FilteredRequest *AugmentedRequest
    Error           *StageError
}
```

**Performance:** 5-10ms

**Error Codes:**
- `FILTER_TOPIC_BLOCKED` (403)
- `FILTER_TIME_RESTRICTED` (403)

---

### STAGE 8: PROVIDER ROUTING

**AWS Services:** Secrets Manager

**Responsibilities:**
- Select provider for requested model
- Fetch API credentials
- Prepare fallback options

**Entry Contract:**
```go
type Stage8Input struct {
    FilteredRequest  *AugmentedRequest
    ResolvedPolicies *ResolvedPolicies
}
```

**Exit Contract:**
```go
type Stage8Output struct {
    SelectedProvider *ProviderSelection
    Error            *StageError
}

type ProviderSelection struct {
    ProviderName string   // "openai", "anthropic", "azure"
    Model        string
    Credentials  *ProviderCredentials
    Endpoint     string
}
```

**Performance:** 15-25ms (Secrets Manager cached)

**Error Codes:**
- `ROUTE_NO_PROVIDER` (400)
- `ROUTE_CREDS_FETCH_FAILED` (503)

---

### STAGE 9: LLM INVOCATION

**AWS Services:** Lambda + VPC NAT Gateway  
**External:** OpenAI/Anthropic/Azure APIs

**Responsibilities:**
- Call LLM provider
- Send augmented prompt (with RAG context if applied)
- Handle provider errors
- Measure latency and tokens

**Entry Contract:**
```go
type Stage9Input struct {
    FilteredRequest  *AugmentedRequest
    SelectedProvider *ProviderSelection
}
```

**Exit Contract:**
```go
type Stage9Output struct {
    RawLLMResponse *LLMResponse
    UsageMetrics   *UsageMetrics
    Error          *StageError
}

type LLMResponse struct {
    ID      string
    Model   string
    Choices []Choice
    Usage   Usage
}

type UsageMetrics struct {
    Provider      string
    Latency       time.Duration
    InputTokens   int
    OutputTokens  int
    TotalTokens   int
    EstimatedCost float64
}
```

**Implementation:**
```go
func Stage9_LLMInvocation(ctx context.Context, input *Stage9Input) (*Stage9Output, error) {
    start := time.Now()
    
    // Get provider adapter
    provider := getProvider(input.SelectedProvider.ProviderName)
    
    // Call LLM with augmented messages (includes RAG context)
    response, err := provider.ChatCompletion(ctx, &ChatRequest{
        Model:    input.FilteredRequest.OriginalRequest.Model,
        Messages: input.FilteredRequest.AugmentedMessages, // ◄── Augmented!
        // ... other params
    })
    if err != nil {
        return nil, &StageError{
            Stage:      "STAGE_9",
            Code:       "LLM_PROVIDER_ERROR",
            Message:    err.Error(),
            HTTPStatus: 502,
        }
    }
    
    metrics := &UsageMetrics{
        Provider:     input.SelectedProvider.ProviderName,
        Latency:      time.Since(start),
        InputTokens:  response.Usage.PromptTokens,
        OutputTokens: response.Usage.CompletionTokens,
        TotalTokens:  response.Usage.TotalTokens,
    }
    
    // Calculate actual cost (may differ from estimate)
    metrics.EstimatedCost = calculateCost(
        input.SelectedProvider.Model,
        metrics.InputTokens,
        metrics.OutputTokens,
    )
    
    return &Stage9Output{
        RawLLMResponse: response,
        UsageMetrics:   metrics,
    }, nil
}
```

**Performance:** 500-2000ms (external API)

**Error Codes:**
- `LLM_TIMEOUT` (504)
- `LLM_RATE_LIMITED` (429)
- `LLM_PROVIDER_ERROR` (502)
- `LLM_CONTENT_FILTERED` (451)

---

### STAGE 10: RESPONSE INSPECTION

**AWS Services:** None (in-memory), Optional: Amazon Comprehend

**Responsibilities:**
- Validate LLM response for PII
- Detect harmful content
- **POST-RAG Verification:** Ensure LLM didn't expose sensitive RAG content
- Redact sensitive information

**Entry Contract:**
```go
type Stage10Input struct {
    RawLLMResponse   *LLMResponse
    AugmentedRequest *AugmentedRequest // To check if RAG was used
    ResolvedPolicies *ResolvedPolicies
}
```

**Exit Contract:**
```go
type Stage10Output struct {
    InspectedResponse *InspectedResponse
    ValidationReport  *ResponseValidationReport
    Error             *StageError
}

type InspectedResponse struct {
    Response         *LLMResponse
    RedactionApplied bool
    FilterActions    []string
}

type ResponseValidationReport struct {
    Valid           bool
    Violations      []Violation
    PIIDetected     bool
    HarmfulContent  bool
    RAGLeakage      bool // Did LLM expose sensitive RAG content?
}
```

**Post-RAG Verification Logic:**
```go
func VerifyRAGSafety(
    response *LLMResponse,
    ragContexts []RetrievedContext,
) []Violation {
    violations := []Violation{}
    
    if len(ragContexts) == 0 {
        return violations // No RAG used, skip check
    }
    
    responseText := extractResponseText(response)
    
    // Check if LLM leaked sensitive information from RAG context
    for _, ctx := range ragContexts {
        // If context was marked as containing PII, check if it leaked
        if ctx.ContainsPII {
            // Check if redacted portions appear in response
            if containsRedactedContent(responseText, ctx.Snippet) {
                violations = append(violations, Violation{
                    Type:     "rag_leakage",
                    Subtype:  "pii_exposure",
                    Source:   "rag_context",
                    Severity: "critical",
                    Value:    fmt.Sprintf("Document %s", ctx.DocumentID),
                })
            }
        }
        
        // Check if LLM cited document IDs or metadata (potential info leak)
        if strings.Contains(responseText, ctx.DocumentID) {
            violations = append(violations, Violation{
                Type:     "rag_leakage",
                Subtype:  "metadata_exposure",
                Source:   "rag_context",
                Severity: "medium",
                Value:    ctx.DocumentID,
            })
        }
    }
    
    return violations
}
```

**Complete Inspection Logic:**
```go
func Stage10_ResponseInspection(ctx context.Context, input *Stage10Input) (*Stage10Output, error) {
    report := &ResponseValidationReport{
        Valid: true,
    }
    
    responseText := extractResponseText(input.RawLLMResponse)
    
    // 1. Check for PII in response
    piiViolations := detectPII(responseText)
    if len(piiViolations) > 0 {
        report.PIIDetected = true
        for _, v := range piiViolations {
            v.Source = "llm_response"
        }
        report.Violations = append(report.Violations, piiViolations...)
    }
    
    // 2. Check for harmful content
    harmfulViolations := detectHarmfulContent(responseText)
    if len(harmfulViolations) > 0 {
        report.HarmfulContent = true
        report.Violations = append(report.Violations, harmfulViolations...)
    }
    
    // 3. POST-RAG VERIFICATION: Check for RAG leakage
    if input.AugmentedRequest.RAGApplied {
        ragViolations := VerifyRAGSafety(
            input.RawLLMResponse,
            input.AugmentedRequest.RetrievedContexts,
        )
        if len(ragViolations) > 0 {
            report.RAGLeakage = true
            report.Violations = append(report.Violations, ragViolations...)
        }
    }
    
    // 4. Determine action
    if report.HarmfulContent || report.RAGLeakage {
        report.Valid = false
        return nil, report, &StageError{
            Stage:      "STAGE_10",
            Code:       "INSPECT_CONTENT_VIOLATION",
            Message:    "Response contains harmful or leaked content",
            HTTPStatus: 451,
        }
    }
    
    // 5. Redact PII if found
    response := input.RawLLMResponse
    redactionApplied := false
    if report.PIIDetected {
        response = redactPIIFromResponse(response)
        redactionApplied = true
    }
    
    return &Stage10Output{
        InspectedResponse: &InspectedResponse{
            Response:         response,
            RedactionApplied: redactionApplied,
        },
        ValidationReport: report,
    }, nil
}
```

**Performance:** 20-40ms

**Error Codes:**
- `INSPECT_HARMFUL_CONTENT` (451)
- `INSPECT_RAG_LEAKAGE` (451)
- `INSPECT_PII_DETECTED` (warning, redacted)

---

### STAGE 11: AUDIT, METRICS & TRACE CORRELATION (ASYNC)

**AWS Services:** SQS → Lambda → Aurora PostgreSQL + S3

**Responsibilities:**
- Log complete request/response
- Track RAG usage metrics
- Store in PostgreSQL (30 days)
- Archive to S3 (7 years)

**Entry Contract:**
```go
type Stage11Input struct {
    RequestContext *RequestContext // All stage data
}
```

**Exit Contract:**
```go
type Stage11Output struct {
    AuditLogID string
    Logged     bool
}
```

**Audit Log Structure (with RAG):**
```go
type AuditLog struct {
    // Standard fields
    ID            string
    OrgID         string
    UserID        string
    Timestamp     time.Time
    Model         string
    Provider      string
    Status        string
    
    // Token usage
    TokensInput   int
    TokensOutput  int
    TotalTokens   int
    CostUSD       float64
    LatencyMS     int
    
    // RAG-specific fields
    RAGUsed           bool
    RAGDocuments      int
    RAGTokens         int
    RAGLatencyMS      int
    RAGSearchMethod   string
    RAGValidationFlags map[string]bool // {"pre_rag_passed": true, "post_rag_passed": true}
    
    // Violations
    ValidationViolations []string
    RAGLeakageDetected   bool
}
```

**Performance:** Async (non-blocking)

---

### STAGE 12: RESPONSE DELIVERY

**AWS Services:** API Gateway → CloudFront

**Responsibilities:**
- Format final response
- Add metadata headers
- Return to client

**Entry Contract:**
```go
type Stage12Input struct {
    InspectedResponse *InspectedResponse
    RequestContext    *RequestContext
}
```

**Exit Contract:**
```go
type Stage12Output struct {
    HTTPResponse *http.Response
}
```

**Response Format:**
```json
{
  "id": "req-xyz789",
  "object": "chat.completion",
  "model": "gpt-4",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "Based on the company handbook, you receive 20 days of vacation per year..."
      }
    }
  ],
  "usage": {
    "prompt_tokens": 1250,
    "completion_tokens": 85,
    "total_tokens": 1335
  },
  "metadata": {
    "provider": "openai",
    "cost_usd": 0.0395,
    "latency_ms": 1842,
    "rag_used": true,
    "rag_documents": 2,
    "rag_tokens": 1100
  }
}
```

**Performance:** 5-10ms

---

## 3. Error Handling Contracts

### 3.1 Error Structure

```go
type StageError struct {
    Stage       string
    Code        string
    Message     string
    HTTPStatus  int
    Recoverable bool
    Details     map[string]interface{}
}
```

### 3.2 Complete Error Taxonomy

| Stage | Error Code | HTTP | Recoverable |
|-------|------------|------|-------------|
| **1** | `AUTH_MISSING_TOKEN` | 401 | No |
| **1** | `AUTH_INVALID_TOKEN` | 401 | No |
| **1** | `AUTH_EXPIRED_TOKEN` | 401 | Yes (refresh) |
| **2** | `TENANT_MISSING_ORG_ID` | 400 | No |
| **3** | `NORM_INVALID_JSON` | 400 | No |
| **3** | `NORM_EMPTY_PROMPT` | 400 | No |
| **3b** | `RAG_QUERY_CONTAINS_SECRETS` | 400 | No (Pre-RAG) |
| **3b** | `RAG_QUERY_INJECTION` | 400 | No (Pre-RAG) |
| **3b** | `RAG_ALL_DOCS_FILTERED` | Warning | N/A (Post-RAG) |
| **4** | `AUTHZ_DENIED` | 403 | No |
| **4** | `AUTHZ_MODEL_BLOCKED` | 403 | No |
| **5** | `QUOTA_RATE_LIMIT_EXCEEDED` | 429 | Yes |
| **5** | `QUOTA_BUDGET_EXCEEDED` | 402 | No |
| **6** | `VALIDATE_PII_DETECTED` | 400 | No |
| **6** | `VALIDATE_SECRET_DETECTED` | 400 | No |
| **7** | `FILTER_TOPIC_BLOCKED` | 403 | No |
| **8** | `ROUTE_NO_PROVIDER` | 400 | No |
| **9** | `LLM_TIMEOUT` | 504 | Yes |
| **9** | `LLM_PROVIDER_ERROR` | 502 | Yes |
| **10** | `INSPECT_HARMFUL_CONTENT` | 451 | No |
| **10** | `INSPECT_RAG_LEAKAGE` | 451 | No (Post-RAG) |

---

## 4. AWS Service Integration Map

| Stage | AWS Service | Purpose | Critical | Monthly Cost (Prod) |
|-------|-------------|---------|----------|---------------------|
| **0** | API Gateway | HTTP routing | Yes | $9,100 |
| **0** | WAF | DDoS protection | Recommended | $5,200 |
| **0** | Lambda | Compute (all stages) | Yes | $5,200 |
| **1** | Cognito | JWT validation | Yes | Included |
| **2** | N/A | In-memory | N/A | $0 |
| **3** | N/A | In-memory | N/A | $0 |
| **3b** | Aurora PostgreSQL | Full-text search | Yes | $200 |
| **3b** | OpenSearch (Phase 2) | Vector search | No | $500 |
| **3b** | S3 | Document storage | Yes | $10 |
| **4** | Aurora PostgreSQL | Policies | Yes | $200 |
| **4** | ElastiCache Redis | Policy cache | Yes | $300 |
| **5** | ElastiCache Redis | Rate limiting | Yes | $300 |
| **5** | Aurora PostgreSQL | Cost tracking | Yes | $200 |
| **6** | N/A | Regex in-memory | N/A | $0 |
| **6** | Comprehend (optional) | ML PII detection | No | $100 |
| **7** | N/A | In-memory | N/A | $0 |
| **8** | Secrets Manager | Provider creds | Yes | $2 |
| **9** | VPC NAT Gateway | Internet egress | Yes | $50 |
| **10** | N/A | In-memory | N/A | $0 |
| **10** | Comprehend (optional) | Content moderation | No | $100 |
| **11** | SQS | Async audit queue | Yes | $10 |
| **11** | Aurora PostgreSQL | Audit logs (30d) | Yes | $200 |
| **11** | S3 | Long-term archive | Yes | $25 |
| **11** | EventBridge | Scheduled archival | Yes | $5 |
| **12** | CloudFront (optional) | Frontend CDN | No | $850 |
| **TOTAL** | | | | **~$22,352/month** |

**Cost breakdown:**
- Core infrastructure (no RAG): ~$21,852/month
- RAG addition (PostgreSQL FTS): +$0/month (reuses Aurora)
- RAG addition (OpenSearch): +$500/month (Phase 2)

---

## 5. Performance Budgets

### 5.1 Latency Targets (P95)

**Without RAG:**
```
Stage 1:  Authentication                     20ms
Stage 2:  Identification                      2ms
Stage 3:  Normalization                       5ms
Stage 4:  Policy Resolution                  50ms
Stage 5:  Budget & Rate Check               30ms
Stage 6:  Pre-Processing Validation         20ms
Stage 7:  Content Filtering                 10ms
Stage 8:  Provider Routing                  20ms
Stage 9:  LLM Invocation               500-2000ms ◄── Dominant
Stage 10: Response Inspection               30ms
Stage 11: Audit (async)                    N/A
Stage 12: Response Delivery                 10ms
─────────────────────────────────────────────────
Total:                                 697-2197ms
```

**With RAG (Conditional - 20% of requests):**
```
Stage 1:  Authentication                     20ms
Stage 2:  Identification                      2ms
Stage 3:  Normalization                       5ms
Stage 3b: RAG Context Retrieval          30-80ms ◄── NEW
  - Pre-RAG validation                       5ms
  - PostgreSQL full-text search          20-60ms
  - Post-RAG validation                     5-15ms
Stage 4:  Policy Resolution                  50ms
Stage 5:  Budget & Rate Check               30ms
Stage 6:  Pre-Processing Validation         20ms
Stage 7:  Content Filtering                 10ms
Stage 8:  Provider Routing                  20ms
Stage 9:  LLM Invocation               500-2000ms
Stage 10: Response Inspection               40ms ◄── +10ms for RAG checks
Stage 11: Audit (async)                    N/A
Stage 12: Response Delivery                 10ms
─────────────────────────────────────────────────
Total with RAG:                        737-2287ms (+5-10%)
```

**Average latency (mixed workload):**
- 80% requests without RAG: 697-2197ms
- 20% requests with RAG: 737-2287ms
- **Weighted average: 705-2215ms** (only +8-18ms impact)

### 5.2 Token Budget (with RAG)

**Example request:**
```
User prompt:                     50 tokens
RAG context (3 documents):    1,100 tokens
Total input:                  1,150 tokens
LLM response:                   150 tokens
─────────────────────────────────────────
Total:                        1,300 tokens

Cost (GPT-4):
  Input:  1,150 tokens × $30/1M = $0.0345
  Output:   150 tokens × $60/1M = $0.0090
  Total:                          $0.0435
```

**Without RAG (same request):**
```
User prompt:      50 tokens
LLM response:    150 tokens
Total:           200 tokens

Cost (GPT-4):
  Input:   50 × $30/1M = $0.0015
  Output: 150 × $60/1M = $0.0090
  Total:                 $0.0105

RAG increased cost by: $0.033 (314% increase)
```

**Implication:** RAG significantly increases token costs due to context injection

### 5.3 Throughput Impact

| Scenario | Requests/Second | Notes |
|----------|-----------------|-------|
| **No RAG** | 1000 req/sec | Lambda can handle |
| **100% RAG** | 800 req/sec | PostgreSQL becomes bottleneck |
| **20% RAG (conditional)** | 980 req/sec | Minimal impact |

**Bottleneck with RAG:**
- Aurora PostgreSQL full-text search: ~200 queries/sec per vCPU
- With 2 vCPUs (1 ACU): ~400 queries/sec
- **Solution:** Scale Aurora or use OpenSearch

---

## 6. Updated Data Model for RAG

### 6.1 Documents Table

```sql
-- Documents for RAG (knowledge base)
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    
    -- Content
    title VARCHAR(500) NOT NULL,
    content TEXT NOT NULL,
    content_type VARCHAR(50), -- "markdown", "pdf", "html"
    
    -- Metadata
    source VARCHAR(100), -- "upload", "api", "crawler"
    author_id UUID REFERENCES users(id),
    tags TEXT[],
    
    -- Access control
    access_level VARCHAR(50) NOT NULL DEFAULT 'internal',
    -- Values: 'public', 'internal', 'confidential', 'restricted'
    
    -- Full-text search
    search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector('english', coalesce(title, '') || ' ' || coalesce(content, ''))
    ) STORED,
    
    -- Vector embedding (for Phase 2 with pgvector extension)
    -- embedding_vector vector(1536), -- Uncomment when adding OpenSearch
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    indexed_at TIMESTAMP
);

-- Indexes
CREATE INDEX idx_documents_org_id ON documents(org_id);
CREATE INDEX idx_documents_access_level ON documents(access_level);
CREATE INDEX idx_documents_search_vector ON documents USING GIN(search_vector);
CREATE INDEX idx_documents_tags ON documents USING GIN(tags);

-- Full-text search function
CREATE FUNCTION search_documents(query_text TEXT, org_filter UUID, access_levels TEXT[])
RETURNS TABLE(
    doc_id UUID,
    title VARCHAR,
    snippet TEXT,
    relevance REAL
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        id,
        documents.title,
        substring(content from 1 for 500),
        ts_rank(search_vector, plainto_tsquery('english', query_text))::REAL
    FROM documents
    WHERE org_id = org_filter
      AND search_vector @@ plainto_tsquery('english', query_text)
      AND access_level = ANY(access_levels)
    ORDER BY ts_rank(search_vector, plainto_tsquery('english', query_text)) DESC
    LIMIT 5;
END;
$$ LANGUAGE plpgsql;
```

### 6.2 RAG Metrics Table

```sql
-- Track RAG usage and performance
CREATE TABLE rag_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL, -- Links to request_logs
    org_id UUID NOT NULL,
    
    -- Query
    query TEXT NOT NULL,
    query_tokens INT,
    
    -- Retrieval
    documents_searched INT,
    documents_retrieved INT,
    documents_filtered INT,
    search_method VARCHAR(50), -- "postgres_fts", "opensearch_vector"
    search_latency_ms INT,
    
    -- Results
    top_score DECIMAL(4, 3),
    context_tokens INT,
    
    -- Validation
    pre_rag_passed BOOLEAN,
    post_rag_passed BOOLEAN,
    pii_found_in_docs BOOLEAN,
    secrets_found_in_docs BOOLEAN,
    
    timestamp TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_rag_metrics_org_id ON rag_metrics(org_id);
CREATE INDEX idx_rag_metrics_timestamp ON rag_metrics(timestamp DESC);
```

### 6.3 Updated Policies Table

```sql
-- Add RAG-specific policies
ALTER TABLE policies ADD COLUMN IF NOT EXISTS rag_config JSONB;

-- Example RAG policy:
INSERT INTO policies (org_id, policy_type, config) VALUES (
    'org-123',
    'rag',
    '{
        "enabled": true,
        "max_contexts": 3,
        "max_context_tokens": 2000,
        "min_relevance_score": 0.7,
        "allowed_sources": ["knowledge_base", "documentation"],
        "fail_on_error": false
    }'
);
```

---

## 7. Implementation Checklist

### Phase 1: Core Pipeline (Weeks 1-4) - NO RAG

```
✅ Stage 1: Authentication
✅ Stage 2: Identification  
✅ Stage 3: Normalization
✅ Stage 4: Policy Resolution (without RAG policies)
✅ Stage 5: Budget & Rate Check
✅ Stage 6: Validation
✅ Stage 7: Content Filtering
✅ Stage 8: Provider Routing
✅ Stage 9: LLM Invocation (OpenAI only)
✅ Stage 10: Response Inspection
✅ Stage 11: Audit Logging
✅ Stage 12: Response Delivery
```

**Deliverable:** Working governance pipeline without RAG

---

### Phase 2: Add RAG (Weeks 5-8)

```
✅ Create documents table
✅ Implement document upload API
✅ Stage 3b: RAG Context Retrieval
  ├─ Pre-RAG validation
  ├─ PostgreSQL full-text search
  └─ Post-RAG validation
✅ Update Stage 6: Validate augmented prompts
✅ Update Stage 10: Check for RAG leakage
✅ Update Stage 11: Log RAG metrics
✅ Add RAG policies to policy engine
```

**Deliverable:** RAG-enabled pipeline with conditional retrieval

---

### Phase 3: Advanced RAG (Weeks 9-12) - OPTIONAL

```
⬜ Migrate to OpenSearch Serverless (vector search)
⬜ Implement embedding generation (Bedrock or OpenAI)
⬜ Document chunking strategy
⬜ Hybrid search (keyword + semantic)
⬜ RAG cache (for repeated queries)
⬜ Document versioning
```

---

## 8. Key Changes from v1.0

| Change | Reason |
|--------|--------|
| ❌ **Removed Stage 10: Circuit Breaker** | Complexity reduction for MVP; can add later |
| ❌ **Removed Stage 5: Cache Check** | Optimization, not critical for MVP |
| ❌ **Removed Stage 13: Cache Store** | Optimization, not critical for MVP |
| ✅ **Added Stage 3b: RAG Context Retrieval** | Value-add feature with Pre/Post validation |
| 🔄 **Updated Stage 6:** | Now validates augmented prompts (user + RAG) |
| 🔄 **Updated Stage 10:** | Now includes RAG leakage detection |
| 🔄 **Updated Stage 11:** | Now logs RAG metrics |
| 📝 **Renumbered stages** | Removed gaps (5, 10, 13) |

---

## 9. Migration Path: PostgreSQL FTS → OpenSearch

### Week 1-8: PostgreSQL Full-Text Search

```go
// backend/internal/rag/postgres.go
type PostgresRAG struct {
    db *sql.DB
}

func (r *PostgresRAG) Retrieve(ctx context.Context, query string, orgID string) ([]RetrievedContext, error) {
    return r.fullTextSearch(ctx, query, orgID)
}
```

**Pros:** No new infrastructure, $0 cost  
**Cons:** Keyword-only, 60-70% accuracy

---

### Week 9+: OpenSearch Vector Search

```go
// backend/internal/rag/opensearch.go
type OpenSearchRAG struct {
    client *opensearch.Client
    bedrock *bedrock.Client
}

func (r *OpenSearchRAG) Retrieve(ctx context.Context, query string, orgID string) ([]RetrievedContext, error) {
    // 1. Generate embedding
    embedding, _ := r.bedrock.InvokeModel(ctx, &bedrock.InvokeModelInput{
        ModelId: aws.String("amazon.titan-embed-text-v1"),
        Body:    buildEmbeddingRequest(query),
    })
    
    // 2. Vector search
    return r.vectorSearch(ctx, embedding, orgID)
}
```

**Pros:** Semantic search, 85-95% accuracy  
**Cons:** +$500/month, requires migration

---

### Feature Flag Pattern

```go
// Graceful migration
type HybridRAG struct {
    postgres   *PostgresRAG
    opensearch *OpenSearchRAG
    useVector  bool // Feature flag
}

func (r *HybridRAG) Retrieve(ctx context.Context, query string, orgID string) ([]RetrievedContext, error) {
    if r.useVector && r.opensearch != nil {
        return r.opensearch.Retrieve(ctx, query, orgID)
    }
    return r.postgres.Retrieve(ctx, query, orgID)
}
```

**Deployment:**
```yaml
# Enable per organization
UPDATE organizations
SET features = jsonb_set(features, '{rag_vector_search}', 'true')
WHERE id = 'org-enterprise';
```

---

## 10. Quick Reference

### Pipeline Summary

| # | Stage | AWS | Latency | Critical |
|---|-------|-----|---------|----------|
| 1 | Authentication | Cognito | 20ms | Yes |
| 2 | Identification | - | 2ms | Yes |
| 3 | Normalization | - | 5ms | Yes |
| 3b | **RAG (conditional)** | Aurora | 30-80ms | Optional |
| 4 | Policy Resolution | Aurora+Redis | 50ms | Yes |
| 5 | Budget & Rate | Redis+Aurora | 30ms | Yes |
| 6 | Validation | - | 20ms | Yes |
| 7 | Content Filter | - | 10ms | Yes |
| 8 | Routing | Secrets | 20ms | Yes |
| 9 | LLM Invocation | NAT | 500-2000ms | Yes |
| 10 | Response Inspection | - | 40ms | Yes |
| 11 | Audit (async) | SQS+Aurora+S3 | N/A | Yes |
| 12 | Delivery | API Gateway | 10ms | Yes |

**Total:** 737-2287ms (with RAG), 697-2197ms (without)

---

**End of Pipeline Specification v2.0**

**Version:** 2.0  
**Last Updated:** February 6, 2026  
**Changes:** Simplified for MVP (removed Circuit Breaker, Caching), Added RAG with Pre/Post validation
