package ratelimit

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/models"
	"go.uber.org/zap"
)

// RateLimitWindow represents the time window for rate limiting
type RateLimitWindow string

const (
	WindowMinute RateLimitWindow = "minute"
	WindowHour   RateLimitWindow = "hour"
	WindowDay    RateLimitWindow = "day"
)

// RateLimitRequest represents a rate limit check request
type RateLimitRequest struct {
	OrgID    uuid.UUID
	AppID    uuid.UUID
	UserID   *uuid.UUID
	Config   *models.RateLimitConfig
	TokensUsed int
}

// RateLimitResult represents the result of a rate limit check
type RateLimitResult struct {
	Allowed              bool
	RequestsRemaining    int
	TokensRemaining      int
	ResetAt              time.Time
	ViolatedWindow       RateLimitWindow
	ViolationReason      string
}

// RateLimitService handles rate limiting using PostgreSQL
type RateLimitService struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewRateLimitService creates a new RateLimitService instance
func NewRateLimitService(db *sql.DB, logger *zap.Logger) *RateLimitService {
	return &RateLimitService{
		db:     db,
		logger: logger,
	}
}

// CheckLimit checks if the request is within rate limits
// Uses sliding window algorithm with PostgreSQL for Phase 1
func (s *RateLimitService) CheckLimit(ctx context.Context, req RateLimitRequest) (*RateLimitResult, error) {
	if req.Config == nil {
		// No rate limit configured
		return &RateLimitResult{
			Allowed: true,
		}, nil
	}

	// Build scope key for rate limiting
	scopeKey := s.buildScopeKey(req.OrgID, req.AppID, req.UserID)

	// Check each time window
	now := time.Now()

	// Check minute window
	if req.Config.RequestsPerMinute > 0 {
		allowed, remaining, resetAt, err := s.checkWindow(ctx, scopeKey, WindowMinute, now, req.Config.RequestsPerMinute)
		if err != nil {
			return nil, fmt.Errorf("failed to check minute window: %w", err)
		}
		if !allowed {
			return &RateLimitResult{
				Allowed:           false,
				RequestsRemaining: remaining,
				ResetAt:           resetAt,
				ViolatedWindow:    WindowMinute,
				ViolationReason:   fmt.Sprintf("exceeded %d requests per minute", req.Config.RequestsPerMinute),
			}, nil
		}
	}

	// Check hour window
	if req.Config.RequestsPerHour > 0 {
		allowed, remaining, resetAt, err := s.checkWindow(ctx, scopeKey, WindowHour, now, req.Config.RequestsPerHour)
		if err != nil {
			return nil, fmt.Errorf("failed to check hour window: %w", err)
		}
		if !allowed {
			return &RateLimitResult{
				Allowed:           false,
				RequestsRemaining: remaining,
				ResetAt:           resetAt,
				ViolatedWindow:    WindowHour,
				ViolationReason:   fmt.Sprintf("exceeded %d requests per hour", req.Config.RequestsPerHour),
			}, nil
		}
	}

	// Check day window
	if req.Config.RequestsPerDay > 0 {
		allowed, remaining, resetAt, err := s.checkWindow(ctx, scopeKey, WindowDay, now, req.Config.RequestsPerDay)
		if err != nil {
			return nil, fmt.Errorf("failed to check day window: %w", err)
		}
		if !allowed {
			return &RateLimitResult{
				Allowed:           false,
				RequestsRemaining: remaining,
				ResetAt:           resetAt,
				ViolatedWindow:    WindowDay,
				ViolationReason:   fmt.Sprintf("exceeded %d requests per day", req.Config.RequestsPerDay),
			}, nil
		}
	}

	// Check token limits (similar to request limits)
	if req.Config.TokensPerMinute > 0 {
		scopeKeyTokens := scopeKey + ":tokens"
		allowed, remaining, resetAt, err := s.checkWindow(ctx, scopeKeyTokens, WindowMinute, now, req.Config.TokensPerMinute)
		if err != nil {
			return nil, fmt.Errorf("failed to check token minute window: %w", err)
		}
		if !allowed {
			return &RateLimitResult{
				Allowed:         false,
				TokensRemaining: remaining,
				ResetAt:         resetAt,
				ViolatedWindow:  WindowMinute,
				ViolationReason: fmt.Sprintf("exceeded %d tokens per minute", req.Config.TokensPerMinute),
			}, nil
		}
	}

	// All checks passed
	return &RateLimitResult{
		Allowed: true,
	}, nil
}

// RecordRequest records a request for rate limiting
func (s *RateLimitService) RecordRequest(ctx context.Context, req RateLimitRequest) error {
	if req.Config == nil {
		return nil
	}

	scopeKey := s.buildScopeKey(req.OrgID, req.AppID, req.UserID)
	now := time.Now()

	// Record request count
	if err := s.recordEvent(ctx, scopeKey, now); err != nil {
		return fmt.Errorf("failed to record request: %w", err)
	}

	// Record token count if applicable
	if req.TokensUsed > 0 {
		scopeKeyTokens := scopeKey + ":tokens"
		// Record multiple times for token count (or use a separate table)
		for i := 0; i < req.TokensUsed; i++ {
			if err := s.recordEvent(ctx, scopeKeyTokens, now); err != nil {
				return fmt.Errorf("failed to record tokens: %w", err)
			}
		}
	}

	return nil
}

// checkWindow checks if the limit is exceeded for a specific time window
func (s *RateLimitService) checkWindow(ctx context.Context, scopeKey string, window RateLimitWindow, now time.Time, limit int) (allowed bool, remaining int, resetAt time.Time, err error) {
	windowStart, resetAt := s.getWindowBounds(now, window)

	// Count requests in this window using sliding window
	query := `
		SELECT COUNT(*) 
		FROM rate_limit_events 
		WHERE scope_key = $1 
		  AND timestamp >= $2 
		  AND timestamp < $3
	`

	var count int
	err = s.db.QueryRowContext(ctx, query, scopeKey, windowStart, now).Scan(&count)
	if err != nil {
		return false, 0, resetAt, fmt.Errorf("failed to query rate limit: %w", err)
	}

	// Check if limit is exceeded
	if count >= limit {
		return false, 0, resetAt, nil
	}

	remaining = limit - count
	return true, remaining, resetAt, nil
}

// recordEvent records a rate limit event
func (s *RateLimitService) recordEvent(ctx context.Context, scopeKey string, timestamp time.Time) error {
	query := `
		INSERT INTO rate_limit_events (scope_key, timestamp)
		VALUES ($1, $2)
	`

	_, err := s.db.ExecContext(ctx, query, scopeKey, timestamp)
	if err != nil {
		return fmt.Errorf("failed to insert rate limit event: %w", err)
	}

	return nil
}

// getWindowBounds returns the start and reset time for a time window
func (s *RateLimitService) getWindowBounds(now time.Time, window RateLimitWindow) (start time.Time, reset time.Time) {
	switch window {
	case WindowMinute:
		start = now.Add(-1 * time.Minute)
		reset = now.Truncate(time.Minute).Add(time.Minute)
	case WindowHour:
		start = now.Add(-1 * time.Hour)
		reset = now.Truncate(time.Hour).Add(time.Hour)
	case WindowDay:
		start = now.Add(-24 * time.Hour)
		reset = now.Truncate(24 * time.Hour).Add(24 * time.Hour)
	}
	return start, reset
}

// buildScopeKey builds a unique key for the rate limit scope
func (s *RateLimitService) buildScopeKey(orgID, appID uuid.UUID, userID *uuid.UUID) string {
	if userID != nil {
		return fmt.Sprintf("org:%s:app:%s:user:%s", orgID.String(), appID.String(), userID.String())
	}
	return fmt.Sprintf("org:%s:app:%s", orgID.String(), appID.String())
}

// CleanupOldRequests removes old rate limit events to keep the table size manageable
// Should be called periodically (e.g., daily)
func (s *RateLimitService) CleanupOldRequests(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)

	query := `
		DELETE FROM rate_limit_events
		WHERE timestamp < $1
	`

	result, err := s.db.ExecContext(ctx, query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old requests: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	s.logger.Info("cleaned up old rate limit events",
		zap.Int64("rows_deleted", rowsAffected),
		zap.Time("cutoff_time", cutoffTime))

	return rowsAffected, nil
}

// StartCleanupWorker starts a background worker to periodically clean up old events
func (s *RateLimitService) StartCleanupWorker(ctx context.Context, interval time.Duration, retention time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.logger.Info("started rate limit cleanup worker",
		zap.Duration("interval", interval),
		zap.Duration("retention", retention))

	for {
		select {
		case <-ticker.C:
			if _, err := s.CleanupOldRequests(ctx, retention); err != nil {
				s.logger.Error("failed to cleanup old requests", zap.Error(err))
			}
		case <-ctx.Done():
			s.logger.Info("stopping rate limit cleanup worker")
			return
		}
	}
}

// GetCurrentUsage returns the current usage for a scope
func (s *RateLimitService) GetCurrentUsage(ctx context.Context, orgID, appID uuid.UUID, userID *uuid.UUID) (*UsageStats, error) {
	scopeKey := s.buildScopeKey(orgID, appID, userID)
	now := time.Now()

	stats := &UsageStats{}

	// Count minute window
	minuteStart, _ := s.getWindowBounds(now, WindowMinute)
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM rate_limit_events WHERE scope_key = $1 AND timestamp >= $2",
		scopeKey, minuteStart).Scan(&stats.RequestsLastMinute); err != nil {
		return nil, err
	}

	// Count hour window
	hourStart, _ := s.getWindowBounds(now, WindowHour)
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM rate_limit_events WHERE scope_key = $1 AND timestamp >= $2",
		scopeKey, hourStart).Scan(&stats.RequestsLastHour); err != nil {
		return nil, err
	}

	// Count day window
	dayStart, _ := s.getWindowBounds(now, WindowDay)
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM rate_limit_events WHERE scope_key = $1 AND timestamp >= $2",
		scopeKey, dayStart).Scan(&stats.RequestsLastDay); err != nil {
		return nil, err
	}

	return stats, nil
}

// UsageStats represents current usage statistics
type UsageStats struct {
	RequestsLastMinute int
	RequestsLastHour   int
	RequestsLastDay    int
}
