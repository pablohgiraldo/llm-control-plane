package prompt

import (
	"regexp"
)

// SecretType represents different types of secrets that can be detected
type SecretType string

const (
	SecretTypeAPIKey           SecretType = "api_key"
	SecretTypeAWSKey           SecretType = "aws_key"
	SecretTypeGCPKey           SecretType = "gcp_key"
	SecretTypePassword         SecretType = "password"
	SecretTypeToken            SecretType = "token"
	SecretTypePrivateKey       SecretType = "private_key"
	SecretTypeJWT              SecretType = "jwt"
	SecretTypeSlackToken       SecretType = "slack_token"
	SecretTypeGitHubToken      SecretType = "github_token"
	SecretTypeStripeKey        SecretType = "stripe_key"
	SecretTypeOpenAIKey        SecretType = "openai_key"
	SecretTypeAnthropicKey     SecretType = "anthropic_key"
	SecretTypeDatabaseURL      SecretType = "database_url"
	SecretTypeConnectionString SecretType = "connection_string"
)

// SecretDetection represents a detected secret instance
type SecretDetection struct {
	Type        SecretType
	Value       string
	StartPos    int
	EndPos      int
	Confidence  float64 // 0.0 to 1.0
	Description string
}

var (
	// Generic API key patterns
	genericAPIKeyPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b[A-Za-z0-9_\-]{32,}\b`), // Generic long strings
		regexp.MustCompile(`(?i)api[_\-]?key[:\s=]+['"]?([A-Za-z0-9_\-]{20,})['"]?`),
		regexp.MustCompile(`(?i)apikey[:\s=]+['"]?([A-Za-z0-9_\-]{20,})['"]?`),
	}
	
	// AWS patterns
	awsAccessKeyPattern = regexp.MustCompile(`\b(AKIA[0-9A-Z]{16})\b`)
	awsSecretKeyPattern = regexp.MustCompile(`\b([A-Za-z0-9/+=]{40})\b`)
	
	// GCP patterns
	gcpAPIKeyPattern = regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{35}\b`)
	
	// Password patterns
	passwordPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)password[:\s=]+['"]?([^\s'"]{8,})['"]?`),
		regexp.MustCompile(`(?i)passwd[:\s=]+['"]?([^\s'"]{8,})['"]?`),
		regexp.MustCompile(`(?i)pwd[:\s=]+['"]?([^\s'"]{8,})['"]?`),
	}
	
	// Token patterns
	tokenPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)token[:\s=]+['"]?([A-Za-z0-9_\-\.]{20,})['"]?`),
		regexp.MustCompile(`(?i)access[_\-]?token[:\s=]+['"]?([A-Za-z0-9_\-\.]{20,})['"]?`),
		regexp.MustCompile(`(?i)bearer\s+([A-Za-z0-9_\-\.]{20,})`),
	}
	
	// JWT pattern
	jwtPattern = regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b`)
	
	// Private key patterns
	privateKeyPatterns = []*regexp.Regexp{
		regexp.MustCompile(`-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----`),
		regexp.MustCompile(`-----BEGIN\s+OPENSSH\s+PRIVATE\s+KEY-----`),
		regexp.MustCompile(`-----BEGIN\s+EC\s+PRIVATE\s+KEY-----`),
		regexp.MustCompile(`-----BEGIN\s+DSA\s+PRIVATE\s+KEY-----`),
	}
	
	// Slack token patterns
	slackTokenPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\bxox[baprs]-[0-9]{10,13}-[0-9]{10,13}-[A-Za-z0-9]{24,}\b`),
		regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9]{10,48}\b`),
	}
	
	// GitHub token patterns
	githubTokenPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\bghp_[A-Za-z0-9]{36,}\b`), // Personal access token
		regexp.MustCompile(`\bgho_[A-Za-z0-9]{36,}\b`), // OAuth access token
		regexp.MustCompile(`\bghu_[A-Za-z0-9]{36,}\b`), // User-to-server token
		regexp.MustCompile(`\bghs_[A-Za-z0-9]{36,}\b`), // Server-to-server token
		regexp.MustCompile(`\bghr_[A-Za-z0-9]{36,}\b`), // Refresh token
	}
	
	// Stripe key patterns
	stripeKeyPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\bsk_live_[0-9a-zA-Z]{24,}\b`),
		regexp.MustCompile(`\brk_live_[0-9a-zA-Z]{24,}\b`),
		regexp.MustCompile(`\bsk_test_[0-9a-zA-Z]{24,}\b`),
	}
	
	// OpenAI key pattern
	openaiKeyPattern = regexp.MustCompile(`\bsk-[A-Za-z0-9]{48}\b`)
	
	// Anthropic key pattern
	anthropicKeyPattern = regexp.MustCompile(`\bsk-ant-[A-Za-z0-9\-]{95,}\b`)
	
	// Database URL patterns
	databaseURLPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(postgres|postgresql|mysql|mongodb|redis)://[^\s'"]+:[^\s'"]+@[^\s'"]+`),
		regexp.MustCompile(`(?i)jdbc:[^\s'"]+`),
	}
	
	// Connection string patterns
	connectionStringPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)Server=[^;]+;.*Password=[^;]+`),
		regexp.MustCompile(`(?i)Data\s+Source=[^;]+;.*Password=[^;]+`),
	}
)

// DetectSecrets detects all secrets in the given text
func DetectSecrets(text string) []SecretDetection {
	var detections []SecretDetection
	
	// Detect AWS keys
	matches := awsAccessKeyPattern.FindAllStringIndex(text, -1)
	for _, match := range matches {
		detections = append(detections, SecretDetection{
			Type:        SecretTypeAWSKey,
			Value:       text[match[0]:match[1]],
			StartPos:    match[0],
			EndPos:      match[1],
			Confidence:  0.95,
			Description: "AWS Access Key ID",
		})
	}
	
	// Detect GCP API keys
	matches = gcpAPIKeyPattern.FindAllStringIndex(text, -1)
	for _, match := range matches {
		detections = append(detections, SecretDetection{
			Type:        SecretTypeGCPKey,
			Value:       text[match[0]:match[1]],
			StartPos:    match[0],
			EndPos:      match[1],
			Confidence:  0.95,
			Description: "Google Cloud Platform API Key",
		})
	}
	
	// Detect JWT tokens
	matches = jwtPattern.FindAllStringIndex(text, -1)
	for _, match := range matches {
		detections = append(detections, SecretDetection{
			Type:        SecretTypeJWT,
			Value:       text[match[0]:match[1]],
			StartPos:    match[0],
			EndPos:      match[1],
			Confidence:  0.90,
			Description: "JSON Web Token",
		})
	}
	
	// Detect private keys
	for _, pattern := range privateKeyPatterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			detections = append(detections, SecretDetection{
				Type:        SecretTypePrivateKey,
				Value:       text[match[0]:match[1]],
				StartPos:    match[0],
				EndPos:      match[1],
				Confidence:  1.0,
				Description: "Private Key Header",
			})
		}
	}
	
	// Detect Slack tokens
	for _, pattern := range slackTokenPatterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			detections = append(detections, SecretDetection{
				Type:        SecretTypeSlackToken,
				Value:       text[match[0]:match[1]],
				StartPos:    match[0],
				EndPos:      match[1],
				Confidence:  0.95,
				Description: "Slack Token",
			})
		}
	}
	
	// Detect GitHub tokens
	for _, pattern := range githubTokenPatterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			detections = append(detections, SecretDetection{
				Type:        SecretTypeGitHubToken,
				Value:       text[match[0]:match[1]],
				StartPos:    match[0],
				EndPos:      match[1],
				Confidence:  0.95,
				Description: "GitHub Token",
			})
		}
	}
	
	// Detect Stripe keys
	for _, pattern := range stripeKeyPatterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			detections = append(detections, SecretDetection{
				Type:        SecretTypeStripeKey,
				Value:       text[match[0]:match[1]],
				StartPos:    match[0],
				EndPos:      match[1],
				Confidence:  0.95,
				Description: "Stripe API Key",
			})
		}
	}
	
	// Detect OpenAI keys
	matches = openaiKeyPattern.FindAllStringIndex(text, -1)
	for _, match := range matches {
		detections = append(detections, SecretDetection{
			Type:        SecretTypeOpenAIKey,
			Value:       text[match[0]:match[1]],
			StartPos:    match[0],
			EndPos:      match[1],
			Confidence:  0.90,
			Description: "OpenAI API Key",
		})
	}
	
	// Detect Anthropic keys
	matches = anthropicKeyPattern.FindAllStringIndex(text, -1)
	for _, match := range matches {
		detections = append(detections, SecretDetection{
			Type:        SecretTypeAnthropicKey,
			Value:       text[match[0]:match[1]],
			StartPos:    match[0],
			EndPos:      match[1],
			Confidence:  0.95,
			Description: "Anthropic API Key",
		})
	}
	
	// Detect database URLs
	for _, pattern := range databaseURLPatterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			detections = append(detections, SecretDetection{
				Type:        SecretTypeDatabaseURL,
				Value:       text[match[0]:match[1]],
				StartPos:    match[0],
				EndPos:      match[1],
				Confidence:  0.90,
				Description: "Database URL with credentials",
			})
		}
	}
	
	// Detect connection strings
	for _, pattern := range connectionStringPatterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			detections = append(detections, SecretDetection{
				Type:        SecretTypeConnectionString,
				Value:       text[match[0]:match[1]],
				StartPos:    match[0],
				EndPos:      match[1],
				Confidence:  0.85,
				Description: "Database connection string",
			})
		}
	}
	
	// Detect passwords (lower confidence due to potential false positives)
	for _, pattern := range passwordPatterns {
		matches := pattern.FindAllStringSubmatchIndex(text, -1)
		for _, match := range matches {
			if len(match) >= 4 {
				detections = append(detections, SecretDetection{
					Type:        SecretTypePassword,
					Value:       text[match[2]:match[3]],
					StartPos:    match[2],
					EndPos:      match[3],
					Confidence:  0.70,
					Description: "Password value",
				})
			}
		}
	}
	
	// Detect tokens
	for _, pattern := range tokenPatterns {
		matches := pattern.FindAllStringSubmatchIndex(text, -1)
		for _, match := range matches {
			if len(match) >= 4 {
				// Skip if already detected as a specific token type
				value := text[match[2]:match[3]]
				if !isAlreadyDetected(detections, match[2], match[3]) {
					detections = append(detections, SecretDetection{
						Type:        SecretTypeToken,
						Value:       value,
						StartPos:    match[2],
						EndPos:      match[3],
						Confidence:  0.65,
						Description: "Generic token",
					})
				}
			}
		}
	}
	
	// Remove duplicates and overlapping detections
	detections = deduplicateDetections(detections)
	
	return detections
}

// HasSecrets returns true if any secrets are detected
func HasSecrets(text string) bool {
	return len(DetectSecrets(text)) > 0
}

// RedactSecrets redacts all detected secrets
func RedactSecrets(text string, minConfidence float64) string {
	detections := DetectSecrets(text)
	
	// Filter by confidence
	var filtered []SecretDetection
	for _, d := range detections {
		if d.Confidence >= minConfidence {
			filtered = append(filtered, d)
		}
	}
	
	// Sort by position (descending) to avoid index shifts
	for i := 0; i < len(filtered); i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[i].StartPos < filtered[j].StartPos {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}
	
	result := text
	for _, detection := range filtered {
		redacted := getSecretRedactionString(detection.Type)
		result = result[:detection.StartPos] + redacted + result[detection.EndPos:]
	}
	
	return result
}

// getSecretRedactionString returns an appropriate redaction string for the secret type
func getSecretRedactionString(secretType SecretType) string {
	switch secretType {
	case SecretTypeAPIKey:
		return "[API_KEY_REDACTED]"
	case SecretTypeAWSKey:
		return "[AWS_KEY_REDACTED]"
	case SecretTypeGCPKey:
		return "[GCP_KEY_REDACTED]"
	case SecretTypePassword:
		return "[PASSWORD_REDACTED]"
	case SecretTypeToken:
		return "[TOKEN_REDACTED]"
	case SecretTypePrivateKey:
		return "[PRIVATE_KEY_REDACTED]"
	case SecretTypeJWT:
		return "[JWT_REDACTED]"
	case SecretTypeSlackToken:
		return "[SLACK_TOKEN_REDACTED]"
	case SecretTypeGitHubToken:
		return "[GITHUB_TOKEN_REDACTED]"
	case SecretTypeStripeKey:
		return "[STRIPE_KEY_REDACTED]"
	case SecretTypeOpenAIKey:
		return "[OPENAI_KEY_REDACTED]"
	case SecretTypeAnthropicKey:
		return "[ANTHROPIC_KEY_REDACTED]"
	case SecretTypeDatabaseURL:
		return "[DATABASE_URL_REDACTED]"
	case SecretTypeConnectionString:
		return "[CONNECTION_STRING_REDACTED]"
	default:
		return "[SECRET_REDACTED]"
	}
}

// isAlreadyDetected checks if a position range is already covered by existing detections
func isAlreadyDetected(detections []SecretDetection, start, end int) bool {
	for _, d := range detections {
		// Check for overlap
		if (start >= d.StartPos && start < d.EndPos) || (end > d.StartPos && end <= d.EndPos) {
			return true
		}
	}
	return false
}

// deduplicateDetections removes duplicate and overlapping detections, keeping higher confidence ones
func deduplicateDetections(detections []SecretDetection) []SecretDetection {
	if len(detections) <= 1 {
		return detections
	}
	
	// Sort by confidence (descending)
	sorted := make([]SecretDetection, len(detections))
	copy(sorted, detections)
	
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Confidence < sorted[j].Confidence {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	var result []SecretDetection
	for _, d := range sorted {
		overlap := false
		for _, existing := range result {
			// Check for significant overlap
			if hasSignificantOverlap(d, existing) {
				overlap = true
				break
			}
		}
		if !overlap {
			result = append(result, d)
		}
	}
	
	return result
}

// hasSignificantOverlap checks if two detections have significant overlap
func hasSignificantOverlap(a, b SecretDetection) bool {
	// Calculate overlap
	overlapStart := a.StartPos
	if b.StartPos > overlapStart {
		overlapStart = b.StartPos
	}
	
	overlapEnd := a.EndPos
	if b.EndPos < overlapEnd {
		overlapEnd = b.EndPos
	}
	
	if overlapStart >= overlapEnd {
		return false
	}
	
	overlapLen := overlapEnd - overlapStart
	aLen := a.EndPos - a.StartPos
	bLen := b.EndPos - b.StartPos
	
	// Consider it significant overlap if >50% of either detection overlaps
	return float64(overlapLen)/float64(aLen) > 0.5 || float64(overlapLen)/float64(bLen) > 0.5
}

// GetHighConfidenceSecrets returns only high-confidence secret detections
func GetHighConfidenceSecrets(text string, minConfidence float64) []SecretDetection {
	all := DetectSecrets(text)
	var filtered []SecretDetection
	
	for _, d := range all {
		if d.Confidence >= minConfidence {
			filtered = append(filtered, d)
		}
	}
	
	return filtered
}
