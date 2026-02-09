package policy

import "context"

// Engine evaluates policies against incoming requests.
type Engine interface {
	Evaluate(ctx context.Context, req *EvaluationRequest) (*Decision, error)
}

// EvaluationRequest contains the context needed for policy evaluation.
type EvaluationRequest struct {
	OrgID  string
	AppID  string
	UserID string
	Model  string
	Tokens int
}

// Decision represents the result of policy evaluation.
type Decision struct {
	Allowed    bool
	Reason     string
	Violations []Violation
}

// Violation represents a specific policy violation.
type Violation struct {
	PolicyID string
	Type     ViolationType
	Message  string
}

// ViolationType categorizes policy violations.
type ViolationType string

const (
	ViolationRateLimit ViolationType = "rate_limit"
	ViolationCostCap   ViolationType = "cost_cap"
	ViolationQuota     ViolationType = "quota"
	ViolationModel     ViolationType = "model_restriction"
)

// Repository provides data access for policies.
type Repository interface {
	GetPoliciesForOrg(ctx context.Context, orgID string) ([]Policy, error)
}

// Policy represents a governance policy.
type Policy struct {
	ID     string
	OrgID  string
	Type   ViolationType
	Config map[string]interface{}
}
