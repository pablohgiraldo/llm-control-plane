package router

// TODO: Fallback logic for provider errors, timeouts, and SLA breaches.
// Implement circuit breakers and retries with backoff.
func Fallback(previous RouteDecision) RouteDecision {
	return RouteDecision{Provider: "TODO-FALLBACK", Model: previous.Model}
}

