package policy

// Engine evaluates a set of policies for a request context.
// TODO: define policy model, evaluation order, and decision outputs.
type Engine struct{}

type Decision struct {
	Allowed bool
	Reason  string
}

func (e *Engine) Evaluate(ctx interface{}) (Decision, error) {
	// TODO: evaluate quota, cost, content, tenant constraints
	return Decision{Allowed: true, Reason: "TODO"}, nil
}

