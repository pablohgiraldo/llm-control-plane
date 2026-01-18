package policy

// TODO: Define quota model and enforcement, backed by Redis and recorded in Postgres.
type Quota struct {
	RequestsPerMinute int
	TokensPerDay      int
}

func CheckQuota(subject string) (bool, error) {
	// TODO: implement quota check
	return true, nil
}

