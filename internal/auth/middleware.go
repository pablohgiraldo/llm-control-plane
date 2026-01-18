package auth

import "net/http"

// Middleware enforces authentication and RBAC prior to any pipeline processing.
// TODO: implement JWT/API key auth, attach principal to request context, enforce RBAC.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: validate and set authenticated principal in context
		next.ServeHTTP(w, r)
	})
}

