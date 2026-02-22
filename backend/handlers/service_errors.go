package handlers

import (
	"net/http"

	"github.com/upb/llm-control-plane/backend/services"
	"github.com/upb/llm-control-plane/backend/utils"
	"go.uber.org/zap"
)

// HandleServiceError maps domain errors to HTTP responses
// Following the GrantPulse thin handler pattern
func HandleServiceError(w http.ResponseWriter, err error, logger *zap.Logger) {
	if err == nil {
		return
	}

	// Extract domain error details
	var domainErr *services.DomainError
	details := services.GetErrorDetails(err)
	
	// Map error type to HTTP status and response
	switch {
	case services.IsNotFoundError(err):
		if err := utils.WriteNotFound(w, err.Error()); err != nil {
			logger.Error("failed to write not found response", zap.Error(err))
		}

	case services.IsValidationError(err):
		if err := utils.WriteBadRequest(w, err.Error(), details); err != nil {
			logger.Error("failed to write bad request response", zap.Error(err))
		}

	case services.IsUnauthorizedError(err):
		if err := utils.WriteUnauthorized(w, err.Error()); err != nil {
			logger.Error("failed to write unauthorized response", zap.Error(err))
		}

	case services.IsForbiddenError(err):
		if err := utils.WriteForbidden(w, err.Error()); err != nil {
			logger.Error("failed to write forbidden response", zap.Error(err))
		}

	case services.IsRateLimitError(err):
		if err := utils.WriteTooManyRequests(w, err.Error(), details); err != nil {
			logger.Error("failed to write rate limit response", zap.Error(err))
		}

	case services.IsBudgetError(err):
		// Budget errors are mapped to 429 with specific details
		if err := utils.WriteTooManyRequests(w, err.Error(), details); err != nil {
			logger.Error("failed to write budget error response", zap.Error(err))
		}

	case services.IsConflictError(err):
		if err := utils.WriteConflict(w, err.Error(), details); err != nil {
			logger.Error("failed to write conflict response", zap.Error(err))
		}

	case services.IsPolicyViolationError(err):
		// Policy violations are mapped to 403 Forbidden
		if err := utils.WriteForbidden(w, err.Error()); err != nil {
			logger.Error("failed to write policy violation response", zap.Error(err))
		}

	case services.IsExternalError(err):
		// External provider errors are mapped to 502 Bad Gateway
		if err := utils.WriteJSON(w, http.StatusBadGateway, utils.ErrorResponse{
			Error:   "bad_gateway",
			Message: err.Error(),
			Details: details,
		}); err != nil {
			logger.Error("failed to write bad gateway response", zap.Error(err))
		}

	case services.IsInternalError(err):
		// Log internal errors but return generic message
		logger.Error("internal server error", zap.Error(err))
		if err := utils.WriteInternalServerError(w, "An internal error occurred"); err != nil {
			logger.Error("failed to write internal error response", zap.Error(err))
		}

	default:
		// Unknown error type - log and return internal error
		logger.Error("unhandled error type", 
			zap.Error(err),
			zap.String("error_type", string(services.GetErrorType(err))))
		if err := utils.WriteInternalServerError(w, "An unexpected error occurred"); err != nil {
			logger.Error("failed to write internal error response", zap.Error(err))
		}
	}
	
	// Log error details for debugging
	if domainErr != nil {
		logger.Debug("handled service error",
			zap.String("type", string(domainErr.Type)),
			zap.String("message", domainErr.Message),
			zap.Any("details", domainErr.Details))
	}
}

// HandleValidationError handles validation errors from request parsing
func HandleValidationError(w http.ResponseWriter, err error, logger *zap.Logger) {
	if utils.IsValidationError(err) {
		fields := utils.GetValidationFields(err)
		details := make(map[string]interface{})
		for k, v := range fields {
			details[k] = v
		}
		if err := utils.WriteBadRequest(w, "Validation failed", details); err != nil {
			logger.Error("failed to write validation error response", zap.Error(err))
		}
		return
	}
	
	// Generic validation error
	if err := utils.WriteBadRequest(w, err.Error(), nil); err != nil {
		logger.Error("failed to write validation error response", zap.Error(err))
	}
}
