package prompt

import (
	"testing"
)

func TestDetectPII(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		expected bool
	}{
		{
			name:     "no PII",
			prompt:   "What is the weather like today?",
			expected: false,
		},
		{
			name:     "contains email",
			prompt:   "Contact me at john.doe@example.com for more info",
			expected: true,
		},
		{
			name:     "contains phone",
			prompt:   "Call me at 555-123-4567",
			expected: true,
		},
		{
			name:     "contains SSN",
			prompt:   "My SSN is 123-45-6789",
			expected: true,
		},
		{
			name:     "contains credit card",
			prompt:   "Use card 4532015112830366",
			expected: true,
		},
		{
			name:     "contains IP address",
			prompt:   "Server at 192.168.1.1",
			expected: true,
		},
		{
			name:     "multiple PII types",
			prompt:   "Email: user@test.com, Phone: 555-0123, IP: 10.0.0.1",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectPII(tt.prompt)
			if result != tt.expected {
				t.Errorf("DetectPII() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectAllPII(t *testing.T) {
	tests := []struct {
		name          string
		prompt        string
		expectedTypes []PIIType
		expectedCount int
	}{
		{
			name:          "no PII",
			prompt:        "Hello world",
			expectedTypes: []PIIType{},
			expectedCount: 0,
		},
		{
			name:          "single email",
			prompt:        "Contact: user@example.com",
			expectedTypes: []PIIType{PIITypeEmail},
			expectedCount: 1,
		},
		{
			name:          "multiple emails",
			prompt:        "Email user1@test.com or user2@test.org",
			expectedTypes: []PIIType{PIITypeEmail, PIITypeEmail},
			expectedCount: 2,
		},
		{
			name:          "phone number US format",
			prompt:        "Call (555) 123-4567",
			expectedTypes: []PIIType{PIITypePhone},
			expectedCount: 1,
		},
		{
			name:          "SSN with dashes",
			prompt:        "SSN: 123-45-6789",
			expectedTypes: []PIIType{PIITypeSSN},
			expectedCount: 1,
		},
		{
			name:          "valid credit card (Visa)",
			prompt:        "Card: 4532015112830366",
			expectedTypes: []PIIType{PIITypeCreditCard},
			expectedCount: 1,
		},
		{
			name:          "IPv4 address",
			prompt:        "Server: 192.168.1.100",
			expectedTypes: []PIIType{PIITypeIPAddress},
			expectedCount: 1,
		},
		{
			name:          "mixed PII types",
			prompt:        "Email: admin@test.com, Phone: 555-9999, IP: 10.0.0.1",
			expectedTypes: []PIIType{PIITypeEmail, PIITypePhone, PIITypeIPAddress},
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detections := DetectAllPII(tt.prompt)
			
			if len(detections) != tt.expectedCount {
				t.Errorf("DetectAllPII() found %d detections, want %d", len(detections), tt.expectedCount)
			}
			
			if len(tt.expectedTypes) > 0 {
				for i, expectedType := range tt.expectedTypes {
					if i >= len(detections) {
						t.Errorf("Expected detection %d with type %v, but only found %d detections", i, expectedType, len(detections))
						continue
					}
					if detections[i].Type != expectedType {
						t.Errorf("Detection %d: got type %v, want %v", i, detections[i].Type, expectedType)
					}
				}
			}
		})
	}
}

func TestRedactPII(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{
			name:     "no PII",
			prompt:   "Hello world",
			expected: "Hello world",
		},
		{
			name:     "redact email",
			prompt:   "Contact user@example.com for help",
			expected: "Contact [EMAIL_REDACTED] for help",
		},
		{
			name:     "redact phone",
			prompt:   "Call 555-123-4567 today",
			expected: "Call [PHONE_REDACTED] today",
		},
		{
			name:     "redact SSN",
			prompt:   "SSN: 123-45-6789",
			expected: "SSN: [SSN_REDACTED]",
		},
		{
			name:     "redact credit card",
			prompt:   "Card number: 4532015112830366",
			expected: "Card number: [CC_REDACTED]",
		},
		{
			name:     "redact IP",
			prompt:   "Server at 192.168.1.1",
			expected: "Server at [IP_REDACTED]",
		},
		{
			name:     "redact multiple",
			prompt:   "Email: admin@test.com, Phone: 555-1234",
			expected: "Email: [EMAIL_REDACTED], Phone: [PHONE_REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactPII(tt.prompt)
			if result != tt.expected {
				t.Errorf("RedactPII() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLuhnCheck(t *testing.T) {
	tests := []struct {
		name   string
		number string
		valid  bool
	}{
		{
			name:   "valid Visa",
			number: "4532015112830366",
			valid:  true,
		},
		{
			name:   "valid MasterCard",
			number: "5425233430109903",
			valid:  true,
		},
		{
			name:   "valid Amex",
			number: "374245455400126",
			valid:  true,
		},
		{
			name:   "invalid checksum",
			number: "4532015112830367",
			valid:  false,
		},
		{
			name:   "too short",
			number: "123456",
			valid:  false,
		},
		{
			name:   "too long",
			number: "12345678901234567890",
			valid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := luhnCheck(tt.number)
			if result != tt.valid {
				t.Errorf("luhnCheck(%s) = %v, want %v", tt.number, result, tt.valid)
			}
		})
	}
}

func TestLooksLikeSSN(t *testing.T) {
	tests := []struct {
		name  string
		ssn   string
		valid bool
	}{
		{
			name:  "valid SSN",
			ssn:   "123456789",
			valid: true,
		},
		{
			name:  "starts with 000",
			ssn:   "000123456",
			valid: false,
		},
		{
			name:  "middle 00",
			ssn:   "123001234",
			valid: false,
		},
		{
			name:  "ends with 0000",
			ssn:   "123450000",
			valid: false,
		},
		{
			name:  "starts with 666",
			ssn:   "666123456",
			valid: false,
		},
		{
			name:  "starts with 9",
			ssn:   "912345678",
			valid: false,
		},
		{
			name:  "wrong length",
			ssn:   "12345678",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := looksLikeSSN(tt.ssn)
			if result != tt.valid {
				t.Errorf("looksLikeSSN(%s) = %v, want %v", tt.ssn, result, tt.valid)
			}
		})
	}
}

func BenchmarkDetectPII(b *testing.B) {
	prompt := "Contact user@example.com at 555-123-4567 or visit 192.168.1.1"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectPII(prompt)
	}
}

func BenchmarkRedactPII(b *testing.B) {
	prompt := "Email: admin@test.com, Phone: 555-9999, SSN: 123-45-6789, Card: 4532015112830366"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RedactPII(prompt)
	}
}
