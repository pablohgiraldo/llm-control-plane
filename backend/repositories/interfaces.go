package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/models"
)

// TransactionManager manages database transactions following the GrantPulse pattern
type TransactionManager interface {
	// Begin starts a new transaction
	Begin(ctx context.Context) (Transaction, error)
	
	// InTransaction executes a function within a transaction
	// Automatically commits if function succeeds, rolls back on error
	InTransaction(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error
}

// Transaction represents a database transaction
type Transaction interface {
	// Commit commits the transaction
	Commit() error
	
	// Rollback rolls back the transaction
	Rollback() error
	
	// Context returns the transaction context
	Context() context.Context
}

// PolicyRepository handles policy data operations
type PolicyRepository interface {
	// Create creates a new policy
	Create(ctx context.Context, policy *models.Policy) error
	
	// GetByID retrieves a policy by ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.Policy, error)
	
	// GetByOrgID retrieves all policies for an organization
	GetByOrgID(ctx context.Context, orgID uuid.UUID) ([]*models.Policy, error)
	
	// GetByAppID retrieves all policies for an application
	GetByAppID(ctx context.Context, appID uuid.UUID) ([]*models.Policy, error)
	
	// GetByUserID retrieves all policies for a user
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Policy, error)
	
	// GetApplicablePolicies retrieves all applicable policies for a request
	// in priority order (org-level, app-level, user-level)
	GetApplicablePolicies(ctx context.Context, orgID, appID uuid.UUID, userID *uuid.UUID) ([]*models.Policy, error)
	
	// Update updates a policy
	Update(ctx context.Context, policy *models.Policy) error
	
	// Delete deletes a policy
	Delete(ctx context.Context, id uuid.UUID) error
	
	// WithTx returns a new repository instance bound to the transaction
	WithTx(tx Transaction) PolicyRepository
}

// AuditRepository handles audit log data operations
type AuditRepository interface {
	// Insert inserts a new audit log entry
	Insert(ctx context.Context, log *models.AuditLog) error
	
	// GetByID retrieves an audit log by ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error)
	
	// GetByOrgID retrieves audit logs for an organization with pagination
	GetByOrgID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.AuditLog, error)
	
	// GetByAppID retrieves audit logs for an application with pagination
	GetByAppID(ctx context.Context, appID uuid.UUID, limit, offset int) ([]*models.AuditLog, error)
	
	// GetByUserID retrieves audit logs for a user with pagination
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AuditLog, error)
	
	// GetByDateRange retrieves audit logs within a date range
	GetByDateRange(ctx context.Context, orgID uuid.UUID, start, end time.Time, limit, offset int) ([]*models.AuditLog, error)
	
	// GetByAction retrieves audit logs by action type
	GetByAction(ctx context.Context, orgID uuid.UUID, action models.AuditAction, limit, offset int) ([]*models.AuditLog, error)
	
	// GetByRequestID retrieves audit logs by request ID
	GetByRequestID(ctx context.Context, requestID string) ([]*models.AuditLog, error)
	
	// WithTx returns a new repository instance bound to the transaction
	WithTx(tx Transaction) AuditRepository
}

// OrganizationRepository handles organization data operations
type OrganizationRepository interface {
	// Create creates a new organization
	Create(ctx context.Context, org *models.Organization) error
	
	// GetByID retrieves an organization by ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error)
	
	// GetBySlug retrieves an organization by slug
	GetBySlug(ctx context.Context, slug string) (*models.Organization, error)
	
	// List retrieves all organizations with pagination
	List(ctx context.Context, limit, offset int) ([]*models.Organization, error)
	
	// Update updates an organization
	Update(ctx context.Context, org *models.Organization) error
	
	// Delete deletes an organization
	Delete(ctx context.Context, id uuid.UUID) error
	
	// WithTx returns a new repository instance bound to the transaction
	WithTx(tx Transaction) OrganizationRepository
}

// ApplicationRepository handles application data operations
type ApplicationRepository interface {
	// Create creates a new application
	Create(ctx context.Context, app *models.Application) error
	
	// GetByID retrieves an application by ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.Application, error)
	
	// GetByAPIKeyHash retrieves an application by API key hash
	GetByAPIKeyHash(ctx context.Context, apiKeyHash string) (*models.Application, error)
	
	// GetByOrgID retrieves all applications for an organization
	GetByOrgID(ctx context.Context, orgID uuid.UUID) ([]*models.Application, error)
	
	// Update updates an application
	Update(ctx context.Context, app *models.Application) error
	
	// Delete deletes an application
	Delete(ctx context.Context, id uuid.UUID) error
	
	// WithTx returns a new repository instance bound to the transaction
	WithTx(tx Transaction) ApplicationRepository
}

// UserRepository handles user data operations
type UserRepository interface {
	// Create creates a new user
	Create(ctx context.Context, user *models.User) error
	
	// GetByID retrieves a user by ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	
	// GetByCognitoSub retrieves a user by Cognito subject
	GetByCognitoSub(ctx context.Context, cognitoSub string) (*models.User, error)
	
	// GetByEmail retrieves a user by email
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	
	// GetByOrgID retrieves all users for an organization
	GetByOrgID(ctx context.Context, orgID uuid.UUID) ([]*models.User, error)
	
	// Update updates a user
	Update(ctx context.Context, user *models.User) error
	
	// Delete deletes a user
	Delete(ctx context.Context, id uuid.UUID) error
	
	// WithTx returns a new repository instance bound to the transaction
	WithTx(tx Transaction) UserRepository
}

// InferenceRequestRepository handles inference request data operations
type InferenceRequestRepository interface {
	// Create creates a new inference request
	Create(ctx context.Context, req *models.InferenceRequest) error
	
	// GetByID retrieves an inference request by ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.InferenceRequest, error)
	
	// GetByRequestID retrieves an inference request by external request ID
	GetByRequestID(ctx context.Context, requestID string) (*models.InferenceRequest, error)
	
	// GetByOrgID retrieves inference requests for an organization with pagination
	GetByOrgID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.InferenceRequest, error)
	
	// GetByAppID retrieves inference requests for an application with pagination
	GetByAppID(ctx context.Context, appID uuid.UUID, limit, offset int) ([]*models.InferenceRequest, error)
	
	// GetByUserID retrieves inference requests for a user with pagination
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.InferenceRequest, error)
	
	// GetByStatus retrieves inference requests by status
	GetByStatus(ctx context.Context, orgID uuid.UUID, status models.InferenceStatus, limit, offset int) ([]*models.InferenceRequest, error)
	
	// GetByDateRange retrieves inference requests within a date range
	GetByDateRange(ctx context.Context, orgID uuid.UUID, start, end time.Time, limit, offset int) ([]*models.InferenceRequest, error)
	
	// Update updates an inference request
	Update(ctx context.Context, req *models.InferenceRequest) error
	
	// GetMetrics retrieves aggregate metrics for an organization
	GetMetrics(ctx context.Context, orgID uuid.UUID, start, end time.Time) (*InferenceMetrics, error)
	
	// WithTx returns a new repository instance bound to the transaction
	WithTx(tx Transaction) InferenceRequestRepository
}

// InferenceMetrics represents aggregated inference metrics
type InferenceMetrics struct {
	TotalRequests     int     `json:"total_requests"`
	CompletedRequests int     `json:"completed_requests"`
	FailedRequests    int     `json:"failed_requests"`
	RejectedRequests  int     `json:"rejected_requests"`
	TotalTokens       int     `json:"total_tokens"`
	TotalCost         float64 `json:"total_cost"`
	AvgLatencyMs      float64 `json:"avg_latency_ms"`
}

// Repositories aggregates all repository interfaces
type Repositories struct {
	Organizations     OrganizationRepository
	Applications      ApplicationRepository
	Users             UserRepository
	Policies          PolicyRepository
	AuditLogs         AuditRepository
	InferenceRequests InferenceRequestRepository
}
