package policy

// TODO: Cost estimation rules to prevent overruns and enforce budgets.
type CostLimit struct {
	MaxUSDPerRequest float64
	MaxUSDPerDay     float64
}

func CheckCost(subject string, estimateUSD float64) (bool, error) {
	// TODO: implement cost checks against limits
	return true, nil
}

