package routing

import "context"

// Router selects the appropriate provider for a given request.
type Router interface {
	Route(ctx context.Context, req *RoutingRequest) (*RoutingDecision, error)
}

// RoutingRequest contains information needed for routing decisions.
type RoutingRequest struct {
	Model       string
	OrgID       string
	Preferences RoutingPreferences
}

// RoutingPreferences specifies routing behavior.
type RoutingPreferences struct {
	PreferredProvider string
	CostOptimized     bool
	LatencySensitive  bool
}

// RoutingDecision specifies which provider to use.
type RoutingDecision struct {
	ProviderName string
	Endpoint     string
	Fallbacks    []string
}

// Strategy defines how routing decisions are made.
type Strategy interface {
	Select(ctx context.Context, candidates []string) (string, error)
}
