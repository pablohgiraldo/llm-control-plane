package prompt

// GuardAgainstPromptInjection returns a sanitized prompt if safe.
// TODO: enforce guardrails for system prompt leakage and data exfiltration patterns.
func GuardAgainstPromptInjection(prompt string) (string, error) {
	return prompt, nil
}

