package ratelimit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/upb/llm-control-plane/backend/models"
	"go.uber.org/zap"

	_ "github.com/lib/pq"
)

// Note: These tests require a PostgreSQL database for integration testing
// For unit tests, we would need to mock the database

func setupTestDB(t *testing.T) *sql.DB {
	// This is a placeholder - in real tests, you'd set up a test database
	// For now, we'll skip tests that require DB
	t.Skip("Database integration tests require PostgreSQL setup")
	return nil
}

func TestRateLimitService_BuildScopeKey(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewRateLimitService(nil, logger)

	orgID := uuid.New()
	appID := uuid.New()
	userID := uuid.New()

	t.Run("without user ID", func(t *testing.T) {
		key := service.buildScopeKey(orgID, appID, nil)
		expected := "org:" + orgID.String() + ":app:" + appID.String()
		assert.Equal(t, expected, key)
	})

	t.Run("with user ID", func(t *testing.T) {
		key := service.buildScopeKey(orgID, appID, &userID)
		expected := "org:" + orgID.String() + ":app:" + appID.String() + ":user:" + userID.String()
		assert.Equal(t, expected, key)
	})
}

func TestRateLimitService_GetWindowBounds(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewRateLimitService(nil, logger)

	now := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)

	t.Run("minute window", func(t *testing.T) {
		start, reset := service.getWindowBounds(now, WindowMinute)
		
		expectedStart := now.Add(-1 * time.Minute)
		expectedReset := time.Date(2024, 1, 15, 14, 31, 0, 0, time.UTC)
		
		assert.Equal(t, expectedStart, start)
		assert.Equal(t, expectedReset, reset)
	})

	t.Run("hour window", func(t *testing.T) {
		start, reset := service.getWindowBounds(now, WindowHour)
		
		expectedStart := now.Add(-1 * time.Hour)
		expectedReset := time.Date(2024, 1, 15, 15, 0, 0, 0, time.UTC)
		
		assert.Equal(t, expectedStart, start)
		assert.Equal(t, expectedReset, reset)
	})

	t.Run("day window", func(t *testing.T) {
		start, reset := service.getWindowBounds(now, WindowDay)
		
		expectedStart := now.Add(-24 * time.Hour)
		expectedReset := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
		
		assert.Equal(t, expectedStart, start)
		assert.Equal(t, expectedReset, reset)
	})
}

func TestRateLimitService_CheckLimit_NoConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewRateLimitService(nil, logger)

	ctx := context.Background()
	req := RateLimitRequest{
		OrgID:  uuid.New(),
		AppID:  uuid.New(),
		Config: nil, // No rate limit config
	}

	result, err := service.CheckLimit(ctx, req)
	
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestRateLimitService_CheckLimit_ZeroLimits(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewRateLimitService(nil, logger)

	ctx := context.Background()
	req := RateLimitRequest{
		OrgID: uuid.New(),
		AppID: uuid.New(),
		Config: &models.RateLimitConfig{
			RequestsPerMinute: 0, // No limit
			RequestsPerHour:   0,
			RequestsPerDay:    0,
		},
	}

	result, err := service.CheckLimit(ctx, req)
	
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

// Integration tests (require database)

func TestRateLimitService_Integration_CheckAndRecord(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewRateLimitService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()

	config := &models.RateLimitConfig{
		RequestsPerMinute: 5,
		RequestsPerHour:   100,
		RequestsPerDay:    1000,
	}

	req := RateLimitRequest{
		OrgID:  orgID,
		AppID:  appID,
		Config: config,
	}

	// First 5 requests should succeed
	for i := 0; i < 5; i++ {
		result, err := service.CheckLimit(ctx, req)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "request %d should be allowed", i+1)

		err = service.RecordRequest(ctx, req)
		require.NoError(t, err)
	}

	// 6th request should be rate limited
	result, err := service.CheckLimit(ctx, req)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, WindowMinute, result.ViolatedWindow)
	assert.Contains(t, result.ViolationReason, "exceeded 5 requests per minute")
}

func TestRateLimitService_Integration_SlidingWindow(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewRateLimitService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()

	config := &models.RateLimitConfig{
		RequestsPerMinute: 3,
	}

	req := RateLimitRequest{
		OrgID:  orgID,
		AppID:  appID,
		Config: config,
	}

	// Record 3 requests
	for i := 0; i < 3; i++ {
		result, err := service.CheckLimit(ctx, req)
		require.NoError(t, err)
		assert.True(t, result.Allowed)

		err = service.RecordRequest(ctx, req)
		require.NoError(t, err)
	}

	// 4th request should be rate limited
	result, err := service.CheckLimit(ctx, req)
	require.NoError(t, err)
	assert.False(t, result.Allowed)

	// Wait for 1 minute (sliding window)
	time.Sleep(61 * time.Second)

	// Should be allowed again
	result, err = service.CheckLimit(ctx, req)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestRateLimitService_Integration_DifferentScopes(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewRateLimitService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID1 := uuid.New()
	appID2 := uuid.New()

	config := &models.RateLimitConfig{
		RequestsPerMinute: 2,
	}

	req1 := RateLimitRequest{
		OrgID:  orgID,
		AppID:  appID1,
		Config: config,
	}

	req2 := RateLimitRequest{
		OrgID:  orgID,
		AppID:  appID2,
		Config: config,
	}

	// Record 2 requests for app1
	for i := 0; i < 2; i++ {
		result, err := service.CheckLimit(ctx, req1)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		err = service.RecordRequest(ctx, req1)
		require.NoError(t, err)
	}

	// app1 should be rate limited
	result, err := service.CheckLimit(ctx, req1)
	require.NoError(t, err)
	assert.False(t, result.Allowed)

	// app2 should still be allowed (different scope)
	result, err = service.CheckLimit(ctx, req2)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestRateLimitService_Integration_TokenLimits(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewRateLimitService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()

	config := &models.RateLimitConfig{
		TokensPerMinute: 100,
	}

	req := RateLimitRequest{
		OrgID:      orgID,
		AppID:      appID,
		Config:     config,
		TokensUsed: 50,
	}

	// First request (50 tokens)
	result, err := service.CheckLimit(ctx, req)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	err = service.RecordRequest(ctx, req)
	require.NoError(t, err)

	// Second request (50 tokens)
	result, err = service.CheckLimit(ctx, req)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	err = service.RecordRequest(ctx, req)
	require.NoError(t, err)

	// Third request should be rate limited (would exceed 100 tokens)
	result, err = service.CheckLimit(ctx, req)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Contains(t, result.ViolationReason, "exceeded 100 tokens per minute")
}

func TestRateLimitService_Integration_CleanupOldRequests(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewRateLimitService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()

	config := &models.RateLimitConfig{
		RequestsPerMinute: 100,
	}

	req := RateLimitRequest{
		OrgID:  orgID,
		AppID:  appID,
		Config: config,
	}

	// Record some requests
	for i := 0; i < 5; i++ {
		err := service.RecordRequest(ctx, req)
		require.NoError(t, err)
	}

	// Cleanup requests older than 1 hour
	rowsDeleted, err := service.CleanupOldRequests(ctx, 1*time.Hour)
	require.NoError(t, err)
	
	// Should not delete recent requests
	assert.Equal(t, int64(0), rowsDeleted)

	// Cleanup requests older than 1 second
	time.Sleep(2 * time.Second)
	rowsDeleted, err = service.CleanupOldRequests(ctx, 1*time.Second)
	require.NoError(t, err)
	
	// Should delete the 5 requests we recorded
	assert.Equal(t, int64(5), rowsDeleted)
}

func TestRateLimitService_Integration_GetCurrentUsage(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewRateLimitService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()

	config := &models.RateLimitConfig{
		RequestsPerMinute: 100,
	}

	req := RateLimitRequest{
		OrgID:  orgID,
		AppID:  appID,
		Config: config,
	}

	// Record some requests
	for i := 0; i < 5; i++ {
		err := service.RecordRequest(ctx, req)
		require.NoError(t, err)
	}

	// Get current usage
	stats, err := service.GetCurrentUsage(ctx, orgID, appID, nil)
	require.NoError(t, err)
	assert.Equal(t, 5, stats.RequestsLastMinute)
	assert.Equal(t, 5, stats.RequestsLastHour)
	assert.Equal(t, 5, stats.RequestsLastDay)
}
