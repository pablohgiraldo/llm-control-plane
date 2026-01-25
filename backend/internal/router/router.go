package router

// TODO: Define routing strategies, SLO-aware selection, and multi-region awareness.
type RouteDecision struct {
	Provider string
	Model    string
}

type Context struct {
	Tenant       string
	RequestedModel string
	// TODO: add SLA, cost, compliance, and latency preferences
}

func Decide(ctx Context) (RouteDecision, error) {
	// TODO: implement decision logic
	return RouteDecision{Provider: "TODO", Model: ctx.RequestedModel}, nil
}

