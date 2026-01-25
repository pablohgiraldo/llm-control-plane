package providers

// Provider is the minimal interface adapters must implement.
// Core logic should not depend on specific vendors.
type Provider interface {
	// TODO: standardize request/response contracts to avoid provider leakage.
	Name() string
}

