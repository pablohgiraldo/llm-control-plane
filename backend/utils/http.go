package utils

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents a structured error response
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// SuccessResponse represents a generic success response
type SuccessResponse struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// WriteJSON writes a JSON response with the given status code
func WriteJSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	if data == nil {
		return nil
	}
	
	return json.NewEncoder(w).Encode(data)
}

// WriteOK writes a 200 OK response with optional data
func WriteOK(w http.ResponseWriter, data interface{}) error {
	return WriteJSON(w, http.StatusOK, SuccessResponse{Data: data})
}

// WriteCreated writes a 201 Created response with optional data
func WriteCreated(w http.ResponseWriter, data interface{}) error {
	return WriteJSON(w, http.StatusCreated, SuccessResponse{Data: data})
}

// WriteNoContent writes a 204 No Content response
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// WriteBadRequest writes a 400 Bad Request response with error details
func WriteBadRequest(w http.ResponseWriter, message string, details map[string]interface{}) error {
	return WriteJSON(w, http.StatusBadRequest, ErrorResponse{
		Error:   "bad_request",
		Message: message,
		Details: details,
	})
}

// WriteUnauthorized writes a 401 Unauthorized response
func WriteUnauthorized(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Authentication required"
	}
	return WriteJSON(w, http.StatusUnauthorized, ErrorResponse{
		Error:   "unauthorized",
		Message: message,
	})
}

// WriteForbidden writes a 403 Forbidden response
func WriteForbidden(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Access forbidden"
	}
	return WriteJSON(w, http.StatusForbidden, ErrorResponse{
		Error:   "forbidden",
		Message: message,
	})
}

// WriteNotFound writes a 404 Not Found response
func WriteNotFound(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Resource not found"
	}
	return WriteJSON(w, http.StatusNotFound, ErrorResponse{
		Error:   "not_found",
		Message: message,
	})
}

// WriteConflict writes a 409 Conflict response
func WriteConflict(w http.ResponseWriter, message string, details map[string]interface{}) error {
	return WriteJSON(w, http.StatusConflict, ErrorResponse{
		Error:   "conflict",
		Message: message,
		Details: details,
	})
}

// WriteTooManyRequests writes a 429 Too Many Requests response
func WriteTooManyRequests(w http.ResponseWriter, message string, details map[string]interface{}) error {
	if message == "" {
		message = "Rate limit exceeded"
	}
	return WriteJSON(w, http.StatusTooManyRequests, ErrorResponse{
		Error:   "rate_limit_exceeded",
		Message: message,
		Details: details,
	})
}

// WriteInternalServerError writes a 500 Internal Server Error response
func WriteInternalServerError(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Internal server error"
	}
	return WriteJSON(w, http.StatusInternalServerError, ErrorResponse{
		Error:   "internal_error",
		Message: message,
	})
}

// WriteError writes an error response based on the status code
func WriteError(w http.ResponseWriter, status int, message string, details map[string]interface{}) error {
	var errorType string
	switch status {
	case http.StatusBadRequest:
		errorType = "bad_request"
	case http.StatusUnauthorized:
		errorType = "unauthorized"
	case http.StatusForbidden:
		errorType = "forbidden"
	case http.StatusNotFound:
		errorType = "not_found"
	case http.StatusConflict:
		errorType = "conflict"
	case http.StatusTooManyRequests:
		errorType = "rate_limit_exceeded"
	default:
		errorType = "internal_error"
	}
	
	return WriteJSON(w, status, ErrorResponse{
		Error:   errorType,
		Message: message,
		Details: details,
	})
}
