package audit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/upb/llm-control-plane/backend/models"
	"github.com/upb/llm-control-plane/backend/repositories"
	"go.uber.org/zap"
)

// MockAuditRepository is a mock implementation of AuditRepository
type MockAuditRepository struct {
	mock.Mock
	mu         sync.Mutex
	insertedLogs []*models.AuditLog
}

func (m *MockAuditRepository) Insert(ctx context.Context, log *models.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	args := m.Called(ctx, log)
	m.insertedLogs = append(m.insertedLogs, log)
	return args.Error(0)
}

func (m *MockAuditRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	args := m.Called(ctx, id)
	if log := args.Get(0); log != nil {
		return log.(*models.AuditLog), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuditRepository) GetByOrgID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.AuditLog, error) {
	args := m.Called(ctx, orgID, limit, offset)
	if logs := args.Get(0); logs != nil {
		return logs.([]*models.AuditLog), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuditRepository) GetByAppID(ctx context.Context, appID uuid.UUID, limit, offset int) ([]*models.AuditLog, error) {
	args := m.Called(ctx, appID, limit, offset)
	if logs := args.Get(0); logs != nil {
		return logs.([]*models.AuditLog), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuditRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AuditLog, error) {
	args := m.Called(ctx, userID, limit, offset)
	if logs := args.Get(0); logs != nil {
		return logs.([]*models.AuditLog), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuditRepository) GetByDateRange(ctx context.Context, orgID uuid.UUID, start, end time.Time, limit, offset int) ([]*models.AuditLog, error) {
	args := m.Called(ctx, orgID, start, end, limit, offset)
	if logs := args.Get(0); logs != nil {
		return logs.([]*models.AuditLog), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuditRepository) GetByAction(ctx context.Context, orgID uuid.UUID, action models.AuditAction, limit, offset int) ([]*models.AuditLog, error) {
	args := m.Called(ctx, orgID, action, limit, offset)
	if logs := args.Get(0); logs != nil {
		return logs.([]*models.AuditLog), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuditRepository) GetByRequestID(ctx context.Context, requestID string) ([]*models.AuditLog, error) {
	args := m.Called(ctx, requestID)
	if logs := args.Get(0); logs != nil {
		return logs.([]*models.AuditLog), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuditRepository) WithTx(tx repositories.Transaction) repositories.AuditRepository {
	args := m.Called(tx)
	return args.Get(0).(repositories.AuditRepository)
}

func (m *MockAuditRepository) GetInsertedLogs() []*models.AuditLog {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.insertedLogs
}

func TestAuditService_StartStop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockAuditRepository)
	config := Config{
		BufferSize:  10,
		WorkerCount: 2,
	}

	service := NewAuditService(mockRepo, logger, config)

	// Start service
	err := service.Start()
	require.NoError(t, err)

	stats := service.GetStats()
	assert.True(t, stats.Started)
	assert.Equal(t, 2, stats.WorkerCount)
	assert.Equal(t, 10, stats.BufferSize)

	// Cannot start again
	err = service.Start()
	assert.Error(t, err)

	// Stop service
	err = service.Stop(5 * time.Second)
	require.NoError(t, err)
}

func TestAuditService_LogEvent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockAuditRepository)
	config := Config{
		BufferSize:  100,
		WorkerCount: 2,
	}

	service := NewAuditService(mockRepo, logger, config)
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop(5 * time.Second)

	orgID := uuid.New()
	log := models.NewAuditLog(orgID, models.AuditActionInferenceRequest, "inference_request")

	mockRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)

	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}

	// Log event (non-blocking)
	err = service.LogEvent(event)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify event was processed
	insertedLogs := mockRepo.GetInsertedLogs()
	assert.Equal(t, 1, len(insertedLogs))
	assert.Equal(t, orgID, insertedLogs[0].OrgID)
	assert.Equal(t, models.AuditActionInferenceRequest, insertedLogs[0].Action)
}

func TestAuditService_LogEventBlocking(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockAuditRepository)
	config := Config{
		BufferSize:  100,
		WorkerCount: 2,
	}

	service := NewAuditService(mockRepo, logger, config)
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop(5 * time.Second)

	orgID := uuid.New()
	log := models.NewAuditLog(orgID, models.AuditActionPolicyCreated, "policy")

	mockRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)

	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}

	ctx := context.Background()
	err = service.LogEventBlocking(ctx, event)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify event was processed
	insertedLogs := mockRepo.GetInsertedLogs()
	assert.GreaterOrEqual(t, len(insertedLogs), 1)
}

func TestAuditService_MultipleEvents(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockAuditRepository)
	config := Config{
		BufferSize:  100,
		WorkerCount: 3,
	}

	service := NewAuditService(mockRepo, logger, config)
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop(5 * time.Second)

	orgID := uuid.New()
	mockRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)

	// Log multiple events
	eventCount := 50
	for i := 0; i < eventCount; i++ {
		log := models.NewAuditLog(orgID, models.AuditActionInferenceRequest, "inference_request")
		event := &AuditEvent{
			Log:      log,
			Priority: 1,
		}
		err = service.LogEvent(event)
		require.NoError(t, err)
	}

	// Wait for all events to be processed
	time.Sleep(500 * time.Millisecond)

	// Verify all events were processed
	insertedLogs := mockRepo.GetInsertedLogs()
	assert.Equal(t, eventCount, len(insertedLogs))
}

func TestAuditService_ConcurrentLogging(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockAuditRepository)
	config := Config{
		BufferSize:  1000,
		WorkerCount: 5,
	}

	service := NewAuditService(mockRepo, logger, config)
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop(5 * time.Second)

	orgID := uuid.New()
	mockRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)

	// Log events concurrently
	goroutineCount := 10
	eventsPerGoroutine := 10
	var wg sync.WaitGroup

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				log := models.NewAuditLog(orgID, models.AuditActionInferenceRequest, "inference_request")
				event := &AuditEvent{
					Log:      log,
					Priority: 1,
				}
				service.LogEvent(event)
			}
		}()
	}

	wg.Wait()

	// Wait for all events to be processed
	time.Sleep(1 * time.Second)

	// Verify all events were processed
	insertedLogs := mockRepo.GetInsertedLogs()
	expectedCount := goroutineCount * eventsPerGoroutine
	assert.Equal(t, expectedCount, len(insertedLogs))
}

func TestAuditService_LogInferenceRequest(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockAuditRepository)
	config := DefaultConfig()

	service := NewAuditService(mockRepo, logger, config)
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop(5 * time.Second)

	mockRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)

	inferenceReq := models.NewInferenceRequest(uuid.New(), uuid.New(), "openai", "gpt-4", "test prompt")
	inferenceReq.TotalTokens = 100
	inferenceReq.LatencyMs = 500
	inferenceReq.Cost = 0.05

	err = service.LogInferenceRequest(inferenceReq)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	insertedLogs := mockRepo.GetInsertedLogs()
	assert.Equal(t, 1, len(insertedLogs))
	assert.Equal(t, models.AuditActionInferenceRequest, insertedLogs[0].Action)
	assert.NotNil(t, insertedLogs[0].Model)
	assert.Equal(t, "gpt-4", *insertedLogs[0].Model)
}

func TestAuditService_LogPolicyViolation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockAuditRepository)
	config := DefaultConfig()

	service := NewAuditService(mockRepo, logger, config)
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop(5 * time.Second)

	mockRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)

	inferenceReq := models.NewInferenceRequest(uuid.New(), uuid.New(), "openai", "gpt-4", "test prompt")
	policyID := uuid.New()

	err = service.LogPolicyViolation(inferenceReq, policyID, "rate limit exceeded", map[string]interface{}{
		"limit": 100,
		"used":  150,
	})
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	insertedLogs := mockRepo.GetInsertedLogs()
	assert.Equal(t, 1, len(insertedLogs))
	assert.Equal(t, models.AuditActionPolicyViolation, insertedLogs[0].Action)
	assert.NotNil(t, insertedLogs[0].ResourceID)
	assert.Equal(t, policyID, *insertedLogs[0].ResourceID)
}

func TestAuditService_LogPolicyCreated(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockAuditRepository)
	config := DefaultConfig()

	service := NewAuditService(mockRepo, logger, config)
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop(5 * time.Second)

	mockRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)

	policy := models.NewPolicy(uuid.New(), models.PolicyTypeRateLimit, nil, 10)
	userID := uuid.New()

	err = service.LogPolicyCreated(policy, userID)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	insertedLogs := mockRepo.GetInsertedLogs()
	assert.Equal(t, 1, len(insertedLogs))
	assert.Equal(t, models.AuditActionPolicyCreated, insertedLogs[0].Action)
	assert.NotNil(t, insertedLogs[0].UserID)
	assert.Equal(t, userID, *insertedLogs[0].UserID)
}

func TestAuditService_BufferFull(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockAuditRepository)
	config := Config{
		BufferSize:  5,
		WorkerCount: 1,
	}

	service := NewAuditService(mockRepo, logger, config)
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop(5 * time.Second)

	orgID := uuid.New()
	
	// Slow down processing
	mockRepo.On("Insert", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(100 * time.Millisecond)
	})

	// Fill buffer
	successCount := 0
	for i := 0; i < 20; i++ {
		log := models.NewAuditLog(orgID, models.AuditActionInferenceRequest, "inference_request")
		event := &AuditEvent{
			Log:      log,
			Priority: 1,
		}
		err = service.LogEvent(event)
		if err == nil {
			successCount++
		}
	}

	// Should have some failures due to buffer full
	assert.Less(t, successCount, 20)

	// Wait for processing
	time.Sleep(3 * time.Second)
}

func TestAuditService_StopTimeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockAuditRepository)
	config := Config{
		BufferSize:  100,
		WorkerCount: 1,
	}

	service := NewAuditService(mockRepo, logger, config)
	err := service.Start()
	require.NoError(t, err)

	orgID := uuid.New()

	// Very slow processing
	mockRepo.On("Insert", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Second)
	})

	// Add one event that will take long to process
	log := models.NewAuditLog(orgID, models.AuditActionInferenceRequest, "inference_request")
	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}
	service.LogEvent(event)

	// Stop with short timeout
	err = service.Stop(100 * time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestAuditService_GetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockAuditRepository)
	config := Config{
		BufferSize:  100,
		WorkerCount: 5,
	}

	service := NewAuditService(mockRepo, logger, config)

	// Before start
	stats := service.GetStats()
	assert.False(t, stats.Started)
	assert.Equal(t, 5, stats.WorkerCount)
	assert.Equal(t, 100, stats.BufferSize)
	assert.Equal(t, 0, stats.PendingEvents)

	// After start
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop(5 * time.Second)

	stats = service.GetStats()
	assert.True(t, stats.Started)

	// Add some events
	orgID := uuid.New()
	mockRepo.On("Insert", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(100 * time.Millisecond)
	})

	for i := 0; i < 10; i++ {
		log := models.NewAuditLog(orgID, models.AuditActionInferenceRequest, "inference_request")
		event := &AuditEvent{
			Log:      log,
			Priority: 1,
		}
		service.LogEvent(event)
	}

	// Check pending events
	stats = service.GetStats()
	assert.Greater(t, stats.PendingEvents, 0)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.Equal(t, 10000, config.BufferSize)
	assert.Equal(t, 5, config.WorkerCount)
}
