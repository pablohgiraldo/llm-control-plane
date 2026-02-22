package providers

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	// ErrProviderNotFound is returned when a provider is not registered
	ErrProviderNotFound = errors.New("provider not found")

	// ErrModelNotSupported is returned when a model is not supported by any provider
	ErrModelNotSupported = errors.New("model not supported")

	// ErrProviderAlreadyRegistered is returned when trying to register a duplicate provider
	ErrProviderAlreadyRegistered = errors.New("provider already registered")
)

// Registry manages provider instances and model mappings
type Registry struct {
	mu             sync.RWMutex
	providers      map[string]Provider
	modelProviders map[string]string // model -> provider name
	modelPrefixes  map[string]string // model prefix -> provider name
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers:      make(map[string]Provider),
		modelProviders: make(map[string]string),
		modelPrefixes:  make(map[string]string),
	}
}

// RegisterProvider registers a provider instance
func (r *Registry) RegisterProvider(provider Provider) error {
	if provider == nil {
		return errors.New("provider cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	name := provider.Name()
	if name == "" {
		return errors.New("provider name cannot be empty")
	}

	// Check if already registered
	if _, exists := r.providers[name]; exists {
		return ErrProviderAlreadyRegistered
	}

	// Register the provider
	r.providers[name] = provider

	// Register all models from the provider
	models := provider.ListModels()
	for _, model := range models {
		r.modelProviders[model] = name
	}

	return nil
}

// UnregisterProvider removes a provider from the registry
func (r *Registry) UnregisterProvider(providerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[providerName]; !exists {
		return ErrProviderNotFound
	}

	// Remove provider
	delete(r.providers, providerName)

	// Remove all model mappings for this provider
	for model, provider := range r.modelProviders {
		if provider == providerName {
			delete(r.modelProviders, model)
		}
	}

	return nil
}

// GetProvider retrieves a provider by name
func (r *Registry) GetProvider(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, ErrProviderNotFound
	}

	return provider, nil
}

// GetProviderForModel finds the provider that supports a given model
func (r *Registry) GetProviderForModel(model string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Direct model lookup
	if providerName, exists := r.modelProviders[model]; exists {
		if provider, ok := r.providers[providerName]; ok {
			return provider, nil
		}
	}

	// Try prefix matching (e.g., "gpt-" -> "openai")
	for prefix, providerName := range r.modelPrefixes {
		if strings.HasPrefix(model, prefix) {
			if provider, ok := r.providers[providerName]; ok {
				// Validate that the provider actually supports this model
				if err := provider.ValidateModel(model); err == nil {
					// Cache this mapping for future lookups
					r.mu.RUnlock()
					r.mu.Lock()
					r.modelProviders[model] = providerName
					r.mu.Unlock()
					r.mu.RLock()
					return provider, nil
				}
			}
		}
	}

	// Try each provider to see if it supports the model
	for _, provider := range r.providers {
		if err := provider.ValidateModel(model); err == nil {
			// Cache this mapping
			r.mu.RUnlock()
			r.mu.Lock()
			r.modelProviders[model] = provider.Name()
			r.mu.Unlock()
			r.mu.RLock()
			return provider, nil
		}
	}

	return nil, ErrModelNotSupported
}

// RegisterModelPrefix registers a model prefix to provider mapping
// This is useful for efficient lookups (e.g., "gpt-" -> "openai")
func (r *Registry) RegisterModelPrefix(prefix, providerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[providerName]; !exists {
		return ErrProviderNotFound
	}

	r.modelPrefixes[prefix] = providerName
	return nil
}

// ListProviders returns all registered provider names
func (r *Registry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}

// ListModels returns all supported models across all providers
func (r *Registry) ListModels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]string, 0, len(r.modelProviders))
	for model := range r.modelProviders {
		models = append(models, model)
	}

	return models
}

// GetModelsByProvider returns all models supported by a specific provider
func (r *Registry) GetModelsByProvider(providerName string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[providerName]
	if !exists {
		return nil, ErrProviderNotFound
	}

	return provider.ListModels(), nil
}

// GetProviderCount returns the number of registered providers
func (r *Registry) GetProviderCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.providers)
}

// ValidateModel checks if a model is supported by any provider
func (r *Registry) ValidateModel(model string) error {
	_, err := r.GetProviderForModel(model)
	return err
}

// GetModelInfo retrieves model information
func (r *Registry) GetModelInfo(model string) (*ModelInfo, error) {
	provider, err := r.GetProviderForModel(model)
	if err != nil {
		return nil, err
	}

	return provider.GetModelInfo(model)
}

// GetAllModelInfo returns information for all models across all providers
func (r *Registry) GetAllModelInfo() ([]*ModelInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allInfo []*ModelInfo

	for _, provider := range r.providers {
		models := provider.ListModels()
		for _, model := range models {
			info, err := provider.GetModelInfo(model)
			if err != nil {
				// Skip models that fail to provide info
				continue
			}
			allInfo = append(allInfo, info)
		}
	}

	return allInfo, nil
}

// Clear removes all providers from the registry
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers = make(map[string]Provider)
	r.modelProviders = make(map[string]string)
	r.modelPrefixes = make(map[string]string)
}

// RegisterModelMapping manually registers a model to provider mapping
func (r *Registry) RegisterModelMapping(model, providerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[providerName]; !exists {
		return ErrProviderNotFound
	}

	r.modelProviders[model] = providerName
	return nil
}

// UnregisterModelMapping removes a model to provider mapping
func (r *Registry) UnregisterModelMapping(model string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.modelProviders, model)
}

// GetProviderStats returns statistics about providers and models
func (r *Registry) GetProviderStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["provider_count"] = len(r.providers)
	stats["model_count"] = len(r.modelProviders)
	stats["prefix_count"] = len(r.modelPrefixes)

	// Count models per provider
	providerModels := make(map[string]int)
	for _, providerName := range r.modelProviders {
		providerModels[providerName]++
	}
	stats["models_per_provider"] = providerModels

	return stats
}

// FindModels searches for models matching a pattern
func (r *Registry) FindModels(pattern string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []string
	pattern = strings.ToLower(pattern)

	for model := range r.modelProviders {
		if strings.Contains(strings.ToLower(model), pattern) {
			matches = append(matches, model)
		}
	}

	return matches
}

// Global default registry
var defaultRegistry *Registry
var registryOnce sync.Once

// DefaultRegistry returns the global default registry
func DefaultRegistry() *Registry {
	registryOnce.Do(func() {
		defaultRegistry = NewRegistry()
	})
	return defaultRegistry
}

// Register registers a provider in the default registry
func Register(provider Provider) error {
	return DefaultRegistry().RegisterProvider(provider)
}

// GetProvider retrieves a provider from the default registry
func GetProvider(name string) (Provider, error) {
	return DefaultRegistry().GetProvider(name)
}

// GetProviderForModel finds a provider for a model in the default registry
func GetProviderForModel(model string) (Provider, error) {
	return DefaultRegistry().GetProviderForModel(model)
}

// ListProviders lists all providers in the default registry
func ListProviders() []string {
	return DefaultRegistry().ListProviders()
}

// ListModels lists all models in the default registry
func ListModels() []string {
	return DefaultRegistry().ListModels()
}

// ProviderBuilder is a function that creates a provider instance
type ProviderBuilder func(config ProviderConfig) (Provider, error)

// RegistryBuilder helps build a registry with multiple providers
type RegistryBuilder struct {
	registry *Registry
	builders map[string]ProviderBuilder
}

// NewRegistryBuilder creates a new registry builder
func NewRegistryBuilder() *RegistryBuilder {
	return &RegistryBuilder{
		registry: NewRegistry(),
		builders: make(map[string]ProviderBuilder),
	}
}

// WithProviderBuilder registers a provider builder
func (rb *RegistryBuilder) WithProviderBuilder(name string, builder ProviderBuilder) *RegistryBuilder {
	rb.builders[name] = builder
	return rb
}

// WithProvider directly adds a provider instance
func (rb *RegistryBuilder) WithProvider(provider Provider) *RegistryBuilder {
	rb.registry.RegisterProvider(provider)
	return rb
}

// WithModelPrefix registers a model prefix mapping
func (rb *RegistryBuilder) WithModelPrefix(prefix, providerName string) *RegistryBuilder {
	rb.registry.RegisterModelPrefix(prefix, providerName)
	return rb
}

// Build creates providers and returns the registry
func (rb *RegistryBuilder) Build(configs map[string]ProviderConfig) (*Registry, error) {
	for name, config := range configs {
		if builder, exists := rb.builders[name]; exists {
			provider, err := builder(config)
			if err != nil {
				return nil, fmt.Errorf("failed to build provider %s: %w", name, err)
			}
			if err := rb.registry.RegisterProvider(provider); err != nil {
				return nil, fmt.Errorf("failed to register provider %s: %w", name, err)
			}
		}
	}

	return rb.registry, nil
}
