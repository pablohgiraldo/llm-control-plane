package storage

// TODO: Postgres access layer (schema migrations, queries).
// Keep interfaces explicit; ensure context usage and timeouts.
type Postgres struct {
	DSN string
}

func (p *Postgres) Connect() error {
	// TODO: implement connection init and health checks
	return nil
}

