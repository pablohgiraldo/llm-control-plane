package models

import (
	"time"

	"github.com/google/uuid"
)

// UserRole represents the role of a user within an organization
type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleMember UserRole = "member"
	RoleViewer UserRole = "viewer"
)

// User represents a user in the system authenticated via Cognito
type User struct {
	ID         uuid.UUID `json:"id" db:"id"`
	Email      string    `json:"email" db:"email"`
	CognitoSub string    `json:"cognito_sub" db:"cognito_sub"` // Cognito user identifier
	OrgID      uuid.UUID `json:"org_id" db:"org_id"`
	Role       UserRole  `json:"role" db:"role"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// TableName returns the table name for the User model
func (User) TableName() string {
	return "users"
}

// NewUser creates a new User instance
func NewUser(email, cognitoSub string, orgID uuid.UUID, role UserRole) *User {
	now := time.Now()
	return &User{
		ID:         uuid.New(),
		Email:      email,
		CognitoSub: cognitoSub,
		OrgID:      orgID,
		Role:       role,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// IsAdmin returns true if the user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// CanManagePolicies returns true if the user can manage policies
func (u *User) CanManagePolicies() bool {
	return u.Role == RoleAdmin || u.Role == RoleMember
}
