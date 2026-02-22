package routing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/upb/llm-control-plane/backend/services/providers"
)

var (
	// ErrNoProviderAvailable is returned when no provider can handle the request
	ErrNoProviderAvailable = errors.New("no provider available")

	// ErrModelValidationFailed is returned when model validation fails
	ErrModelValidationFailed = errors.New("model validation failed")

	// ErrRoutingStrategyNotFound is returned when a routing strategy is not found
	ErrRoutingStrategyNotFound = errors.New("routing strategy not found")
)

// RoutingStrategy defines how to select a provider
type RoutingStrategy string

const (
	// StrategyModelBased routes based on the model name
	StrategyModelBased RoutingStrategy = "model_based"

	// StrategyRoundRobin distributes requests across providers
	StrategyRoundRobin RoutingStrategy = "round_robin"

	// StrategyLowestCost selects the provider with lowest estimated cost
	StrategyLowestCost RoutingStrategy = "lowest_cost"

	// StrategyFastest selects the provider with lowest latency
	StrategyFastest RoutingStrategy = "fastest"

	// StrategyFailover tries providers in order until one succeeds
	StrategyFailover RoutingStrategy = "failover"
)

// RoutingConfig holds configuration for the routing service
type RoutingConfig struct {
	// DefaultStrategy is the default routing strategy
	DefaultStrategy RoutingStrategy

	// EnableFallback enables automatic fallback to alternative providers
	EnableFallback bool

	// FallbackProviders lists providers to try as fallbacks
	FallbackProviders []string

	// MaxRetries for failed requests
	MaxRetries int

	// Timeout for routing decisions
	Timeout time.Duration

	// EnableCostTracking tracks costs across requests
	EnableCostTracking bool

	// EnableLatencyTracking tracks latency across providers
	EnableLatencyTracking bool
}

// DefaultRoutingConfig returns a sensible default configuration
func DefaultRoutingConfig() RoutingConfig {
	return RoutingConfig{
		DefaultStrategy:       StrategyModelBased,
		EnableFallback:        true,
		MaxRetries:            2,
		Timeout:               30 * time.Second,
		EnableCostTracking:    true,
		EnableLatencyTracking: true,
	}
}

// RoutingService handles request routing to appropriate providers
type RoutingService struct {
	config          RoutingConfig
	registry        *providers.Registry
	latencyTracker  map[string]time.Duration
	requestCounter  map[string]int
	roundRobinIndex map[string]int
}

// NewRoutingService creates a new routing service
func NewRoutingService(config RoutingConfig, registry *providers.Registry) *RoutingService {
	return &RoutingService{
		config:          config,
		registry:        registry,
		latencyTracker:  make(map[string]time.Duration),
		requestCounter:  make(map[string]int),
		roundRobinIndex: make(map[string]int),
	}
}

// RouteRequest routes a request to the appropriate provider
func (s *RoutingService) RouteRequest(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	// Validate model
	if err := s.ValidateModel(req.Model); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrModelValidationFailed, err)
	}

	// Select provider based on strategy
	provider, err := s.selectProvider(ctx, req)
	if err != nil {
		return nil, err
	}

	// Execute request with retries
	var lastErr error
	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		resp, err := s.executeRequest(ctx, provider, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Try fallback if enabled
		if s.config.EnableFallback && providers.IsRetryable(err) {
			fallbackProvider, fallbackErr := s.selectFallbackProvider(ctx, req, provider.Name())
			if fallbackErr == nil {
				provider = fallbackProvider
				continue
			}
		}

		// If not retryable, break
		if !providers.IsRetryable(err) {
			break
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", s.config.MaxRetries+1, lastErr)
}

// ValidateModel validates if a model is supported
func (s *RoutingService) ValidateModel(model string) error {
	return s.registry.ValidateModel(model)
}

// GetProviderForModel returns the provider that supports a model
func (s *RoutingService) GetProviderForModel(model string) (providers.Provider, error) {
	return s.registry.GetProviderForModel(model)
}

// selectProvider selects a provider based on the configured strategy
func (s *RoutingService) selectProvider(ctx context.Context, req *providers.ChatRequest) (providers.Provider, error) {
	strategy := s.config.DefaultStrategy

	// Override strategy from request metadata if specified
	if strategyStr, ok := req.Metadata["routing_strategy"]; ok {
		strategy = RoutingStrategy(strategyStr)
	}

	switch strategy {
	case StrategyModelBased:
		return s.selectByModel(ctx, req)
	case StrategyRoundRobin:
		return s.selectRoundRobin(ctx, req)
	case StrategyLowestCost:
		return s.selectLowestCost(ctx, req)
	case StrategyFastest:
		return s.selectFastest(ctx, req)
	case StrategyFailover:
		return s.selectFailover(ctx, req)
	default:
		return nil, fmt.Errorf("%w: %s", ErrRoutingStrategyNotFound, strategy)
	}
}

// selectByModel selects provider based on model
func (s *RoutingService) selectByModel(ctx context.Context, req *providers.ChatRequest) (providers.Provider, error) {
	provider, err := s.registry.GetProviderForModel(req.Model)
	if err != nil {
		return nil, err
	}

	// Check availability
	if !provider.IsAvailable(ctx) {
		if s.config.EnableFallback {
			return s.selectFallbackProvider(ctx, req, provider.Name())
		}
		return nil, fmt.Errorf("provider %s is not available", provider.Name())
	}

	return provider, nil
}

// selectRoundRobin selects provider using round-robin
func (s *RoutingService) selectRoundRobin(ctx context.Context, req *providers.ChatRequest) (providers.Provider, error) {
	providerNames := s.registry.ListProviders()
	if len(providerNames) == 0 {
		return nil, ErrNoProviderAvailable
	}

	// Get or initialize round-robin index for this model
	key := req.Model
	index := s.roundRobinIndex[key]
	s.roundRobinIndex[key] = (index + 1) % len(providerNames)

	// Try to get provider
	providerName := providerNames[index]
	provider, err := s.registry.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	// Validate that provider supports the model
	if err := provider.ValidateModel(req.Model); err != nil {
		// Try next provider in round-robin
		return s.selectRoundRobin(ctx, req)
	}

	return provider, nil
}

// selectLowestCost selects provider with lowest estimated cost
func (s *RoutingService) selectLowestCost(ctx context.Context, req *providers.ChatRequest) (providers.Provider, error) {
	providerNames := s.registry.ListProviders()
	if len(providerNames) == 0 {
		return nil, ErrNoProviderAvailable
	}

	var bestProvider providers.Provider
	var lowestCost float64 = -1

	for _, name := range providerNames {
		provider, err := s.registry.GetProvider(name)
		if err != nil {
			continue
		}

		// Check if provider supports the model
		if err := provider.ValidateModel(req.Model); err != nil {
			continue
		}

		// Estimate cost
		cost, err := provider.EstimateCost(req)
		if err != nil {
			continue
		}

		if lowestCost < 0 || cost < lowestCost {
			lowestCost = cost
			bestProvider = provider
		}
	}

	if bestProvider == nil {
		return nil, ErrNoProviderAvailable
	}

	return bestProvider, nil
}

// selectFastest selects provider with lowest recorded latency
func (s *RoutingService) selectFastest(ctx context.Context, req *providers.ChatRequest) (providers.Provider, error) {
	providerNames := s.registry.ListProviders()
	if len(providerNames) == 0 {
		return nil, ErrNoProviderAvailable
	}

	var bestProvider providers.Provider
	var lowestLatency time.Duration = -1

	for _, name := range providerNames {
		provider, err := s.registry.GetProvider(name)
		if err != nil {
			continue
		}

		// Check if provider supports the model
		if err := provider.ValidateModel(req.Model); err != nil {
			continue
		}

		// Get recorded latency
		latency, exists := s.latencyTracker[name]
		if !exists {
			// No latency data, use this provider with default priority
			if bestProvider == nil {
				bestProvider = provider
			}
			continue
		}

		if lowestLatency < 0 || latency < lowestLatency {
			lowestLatency = latency
			bestProvider = provider
		}
	}

	if bestProvider == nil {
		// No latency data exists, fall back to model-based selection
		return s.selectByModel(ctx, req)
	}

	return bestProvider, nil
}

// selectFailover tries providers in order
func (s *RoutingService) selectFailover(ctx context.Context, req *providers.ChatRequest) (providers.Provider, error) {
	// First try model-based selection
	provider, err := s.selectByModel(ctx, req)
	if err == nil && provider.IsAvailable(ctx) {
		return provider, nil
	}

	// Try configured fallback providers
	for _, providerName := range s.config.FallbackProviders {
		provider, err := s.registry.GetProvider(providerName)
		if err != nil {
			continue
		}

		if err := provider.ValidateModel(req.Model); err != nil {
			continue
		}

		if provider.IsAvailable(ctx) {
			return provider, nil
		}
	}

	// Try any available provider
	providerNames := s.registry.ListProviders()
	for _, name := range providerNames {
		provider, err := s.registry.GetProvider(name)
		if err != nil {
			continue
		}

		if err := provider.ValidateModel(req.Model); err != nil {
			continue
		}

		if provider.IsAvailable(ctx) {
			return provider, nil
		}
	}

	return nil, ErrNoProviderAvailable
}

// selectFallbackProvider selects an alternative provider
func (s *RoutingService) selectFallbackProvider(ctx context.Context, req *providers.ChatRequest, excludeProvider string) (providers.Provider, error) {
	providerNames := s.registry.ListProviders()

	for _, name := range providerNames {
		if name == excludeProvider {
			continue
		}

		provider, err := s.registry.GetProvider(name)
		if err != nil {
			continue
		}

		// Check if provider supports the model
		if err := provider.ValidateModel(req.Model); err != nil {
			continue
		}

		// Check availability
		if !provider.IsAvailable(ctx) {
			continue
		}

		return provider, nil
	}

	return nil, ErrNoProviderAvailable
}

// executeRequest executes the request with the selected provider
func (s *RoutingService) executeRequest(ctx context.Context, provider providers.Provider, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	startTime := time.Now()

	resp, err := provider.ChatCompletion(ctx, req)

	if err == nil {
		// Track latency
		if s.config.EnableLatencyTracking {
			latency := time.Since(startTime)
			s.latencyTracker[provider.Name()] = latency
		}

		// Track request count
		s.requestCounter[provider.Name()]++
	}

	return resp, err
}

// GetStats returns routing statistics
func (s *RoutingService) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Request counts
	stats["request_counts"] = s.requestCounter

	// Latency stats
	if s.config.EnableLatencyTracking {
		stats["latencies"] = s.latencyTracker
	}

	// Provider stats
	stats["provider_stats"] = s.registry.GetProviderStats()

	return stats
}

// ResetStats resets all tracking statistics
func (s *RoutingService) ResetStats() {
	s.latencyTracker = make(map[string]time.Duration)
	s.requestCounter = make(map[string]int)
	s.roundRobinIndex = make(map[string]int)
}

// SetStrategy updates the default routing strategy
func (s *RoutingService) SetStrategy(strategy RoutingStrategy) {
	s.config.DefaultStrategy = strategy
}

// GetStrategy returns the current default routing strategy
func (s *RoutingService) GetStrategy() RoutingStrategy {
	return s.config.DefaultStrategy
}

// ListAvailableProviders returns all currently available providers
func (s *RoutingService) ListAvailableProviders(ctx context.Context) []string {
	var available []string

	providerNames := s.registry.ListProviders()
	for _, name := range providerNames {
		provider, err := s.registry.GetProvider(name)
		if err != nil {
			continue
		}

		if provider.IsAvailable(ctx) {
			available = append(available, name)
		}
	}

	return available
}

// ListSupportedModels returns all models supported by available providers
func (s *RoutingService) ListSupportedModels(ctx context.Context) []string {
	return s.registry.ListModels()
}

// GetModelInfo retrieves information about a specific model
func (s *RoutingService) GetModelInfo(model string) (*providers.ModelInfo, error) {
	return s.registry.GetModelInfo(model)
}

// EstimateCost estimates the cost for a request using the best provider
func (s *RoutingService) EstimateCost(ctx context.Context, req *providers.ChatRequest) (float64, string, error) {
	provider, err := s.selectProvider(ctx, req)
	if err != nil {
		return 0, "", err
	}

	cost, err := provider.EstimateCost(req)
	if err != nil {
		return 0, "", err
	}

	return cost, provider.Name(), nil
}
