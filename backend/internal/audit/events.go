package audit

// Event captures auditable actions in the pipeline.
// TODO: finalize schema; avoid storing raw secrets/PII.
type Event struct {
	TimestampMs int64
	Principal   string
	Action      string
	Tenant      string
	Status      string
	Metadata    map[string]string
}

