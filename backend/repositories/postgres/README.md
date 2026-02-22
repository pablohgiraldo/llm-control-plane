# PostgreSQL Repository Layer

This package implements the PostgreSQL repository layer for the LLM Control Plane following the GrantPulse pattern.

## Architecture

The repository layer follows clean architecture principles:

- **Repository Pattern**: Each domain entity has a dedicated repository interface and implementation
- **Transaction Management**: Context-based transaction propagation using the GrantPulse pattern
- **Connection Pooling**: Efficient database connection management
- **Type Safety**: Strong typing with Go interfaces and structs

## Components

### Core Infrastructure

#### `connection.go`
- Database connection pool setup
- Health check functionality
- Schema initialization
- Migration support

```go
db, err := NewDB(cfg.Database, logger)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Initialize schema
if err := db.InitSchema(context.Background()); err != nil {
    log.Fatal(err)
}
```

#### `transaction.go`
- TransactionManager implementation
- Context-based transaction propagation
- Automatic commit/rollback

```go
txManager := NewTransactionManager(db, logger)

err := txManager.InTransaction(ctx, func(ctx context.Context, tx repositories.Transaction) error {
    // All repository operations within this function use the same transaction
    if err := policyRepo.Create(ctx, policy); err != nil {
        return err // Automatic rollback
    }
    if err := auditRepo.Insert(ctx, auditLog); err != nil {
        return err // Automatic rollback
    }
    return nil // Automatic commit
})
```

#### `factory.go`
- Repository factory for creating all repositories
- Centralized dependency injection

```go
factory, err := NewRepositoryFactory(cfg.Database, logger)
if err != nil {
    log.Fatal(err)
}
defer factory.Close()

repos := factory.NewRepositories()
txManager := factory.GetTransactionManager()
```

### Repository Implementations

#### `organization_repository.go`
- CRUD operations for organizations
- Cascade delete support
- Transaction support

#### `application_repository.go`
- Application management
- API key hash lookup
- Organization-scoped queries

#### `user_repository.go`
- User management
- Cognito integration
- Email and Cognito sub lookups

#### `policy_repository.go`
- Policy CRUD operations
- Hierarchical policy queries (org-level, app-level, user-level)
- GetApplicablePolicies for policy resolution

```go
// Get policies in priority order: user > app > org
policies, err := policyRepo.GetApplicablePolicies(ctx, orgID, appID, &userID)
```

#### `audit_repository.go`
- Audit log insertion (sync and async)
- Query by organization, application, user, action, date range
- Request ID tracking for traceability

```go
// Async insert for high-throughput scenarios
auditRepo.(*AuditRepository).AsyncInsert(auditLog)
```

#### `inference_request_repository.go`
- Inference request tracking
- Metrics aggregation
- Status-based queries

```go
metrics, err := inferenceRepo.GetMetrics(ctx, orgID, startTime, endTime)
```

## Database Schema

The schema includes:

- `organizations` - Multi-tenant organization table
- `applications` - Client applications with API keys
- `users` - Users authenticated via AWS Cognito
- `policies` - Policy configurations with hierarchical support
- `audit_logs` - Comprehensive audit trail
- `inference_requests` - LLM inference request tracking

### Key Features

- **UUID Primary Keys**: Using UUID v4 for distributed systems
- **Foreign Key Constraints**: Ensuring referential integrity
- **Cascade Deletes**: Automatic cleanup of related records
- **Indexes**: Optimized for common query patterns
- **JSONB Fields**: Flexible configuration and details storage

## Testing

### Running Tests

Tests require a PostgreSQL database. You can either:

1. **Use a local PostgreSQL instance**:
```bash
# Create test database
createdb llmcp_test
createuser -P llmcp_test  # password: llmcp_test

# Run tests
cd backend/repositories/postgres
go test -v
```

2. **Use Docker**:
```bash
docker run -d \
  --name postgres-test \
  -e POSTGRES_USER=llmcp_test \
  -e POSTGRES_PASSWORD=llmcp_test \
  -e POSTGRES_DB=llmcp_test \
  -p 5432:5432 \
  postgres:15-alpine

# Run tests
go test -v
```

3. **Use Testcontainers** (recommended for CI):
```go
// Update test setup to use testcontainers
import "github.com/testcontainers/testcontainers-go/modules/postgres"
```

### Test Coverage

Each repository has comprehensive tests covering:

- ✅ Create operations
- ✅ Read operations (by ID, by various filters)
- ✅ Update operations
- ✅ Delete operations
- ✅ Transaction commit and rollback
- ✅ Pagination
- ✅ Error cases

## Usage Examples

### Basic CRUD

```go
// Create factory
factory, err := NewRepositoryFactory(cfg.Database, logger)
if err != nil {
    log.Fatal(err)
}
defer factory.Close()

repos := factory.NewRepositories()

// Create an organization
org := models.NewOrganization("Acme Corp", "acme-corp")
if err := repos.Organizations.Create(ctx, org); err != nil {
    log.Fatal(err)
}

// Retrieve by ID
retrieved, err := repos.Organizations.GetByID(ctx, org.ID)
```

### Transactions

```go
txManager := factory.GetTransactionManager()

err := txManager.InTransaction(ctx, func(ctx context.Context, tx repositories.Transaction) error {
    // Create policy
    policy := models.NewPolicy(orgID, models.PolicyTypeRateLimit, config, 10)
    if err := repos.Policies.Create(ctx, policy); err != nil {
        return err
    }
    
    // Create audit log
    auditLog := models.NewAuditLog(orgID, models.AuditActionPolicyCreated, "policy")
    auditLog.WithResource(policy.ID)
    if err := repos.AuditLogs.Insert(ctx, auditLog); err != nil {
        return err
    }
    
    return nil
})
```

### Hierarchical Queries

```go
// Get all applicable policies for a request
// Returns policies in priority order: user-level > app-level > org-level
policies, err := repos.Policies.GetApplicablePolicies(
    ctx,
    orgID,
    appID,
    &userID, // nil if no user context
)

for _, policy := range policies {
    // Apply policy in order
}
```

### Pagination

```go
// Get audit logs with pagination
logs, err := repos.AuditLogs.GetByOrgID(ctx, orgID, limit, offset)
```

### Metrics

```go
// Get inference metrics for date range
metrics, err := repos.InferenceRequests.GetMetrics(
    ctx,
    orgID,
    time.Now().Add(-24*time.Hour),
    time.Now(),
)

fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
fmt.Printf("Total Cost: $%.2f\n", metrics.TotalCost)
fmt.Printf("Avg Latency: %.2fms\n", metrics.AvgLatencyMs)
```

## Performance Considerations

### Connection Pool

Configure connection pool settings in your config:

```go
Database: DatabaseConfig{
    MaxOpenConns:    25,  // Maximum open connections
    MaxIdleConns:    5,   // Maximum idle connections
    ConnMaxLifetime: 5 * time.Minute,
}
```

### Indexes

The schema includes indexes on:
- Foreign keys (org_id, app_id, user_id)
- Lookup fields (cognito_sub, email, api_key_hash)
- Filter fields (status, action, enabled)
- Sort fields (timestamp, created_at, priority)

### Async Operations

For high-throughput scenarios, use async inserts:

```go
auditLog := models.NewAuditLog(orgID, action, resourceType)
if repo, ok := repos.AuditLogs.(*AuditRepository); ok {
    repo.AsyncInsert(auditLog)
}
```

## Best Practices

1. **Always use contexts**: Pass context to all repository methods for cancellation and timeout support

2. **Use transactions for multi-step operations**: Wrap related operations in transactions

3. **Handle errors appropriately**: Distinguish between not found errors and system errors

4. **Use prepared statements**: All queries use parameterized queries to prevent SQL injection

5. **Log appropriately**: Use structured logging with zap for debugging

6. **Test with real database**: Integration tests with actual PostgreSQL provide confidence

## Migration Strategy

For production migrations, use a migration tool like:

- [golang-migrate](https://github.com/golang-migrate/migrate)
- [goose](https://github.com/pressly/goose)
- [atlas](https://atlasgo.io/)

Example migration file structure:
```
migrations/
  001_initial_schema.up.sql
  001_initial_schema.down.sql
  002_add_inference_fields.up.sql
  002_add_inference_fields.down.sql
```

## Contributing

When adding new repositories:

1. Define the interface in `backend/repositories/interfaces.go`
2. Implement the repository in `backend/repositories/postgres/`
3. Add comprehensive tests
4. Update the factory to include the new repository
5. Document usage examples

## License

Copyright © 2026 LLM Control Plane
