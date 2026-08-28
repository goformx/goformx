// Command api runs the schema-first GoFormX HTTP service.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/goformx/goforms/internal/application/handlers/web"
	"github.com/goformx/goforms/internal/application/middleware/serviceauth"
	"github.com/goformx/goforms/internal/infrastructure/config"
	"github.com/goformx/goforms/internal/infrastructure/database"
	"github.com/goformx/goforms/internal/infrastructure/logging"
	formstore "github.com/goformx/goforms/internal/infrastructure/repository/form"
	tokenstore "github.com/goformx/goforms/internal/infrastructure/repository/token"
	"github.com/goformx/goforms/internal/infrastructure/sanitization"
	"github.com/goformx/goforms/internal/infrastructure/version"
)

const shutdownTimeout = 10 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "goformx-api:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.NewViperConfig().LoadSchemaFirstAPI()
	if err != nil {
		return err
	}
	logger, err := newLogger(cfg)
	if err != nil {
		return err
	}
	db, err := database.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			logger.Error("close PostgreSQL", "error", closeErr)
		}
	}()

	forms := formstore.NewStore(db, logger)
	tokens := tokenstore.NewStore(db)
	router := newRouter(cfg, forms, tokens, logger)
	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port),
		Handler:           router,
		ReadTimeout:       cfg.App.ReadTimeout,
		ReadHeaderTimeout: cfg.App.ReadTimeout,
		WriteTimeout:      cfg.App.WriteTimeout,
		IdleTimeout:       cfg.App.IdleTimeout,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()
	build := version.GetInfo()
	logger.Info("schema-first API started", "address", server.Addr, "version", build.Version,
		"git_commit", build.GitCommit)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		logger.Info("schema-first API stopped")
		return nil
	}
}

func newRouter(
	cfg *config.Config,
	forms web.V1Repository,
	tokens serviceauth.Repository,
	logger logging.Logger,
) *echo.Echo {
	router := echo.New()
	router.HideBanner = true
	router.HidePort = true

	// This is the complete production middleware chain. Browser sessions, CSRF,
	// HMAC assertions, plan headers, and the legacy middleware container are absent.
	router.Use(echomiddleware.RequestID())
	router.Use(echomiddleware.Recover())
	router.Use(echomiddleware.Secure())
	if cfg.Security.RateLimit.Enabled {
		store := echomiddleware.NewRateLimiterMemoryStoreWithConfig(
			echomiddleware.RateLimiterMemoryStoreConfig{
				Rate:  rate.Limit(cfg.Security.RateLimit.RPS),
				Burst: cfg.Security.RateLimit.Burst,
			},
		)
		router.Use(echomiddleware.RateLimiter(store))
	}
	health := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"data": map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)},
		})
	}
	router.GET("/health", health)
	router.HEAD("/health", health)
	web.NewV1APIHandler(forms, tokens, logger).RegisterRoutes(router)
	return router
}

func newLogger(cfg *config.Config) (logging.Logger, error) {
	level := cfg.App.LogLevel
	if level == "" {
		level = "info"
		if cfg.App.IsDevelopment() || cfg.App.Debug {
			level = "debug"
		}
	}
	sanitizer := sanitization.NewService()
	factory, err := logging.NewFactory(&logging.FactoryConfig{
		AppName: cfg.App.Name, Version: version.Version, Environment: cfg.App.Environment,
		LogLevel: level, OutputPaths: []string{"stdout"}, ErrorOutputPaths: []string{"stderr"},
	}, sanitizer)
	if err != nil {
		return nil, fmt.Errorf("create logger factory: %w", err)
	}
	logger, err := factory.CreateLogger()
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}
	return logger, nil
}
