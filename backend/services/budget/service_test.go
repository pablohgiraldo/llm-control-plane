package budget

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

func TestBudgetService_BuildScopeKey(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewBudgetService(nil, logger)

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

func TestBudgetService_GetPeriodKey(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewBudgetService(nil, logger)

	now := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

	t.Run("daily period", func(t *testing.T) {
		key := service.getPeriodKey(now, PeriodDaily)
		assert.Equal(t, "2024-01-15", key)
	})

	t.Run("monthly period", func(t *testing.T) {
		key := service.getPeriodKey(now, PeriodMonthly)
		assert.Equal(t, "2024-01", key)
	})
}

func TestBudgetService_CheckBudget_NoConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewBudgetService(nil, logger)

	ctx := context.Background()
	req := BudgetCheckRequest{
		OrgID:  uuid.New(),
		AppID:  uuid.New(),
		Config: nil, // No budget config
		Cost:   10.0,
	}

	result, err := service.CheckBudget(ctx, req)

	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestBudgetService_CheckBudget_ZeroLimits(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewBudgetService(nil, logger)

	ctx := context.Background()
	req := BudgetCheckRequest{
		OrgID: uuid.New(),
		AppID: uuid.New(),
		Config: &models.BudgetConfig{
			MaxDailyCost:   0, // No limit
			MaxMonthlyCost: 0,
			Currency:       "USD",
		},
		Cost: 10.0,
	}

	result, err := service.CheckBudget(ctx, req)

	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

// Integration tests (require database)

func TestBudgetService_Integration_CheckAndRecord(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewBudgetService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()

	config := &models.BudgetConfig{
		MaxDailyCost:   100.0,
		MaxMonthlyCost: 3000.0,
		Currency:       "USD",
	}

	// First request - should succeed
	checkReq := BudgetCheckRequest{
		OrgID:  orgID,
		AppID:  appID,
		Config: config,
		Cost:   50.0,
	}

	result, err := service.CheckBudget(ctx, checkReq)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	// Record the cost
	recordReq := CostRecordRequest{
		OrgID:      orgID,
		AppID:      appID,
		Cost:       50.0,
		Currency:   "USD",
		Provider:   "openai",
		Model:      "gpt-4",
		RequestID:  uuid.New().String(),
		TokensUsed: 1000,
	}

	err = service.RecordCost(ctx, recordReq)
	require.NoError(t, err)

	// Second request - should succeed (total will be 100.0)
	checkReq.Cost = 50.0
	result, err = service.CheckBudget(ctx, checkReq)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 50.0, result.DailySpend)

	// Record the cost
	recordReq.Cost = 50.0
	recordReq.RequestID = uuid.New().String()
	err = service.RecordCost(ctx, recordReq)
	require.NoError(t, err)

	// Third request - should fail (would exceed daily budget)
	checkReq.Cost = 1.0
	result, err = service.CheckBudget(ctx, checkReq)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, PeriodDaily, result.ViolatedPeriod)
	assert.Contains(t, result.ViolationReason, "would exceed daily budget")
	assert.Equal(t, 100.0, result.DailySpend)
}

func TestBudgetService_Integration_MonthlyBudget(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewBudgetService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()

	config := &models.BudgetConfig{
		MaxDailyCost:   0, // No daily limit
		MaxMonthlyCost: 100.0,
		Currency:       "USD",
	}

	// Record multiple requests
	for i := 0; i < 5; i++ {
		checkReq := BudgetCheckRequest{
			OrgID:  orgID,
			AppID:  appID,
			Config: config,
			Cost:   20.0,
		}

		result, err := service.CheckBudget(ctx, checkReq)
		require.NoError(t, err)

		if i < 5 {
			assert.True(t, result.Allowed, "request %d should be allowed", i+1)

			recordReq := CostRecordRequest{
				OrgID:      orgID,
				AppID:      appID,
				Cost:       20.0,
				Currency:   "USD",
				Provider:   "openai",
				Model:      "gpt-4",
				RequestID:  uuid.New().String(),
				TokensUsed: 1000,
			}
			err = service.RecordCost(ctx, recordReq)
			require.NoError(t, err)
		}
	}

	// 6th request should fail (would exceed monthly budget of 100.0)
	checkReq := BudgetCheckRequest{
		OrgID:  orgID,
		AppID:  appID,
		Config: config,
		Cost:   1.0,
	}

	result, err := service.CheckBudget(ctx, checkReq)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, PeriodMonthly, result.ViolatedPeriod)
	assert.Contains(t, result.ViolationReason, "would exceed monthly budget")
}

func TestBudgetService_Integration_DifferentScopes(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewBudgetService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID1 := uuid.New()
	appID2 := uuid.New()

	config := &models.BudgetConfig{
		MaxDailyCost: 50.0,
		Currency:     "USD",
	}

	// Record cost for app1
	recordReq1 := CostRecordRequest{
		OrgID:      orgID,
		AppID:      appID1,
		Cost:       50.0,
		Currency:   "USD",
		Provider:   "openai",
		Model:      "gpt-4",
		RequestID:  uuid.New().String(),
		TokensUsed: 1000,
	}
	err := service.RecordCost(ctx, recordReq1)
	require.NoError(t, err)

	// app1 should be at budget
	checkReq1 := BudgetCheckRequest{
		OrgID:  orgID,
		AppID:  appID1,
		Config: config,
		Cost:   1.0,
	}
	result, err := service.CheckBudget(ctx, checkReq1)
	require.NoError(t, err)
	assert.False(t, result.Allowed)

	// app2 should still be allowed (different scope)
	checkReq2 := BudgetCheckRequest{
		OrgID:  orgID,
		AppID:  appID2,
		Config: config,
		Cost:   50.0,
	}
	result, err = service.CheckBudget(ctx, checkReq2)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestBudgetService_Integration_GetSpendSummary(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewBudgetService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()

	// Record some costs
	for i := 0; i < 5; i++ {
		recordReq := CostRecordRequest{
			OrgID:      orgID,
			AppID:      appID,
			Cost:       10.0,
			Currency:   "USD",
			Provider:   "openai",
			Model:      "gpt-4",
			RequestID:  uuid.New().String(),
			TokensUsed: 1000,
		}
		err := service.RecordCost(ctx, recordReq)
		require.NoError(t, err)
	}

	// Get summary
	summary, err := service.GetSpendSummary(ctx, orgID, appID, nil)
	require.NoError(t, err)
	assert.Equal(t, 50.0, summary.DailySpend)
	assert.Equal(t, 50.0, summary.MonthlySpend)
	assert.Equal(t, 5, summary.DailyTransactions)
	assert.Equal(t, 5, summary.MonthlyTransactions)
}

func TestBudgetService_Integration_GetTopSpenders(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewBudgetService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID1 := uuid.New()
	appID2 := uuid.New()
	appID3 := uuid.New()

	// Record costs for different apps
	costs := map[uuid.UUID]float64{
		appID1: 100.0,
		appID2: 200.0,
		appID3: 50.0,
	}

	for appID, cost := range costs {
		recordReq := CostRecordRequest{
			OrgID:      orgID,
			AppID:      appID,
			Cost:       cost,
			Currency:   "USD",
			Provider:   "openai",
			Model:      "gpt-4",
			RequestID:  uuid.New().String(),
			TokensUsed: 1000,
		}
		err := service.RecordCost(ctx, recordReq)
		require.NoError(t, err)
	}

	// Get top spenders
	spenders, err := service.GetTopSpenders(ctx, orgID, PeriodDaily, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, len(spenders))

	// Should be sorted by cost (highest first)
	assert.Equal(t, 200.0, spenders[0].TotalCost)
	assert.Equal(t, 100.0, spenders[1].TotalCost)
}

func TestBudgetService_Integration_GetPeriodSpend(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewBudgetService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()

	scopeKey := service.buildScopeKey(orgID, appID, nil)

	// No spend initially
	spend, err := service.GetPeriodSpend(ctx, scopeKey, PeriodDaily, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0.0, spend)

	// Record some cost
	recordReq := CostRecordRequest{
		OrgID:      orgID,
		AppID:      appID,
		Cost:       25.5,
		Currency:   "USD",
		Provider:   "openai",
		Model:      "gpt-4",
		RequestID:  uuid.New().String(),
		TokensUsed: 1000,
	}
	err = service.RecordCost(ctx, recordReq)
	require.NoError(t, err)

	// Check spend
	spend, err = service.GetPeriodSpend(ctx, scopeKey, PeriodDaily, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 25.5, spend)

	// Record more cost
	recordReq.Cost = 10.0
	recordReq.RequestID = uuid.New().String()
	err = service.RecordCost(ctx, recordReq)
	require.NoError(t, err)

	// Check updated spend
	spend, err = service.GetPeriodSpend(ctx, scopeKey, PeriodDaily, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 35.5, spend)
}

func TestBudgetService_Integration_CleanupOldData(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	logger, _ := zap.NewDevelopment()
	service := NewBudgetService(db, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()

	// Record some costs
	for i := 0; i < 5; i++ {
		recordReq := CostRecordRequest{
			OrgID:      orgID,
			AppID:      appID,
			Cost:       10.0,
			Currency:   "USD",
			Provider:   "openai",
			Model:      "gpt-4",
			RequestID:  uuid.New().String(),
			TokensUsed: 1000,
		}
		err := service.RecordCost(ctx, recordReq)
		require.NoError(t, err)
	}

	// Cleanup data older than 1 hour (should not delete anything)
	rowsDeleted, err := service.CleanupOldData(ctx, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rowsDeleted)

	// Cleanup data older than 1 second
	time.Sleep(2 * time.Second)
	rowsDeleted, err = service.CleanupOldData(ctx, 1*time.Second)
	require.NoError(t, err)
	
	// Should delete some records
	assert.Greater(t, rowsDeleted, int64(0))
}
