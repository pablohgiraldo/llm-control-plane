package budget

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/models"
	"go.uber.org/zap"
)

// BudgetPeriod represents the time period for budget tracking
type BudgetPeriod string

const (
	PeriodDaily   BudgetPeriod = "daily"
	PeriodMonthly BudgetPeriod = "monthly"
)

// BudgetCheckRequest represents a budget check request
type BudgetCheckRequest struct {
	OrgID  uuid.UUID
	AppID  uuid.UUID
	UserID *uuid.UUID
	Config *models.BudgetConfig
	Cost   float64
}

// BudgetCheckResult represents the result of a budget check
type BudgetCheckResult struct {
	Allowed         bool
	DailySpend      float64
	DailyLimit      float64
	MonthlySpend    float64
	MonthlyLimit    float64
	ViolatedPeriod  BudgetPeriod
	ViolationReason string
}

// CostRecordRequest represents a request to record cost
type CostRecordRequest struct {
	OrgID      uuid.UUID
	AppID      uuid.UUID
	UserID     *uuid.UUID
	Cost       float64
	Currency   string
	Provider   string
	Model      string
	RequestID  string
	TokensUsed int
}

// BudgetService handles budget tracking using PostgreSQL
type BudgetService struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewBudgetService creates a new BudgetService instance
func NewBudgetService(db *sql.DB, logger *zap.Logger) *BudgetService {
	return &BudgetService{
		db:     db,
		logger: logger,
	}
}

// CheckBudget checks if the request is within budget limits
func (s *BudgetService) CheckBudget(ctx context.Context, req BudgetCheckRequest) (*BudgetCheckResult, error) {
	if req.Config == nil {
		// No budget configured
		return &BudgetCheckResult{
			Allowed: true,
		}, nil
	}

	scopeKey := s.buildScopeKey(req.OrgID, req.AppID, req.UserID)
	now := time.Now()

	result := &BudgetCheckResult{
		Allowed:      true,
		DailyLimit:   req.Config.MaxDailyCost,
		MonthlyLimit: req.Config.MaxMonthlyCost,
	}

	// Check daily budget
	if req.Config.MaxDailyCost > 0 {
		dailySpend, err := s.GetPeriodSpend(ctx, scopeKey, PeriodDaily, now)
		if err != nil {
			return nil, fmt.Errorf("failed to get daily spend: %w", err)
		}

		result.DailySpend = dailySpend

		if dailySpend+req.Cost > req.Config.MaxDailyCost {
			result.Allowed = false
			result.ViolatedPeriod = PeriodDaily
			result.ViolationReason = fmt.Sprintf("would exceed daily budget of %.2f %s (current: %.2f, request: %.2f)",
				req.Config.MaxDailyCost, req.Config.Currency, dailySpend, req.Cost)
			return result, nil
		}
	}

	// Check monthly budget
	if req.Config.MaxMonthlyCost > 0 {
		monthlySpend, err := s.GetPeriodSpend(ctx, scopeKey, PeriodMonthly, now)
		if err != nil {
			return nil, fmt.Errorf("failed to get monthly spend: %w", err)
		}

		result.MonthlySpend = monthlySpend

		if monthlySpend+req.Cost > req.Config.MaxMonthlyCost {
			result.Allowed = false
			result.ViolatedPeriod = PeriodMonthly
			result.ViolationReason = fmt.Sprintf("would exceed monthly budget of %.2f %s (current: %.2f, request: %.2f)",
				req.Config.MaxMonthlyCost, req.Config.Currency, monthlySpend, req.Cost)
			return result, nil
		}
	}

	return result, nil
}

// RecordCost records the cost of a request using upsert
func (s *BudgetService) RecordCost(ctx context.Context, req CostRecordRequest) error {
	scopeKey := s.buildScopeKey(req.OrgID, req.AppID, req.UserID)
	now := time.Now()

	// Get period keys
	dailyPeriod := s.getPeriodKey(now, PeriodDaily)
	monthlyPeriod := s.getPeriodKey(now, PeriodMonthly)

	// Record cost for daily period (upsert)
	if err := s.upsertCost(ctx, scopeKey, dailyPeriod, req.Cost, req.Currency); err != nil {
		return fmt.Errorf("failed to record daily cost: %w", err)
	}

	// Record cost for monthly period (upsert)
	if err := s.upsertCost(ctx, scopeKey, monthlyPeriod, req.Cost, req.Currency); err != nil {
		return fmt.Errorf("failed to record monthly cost: %w", err)
	}

	// Also record the individual transaction for auditing
	if err := s.recordTransaction(ctx, req, now); err != nil {
		return fmt.Errorf("failed to record transaction: %w", err)
	}

	return nil
}

// GetPeriodSpend returns the total spend for a specific period
func (s *BudgetService) GetPeriodSpend(ctx context.Context, scopeKey string, period BudgetPeriod, now time.Time) (float64, error) {
	periodKey := s.getPeriodKey(now, period)

	query := `
		SELECT COALESCE(total_cost, 0) 
		FROM budget_tracking 
		WHERE scope_key = $1 AND period_key = $2
	`

	var totalCost float64
	err := s.db.QueryRowContext(ctx, query, scopeKey, periodKey).Scan(&totalCost)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to query budget: %w", err)
	}

	return totalCost, nil
}

// upsertCost inserts or updates the cost for a period
func (s *BudgetService) upsertCost(ctx context.Context, scopeKey, periodKey string, cost float64, currency string) error {
	query := `
		INSERT INTO budget_tracking (scope_key, period_key, total_cost, currency, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (scope_key, period_key)
		DO UPDATE SET 
			total_cost = budget_tracking.total_cost + EXCLUDED.total_cost,
			updated_at = EXCLUDED.updated_at
	`

	_, err := s.db.ExecContext(ctx, query, scopeKey, periodKey, cost, currency, time.Now())
	if err != nil {
		return fmt.Errorf("failed to upsert cost: %w", err)
	}

	return nil
}

// recordTransaction records an individual transaction for auditing
func (s *BudgetService) recordTransaction(ctx context.Context, req CostRecordRequest, timestamp time.Time) error {
	scopeKey := s.buildScopeKey(req.OrgID, req.AppID, req.UserID)

	query := `
		INSERT INTO budget_transactions 
		(scope_key, cost, currency, provider, model, request_id, tokens_used, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := s.db.ExecContext(ctx, query,
		scopeKey, req.Cost, req.Currency, req.Provider, req.Model, req.RequestID, req.TokensUsed, timestamp)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	return nil
}

// buildScopeKey builds a unique key for the budget scope
func (s *BudgetService) buildScopeKey(orgID, appID uuid.UUID, userID *uuid.UUID) string {
	if userID != nil {
		return fmt.Sprintf("org:%s:app:%s:user:%s", orgID.String(), appID.String(), userID.String())
	}
	return fmt.Sprintf("org:%s:app:%s", orgID.String(), appID.String())
}

// getPeriodKey returns a unique key for a time period
func (s *BudgetService) getPeriodKey(now time.Time, period BudgetPeriod) string {
	switch period {
	case PeriodDaily:
		return now.Format("2006-01-02")
	case PeriodMonthly:
		return now.Format("2006-01")
	default:
		return now.Format("2006-01-02")
	}
}

// GetSpendSummary returns spending summary for different periods
func (s *BudgetService) GetSpendSummary(ctx context.Context, orgID, appID uuid.UUID, userID *uuid.UUID) (*SpendSummary, error) {
	scopeKey := s.buildScopeKey(orgID, appID, userID)
	now := time.Now()

	summary := &SpendSummary{}

	// Get daily spend
	dailySpend, err := s.GetPeriodSpend(ctx, scopeKey, PeriodDaily, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily spend: %w", err)
	}
	summary.DailySpend = dailySpend

	// Get monthly spend
	monthlySpend, err := s.GetPeriodSpend(ctx, scopeKey, PeriodMonthly, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly spend: %w", err)
	}
	summary.MonthlySpend = monthlySpend

	// Get transaction count for today
	dailyPeriod := s.getPeriodKey(now, PeriodDaily)
	query := `
		SELECT COUNT(*) 
		FROM budget_transactions 
		WHERE scope_key = $1 
		  AND DATE(timestamp) = $2
	`
	if err := s.db.QueryRowContext(ctx, query, scopeKey, dailyPeriod).Scan(&summary.DailyTransactions); err != nil {
		return nil, fmt.Errorf("failed to get daily transactions: %w", err)
	}

	// Get transaction count for this month
	monthlyPeriod := s.getPeriodKey(now, PeriodMonthly)
	query = `
		SELECT COUNT(*) 
		FROM budget_transactions 
		WHERE scope_key = $1 
		  AND to_char(timestamp, 'YYYY-MM') = $2
	`
	if err := s.db.QueryRowContext(ctx, query, scopeKey, monthlyPeriod).Scan(&summary.MonthlyTransactions); err != nil {
		return nil, fmt.Errorf("failed to get monthly transactions: %w", err)
	}

	return summary, nil
}

// SpendSummary represents spending summary
type SpendSummary struct {
	DailySpend          float64
	MonthlySpend        float64
	DailyTransactions   int
	MonthlyTransactions int
}

// GetTopSpenders returns the top spenders for a given period
func (s *BudgetService) GetTopSpenders(ctx context.Context, orgID uuid.UUID, period BudgetPeriod, limit int) ([]SpenderInfo, error) {
	periodKey := s.getPeriodKey(time.Now(), period)
	orgPrefix := fmt.Sprintf("org:%s:", orgID.String())

	query := `
		SELECT scope_key, total_cost, currency
		FROM budget_tracking
		WHERE scope_key LIKE $1 || '%'
		  AND period_key = $2
		ORDER BY total_cost DESC
		LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, query, orgPrefix, periodKey, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top spenders: %w", err)
	}
	defer rows.Close()

	spenders := make([]SpenderInfo, 0)
	for rows.Next() {
		var info SpenderInfo
		if err := rows.Scan(&info.ScopeKey, &info.TotalCost, &info.Currency); err != nil {
			return nil, fmt.Errorf("failed to scan spender info: %w", err)
		}
		spenders = append(spenders, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return spenders, nil
}

// SpenderInfo represents information about a spender
type SpenderInfo struct {
	ScopeKey  string
	TotalCost float64
	Currency  string
}

// CleanupOldData removes old budget tracking data
// Should be called periodically to keep database size manageable
func (s *BudgetService) CleanupOldData(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoffDate := time.Now().Add(-olderThan)
	cutoffPeriodKey := s.getPeriodKey(cutoffDate, PeriodDaily)

	// Clean up old budget_tracking records
	query := `
		DELETE FROM budget_tracking
		WHERE period_key < $1
	`

	result, err := s.db.ExecContext(ctx, query, cutoffPeriodKey)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old budget data: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	// Clean up old transactions
	query = `
		DELETE FROM budget_transactions
		WHERE timestamp < $1
	`

	result, err = s.db.ExecContext(ctx, query, cutoffDate)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old transactions: %w", err)
	}

	txRowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get transaction rows affected: %w", err)
	}

	s.logger.Info("cleaned up old budget data",
		zap.Int64("budget_rows_deleted", rowsAffected),
		zap.Int64("transaction_rows_deleted", txRowsAffected),
		zap.Time("cutoff_date", cutoffDate))

	return rowsAffected + txRowsAffected, nil
}

// StartCleanupWorker starts a background worker to periodically clean up old data
func (s *BudgetService) StartCleanupWorker(ctx context.Context, interval time.Duration, retention time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.logger.Info("started budget cleanup worker",
		zap.Duration("interval", interval),
		zap.Duration("retention", retention))

	for {
		select {
		case <-ticker.C:
			if _, err := s.CleanupOldData(ctx, retention); err != nil {
				s.logger.Error("failed to cleanup old budget data", zap.Error(err))
			}
		case <-ctx.Done():
			s.logger.Info("stopping budget cleanup worker")
			return
		}
	}
}
