package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/upb/llm-control-plane/backend/app"
	"github.com/upb/llm-control-plane/backend/config"
	"github.com/upb/llm-control-plane/backend/routes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	ctx := context.Background()

	// Initialize logger first
	logger, err := initLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting api-gateway")

	// Load configuration
	cfg, err := config.New(ctx)
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	logger.Info("configuration loaded",
		zap.String("environment", cfg.Environment),
		zap.String("server_address", cfg.Server.Address()))

	// Initialize dependencies
	deps, err := app.NewDependencies(ctx, cfg, logger)
	if err != nil {
		logger.Fatal("failed to initialize dependencies", zap.Error(err))
	}

	logger.Info("dependencies initialized successfully")

	// Setup routes and middleware
	handler := routes.SetupRoutes(deps)

	// Create HTTP server
	srv := &http.Server{
		Addr:              cfg.Server.Address(),
		Handler:           handler,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Start server in a goroutine with HTTPS support
	go func() {
		if cfg.Server.TLS.Enabled {
			// HTTPS mode (required for Cognito OAuth2)
			logger.Info("api-gateway listening with HTTPS",
				zap.String("address", srv.Addr),
				zap.String("cert_file", cfg.Server.TLS.CertFile),
				zap.String("key_file", cfg.Server.TLS.KeyFile))

			if err := srv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile); err != nil && err != http.ErrServerClosed {
				logger.Fatal("https server error", zap.Error(err))
			}
		} else {
			// HTTP mode (development/testing only)
			logger.Info("api-gateway listening with HTTP",
				zap.String("address", srv.Addr))
			logger.Warn("running in HTTP mode - OAuth2 callbacks require HTTPS")

			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Fatal("http server error", zap.Error(err))
			}
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	} else {
		logger.Info("server shutdown completed")
	}

	// Close dependencies
	if err := deps.Close(context.Background()); err != nil {
		logger.Error("error closing dependencies", zap.Error(err))
	} else {
		logger.Info("dependencies closed successfully")
	}

	logger.Info("api-gateway stopped")
}

// initLogger initializes a production-ready zap logger
func initLogger() (*zap.Logger, error) {
	// Determine log level from environment
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	// Determine log format from environment
	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "" {
		logFormat = "json"
	}

	// Parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Create logger config
	var cfg zap.Config
	if logFormat == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}

	cfg.Level = zap.NewAtomicLevelAt(level)
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Build logger
	logger, err := cfg.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return logger, nil
}
