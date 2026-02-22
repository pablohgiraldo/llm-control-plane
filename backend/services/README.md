# LLM Control Plane - Service Layer

This directory contains the core service layer for the LLM Control Plane, following the GrantPulse pattern with repository injection and clean separation of concerns.

## Architecture Overview

The service layer implements the business logic and orchestrates operations between repositories, external services, and the API layer. All services follow these principles:

- **Repository Pattern**: Services depend on repository interfaces, not implementations
- **Transaction Management**: Database transactions are handled cleanly with helper functions
- **Error Handling**: Consistent error handling using domain-specific errors
- **Testing**: Comprehensive unit tests with mocked dependencies
- **Async Operations**: Non-blocking operations where appropriate (e.g., audit logging)

## Services

### 1. Transaction Service (`transaction.go`)

Helper functions for managing database transactions following the GrantPulse pattern.

**Features:**
- `WithTransaction`: Execute a function within a transaction
- `WithTransactionResult[T]`: Generic version that returns a value
- Automatic commit on success, rollback on error
- Panic recovery with rollback

**Usage:**
```go
err := services.WithTransaction(ctx, txManager, func(ctx context.Context, tx repositories.Transaction) error {
    // Your transactional operations here
    return nil
})
```

### 2. Policy Service (`policy/`)

Handles policy evaluation and management with hierarchical policy resolution and in-memory caching.

**Components:**
- `service.go`: Core policy evaluation and CRUD operations
- `cache.go`: In-memory LRU cache with TTL (Phase 1 - no Redis yet)

**Features:**
- Hierarchical policy fetching (org → app → user)
- Policy merging with priority resolution
- In-memory LRU cache with TTL
- Cache invalidation by org/app/user
- Background cache cleanup worker
- Policy validation by type (rate limit, budget, routing, PII, injection, RAG)

**Usage:**
```go
service := policy.NewPolicyService(policyRepo, cache)

// Evaluate policies for a request
result, err := service.Evaluate(ctx, &policy.EvaluationRequest{
    OrgID:  orgID,
    AppID:  appID,
    UserID: &userID,
})

// Start background cache cleanup
stopCh := make(chan struct{})
service.StartCacheCleanup(5*time.Minute, stopCh)
```

**Cache Configuration:**
- Default: 1000 entries, 5-minute TTL
- LRU eviction when capacity reached
- Thread-safe with sync.Map
- Stats and health monitoring

### 3. Rate Limit Service (`ratelimit/`)

PostgreSQL-based rate limiting with sliding window algorithm (Phase 1 - no Redis yet).

**Features:**
- Sliding window rate limiting
- Request and token-based limits
- Multiple time windows (minute, hour, day)
- Automatic cleanup of old records
- Background cleanup worker

**Rate Limit Types:**
- Requests per minute/hour/day
- Tokens per minute/hour/day

**Usage:**
```go
service := ratelimit.NewRateLimitService(db)

// Check rate limit
err := service.CheckRateLimit(ctx, orgID, appID, userID, rateLimitConfig)
if err != nil {
    // Rate limit exceeded
}

// Record request after passing check
err = service.RecordRequestLimit(ctx, orgID, appID, userID, rateLimitConfig)

// Start cleanup worker
go service.StartCleanupWorker(ctx, 1*time.Hour, 7) // Keep 7 days of data
```

### 4. Budget Service (`budget/`)

PostgreSQL-based budget tracking with daily and monthly periods (Phase 1 - no Redis yet).

**Features:**
- Upsert-based cost recording (atomic updates)
- Daily and monthly budget periods
- Cost per request limits
- Budget summaries and top spenders
- Remaining budget calculation
- Automatic cleanup of old records

**Usage:**
```go
service := budget.NewBudgetService(db)

// Check budget before request
err := service.CheckBudget(ctx, orgID, appID, userID, budgetConfig)
if err != nil {
    // Budget exceeded
}

// Record cost after request
err = service.RecordRequestCost(ctx, orgID, appID, userID, cost)

// Get remaining budget
remaining, err := service.GetRemainingBudget(ctx, orgID, appID, userID, budgetConfig)

// Get budget summary
summary, err := service.GetBudgetSummary(ctx, orgID, budget.PeriodDaily)
```

### 5. Audit Service (`audit/`)

Async audit logging with buffered channels and background workers.

**Features:**
- Non-blocking async logging
- Buffered channel with configurable size
- Multiple background workers
- Graceful shutdown with event flushing
- Batch logging support
- Health monitoring

**Usage:**
```go
service := audit.NewAuditService(auditRepo, &audit.Config{
    BufferSize: 1000,
    Workers:    3,
})

// Start the service
service.Start(ctx)
defer service.Stop(ctx)

// Log events asynchronously
log := models.NewAuditLog(orgID, models.AuditActionInferenceRequest, "inference_request")
err := service.LogInferenceRequest(log)

// Synchronous logging (when needed)
err = service.LogEventSync(ctx, log)

// Batch logging
batchLogger := service.NewBatchLogger()
batchLogger.Add(log1)
batchLogger.Add(log2)
batchLogger.Flush()
```

**Configuration:**
- Default: 1000 buffer size, 3 workers
- Configurable timeout for non-blocking send (100ms)
- Health checks based on buffer utilization

## Error Handling

All services use the domain error system defined in `errors.go`:

```go
// Creating errors
err := services.ErrRateLimitExceeded.WithDetail("remaining", 0)

// Checking error types
if services.IsRateLimitError(err) {
    // Handle rate limit error
}

// Wrapping errors
err := services.WrapInternal("failed to fetch policies", err)
```

**Error Types:**
- `ErrorTypeNotFound`: Resource not found
- `ErrorTypeValidation`: Invalid input or configuration
- `ErrorTypeUnauthorized`: Authentication failed
- `ErrorTypeForbidden`: Insufficient permissions
- `ErrorTypeRateLimit`: Rate limit exceeded
- `ErrorTypeBudget`: Budget exceeded
- `ErrorTypeConflict`: Duplicate resource or concurrent update
- `ErrorTypeInternal`: Internal server error
- `ErrorTypeExternal`: External provider error
- `ErrorTypePolicyViolation`: Policy violation detected

## Testing

All services have comprehensive test coverage:

```bash
# Run all service tests
cd backend
go test ./services/... -v

# Run specific service tests
go test ./services/policy -v
go test ./services/ratelimit -v
go test ./services/budget -v
go test ./services/audit -v
```

**Test Types:**
- Unit tests with mocked dependencies
- Integration tests (requires PostgreSQL - skipped if not available)
- Concurrent access tests
- Error scenario tests

## Phase 1 vs Phase 2

### Phase 1 (Current Implementation)

- **Policy Cache**: In-memory LRU cache with TTL
- **Rate Limiting**: PostgreSQL-based with sliding window
- **Budget Tracking**: PostgreSQL-based with upsert
- **Audit Logging**: Async with buffered channels

### Phase 2 (Future Enhancements)

- **Policy Cache**: Migrate to Redis for distributed caching
- **Rate Limiting**: Migrate to Redis for better performance
- **Budget Tracking**: Add Redis caching for frequently accessed budgets
- **Audit Logging**: Consider Kafka/streaming for high-volume scenarios

## Database Schema Requirements

### Rate Limit Records Table

```sql
CREATE TABLE rate_limit_records (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    app_id UUID,
    user_id UUID,
    rate_type VARCHAR(50) NOT NULL,      -- 'request' or 'token'
    window_type VARCHAR(50) NOT NULL,    -- 'minute', 'hour', 'day'
    amount INTEGER NOT NULL DEFAULT 1,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    INDEX idx_rate_limit_lookup (org_id, rate_type, window_type, timestamp),
    INDEX idx_rate_limit_app (app_id, timestamp),
    INDEX idx_rate_limit_user (user_id, timestamp)
);
```

### Budget Records Table

```sql
CREATE TABLE budget_records (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    app_id UUID,
    user_id UUID,
    period_type VARCHAR(50) NOT NULL,    -- 'daily', 'monthly'
    period_start TIMESTAMP NOT NULL,
    total_cost DECIMAL(10,4) NOT NULL DEFAULT 0,
    request_count INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, COALESCE(app_id, '00000000-0000-0000-0000-000000000000'::uuid), 
            COALESCE(user_id, '00000000-0000-0000-0000-000000000000'::uuid), 
            period_type, period_start),
    INDEX idx_budget_lookup (org_id, period_type, period_start),
    INDEX idx_budget_app (app_id, period_start),
    INDEX idx_budget_cost (total_cost DESC)
);
```

## Best Practices

1. **Transaction Management**: Always use `WithTransaction` for operations that need atomicity
2. **Error Handling**: Use domain errors and check error types appropriately
3. **Caching**: Invalidate caches when data changes
4. **Resource Cleanup**: Use defer for cleanup operations
5. **Context Propagation**: Always pass context through service calls
6. **Testing**: Mock repository interfaces, not implementations
7. **Graceful Shutdown**: Ensure services flush pending operations on shutdown

## Dependencies

- `github.com/google/uuid`: UUID generation
- `github.com/lib/pq`: PostgreSQL driver
- Database connection pool (managed by repository layer)

## Performance Considerations

### Policy Service
- Cache hit rate is critical for performance
- Monitor cache size and eviction rate
- Adjust TTL based on policy change frequency

### Rate Limit Service
- PostgreSQL performance degrades with large datasets
- Regular cleanup is essential
- Consider partitioning for high-volume scenarios
- Migration to Redis recommended for Phase 2

### Budget Service
- Upsert operations are atomic but can be slower
- Index on (org_id, period_type, period_start) is critical
- Cleanup old data regularly to maintain performance

### Audit Service
- Buffer size should match expected event volume
- Monitor pending event count
- Increase workers if buffer fills frequently
- Consider log aggregation service for Phase 2

## Monitoring and Observability

Each service provides metrics and health checks:

```go
// Policy service cache stats
stats := policyService.GetCacheStats()

// Audit service health
isHealthy := auditService.IsHealthy()
pendingCount := auditService.GetPendingCount()

// Rate limit current usage
usage, err := rateLimitService.GetCurrentUsage(ctx, key)
```

## Contributing

When adding new services:

1. Follow the repository pattern
2. Add comprehensive tests
3. Use domain errors consistently
4. Document public APIs
5. Consider async operations for non-critical paths
6. Add health checks and metrics
7. Update this README

## License

See main project LICENSE file.
