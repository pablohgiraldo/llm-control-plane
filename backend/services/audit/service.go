package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/models"
	"github.com/upb/llm-control-plane/backend/repositories"
	"go.uber.org/zap"
)

// AuditEvent represents an event to be audited
type AuditEvent struct {
	Log      *models.AuditLog
	Priority int // Higher priority events are processed first (for future enhancements)
}

// AuditService handles asynchronous audit logging
type AuditService struct {
	auditRepo   repositories.AuditRepository
	logger      *zap.Logger
	eventChan   chan *AuditEvent
	workerCount int
	bufferSize  int
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	started     bool
	mu          sync.Mutex
}

// Config holds configuration for the AuditService
type Config struct {
	BufferSize  int // Size of the event buffer channel
	WorkerCount int // Number of concurrent workers
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		BufferSize:  10000, // Buffer up to 10k events
		WorkerCount: 5,     // 5 concurrent workers
	}
}

// NewAuditService creates a new AuditService instance
func NewAuditService(auditRepo repositories.AuditRepository, logger *zap.Logger, config Config) *AuditService {
	ctx, cancel := context.WithCancel(context.Background())

	return &AuditService{
		auditRepo:   auditRepo,
		logger:      logger,
		eventChan:   make(chan *AuditEvent, config.BufferSize),
		workerCount: config.WorkerCount,
		bufferSize:  config.BufferSize,
		ctx:         ctx,
		cancel:      cancel,
		started:     false,
	}
}

// Start starts the background workers
func (s *AuditService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("audit service already started")
	}

	// Start worker goroutines
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}

	s.started = true
	s.logger.Info("started audit service",
		zap.Int("worker_count", s.workerCount),
		zap.Int("buffer_size", s.bufferSize))

	return nil
}

// Stop gracefully stops the audit service
// Waits for all pending events to be processed
func (s *AuditService) Stop(timeout time.Duration) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return fmt.Errorf("audit service not started")
	}
	s.mu.Unlock()

	s.logger.Info("stopping audit service", zap.Int("pending_events", len(s.eventChan)))

	// Close the event channel (no more events will be accepted)
	close(s.eventChan)

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("audit service stopped gracefully")
		s.cancel()
		return nil
	case <-time.After(timeout):
		s.cancel()
		return fmt.Errorf("audit service stop timeout after %v", timeout)
	}
}

// LogEvent logs an event asynchronously (non-blocking)
// Returns immediately, event is processed in background
func (s *AuditService) LogEvent(event *AuditEvent) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return fmt.Errorf("audit service not started")
	}
	s.mu.Unlock()

	// Try to send event to channel (non-blocking)
	select {
	case s.eventChan <- event:
		return nil
	default:
		// Channel is full, log warning and drop event
		s.logger.Warn("audit event channel full, dropping event",
			zap.String("action", string(event.Log.Action)),
			zap.String("org_id", event.Log.OrgID.String()))
		return fmt.Errorf("audit event buffer full")
	}
}

// LogEventBlocking logs an event synchronously (blocking)
// Waits until event is queued or context is cancelled
func (s *AuditService) LogEventBlocking(ctx context.Context, event *AuditEvent) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return fmt.Errorf("audit service not started")
	}
	s.mu.Unlock()

	select {
	case s.eventChan <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return fmt.Errorf("audit service stopped")
	}
}

// worker processes events from the channel
func (s *AuditService) worker(id int) {
	defer s.wg.Done()

	s.logger.Debug("audit worker started", zap.Int("worker_id", id))

	for event := range s.eventChan {
		if err := s.processEvent(event); err != nil {
			s.logger.Error("failed to process audit event",
				zap.Int("worker_id", id),
				zap.Error(err),
				zap.String("action", string(event.Log.Action)),
				zap.String("org_id", event.Log.OrgID.String()))
		}
	}

	s.logger.Debug("audit worker stopped", zap.Int("worker_id", id))
}

// processEvent processes a single audit event
func (s *AuditService) processEvent(event *AuditEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.auditRepo.Insert(ctx, event.Log); err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	return nil
}

// GetStats returns statistics about the audit service
func (s *AuditService) GetStats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()

	return Stats{
		BufferSize:    s.bufferSize,
		PendingEvents: len(s.eventChan),
		WorkerCount:   s.workerCount,
		Started:       s.started,
	}
}

// Stats represents audit service statistics
type Stats struct {
	BufferSize    int
	PendingEvents int
	WorkerCount   int
	Started       bool
}

// Convenience methods for logging common events

// LogInferenceRequest logs an inference request event
func (s *AuditService) LogInferenceRequest(req *models.InferenceRequest) error {
	log := models.NewAuditLog(req.OrgID, models.AuditActionInferenceRequest, "inference_request")
	log.WithApp(req.AppID)
	if req.UserID != nil {
		log.WithUser(*req.UserID)
	}
	log.WithRequest(req.RequestID, req.IPAddress, req.UserAgent)
	log.WithLLMMetrics(req.Model, req.Provider, req.TotalTokens, req.LatencyMs, req.Cost)

	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}

	return s.LogEvent(event)
}

// LogInferenceResponse logs an inference response event
func (s *AuditService) LogInferenceResponse(req *models.InferenceRequest) error {
	log := models.NewAuditLog(req.OrgID, models.AuditActionInferenceResponse, "inference_request")
	log.WithApp(req.AppID)
	if req.UserID != nil {
		log.WithUser(*req.UserID)
	}
	log.WithRequest(req.RequestID, req.IPAddress, req.UserAgent)
	log.WithLLMMetrics(req.Model, req.Provider, req.TotalTokens, req.LatencyMs, req.Cost)

	if req.Status == models.InferenceStatusFailed && req.ErrorMessage != nil {
		statusCode := 500
		if req.ErrorCode != nil {
			// Map error codes to status codes if needed
			statusCode = 500
		}
		log.WithError(statusCode, *req.ErrorMessage)
	}

	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}

	return s.LogEvent(event)
}

// LogPolicyViolation logs a policy violation event
func (s *AuditService) LogPolicyViolation(req *models.InferenceRequest, policyID uuid.UUID, reason string, details interface{}) error {
	log := models.NewAuditLog(req.OrgID, models.AuditActionPolicyViolation, "policy")
	log.WithApp(req.AppID)
	if req.UserID != nil {
		log.WithUser(*req.UserID)
	}
	log.WithResource(policyID)
	log.WithRequest(req.RequestID, req.IPAddress, req.UserAgent)

	violationDetails := map[string]interface{}{
		"reason":  reason,
		"details": details,
		"model":   req.Model,
		"provider": req.Provider,
	}
	log.WithDetails(violationDetails)

	event := &AuditEvent{
		Log:      log,
		Priority: 2, // Higher priority for violations
	}

	return s.LogEvent(event)
}

// LogPolicyCreated logs a policy creation event
func (s *AuditService) LogPolicyCreated(policy *models.Policy, userID uuid.UUID) error {
	log := models.NewAuditLog(policy.OrgID, models.AuditActionPolicyCreated, "policy")
	if policy.AppID != nil {
		log.WithApp(*policy.AppID)
	}
	log.WithUser(userID)
	log.WithResource(policy.ID)

	details := map[string]interface{}{
		"policy_type": policy.PolicyType,
		"priority":    policy.Priority,
		"enabled":     policy.Enabled,
	}
	log.WithDetails(details)

	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}

	return s.LogEvent(event)
}

// LogPolicyUpdated logs a policy update event
func (s *AuditService) LogPolicyUpdated(policy *models.Policy, userID uuid.UUID, changes map[string]interface{}) error {
	log := models.NewAuditLog(policy.OrgID, models.AuditActionPolicyUpdated, "policy")
	if policy.AppID != nil {
		log.WithApp(*policy.AppID)
	}
	log.WithUser(userID)
	log.WithResource(policy.ID)

	details := map[string]interface{}{
		"policy_type": policy.PolicyType,
		"changes":     changes,
	}
	log.WithDetails(details)

	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}

	return s.LogEvent(event)
}

// LogPolicyDeleted logs a policy deletion event
func (s *AuditService) LogPolicyDeleted(orgID, policyID, userID uuid.UUID) error {
	log := models.NewAuditLog(orgID, models.AuditActionPolicyDeleted, "policy")
	log.WithUser(userID)
	log.WithResource(policyID)

	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}

	return s.LogEvent(event)
}

// LogUserCreated logs a user creation event
func (s *AuditService) LogUserCreated(user *models.User, creatorID uuid.UUID) error {
	log := models.NewAuditLog(user.OrgID, models.AuditActionUserCreated, "user")
	log.WithUser(creatorID)
	log.WithResource(user.ID)

	details := map[string]interface{}{
		"email": user.Email,
		"role":  user.Role,
	}
	log.WithDetails(details)

	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}

	return s.LogEvent(event)
}

// LogUserUpdated logs a user update event
func (s *AuditService) LogUserUpdated(user *models.User, updaterID uuid.UUID, changes map[string]interface{}) error {
	log := models.NewAuditLog(user.OrgID, models.AuditActionUserUpdated, "user")
	log.WithUser(updaterID)
	log.WithResource(user.ID)

	details := map[string]interface{}{
		"changes": changes,
	}
	log.WithDetails(details)

	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}

	return s.LogEvent(event)
}

// LogAppCreated logs an application creation event
func (s *AuditService) LogAppCreated(app *models.Application, creatorID uuid.UUID) error {
	log := models.NewAuditLog(app.OrgID, models.AuditActionAppCreated, "application")
	log.WithApp(app.ID)
	log.WithUser(creatorID)
	log.WithResource(app.ID)

	details := map[string]interface{}{
		"name": app.Name,
	}
	log.WithDetails(details)

	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}

	return s.LogEvent(event)
}

// LogAppUpdated logs an application update event
func (s *AuditService) LogAppUpdated(app *models.Application, updaterID uuid.UUID, changes map[string]interface{}) error {
	log := models.NewAuditLog(app.OrgID, models.AuditActionAppUpdated, "application")
	log.WithApp(app.ID)
	log.WithUser(updaterID)
	log.WithResource(app.ID)

	details := map[string]interface{}{
		"changes": changes,
	}
	log.WithDetails(details)

	event := &AuditEvent{
		Log:      log,
		Priority: 1,
	}

	return s.LogEvent(event)
}
