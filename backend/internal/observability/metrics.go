package observability

import "context"

// Metrics collects application metrics.
type Metrics interface {
	RecordRequest(ctx context.Context, labels RequestLabels)
	RecordLatency(ctx context.Context, duration float64, labels RequestLabels)
	RecordTokens(ctx context.Context, input, output int, labels RequestLabels)
	RecordCost(ctx context.Context, cost float64, labels RequestLabels)
}

// RequestLabels contains metric dimensions.
type RequestLabels struct {
	OrgID    string
	Model    string
	Provider string
	Status   string
}

// TODO: Implement Prometheus metrics collector
