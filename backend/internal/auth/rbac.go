package auth

// TODO: RBAC model and checks. Role/permission mapping should be data-driven.

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleUser   Role = "user"
	RoleViewer Role = "viewer"
)

func HasPermission(principal Principal, permission string) bool {
	// TODO: implement permission lookup
	return true
}

