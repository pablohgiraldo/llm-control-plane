package auth

// TODO: JWT validation utilities (kid lookup, audience, issuer, expiry).
// Avoid provider-specific assumptions; rely on configurable OIDC metadata.

type Principal struct {
	Subject string
	Roles   []string
	Tenant  string
}

