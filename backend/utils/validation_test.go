package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestStruct struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
	Age   int    `validate:"required,gte=0,lte=150"`
}

func TestValidateStruct(t *testing.T) {
	t.Run("valid struct", func(t *testing.T) {
		s := TestStruct{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   30,
		}
		
		err := ValidateStruct(&s)
		assert.NoError(t, err)
	})
	
	t.Run("missing required field", func(t *testing.T) {
		s := TestStruct{
			Email: "john@example.com",
			Age:   30,
		}
		
		err := ValidateStruct(&s)
		assert.Error(t, err)
		assert.True(t, IsValidationError(err))
		
		fields := GetValidationFields(err)
		assert.Contains(t, fields, "Name")
	})
	
	t.Run("invalid email", func(t *testing.T) {
		s := TestStruct{
			Name:  "John Doe",
			Email: "invalid-email",
			Age:   30,
		}
		
		err := ValidateStruct(&s)
		assert.Error(t, err)
		assert.True(t, IsValidationError(err))
		
		fields := GetValidationFields(err)
		assert.Contains(t, fields, "Email")
	})
	
	t.Run("age out of range", func(t *testing.T) {
		s := TestStruct{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   200,
		}
		
		err := ValidateStruct(&s)
		assert.Error(t, err)
		assert.True(t, IsValidationError(err))
		
		fields := GetValidationFields(err)
		assert.Contains(t, fields, "Age")
	})
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name      string
		uuid      string
		wantError bool
	}{
		{
			name:      "valid UUID",
			uuid:      "550e8400-e29b-41d4-a716-446655440000",
			wantError: false,
		},
		{
			name:      "invalid UUID - wrong format",
			uuid:      "not-a-uuid",
			wantError: true,
		},
		{
			name:      "empty string",
			uuid:      "",
			wantError: true,
		},
		{
			name:      "invalid UUID - missing parts",
			uuid:      "550e8400-e29b-41d4",
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUUID(tt.uuid)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		wantError bool
	}{
		{
			name:      "valid email",
			email:     "user@example.com",
			wantError: false,
		},
		{
			name:      "valid email with subdomain",
			email:     "user@mail.example.com",
			wantError: false,
		},
		{
			name:      "valid email with plus",
			email:     "user+tag@example.com",
			wantError: false,
		},
		{
			name:      "invalid email - no @",
			email:     "userexample.com",
			wantError: true,
		},
		{
			name:      "invalid email - no domain",
			email:     "user@",
			wantError: true,
		},
		{
			name:      "invalid email - no TLD",
			email:     "user@example",
			wantError: true,
		},
		{
			name:      "empty string",
			email:     "",
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRequired(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		wantError bool
	}{
		{
			name:      "non-empty value",
			value:     "test",
			fieldName: "field",
			wantError: false,
		},
		{
			name:      "empty value",
			value:     "",
			fieldName: "field",
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequired(tt.value, tt.fieldName)
			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.fieldName)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRequiredField(t *testing.T) {
	t.Run("non-nil value", func(t *testing.T) {
		value := "test"
		err := ValidateRequiredField(value, "field")
		assert.NoError(t, err)
	})
	
	t.Run("nil value", func(t *testing.T) {
		err := ValidateRequiredField(nil, "field")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "field")
	})
}

func TestValidateStringLength(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		min       int
		max       int
		wantError bool
	}{
		{
			name:      "within range",
			value:     "test",
			fieldName: "field",
			min:       1,
			max:       10,
			wantError: false,
		},
		{
			name:      "too short",
			value:     "a",
			fieldName: "field",
			min:       3,
			max:       10,
			wantError: true,
		},
		{
			name:      "too long",
			value:     "this is a very long string",
			fieldName: "field",
			min:       1,
			max:       10,
			wantError: true,
		},
		{
			name:      "no min constraint",
			value:     "",
			fieldName: "field",
			min:       0,
			max:       10,
			wantError: false,
		},
		{
			name:      "no max constraint",
			value:     "very long string here that exceeds normal limits",
			fieldName: "field",
			min:       1,
			max:       0,
			wantError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStringLength(tt.value, tt.fieldName, tt.min, tt.max)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateNumericRange(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		fieldName string
		min       float64
		max       float64
		wantError bool
	}{
		{
			name:      "int within range",
			value:     5,
			fieldName: "field",
			min:       0,
			max:       10,
			wantError: false,
		},
		{
			name:      "int below min",
			value:     -5,
			fieldName: "field",
			min:       0,
			max:       10,
			wantError: true,
		},
		{
			name:      "int above max",
			value:     15,
			fieldName: "field",
			min:       0,
			max:       10,
			wantError: true,
		},
		{
			name:      "float within range",
			value:     5.5,
			fieldName: "field",
			min:       0.0,
			max:       10.0,
			wantError: false,
		},
		{
			name:      "float64 within range",
			value:     float64(7.5),
			fieldName: "field",
			min:       0.0,
			max:       10.0,
			wantError: false,
		},
		{
			name:      "int32 within range",
			value:     int32(5),
			fieldName: "field",
			min:       0,
			max:       10,
			wantError: false,
		},
		{
			name:      "int64 within range",
			value:     int64(5),
			fieldName: "field",
			min:       0,
			max:       10,
			wantError: false,
		},
		{
			name:      "invalid type",
			value:     "not a number",
			fieldName: "field",
			min:       0,
			max:       10,
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNumericRange(tt.value, tt.fieldName, tt.min, tt.max)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateOneOf(t *testing.T) {
	allowed := []string{"admin", "user", "guest"}
	
	tests := []struct {
		name      string
		value     string
		fieldName string
		wantError bool
	}{
		{
			name:      "valid value",
			value:     "admin",
			fieldName: "role",
			wantError: false,
		},
		{
			name:      "another valid value",
			value:     "user",
			fieldName: "role",
			wantError: false,
		},
		{
			name:      "invalid value",
			value:     "superuser",
			fieldName: "role",
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOneOf(tt.value, tt.fieldName, allowed)
			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.fieldName)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewValidationError(t *testing.T) {
	t.Run("creates validation error with field details", func(t *testing.T) {
		s := TestStruct{
			Email: "invalid-email",
			Age:   200,
		}
		
		err := ValidateStruct(&s)
		require.Error(t, err)
		
		validationErr, ok := err.(*ValidationError)
		require.True(t, ok)
		
		assert.Equal(t, "Validation failed", validationErr.Message)
		assert.NotEmpty(t, validationErr.Fields)
		assert.Contains(t, validationErr.Fields, "Name")
		assert.Contains(t, validationErr.Fields, "Email")
		assert.Contains(t, validationErr.Fields, "Age")
	})
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Message: "Test validation error",
		Fields: map[string]string{
			"field1": "error1",
		},
	}
	
	assert.Equal(t, "Test validation error", err.Error())
}

func TestIsValidationError(t *testing.T) {
	t.Run("is validation error", func(t *testing.T) {
		err := &ValidationError{
			Message: "test",
			Fields:  map[string]string{},
		}
		
		assert.True(t, IsValidationError(err))
	})
	
	t.Run("is not validation error", func(t *testing.T) {
		err := assert.AnError
		
		assert.False(t, IsValidationError(err))
	})
}

func TestGetValidationFields(t *testing.T) {
	t.Run("gets fields from validation error", func(t *testing.T) {
		fields := map[string]string{
			"field1": "error1",
			"field2": "error2",
		}
		err := &ValidationError{
			Message: "test",
			Fields:  fields,
		}
		
		extracted := GetValidationFields(err)
		assert.Equal(t, fields, extracted)
	})
	
	t.Run("returns nil for non-validation error", func(t *testing.T) {
		err := assert.AnError
		
		extracted := GetValidationFields(err)
		assert.Nil(t, extracted)
	})
}
