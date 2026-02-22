package prompt

import (
	"context"
	"fmt"
	"strings"

	intprompt "github.com/upb/llm-control-plane/backend/internal/prompt"
)

// ValidationConfig holds configuration for prompt validation
type ValidationConfig struct {
	MaxLength           int
	MinLength           int
	EnablePIIDetection  bool
	EnableSecretDetection bool
	EnableInjectionGuard bool
	MaxInjectionRisk    float64
	RedactPII           bool
	RedactSecrets       bool
	SecretConfidence    float64
	StrictMode          bool
}

// DefaultValidationConfig returns a sensible default configuration
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		MaxLength:            10000,
		MinLength:            1,
		EnablePIIDetection:   true,
		EnableSecretDetection: true,
		EnableInjectionGuard:  true,
		MaxInjectionRisk:     0.7,
		RedactPII:            false,
		RedactSecrets:        false,
		SecretConfidence:     0.8,
		StrictMode:           false,
	}
}

// ValidationResult contains the results of prompt validation
type ValidationResult struct {
	Valid             bool
	Errors            []string
	Warnings          []string
	SanitizedPrompt   string
	PIIDetected       bool
	SecretsDetected   bool
	InjectionDetected bool
	InjectionRiskScore float64
	PIIDetections     []intprompt.PIIDetection
	SecretDetections  []SecretDetection
	InjectionDetections []intprompt.InjectionDetection
}

// PromptService handles comprehensive prompt validation
type PromptService struct {
	config ValidationConfig
}

// NewPromptService creates a new prompt service with the given configuration
func NewPromptService(config ValidationConfig) *PromptService {
	return &PromptService{
		config: config,
	}
}

// NewPromptServiceWithDefaults creates a new prompt service with default configuration
func NewPromptServiceWithDefaults() *PromptService {
	return NewPromptService(DefaultValidationConfig())
}

// Validate performs comprehensive validation on a prompt
func (s *PromptService) Validate(ctx context.Context, prompt string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:           true,
		Errors:          []string{},
		Warnings:        []string{},
		SanitizedPrompt: prompt,
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 1. Length validation
	if len(prompt) < s.config.MinLength {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("prompt too short: minimum %d characters", s.config.MinLength))
	}

	if len(prompt) > s.config.MaxLength {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("prompt too long: maximum %d characters", s.config.MaxLength))
	}

	// 2. PII detection
	if s.config.EnablePIIDetection {
		piiDetections := intprompt.DetectAllPII(prompt)
		if len(piiDetections) > 0 {
			result.PIIDetected = true
			result.PIIDetections = piiDetections

			if s.config.StrictMode {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("PII detected: %d instances", len(piiDetections)))
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("PII detected: %d instances", len(piiDetections)))
			}

			if s.config.RedactPII {
				result.SanitizedPrompt = intprompt.RedactPII(result.SanitizedPrompt)
			}
		}
	}

	// 3. Secrets detection
	if s.config.EnableSecretDetection {
		secretDetections := DetectSecrets(prompt)
		if len(secretDetections) > 0 {
			result.SecretsDetected = true
			result.SecretDetections = secretDetections

			// Count high-confidence secrets
			highConfCount := 0
			for _, d := range secretDetections {
				if d.Confidence >= s.config.SecretConfidence {
					highConfCount++
				}
			}

			if highConfCount > 0 {
				if s.config.StrictMode {
					result.Valid = false
					result.Errors = append(result.Errors, fmt.Sprintf("secrets detected: %d high-confidence instances", highConfCount))
				} else {
					result.Warnings = append(result.Warnings, fmt.Sprintf("secrets detected: %d high-confidence instances", highConfCount))
				}
			}

			if s.config.RedactSecrets {
				result.SanitizedPrompt = RedactSecrets(result.SanitizedPrompt, s.config.SecretConfidence)
			}
		}
	}

	// 4. Injection guard
	if s.config.EnableInjectionGuard {
		injectionDetections := intprompt.DetectInjections(prompt)
		riskScore := intprompt.GetInjectionRiskScore(prompt)
		
		result.InjectionDetections = injectionDetections
		result.InjectionRiskScore = riskScore

		if len(injectionDetections) > 0 {
			result.InjectionDetected = true
		}

		if riskScore >= s.config.MaxInjectionRisk {
			result.Valid = false
			result.Errors = append(result.Errors, 
				fmt.Sprintf("prompt injection risk too high: %.2f (max: %.2f)", riskScore, s.config.MaxInjectionRisk))
		} else if riskScore >= s.config.MaxInjectionRisk*0.5 {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("moderate injection risk detected: %.2f", riskScore))
		}
	}

	// 5. Additional validation rules
	if err := s.validateFormat(prompt, result); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
	}

	return result, nil
}

// ValidateMessages validates a list of messages (for chat completions)
func (s *PromptService) ValidateMessages(ctx context.Context, messages []Message) (*ValidationResult, error) {
	// Combine all message content for validation
	var combined strings.Builder
	for i, msg := range messages {
		if i > 0 {
			combined.WriteString("\n")
		}
		combined.WriteString(msg.Content)
	}

	return s.Validate(ctx, combined.String())
}

// ValidateResponse validates LLM response content
func (s *PromptService) ValidateResponse(ctx context.Context, content string) (*ValidationResult, error) {
	// Response validation typically focuses on:
	// 1. Secrets leakage
	// 2. PII leakage
	// 3. Appropriate length

	result := &ValidationResult{
		Valid:           true,
		Errors:          []string{},
		Warnings:        []string{},
		SanitizedPrompt: content,
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check secrets in response
	if s.config.EnableSecretDetection {
		secretDetections := DetectSecrets(content)
		if len(secretDetections) > 0 {
			result.SecretsDetected = true
			result.SecretDetections = secretDetections

			highConfCount := 0
			for _, d := range secretDetections {
				if d.Confidence >= s.config.SecretConfidence {
					highConfCount++
				}
			}

			if highConfCount > 0 {
				result.Warnings = append(result.Warnings, 
					fmt.Sprintf("response contains potential secrets: %d instances", highConfCount))
				
				if s.config.RedactSecrets {
					result.SanitizedPrompt = RedactSecrets(result.SanitizedPrompt, s.config.SecretConfidence)
				}
			}
		}
	}

	// Check PII in response
	if s.config.EnablePIIDetection {
		piiDetections := intprompt.DetectAllPII(content)
		if len(piiDetections) > 0 {
			result.PIIDetected = true
			result.PIIDetections = piiDetections
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("response contains PII: %d instances", len(piiDetections)))

			if s.config.RedactPII {
				result.SanitizedPrompt = intprompt.RedactPII(result.SanitizedPrompt)
			}
		}
	}

	// Length check
	if len(content) > s.config.MaxLength {
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("response length exceeds maximum: %d > %d", len(content), s.config.MaxLength))
	}

	return result, nil
}

// validateFormat performs additional format validation
func (s *PromptService) validateFormat(prompt string, result *ValidationResult) error {
	// Check for null bytes
	if strings.Contains(prompt, "\x00") {
		return fmt.Errorf("prompt contains null bytes")
	}

	// Check for excessive whitespace
	if strings.Count(prompt, " ") > len(prompt)/2 {
		result.Warnings = append(result.Warnings, "prompt contains excessive whitespace")
	}

	// Check for control characters (except common ones like \n, \r, \t)
	for _, r := range prompt {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return fmt.Errorf("prompt contains invalid control characters")
		}
	}

	return nil
}

// QuickValidate performs a fast validation check for common issues
func (s *PromptService) QuickValidate(prompt string) error {
	if len(prompt) < s.config.MinLength {
		return fmt.Errorf("prompt too short")
	}

	if len(prompt) > s.config.MaxLength {
		return fmt.Errorf("prompt too long")
	}

	if s.config.EnableInjectionGuard {
		if intprompt.IsInjectionAttempt(prompt) {
			return fmt.Errorf("potential injection attempt detected")
		}
	}

	return nil
}

// GetSafePrompt returns a sanitized version of the prompt
func (s *PromptService) GetSafePrompt(ctx context.Context, prompt string) (string, error) {
	result, err := s.Validate(ctx, prompt)
	if err != nil {
		return "", err
	}

	return result.SanitizedPrompt, nil
}

// Message represents a chat message
type Message struct {
	Role    string
	Content string
}
