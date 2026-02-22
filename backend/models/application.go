package models

import (
	"time"

	"github.com/google/uuid"
)

// Application represents a client application that uses the LLM Control Plane
type Application struct {
	ID         uuid.UUID `json:"id" db:"id"`
	OrgID      uuid.UUID `json:"org_id" db:"org_id"`
	Name       string    `json:"name" db:"name"`
	APIKeyHash string    `json:"-" db:"api_key_hash"` // Never expose in JSON
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// TableName returns the table name for the Application model
func (Application) TableName() string {
	return "applications"
}

// NewApplication creates a new Application instance
func NewApplication(orgID uuid.UUID, name, apiKeyHash string) *Application {
	now := time.Now()
	return &Application{
		ID:         uuid.New(),
		OrgID:      orgID,
		Name:       name,
		APIKeyHash: apiKeyHash,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
