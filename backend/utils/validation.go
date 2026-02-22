package utils

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

var (
	// validate is the singleton validator instance
	validate *validator.Validate

	// emailRegex is a simple email validation regex
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

func init() {
	validate = validator.New()
}

// ValidateStruct validates a struct using go-playground/validator
func ValidateStruct(s interface{}) error {
	if err := validate.Struct(s); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			return NewValidationError(validationErrors)
		}
		return err
	}
	return nil
}

// ValidationError wraps validation errors with structured details
type ValidationError struct {
	Message string
	Fields  map[string]string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a ValidationError from validator.ValidationErrors
func NewValidationError(errs validator.ValidationErrors) *ValidationError {
	fields := make(map[string]string)
	for _, err := range errs {
		field := err.Field()
		tag := err.Tag()
		
		switch tag {
		case "required":
			fields[field] = fmt.Sprintf("%s is required", field)
		case "email":
			fields[field] = fmt.Sprintf("%s must be a valid email", field)
		case "uuid":
			fields[field] = fmt.Sprintf("%s must be a valid UUID", field)
		case "min":
			fields[field] = fmt.Sprintf("%s must be at least %s", field, err.Param())
		case "max":
			fields[field] = fmt.Sprintf("%s must be at most %s", field, err.Param())
		case "gt":
			fields[field] = fmt.Sprintf("%s must be greater than %s", field, err.Param())
		case "gte":
			fields[field] = fmt.Sprintf("%s must be greater than or equal to %s", field, err.Param())
		case "lt":
			fields[field] = fmt.Sprintf("%s must be less than %s", field, err.Param())
		case "lte":
			fields[field] = fmt.Sprintf("%s must be less than or equal to %s", field, err.Param())
		case "oneof":
			fields[field] = fmt.Sprintf("%s must be one of: %s", field, err.Param())
		default:
			fields[field] = fmt.Sprintf("%s validation failed on '%s' tag", field, tag)
		}
	}
	
	return &ValidationError{
		Message: "Validation failed",
		Fields:  fields,
	}
}

// IsValidationError checks if an error is a ValidationError
func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}

// GetValidationFields extracts field errors from a ValidationError
func GetValidationFields(err error) map[string]string {
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Fields
	}
	return nil
}

// ValidateUUID validates that a string is a valid UUID
func ValidateUUID(s string) error {
	if _, err := uuid.Parse(s); err != nil {
		return fmt.Errorf("invalid UUID format: %s", s)
	}
	return nil
}

// ValidateEmail validates that a string is a valid email
func ValidateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format: %s", email)
	}
	return nil
}

// ValidateRequired validates that a string is not empty
func ValidateRequired(value string, fieldName string) error {
	if value == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}

// ValidateRequiredField validates that a pointer is not nil
func ValidateRequiredField(value interface{}, fieldName string) error {
	if value == nil {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}

// ValidateStringLength validates string length constraints
func ValidateStringLength(s string, fieldName string, min, max int) error {
	length := len(s)
	if min > 0 && length < min {
		return fmt.Errorf("%s must be at least %d characters", fieldName, min)
	}
	if max > 0 && length > max {
		return fmt.Errorf("%s must be at most %d characters", fieldName, max)
	}
	return nil
}

// ValidateNumericRange validates numeric range constraints
func ValidateNumericRange(value interface{}, fieldName string, min, max float64) error {
	var numValue float64
	
	switch v := value.(type) {
	case int:
		numValue = float64(v)
	case int32:
		numValue = float64(v)
	case int64:
		numValue = float64(v)
	case float32:
		numValue = float64(v)
	case float64:
		numValue = v
	default:
		return fmt.Errorf("%s must be a numeric value", fieldName)
	}
	
	if numValue < min {
		return fmt.Errorf("%s must be at least %v", fieldName, min)
	}
	if numValue > max {
		return fmt.Errorf("%s must be at most %v", fieldName, max)
	}
	return nil
}

// ValidateOneOf validates that a value is one of the allowed values
func ValidateOneOf(value string, fieldName string, allowed []string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of: %v", fieldName, allowed)
}
