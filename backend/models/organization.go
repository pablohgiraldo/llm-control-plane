package models

import (
	"time"

	"github.com/google/uuid"
)

// Organization represents a tenant in the multi-tenant system
type Organization struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Slug      string    `json:"slug" db:"slug"` // URL-friendly identifier
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TableName returns the table name for the Organization model
func (Organization) TableName() string {
	return "organizations"
}

// NewOrganization creates a new Organization instance
func NewOrganization(name, slug string) *Organization {
	now := time.Now()
	return &Organization{
		ID:        uuid.New(),
		Name:      name,
		Slug:      slug,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
