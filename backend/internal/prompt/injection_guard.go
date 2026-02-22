package prompt

import (
	"fmt"
	"regexp"
	"strings"
)

// InjectionType represents different types of prompt injection attacks
type InjectionType string

const (
	InjectionTypeSystemPromptLeak InjectionType = "system_prompt_leak"
	InjectionTypeRoleManipulation InjectionType = "role_manipulation"
	InjectionTypeInstructionOverride InjectionType = "instruction_override"
	InjectionTypeDataExfiltration InjectionType = "data_exfiltration"
	InjectionTypeJailbreak InjectionType = "jailbreak"
	InjectionTypeDelimiterAttack InjectionType = "delimiter_attack"
	InjectionTypeEncodingAttack InjectionType = "encoding_attack"
)

// InjectionDetection represents a detected injection attempt
type InjectionDetection struct {
	Type        InjectionType
	Pattern     string
	Confidence  float64
	StartPos    int
	EndPos      int
	Description string
}

var (
	// System prompt leak patterns
	systemPromptLeakPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore\s+(previous|all|above|prior)\s+(instructions?|prompts?|commands?)`),
		regexp.MustCompile(`(?i)show\s+(me\s+)?(your|the)\s+(system|original|initial)\s+(prompt|instructions?)`),
		regexp.MustCompile(`(?i)what\s+(is|are|was|were)\s+(your|the)\s+(system|original|initial)\s+(prompt|instructions?)`),
		regexp.MustCompile(`(?i)reveal\s+(your|the)\s+(system|hidden|secret)\s+(prompt|instructions?)`),
		regexp.MustCompile(`(?i)print\s+(your|the)\s+system\s+(prompt|instructions?)`),
		regexp.MustCompile(`(?i)repeat\s+(your|the)\s+(system|original)\s+(prompt|instructions?)`),
	}
	
	// Role manipulation patterns
	roleManipulationPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(you|your)\s+(are|role|identity)\s+(now|is|changed)`),
		regexp.MustCompile(`(?i)assume\s+(the\s+)?(role|identity)\s+of`),
		regexp.MustCompile(`(?i)pretend\s+(to\s+)?be\s+(a|an)`),
		regexp.MustCompile(`(?i)act\s+as\s+(if\s+)?(you|you're|you\s+are)`),
		regexp.MustCompile(`(?i)from\s+now\s+on[,]?\s+(you|your)\s+(are|will)`),
		regexp.MustCompile(`(?i)new\s+(instructions?|role|personality)`),
	}
	
	// Instruction override patterns
	instructionOverridePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)disregard\s+(all|previous|above|any)\s+(instructions?|rules|commands?)`),
		regexp.MustCompile(`(?i)override\s+(all|previous|system)\s+(instructions?|rules|settings?)`),
		regexp.MustCompile(`(?i)cancel\s+(all|previous)\s+(instructions?|commands?)`),
		regexp.MustCompile(`(?i)reset\s+(to|your)\s+(default|factory)\s+(settings?|mode)`),
		regexp.MustCompile(`(?i)start\s+over\s+with\s+new\s+instructions?`),
		regexp.MustCompile(`(?i)forget\s+(everything|all\s+previous|what\s+you\s+learned)`),
	}
	
	// Data exfiltration patterns
	dataExfiltrationPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)execute\s+(this|the\s+following)\s+(code|script|command)`),
		regexp.MustCompile(`(?i)run\s+(this|the\s+following)\s+(code|script|command)`),
		regexp.MustCompile(`(?i)eval\s*\(`),
		regexp.MustCompile(`(?i)system\s*\(`),
		regexp.MustCompile(`(?i)exec\s*\(`),
		regexp.MustCompile(`(?i)import\s+(os|sys|subprocess|socket)`),
		regexp.MustCompile(`(?i)fetch\s+from\s+(http|https)://`),
		regexp.MustCompile(`(?i)send\s+(data|information|content)\s+to\s+(http|https)://`),
	}
	
	// Jailbreak patterns
	jailbreakPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)DAN\s+mode`), // Do Anything Now
		regexp.MustCompile(`(?i)developer\s+mode`),
		regexp.MustCompile(`(?i)jailbreak`),
		regexp.MustCompile(`(?i)unrestricted\s+mode`),
		regexp.MustCompile(`(?i)god\s+mode`),
		regexp.MustCompile(`(?i)evil\s+(mode|assistant)`),
		regexp.MustCompile(`(?i)without\s+(any|ethical|moral)\s+(restrictions?|limitations?|guidelines?)`),
	}
	
	// Delimiter attack patterns
	delimiterAttackPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(\[SYSTEM\]|\[\/SYSTEM\]|\[USER\]|\[\/USER\]|\[ASSISTANT\]|\[\/ASSISTANT\])`),
		regexp.MustCompile(`(<\|system\|>|<\|user\|>|<\|assistant\|>|<\|end\|>)`),
		regexp.MustCompile(`(###\s*(SYSTEM|USER|ASSISTANT|INSTRUCTION))`),
		regexp.MustCompile(`(\<\<\<.*?\>\>\>)`),
	}
	
	// Encoding attack patterns
	encodingAttackPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)base64\s*[:\s=]\s*[A-Za-z0-9+/]{20,}={0,2}`),
		regexp.MustCompile(`(?i)hex\s*[:\s=]\s*[0-9a-fA-F]{20,}`),
		regexp.MustCompile(`(?i)unicode\s*[:\s=]\s*\\u[0-9a-fA-F]{4}`),
		regexp.MustCompile(`(?:\\x[0-9a-fA-F]{2}){10,}`), // Multiple hex escapes
	}
)

// GuardAgainstPromptInjection returns a sanitized prompt if safe.
// Returns an error if injection attempts are detected.
func GuardAgainstPromptInjection(prompt string) (string, error) {
	detections := DetectInjections(prompt)
	
	// Filter for high-confidence detections
	var highConfidence []InjectionDetection
	for _, d := range detections {
		if d.Confidence >= 0.8 {
			highConfidence = append(highConfidence, d)
		}
	}
	
	if len(highConfidence) > 0 {
		return "", fmt.Errorf("potential prompt injection detected: %s (confidence: %.2f)", 
			highConfidence[0].Type, highConfidence[0].Confidence)
	}
	
	return prompt, nil
}

// DetectInjections detects all potential injection attempts in the prompt
func DetectInjections(prompt string) []InjectionDetection {
	var detections []InjectionDetection
	
	// Detect system prompt leaks
	for _, pattern := range systemPromptLeakPatterns {
		matches := pattern.FindAllStringIndex(prompt, -1)
		for _, match := range matches {
			detections = append(detections, InjectionDetection{
				Type:        InjectionTypeSystemPromptLeak,
				Pattern:     pattern.String(),
				Confidence:  0.9,
				StartPos:    match[0],
				EndPos:      match[1],
				Description: "Attempt to reveal system prompt",
			})
		}
	}
	
	// Detect role manipulation
	for _, pattern := range roleManipulationPatterns {
		matches := pattern.FindAllStringIndex(prompt, -1)
		for _, match := range matches {
			detections = append(detections, InjectionDetection{
				Type:        InjectionTypeRoleManipulation,
				Pattern:     pattern.String(),
				Confidence:  0.85,
				StartPos:    match[0],
				EndPos:      match[1],
				Description: "Attempt to manipulate AI role or identity",
			})
		}
	}
	
	// Detect instruction override
	for _, pattern := range instructionOverridePatterns {
		matches := pattern.FindAllStringIndex(prompt, -1)
		for _, match := range matches {
			detections = append(detections, InjectionDetection{
				Type:        InjectionTypeInstructionOverride,
				Pattern:     pattern.String(),
				Confidence:  0.9,
				StartPos:    match[0],
				EndPos:      match[1],
				Description: "Attempt to override system instructions",
			})
		}
	}
	
	// Detect data exfiltration
	for _, pattern := range dataExfiltrationPatterns {
		matches := pattern.FindAllStringIndex(prompt, -1)
		for _, match := range matches {
			detections = append(detections, InjectionDetection{
				Type:        InjectionTypeDataExfiltration,
				Pattern:     pattern.String(),
				Confidence:  0.95,
				StartPos:    match[0],
				EndPos:      match[1],
				Description: "Attempt to execute code or exfiltrate data",
			})
		}
	}
	
	// Detect jailbreak attempts
	for _, pattern := range jailbreakPatterns {
		matches := pattern.FindAllStringIndex(prompt, -1)
		for _, match := range matches {
			detections = append(detections, InjectionDetection{
				Type:        InjectionTypeJailbreak,
				Pattern:     pattern.String(),
				Confidence:  0.95,
				StartPos:    match[0],
				EndPos:      match[1],
				Description: "Known jailbreak pattern detected",
			})
		}
	}
	
	// Detect delimiter attacks
	for _, pattern := range delimiterAttackPatterns {
		matches := pattern.FindAllStringIndex(prompt, -1)
		for _, match := range matches {
			detections = append(detections, InjectionDetection{
				Type:        InjectionTypeDelimiterAttack,
				Pattern:     pattern.String(),
				Confidence:  0.8,
				StartPos:    match[0],
				EndPos:      match[1],
				Description: "Attempt to manipulate prompt delimiters",
			})
		}
	}
	
	// Detect encoding attacks
	for _, pattern := range encodingAttackPatterns {
		matches := pattern.FindAllStringIndex(prompt, -1)
		for _, match := range matches {
			detections = append(detections, InjectionDetection{
				Type:        InjectionTypeEncodingAttack,
				Pattern:     pattern.String(),
				Confidence:  0.7,
				StartPos:    match[0],
				EndPos:      match[1],
				Description: "Potential encoded payload detected",
			})
		}
	}
	
	return detections
}

// IsInjectionAttempt returns true if high-confidence injection is detected
func IsInjectionAttempt(prompt string) bool {
	detections := DetectInjections(prompt)
	for _, d := range detections {
		if d.Confidence >= 0.8 {
			return true
		}
	}
	return false
}

// SanitizePrompt attempts to sanitize the prompt by removing suspicious patterns
func SanitizePrompt(prompt string) string {
	detections := DetectInjections(prompt)
	
	// Sort by position (descending) to avoid index shifts
	for i := 0; i < len(detections); i++ {
		for j := i + 1; j < len(detections); j++ {
			if detections[i].StartPos < detections[j].StartPos {
				detections[i], detections[j] = detections[j], detections[i]
			}
		}
	}
	
	result := prompt
	for _, detection := range detections {
		if detection.Confidence >= 0.85 {
			// Replace with safe placeholder
			result = result[:detection.StartPos] + "[REMOVED]" + result[detection.EndPos:]
		}
	}
	
	return result
}

// GetInjectionRiskScore calculates an overall risk score (0.0 to 1.0)
func GetInjectionRiskScore(prompt string) float64 {
	detections := DetectInjections(prompt)
	
	if len(detections) == 0 {
		return 0.0
	}
	
	// Calculate weighted average confidence
	var totalConfidence float64
	var totalWeight float64
	
	for _, d := range detections {
		weight := 1.0
		// Higher weight for critical types
		switch d.Type {
		case InjectionTypeDataExfiltration, InjectionTypeJailbreak:
			weight = 2.0
		case InjectionTypeInstructionOverride, InjectionTypeSystemPromptLeak:
			weight = 1.5
		}
		
		totalConfidence += d.Confidence * weight
		totalWeight += weight
	}
	
	if totalWeight == 0 {
		return 0.0
	}
	
	score := totalConfidence / totalWeight
	if score > 1.0 {
		score = 1.0
	}
	
	return score
}

// ValidatePromptSafety performs comprehensive safety validation
func ValidatePromptSafety(prompt string, maxRiskScore float64) error {
	riskScore := GetInjectionRiskScore(prompt)
	
	if riskScore >= maxRiskScore {
		detections := DetectInjections(prompt)
		var types []string
		seen := make(map[InjectionType]bool)
		
		for _, d := range detections {
			if !seen[d.Type] {
				types = append(types, string(d.Type))
				seen[d.Type] = true
			}
		}
		
		return fmt.Errorf("prompt safety validation failed: risk score %.2f (threshold: %.2f), detected: %s",
			riskScore, maxRiskScore, strings.Join(types, ", "))
	}
	
	return nil
}
