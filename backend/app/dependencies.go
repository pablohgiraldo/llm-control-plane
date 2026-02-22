package app

import (
	"context"
	"fmt"
	"time"

	"github.com/upb/llm-control-plane/backend/auth"
	"github.com/upb/llm-control-plane/backend/config"
	"github.com/upb/llm-control-plane/backend/cognito"
	"github.com/upb/llm-control-plane/backend/internal/providers"
	"github.com/upb/llm-control-plane/backend/middleware"
	"github.com/upb/llm-control-plane/backend/repositories"
	"github.com/upb/llm-control-plane/backend/repositories/postgres"
	"github.com/upb/llm-control-plane/backend/services"
	"go.uber.org/zap"
)

// Dependencies holds all application dependencies following the GrantPulse pattern.
// This is the central wiring point for dependency injection.
type Dependencies struct {
	// Infrastructure
	Config *config.Config
	DB     *postgres.DB
	Logger *zap.Logger

	// Repository Factory
	RepoFactory *postgres.RepositoryFactory

	// Repositories
	Organizations     repositories.OrganizationRepository
	Applications      repositories.ApplicationRepository
	Users             repositories.UserRepository
	Policies          repositories.PolicyRepository
	AuditLogs         repositories.AuditRepository
	InferenceRequests repositories.InferenceRequestRepository
	TxManager         repositories.TransactionManager

	// Provider Registry
	ProviderRegistry *ProviderRegistry

	// Auth
	authHandler    *auth.Handler
	AuthMiddleware *middleware.AuthMiddleware
}

// AuthHandler returns the auth handler for route wiring (implements handlers.AuthDeps)
func (d *Dependencies) AuthHandler() *auth.Handler {
	return d.authHandler
}

// NewDependencies creates and wires up all application dependencies.
// This follows the GrantPulse pattern of centralized dependency injection.
func NewDependencies(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*Dependencies, error) {
	deps := &Dependencies{
		Config: cfg,
		Logger: logger,
	}

	// Initialize PostgreSQL
	if err := deps.initDatabase(ctx, cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize repositories
	if err := deps.initRepositories(); err != nil {
		return nil, fmt.Errorf("failed to initialize repositories: %w", err)
	}

	// Initialize provider registry
	if err := deps.initProviders(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}

	// Initialize auth (Cognito OAuth2)
	deps.initAuth(cfg)

	logger.Info("all dependencies initialized successfully")
	return deps, nil
}

// initDatabase initializes the PostgreSQL database connection and factory
func (d *Dependencies) initDatabase(ctx context.Context, cfg *config.Config) error {
	factory, err := postgres.NewRepositoryFactory(cfg, d.Logger)
	if err != nil {
		return fmt.Errorf("failed to create repository factory: %w", err)
	}

	d.RepoFactory = factory
	d.DB = factory.GetDB()

	// Test the connection
	if err := d.DB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Initialize audit schema when using separate audit DB
	if err := factory.InitAuditSchema(ctx); err != nil {
		return fmt.Errorf("failed to initialize audit schema: %w", err)
	}

	d.Logger.Info("database connection established",
		zap.String("connection", cfg.Database.LogString()))

	return nil
}

// initRepositories initializes all repository instances
func (d *Dependencies) initRepositories() error {
	repos := d.RepoFactory.NewRepositories()

	d.Organizations = repos.Organizations
	d.Applications = repos.Applications
	d.Users = repos.Users
	d.Policies = repos.Policies
	d.AuditLogs = repos.AuditLogs
	d.InferenceRequests = repos.InferenceRequests
	d.TxManager = d.RepoFactory.GetTransactionManager()

	d.Logger.Info("repositories initialized")
	return nil
}

// initProviders initializes the provider registry with configured providers
func (d *Dependencies) initProviders(cfg *config.Config) error {
	registry := NewProviderRegistry(d.Logger)

	// Register OpenAI provider if configured
	if cfg.Providers.OpenAI.APIKey != "" {
		openAIProvider := NewOpenAIAdapter(cfg.Providers.OpenAI, d.Logger)
		registry.Register(openAIProvider)
		d.Logger.Info("registered OpenAI provider")
	}

	// TODO: Register Anthropic provider if configured
	// if cfg.Providers.Anthropic.APIKey != "" {
	//     anthropicProvider := NewAnthropicAdapter(cfg.Providers.Anthropic, d.Logger)
	//     registry.Register(anthropicProvider)
	//     d.Logger.Info("registered Anthropic provider")
	// }

	// TODO: Register Bedrock provider if configured
	// if cfg.Providers.Bedrock.AccessKey != "" {
	//     bedrockProvider := NewBedrockAdapter(cfg.Providers.Bedrock, d.Logger)
	//     registry.Register(bedrockProvider)
	//     d.Logger.Info("registered Bedrock provider")
	// }

	if registry.Count() == 0 {
		d.Logger.Warn("no LLM providers configured")
	}

	d.ProviderRegistry = registry
	return nil
}

func (d *Dependencies) initAuth(cfg *config.Config) {
	if cfg.Cognito.Domain == "" || cfg.Cognito.ClientID == "" {
		d.Logger.Warn("cognito not configured, auth endpoints disabled")
		// Use reject-all validator so protected routes return 401
		d.AuthMiddleware = middleware.NewAuthMiddleware(&rejectAllValidator{}, d.Logger)
		return
	}
	cognitoValidator := cognito.NewCognitoValidator(cognito.Config{
		Region:      cfg.Cognito.Region,
		UserPoolID:  cfg.Cognito.UserPoolID,
		ClientID:    cfg.Cognito.ClientID,
		CacheTTL:    time.Hour,
		HTTPTimeout: 10 * time.Second,
	})
	// Adapter converts cognito.ParsedClaims to middleware.Claims for AuthMiddleware
	tokenValidator := &cognitoTokenValidatorAdapter{validator: cognitoValidator}
	d.AuthMiddleware = middleware.NewAuthMiddleware(tokenValidator, d.Logger)
	exchanger := services.NewCognitoTokenExchanger(cfg.Cognito)
	d.authHandler = auth.NewHandler(cfg, exchanger, cognitoValidator, d.Logger)
	d.Logger.Info("auth handler initialized")
}

// cognitoTokenValidatorAdapter adapts cognito.CognitoValidator to middleware.TokenValidator
type cognitoTokenValidatorAdapter struct {
	validator *cognito.CognitoValidator
}

func (a *cognitoTokenValidatorAdapter) ValidateToken(ctx context.Context, token string) (*middleware.Claims, error) {
	parsed, err := a.validator.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}
	// Convert cognito.ParsedClaims to middleware.Claims
	appIDStr := ""
	if parsed.AppID != nil {
		appIDStr = parsed.AppID.String()
	}
	groups := []string{}
	if parsed.Role != "" {
		groups = append(groups, parsed.Role)
	}
	return &middleware.Claims{
		Sub:           parsed.Sub.String(),
		Email:         parsed.Email,
		EmailVerified: parsed.EmailVerified,
		Groups:        groups,
		OrgID:         parsed.OrgID.String(),
		AppID:         appIDStr,
		UserID:        parsed.Sub.String(),
	}, nil
}

// rejectAllValidator rejects all tokens (used when Cognito is not configured)
type rejectAllValidator struct{}

func (*rejectAllValidator) ValidateToken(context.Context, string) (*middleware.Claims, error) {
	return nil, fmt.Errorf("authentication not configured")
}

// Close gracefully shuts down all dependencies
func (d *Dependencies) Close(ctx context.Context) error {
	d.Logger.Info("shutting down dependencies")

	var errs []error

	// Close database connection
	if d.RepoFactory != nil {
		if err := d.RepoFactory.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close database: %w", err))
		} else {
			d.Logger.Info("database connection closed")
		}
	}

	// Sync logger
	if d.Logger != nil {
		_ = d.Logger.Sync()
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	return nil
}

// ProviderRegistry manages LLM provider instances
type ProviderRegistry struct {
	providers map[string]providers.Provider
	logger    *zap.Logger
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry(logger *zap.Logger) *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]providers.Provider),
		logger:    logger,
	}
}

// Register registers a provider with the registry
func (r *ProviderRegistry) Register(provider providers.Provider) {
	r.providers[provider.Name()] = provider
	r.logger.Info("provider registered", zap.String("provider", provider.Name()))
}

// Get retrieves a provider by name
func (r *ProviderRegistry) Get(name string) (providers.Provider, bool) {
	provider, ok := r.providers[name]
	return provider, ok
}

// List returns all registered provider names
func (r *ProviderRegistry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered providers
func (r *ProviderRegistry) Count() int {
	return len(r.providers)
}

// OpenAIAdapter is a placeholder for OpenAI provider implementation
// TODO: Implement full OpenAI adapter in internal/providers/openai
type OpenAIAdapter struct {
	config config.OpenAIConfig
	logger *zap.Logger
}

// NewOpenAIAdapter creates a new OpenAI adapter
func NewOpenAIAdapter(cfg config.OpenAIConfig, logger *zap.Logger) *OpenAIAdapter {
	return &OpenAIAdapter{
		config: cfg,
		logger: logger,
	}
}

// Name returns the provider name
func (a *OpenAIAdapter) Name() string {
	return "openai"
}

// ChatCompletion performs a chat completion (placeholder)
func (a *OpenAIAdapter) ChatCompletion(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	// TODO: Implement actual OpenAI API call
	return nil, fmt.Errorf("not implemented")
}

// IsAvailable checks if the provider is available
func (a *OpenAIAdapter) IsAvailable(ctx context.Context) bool {
	// TODO: Implement health check
	return a.config.APIKey != ""
}

// CalculateCost calculates the cost of a request/response
func (a *OpenAIAdapter) CalculateCost(req *providers.ChatRequest, resp *providers.ChatResponse) (float64, error) {
	// TODO: Implement cost calculation based on model and token usage
	return 0.0, fmt.Errorf("not implemented")
}
