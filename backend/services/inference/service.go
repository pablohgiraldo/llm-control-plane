package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/models"
	"github.com/upb/llm-control-plane/backend/services/audit"
	"github.com/upb/llm-control-plane/backend/services/budget"
	"github.com/upb/llm-control-plane/backend/services/policy"
	"github.com/upb/llm-control-plane/backend/services/prompt"
	"github.com/upb/llm-control-plane/backend/services/providers"
	"github.com/upb/llm-control-plane/backend/services/ratelimit"
	"github.com/upb/llm-control-plane/backend/services/routing"
	"go.uber.org/zap"
)

// InferenceService orchestrates the complete inference pipeline
type InferenceService struct {
	policyService    *policy.PolicyService
	rateLimitService *ratelimit.RateLimitService
	budgetService    *budget.BudgetService
	promptService    *prompt.PromptService
	routingService   *routing.RoutingService
	auditService     *audit.AuditService
	logger           *zap.Logger
}

// NewInferenceService creates a new inference service with all dependencies
func NewInferenceService(
	policyService *policy.PolicyService,
	rateLimitService *ratelimit.RateLimitService,
	budgetService *budget.BudgetService,
	promptService *prompt.PromptService,
	routingService *routing.RoutingService,
	auditService *audit.AuditService,
	logger *zap.Logger,
) *InferenceService {
	return &InferenceService{
		policyService:    policyService,
		rateLimitService: rateLimitService,
		budgetService:    budgetService,
		promptService:    promptService,
		routingService:   routingService,
		auditService:     auditService,
		logger:           logger,
	}
}

// ProcessChatCompletion processes a chat completion request through the full pipeline
func (s *InferenceService) ProcessChatCompletion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// Initialize pipeline context
	pipelineCtx := &PipelineContext{
		Request:     req,
		InferenceID: uuid.New(),
		StartTime:   time.Now(),
	}

	s.logger.Info("starting inference pipeline",
		zap.String("inference_id", pipelineCtx.InferenceID.String()),
		zap.String("org_id", req.OrgID.String()),
		zap.String("app_id", req.AppID.String()),
		zap.String("model", req.Model))

	// Create inference request record
	inferenceReq := s.createInferenceRequest(req, pipelineCtx.InferenceID)

	// Step 1: Evaluate policies
	s.logger.Debug("step 1: evaluating policies", zap.String("inference_id", pipelineCtx.InferenceID.String()))
	policyResult, err := s.evaluatePolicies(ctx, req, pipelineCtx)
	if err != nil {
		s.handleError(inferenceReq, err)
		return nil, err
	}
	pipelineCtx.PolicyResult = policyResult

	// Step 2: Check rate limits
	s.logger.Debug("step 2: checking rate limits", zap.String("inference_id", pipelineCtx.InferenceID.String()))
	if err := s.checkRateLimit(ctx, req, policyResult, pipelineCtx); err != nil {
		s.handleError(inferenceReq, err)
		return nil, err
	}
	pipelineCtx.RateLimitPassed = true

	// Step 3: Validate prompt
	s.logger.Debug("step 3: validating prompt", zap.String("inference_id", pipelineCtx.InferenceID.String()))
	if err := s.validatePrompt(ctx, req, policyResult, pipelineCtx); err != nil {
		s.handleError(inferenceReq, err)
		return nil, err
	}
	pipelineCtx.PromptValidated = true

	// Step 4: Estimate cost and check budget (pre-check)
	s.logger.Debug("step 4: checking budget", zap.String("inference_id", pipelineCtx.InferenceID.String()))
	if err := s.checkBudget(ctx, req, policyResult, pipelineCtx); err != nil {
		s.handleError(inferenceReq, err)
		return nil, err
	}
	pipelineCtx.BudgetPassed = true

	// Step 5: Route to provider
	s.logger.Debug("step 5: routing to provider", zap.String("inference_id", pipelineCtx.InferenceID.String()))
	selectedProvider, providerReq, err := s.routeToProvider(ctx, req, pipelineCtx)
	if err != nil {
		s.handleError(inferenceReq, err)
		return nil, err
	}
	pipelineCtx.SelectedProvider = selectedProvider.Name()

	// Mark as processing
	inferenceReq.MarkAsProcessing()
	inferenceReq.Provider = selectedProvider.Name()

	// Step 6: Invoke LLM
	s.logger.Debug("step 6: invoking LLM",
		zap.String("inference_id", pipelineCtx.InferenceID.String()),
		zap.String("provider", selectedProvider.Name()))
	
	providerResp, err := s.invokeLLM(ctx, selectedProvider, providerReq)
	if err != nil {
		s.handleError(inferenceReq, err)
		return nil, err
	}
	pipelineCtx.ProviderResponse = providerResp

	// Step 7: Validate response
	s.logger.Debug("step 7: validating response", zap.String("inference_id", pipelineCtx.InferenceID.String()))
	if err := s.validateResponse(ctx, providerResp, pipelineCtx); err != nil {
		s.logger.Warn("response validation failed", zap.Error(err))
		// Don't fail the request, just log
	}

	// Step 8: Calculate actual cost
	s.logger.Debug("step 8: calculating cost", zap.String("inference_id", pipelineCtx.InferenceID.String()))
	actualCost, err := s.calculateCost(selectedProvider, providerResp)
	if err != nil {
		s.logger.Error("failed to calculate cost", zap.Error(err))
		actualCost = pipelineCtx.EstimatedCost // Fallback to estimate
	}
	pipelineCtx.ActualCost = actualCost

	// Step 9: Update budget
	s.logger.Debug("step 9: updating budget", zap.String("inference_id", pipelineCtx.InferenceID.String()))
	if err := s.updateBudget(ctx, req, actualCost, providerResp); err != nil {
		s.logger.Error("failed to update budget", zap.Error(err))
		// Don't fail the request
	}

	// Step 10: Record rate limit
	if err := s.recordRateLimit(ctx, req, policyResult, providerResp); err != nil {
		s.logger.Error("failed to record rate limit", zap.Error(err))
		// Don't fail the request
	}

	// Build response
	response := s.buildResponse(req, inferenceReq, providerResp, pipelineCtx)

	// Mark inference as completed
	latencyMs := int(time.Since(pipelineCtx.StartTime).Milliseconds())
	if len(providerResp.Choices) > 0 {
		content := providerResp.Choices[0].Message.Content
		inferenceReq.MarkAsCompleted(
			content,
			providerResp.Choices[0].FinishReason,
			providerResp.Usage.PromptTokens,
			providerResp.Usage.CompletionTokens,
			latencyMs,
			actualCost,
		)
	}

	// Step 11: Async audit logging
	s.logger.Debug("step 10: logging audit event", zap.String("inference_id", pipelineCtx.InferenceID.String()))
	go s.logAudit(inferenceReq, policyResult)

	s.logger.Info("inference pipeline completed",
		zap.String("inference_id", pipelineCtx.InferenceID.String()),
		zap.Int("latency_ms", latencyMs),
		zap.Float64("cost", actualCost),
		zap.Int("tokens", providerResp.Usage.TotalTokens))

	return response, nil
}

// evaluatePolicies evaluates all applicable policies
func (s *InferenceService) evaluatePolicies(ctx context.Context, req *CompletionRequest, pipelineCtx *PipelineContext) (*policy.EvaluationResult, error) {
	evalReq := policy.EvaluationRequest{
		OrgID:    req.OrgID,
		AppID:    req.AppID,
		UserID:   req.UserID,
		Provider: s.getRequestedProvider(req),
		Model:    req.Model,
		Prompt:   s.combineMessages(req.Messages),
	}

	result, err := s.policyService.Evaluate(ctx, evalReq)
	if err != nil {
		return nil, NewInternalError("failed to evaluate policies", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Check if request is allowed
	if !result.Allowed {
		violations := make([]string, len(result.Violations))
		for i, v := range result.Violations {
			violations[i] = v.Reason
		}
		return nil, NewPolicyViolationError("request denied by policy", map[string]interface{}{
			"violations": violations,
		})
	}

	// Store applied policies
	for _, p := range result.AppliedPolicies {
		pipelineCtx.AppliedPolicies = append(pipelineCtx.AppliedPolicies, p.ID)
	}

	return result, nil
}

// checkRateLimit checks rate limits
func (s *InferenceService) checkRateLimit(ctx context.Context, req *CompletionRequest, policyResult *policy.EvaluationResult, pipelineCtx *PipelineContext) error {
	if policyResult.RateLimitConfig == nil {
		return nil // No rate limit configured
	}

	rateLimitReq := ratelimit.RateLimitRequest{
		OrgID:    req.OrgID,
		AppID:    req.AppID,
		UserID:   req.UserID,
		Config:   policyResult.RateLimitConfig,
		TokensUsed: s.estimatePromptTokens(req.Messages),
	}

	result, err := s.rateLimitService.CheckLimit(ctx, rateLimitReq)
	if err != nil {
		return NewInternalError("failed to check rate limit", map[string]interface{}{
			"error": err.Error(),
		})
	}

	if !result.Allowed {
		return NewRateLimitError(result.ViolationReason, map[string]interface{}{
			"window":     result.ViolatedWindow,
			"reset_at":   result.ResetAt,
			"remaining":  result.RequestsRemaining,
		})
	}

	return nil
}

// validatePrompt validates the prompt content
func (s *InferenceService) validatePrompt(ctx context.Context, req *CompletionRequest, policyResult *policy.EvaluationResult, pipelineCtx *PipelineContext) error {
	// Convert messages to prompt validation format
	promptMessages := make([]prompt.Message, len(req.Messages))
	for i, msg := range req.Messages {
		promptMessages[i] = prompt.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	validationResult, err := s.promptService.ValidateMessages(ctx, promptMessages)
	if err != nil {
		return NewInternalError("failed to validate prompt", map[string]interface{}{
			"error": err.Error(),
		})
	}

	if !validationResult.Valid {
		return NewPolicyViolationError("prompt validation failed", map[string]interface{}{
			"errors":   validationResult.Errors,
			"warnings": validationResult.Warnings,
		})
	}

	// Store sanitized prompt if redaction occurred
	if validationResult.SanitizedPrompt != "" {
		pipelineCtx.SanitizedPrompt = validationResult.SanitizedPrompt
	}

	return nil
}

// checkBudget performs pre-check on budget
func (s *InferenceService) checkBudget(ctx context.Context, req *CompletionRequest, policyResult *policy.EvaluationResult, pipelineCtx *PipelineContext) error {
	if policyResult.BudgetConfig == nil {
		return nil // No budget configured
	}

	// Estimate cost based on model and estimated tokens
	estimatedTokens := s.estimatePromptTokens(req.Messages) + (req.MaxTokens / 2) // Rough estimate
	estimatedCost := s.estimateCostForTokens(req.Model, estimatedTokens)
	pipelineCtx.EstimatedCost = estimatedCost

	budgetReq := budget.BudgetCheckRequest{
		OrgID:  req.OrgID,
		AppID:  req.AppID,
		UserID: req.UserID,
		Config: policyResult.BudgetConfig,
		Cost:   estimatedCost,
	}

	result, err := s.budgetService.CheckBudget(ctx, budgetReq)
	if err != nil {
		return NewInternalError("failed to check budget", map[string]interface{}{
			"error": err.Error(),
		})
	}

	if !result.Allowed {
		return NewBudgetError(result.ViolationReason, map[string]interface{}{
			"period":        result.ViolatedPeriod,
			"daily_spend":   result.DailySpend,
			"daily_limit":   result.DailyLimit,
			"monthly_spend": result.MonthlySpend,
			"monthly_limit": result.MonthlyLimit,
		})
	}

	return nil
}

// routeToProvider selects and routes to the appropriate provider
func (s *InferenceService) routeToProvider(ctx context.Context, req *CompletionRequest, pipelineCtx *PipelineContext) (providers.Provider, *providers.ChatRequest, error) {
	// Build provider request
	providerReq := &providers.ChatRequest{
		Model:            req.Model,
		Messages:         req.Messages,
		MaxTokens:        req.MaxTokens,
		Temperature:      req.Temperature,
		TopP:             req.TopP,
		FrequencyPenalty: req.FrequencyPenalty,
		PresencePenalty:  req.PresencePenalty,
		Stop:             req.Stop,
		Stream:           req.Stream,
		User:             req.UserID.String(),
		Metadata:         req.Metadata,
	}

	// Use routing service to select provider
	selectedProvider, err := s.routingService.GetProviderForModel(req.Model)
	if err != nil {
		return nil, nil, NewProviderError("failed to route request", map[string]interface{}{
			"model": req.Model,
			"error": err.Error(),
		}, false)
	}

	return selectedProvider, providerReq, nil
}

// invokeLLM calls the LLM provider
func (s *InferenceService) invokeLLM(ctx context.Context, provider providers.Provider, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	resp, err := provider.ChatCompletion(ctx, req)
	if err != nil {
		// Check if retryable
		retryable := providers.IsRetryable(err)
		return nil, NewProviderError(fmt.Sprintf("LLM invocation failed: %v", err), map[string]interface{}{
			"provider": provider.Name(),
			"model":    req.Model,
		}, retryable)
	}

	return resp, nil
}

// validateResponse validates the LLM response
func (s *InferenceService) validateResponse(ctx context.Context, resp *providers.ChatResponse, pipelineCtx *PipelineContext) error {
	if len(resp.Choices) == 0 {
		return NewProviderError("empty response from provider", nil, false)
	}

	// Validate response content
	content := resp.Choices[0].Message.Content
	validationResult, err := s.promptService.ValidateResponse(ctx, content)
	if err != nil {
		return err
	}

	// Log warnings if any
	if len(validationResult.Warnings) > 0 {
		s.logger.Warn("response validation warnings",
			zap.Strings("warnings", validationResult.Warnings))
	}

	return nil
}

// calculateCost calculates the actual cost based on token usage
func (s *InferenceService) calculateCost(provider providers.Provider, resp *providers.ChatResponse) (float64, error) {
	// Get model info for pricing
	modelInfo, err := provider.GetModelInfo(resp.Model)
	if err != nil {
		return 0, err
	}

	promptCost := float64(resp.Usage.PromptTokens) * modelInfo.PricingPerPromptToken
	completionCost := float64(resp.Usage.CompletionTokens) * modelInfo.PricingPerCompletionToken

	return promptCost + completionCost, nil
}

// updateBudget records the cost
func (s *InferenceService) updateBudget(ctx context.Context, req *CompletionRequest, cost float64, resp *providers.ChatResponse) error {
	budgetReq := budget.CostRecordRequest{
		OrgID:      req.OrgID,
		AppID:      req.AppID,
		UserID:     req.UserID,
		Cost:       cost,
		Currency:   "USD",
		Provider:   resp.Provider,
		Model:      resp.Model,
		RequestID:  req.RequestID,
		TokensUsed: resp.Usage.TotalTokens,
	}

	return s.budgetService.RecordCost(ctx, budgetReq)
}

// recordRateLimit records the request for rate limiting
func (s *InferenceService) recordRateLimit(ctx context.Context, req *CompletionRequest, policyResult *policy.EvaluationResult, resp *providers.ChatResponse) error {
	if policyResult.RateLimitConfig == nil {
		return nil
	}

	rateLimitReq := ratelimit.RateLimitRequest{
		OrgID:      req.OrgID,
		AppID:      req.AppID,
		UserID:     req.UserID,
		Config:     policyResult.RateLimitConfig,
		TokensUsed: resp.Usage.TotalTokens,
	}

	return s.rateLimitService.RecordRequest(ctx, rateLimitReq)
}

// logAudit logs the inference request to audit log
func (s *InferenceService) logAudit(inferenceReq *models.InferenceRequest, policyResult *policy.EvaluationResult) {
	if err := s.auditService.LogInferenceRequest(inferenceReq); err != nil {
		s.logger.Error("failed to log audit event", zap.Error(err))
	}
}

// Helper methods

func (s *InferenceService) createInferenceRequest(req *CompletionRequest, inferenceID uuid.UUID) *models.InferenceRequest {
	messagesJSON, _ := json.Marshal(req.Messages)
	
	inferenceReq := &models.InferenceRequest{
		ID:        inferenceID,
		OrgID:     req.OrgID,
		AppID:     req.AppID,
		UserID:    req.UserID,
		RequestID: req.RequestID,
		Status:    models.InferenceStatusPending,
		Model:     req.Model,
		Messages:  messagesJSON,
		IPAddress: req.IPAddress,
		UserAgent: req.UserAgent,
		CreatedAt: time.Now(),
	}

	return inferenceReq
}

func (s *InferenceService) buildResponse(req *CompletionRequest, inferenceReq *models.InferenceRequest, providerResp *providers.ChatResponse, pipelineCtx *PipelineContext) *CompletionResponse {
	choices := make([]Choice, len(providerResp.Choices))
	for i, c := range providerResp.Choices {
		choices[i] = Choice{
			Index:        c.Index,
			Message:      c.Message,
			FinishReason: c.FinishReason,
		}
	}

	return &CompletionResponse{
		ID:        pipelineCtx.InferenceID,
		RequestID: req.RequestID,
		Provider:  providerResp.Provider,
		Model:     providerResp.Model,
		Choices:   choices,
		Usage: Usage{
			PromptTokens:     providerResp.Usage.PromptTokens,
			CompletionTokens: providerResp.Usage.CompletionTokens,
			TotalTokens:      providerResp.Usage.TotalTokens,
		},
		Cost:            pipelineCtx.ActualCost,
		Currency:        "USD",
		LatencyMs:       int(time.Since(pipelineCtx.StartTime).Milliseconds()),
		CreatedAt:       pipelineCtx.StartTime,
		CompletedAt:     time.Now(),
		PoliciesApplied: pipelineCtx.AppliedPolicies,
		Metadata:        req.Metadata,
	}
}

func (s *InferenceService) handleError(inferenceReq *models.InferenceRequest, err error) {
	if inferenceErr, ok := err.(*InferenceError); ok {
		inferenceReq.MarkAsFailed(inferenceErr.Code, inferenceErr.Message)
	} else {
		inferenceReq.MarkAsFailed(ErrCodeInternal, err.Error())
	}

	// Log audit event for failed request
	go s.auditService.LogInferenceRequest(inferenceReq)
}

func (s *InferenceService) combineMessages(messages []providers.Message) string {
	var combined string
	for i, msg := range messages {
		if i > 0 {
			combined += "\n"
		}
		combined += msg.Content
	}
	return combined
}

func (s *InferenceService) getRequestedProvider(req *CompletionRequest) string {
	if req.Provider != nil {
		return *req.Provider
	}
	return ""
}

func (s *InferenceService) estimatePromptTokens(messages []providers.Message) int {
	// Rough estimate: ~4 characters per token
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
	}
	return totalChars / 4
}

func (s *InferenceService) estimateCostForTokens(model string, tokens int) float64 {
	// Rough estimates - should be replaced with actual pricing
	pricePerToken := 0.00001 // Default estimate
	
	// Model-specific pricing (examples)
	switch model {
	case "gpt-4":
		pricePerToken = 0.00003
	case "gpt-3.5-turbo":
		pricePerToken = 0.000001
	}
	
	return float64(tokens) * pricePerToken
}
