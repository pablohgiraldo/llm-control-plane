package shared

import "errors"

var (
	ErrUnauthorized   = errors.New("unauthorized")
	ErrInvalidPrompt  = errors.New("invalid prompt")
	ErrPolicyDenied   = errors.New("policy denied")
	ErrRouteNotFound  = errors.New("route not found")
)

