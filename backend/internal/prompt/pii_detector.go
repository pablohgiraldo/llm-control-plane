package prompt

import (
	"regexp"
	"strings"
)

// PIIType represents different types of PII that can be detected
type PIIType string

const (
	PIITypeEmail      PIIType = "email"
	PIITypePhone      PIIType = "phone"
	PIITypeSSN        PIIType = "ssn"
	PIITypeCreditCard PIIType = "credit_card"
	PIITypeIPAddress  PIIType = "ip_address"
	PIITypePassport   PIIType = "passport"
	PIITypeDriverLicense PIIType = "driver_license"
)

// PIIDetection represents a detected PII instance
type PIIDetection struct {
	Type     PIIType
	Value    string
	StartPos int
	EndPos   int
}

var (
	// Email pattern - RFC 5322 simplified
	emailPattern = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Z|a-z]{2,}\b`)
	
	// Phone patterns - US and international formats
	phonePatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b(\+?1[-.]?)?\(?([0-9]{3})\)?[-.]?([0-9]{3})[-.]?([0-9]{4})\b`), // US
		regexp.MustCompile(`\b\+?[0-9]{1,4}[-.\s]?(\([0-9]{1,4}\)|[0-9]{1,4})[-.\s]?[0-9]{1,4}[-.\s]?[0-9]{1,9}\b`), // International
	}
	
	// SSN pattern - XXX-XX-XXXX or XXXXXXXXX
	ssnPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b`),
		regexp.MustCompile(`\b[0-9]{9}\b`),
	}
	
	// Credit card patterns - major card types
	creditCardPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b4[0-9]{12}(?:[0-9]{3})?\b`), // Visa
		regexp.MustCompile(`\b5[1-5][0-9]{14}\b`), // MasterCard
		regexp.MustCompile(`\b3[47][0-9]{13}\b`), // American Express
		regexp.MustCompile(`\b6(?:011|5[0-9]{2})[0-9]{12}\b`), // Discover
		regexp.MustCompile(`\b(?:2131|1800|35\d{3})\d{11}\b`), // JCB
	}
	
	// IP address patterns - IPv4 and IPv6
	ipPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`), // IPv4
		regexp.MustCompile(`\b(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}\b`), // IPv6
	}
	
	// Passport pattern - US passport format
	passportPattern = regexp.MustCompile(`\b[A-Z]{1,2}[0-9]{6,9}\b`)
	
	// Driver's license pattern - varies by state, simplified
	driverLicensePattern = regexp.MustCompile(`\b[A-Z]{1,2}[0-9]{5,8}\b`)
)

// DetectPII returns true if the prompt likely contains PII.
func DetectPII(prompt string) bool {
	detections := DetectAllPII(prompt)
	return len(detections) > 0
}

// DetectAllPII returns all PII detections in the prompt
func DetectAllPII(prompt string) []PIIDetection {
	var detections []PIIDetection
	
	// Detect emails
	matches := emailPattern.FindAllStringIndex(prompt, -1)
	for _, match := range matches {
		detections = append(detections, PIIDetection{
			Type:     PIITypeEmail,
			Value:    prompt[match[0]:match[1]],
			StartPos: match[0],
			EndPos:   match[1],
		})
	}
	
	// Detect phones
	for _, pattern := range phonePatterns {
		matches := pattern.FindAllStringIndex(prompt, -1)
		for _, match := range matches {
			detections = append(detections, PIIDetection{
				Type:     PIITypePhone,
				Value:    prompt[match[0]:match[1]],
				StartPos: match[0],
				EndPos:   match[1],
			})
		}
	}
	
	// Detect SSN
	for _, pattern := range ssnPatterns {
		matches := pattern.FindAllStringIndex(prompt, -1)
		for _, match := range matches {
			// Additional validation for 9-digit numbers
			if pattern == ssnPatterns[1] {
				value := prompt[match[0]:match[1]]
				if !looksLikeSSN(value) {
					continue
				}
			}
			detections = append(detections, PIIDetection{
				Type:     PIITypeSSN,
				Value:    prompt[match[0]:match[1]],
				StartPos: match[0],
				EndPos:   match[1],
			})
		}
	}
	
	// Detect credit cards
	for _, pattern := range creditCardPatterns {
		matches := pattern.FindAllStringIndex(prompt, -1)
		for _, match := range matches {
			value := prompt[match[0]:match[1]]
			if luhnCheck(value) {
				detections = append(detections, PIIDetection{
					Type:     PIITypeCreditCard,
					Value:    value,
					StartPos: match[0],
					EndPos:   match[1],
				})
			}
		}
	}
	
	// Detect IP addresses
	for _, pattern := range ipPatterns {
		matches := pattern.FindAllStringIndex(prompt, -1)
		for _, match := range matches {
			detections = append(detections, PIIDetection{
				Type:     PIITypeIPAddress,
				Value:    prompt[match[0]:match[1]],
				StartPos: match[0],
				EndPos:   match[1],
			})
		}
	}
	
	return detections
}

// RedactPII redacts all detected PII in the prompt
func RedactPII(prompt string) string {
	detections := DetectAllPII(prompt)
	
	// Sort detections by position (descending) to avoid index shifts
	for i := 0; i < len(detections); i++ {
		for j := i + 1; j < len(detections); j++ {
			if detections[i].StartPos < detections[j].StartPos {
				detections[i], detections[j] = detections[j], detections[i]
			}
		}
	}
	
	result := prompt
	for _, detection := range detections {
		redacted := getRedactionString(detection.Type)
		result = result[:detection.StartPos] + redacted + result[detection.EndPos:]
	}
	
	return result
}

// getRedactionString returns an appropriate redaction string for the PII type
func getRedactionString(piiType PIIType) string {
	switch piiType {
	case PIITypeEmail:
		return "[EMAIL_REDACTED]"
	case PIITypePhone:
		return "[PHONE_REDACTED]"
	case PIITypeSSN:
		return "[SSN_REDACTED]"
	case PIITypeCreditCard:
		return "[CC_REDACTED]"
	case PIITypeIPAddress:
		return "[IP_REDACTED]"
	case PIITypePassport:
		return "[PASSPORT_REDACTED]"
	case PIITypeDriverLicense:
		return "[DL_REDACTED]"
	default:
		return "[REDACTED]"
	}
}

// looksLikeSSN performs basic validation on a 9-digit number to check if it looks like an SSN
func looksLikeSSN(s string) bool {
	if len(s) != 9 {
		return false
	}
	
	// SSN cannot be all zeros in any group
	if s[:3] == "000" || s[3:5] == "00" || s[5:] == "0000" {
		return false
	}
	
	// SSN cannot start with 666
	if strings.HasPrefix(s, "666") {
		return false
	}
	
	// SSN cannot start with 9
	if strings.HasPrefix(s, "9") {
		return false
	}
	
	return true
}

// luhnCheck validates a credit card number using the Luhn algorithm
func luhnCheck(cardNumber string) bool {
	// Remove spaces and dashes
	cardNumber = strings.ReplaceAll(cardNumber, " ", "")
	cardNumber = strings.ReplaceAll(cardNumber, "-", "")
	
	if len(cardNumber) < 13 || len(cardNumber) > 19 {
		return false
	}
	
	sum := 0
	isSecond := false
	
	// Traverse from right to left
	for i := len(cardNumber) - 1; i >= 0; i-- {
		digit := int(cardNumber[i] - '0')
		
		if isSecond {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		
		sum += digit
		isSecond = !isSecond
	}
	
	return sum%10 == 0
}
