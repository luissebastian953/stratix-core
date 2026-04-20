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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"github.com/luissebastian953/stratix-core/config"
	aihandler "github.com/luissebastian953/stratix-core/internal/ai/handler"
	aisvc "github.com/luissebastian953/stratix-core/internal/ai/service"
	analyticshandler "github.com/luissebastian953/stratix-core/internal/analytics/handler"
	analyticsrepo "github.com/luissebastian953/stratix-core/internal/analytics/repository"
	analyticssvc "github.com/luissebastian953/stratix-core/internal/analytics/service"
	authhandler "github.com/luissebastian953/stratix-core/internal/auth/handler"
	authrepo "github.com/luissebastian953/stratix-core/internal/auth/repository"
	authsvc "github.com/luissebastian953/stratix-core/internal/auth/service"
	"github.com/luissebastian953/stratix-core/internal/events"
	"github.com/luissebastian953/stratix-core/internal/middleware"
	notifhandler "github.com/luissebastian953/stratix-core/internal/notifications/handler"
	notifrepo "github.com/luissebastian953/stratix-core/internal/notifications/repository"
	notifsvc "github.com/luissebastian953/stratix-core/internal/notifications/service"
	taskhandler "github.com/luissebastian953/stratix-core/internal/tasks/handler"
	taskrepo "github.com/luissebastian953/stratix-core/internal/tasks/repository"
	tasksvc "github.com/luissebastian953/stratix-core/internal/tasks/service"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
	"github.com/luissebastian953/stratix-core/pkg/storage"
)

type App struct {
	Server *echo.Echo
	db     *pgxpool.Pool
}

func NewApp(cfg config.Config) (*App, error) {
	db, err := newDBPool(cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	dispatcher := events.NewDispatcher()

	// TODO: replace with real R2 client once storage credentials are configured
	storageClient := storage.NewFakeClient()

	// ── Auth ──────────────────────────────────────────────────────────────────
	userRepo := authrepo.NewUserRepository(db)
	tokenRepo := authrepo.NewRefreshTokenRepository(db)
	authService := authsvc.NewAuthService(userRepo, tokenRepo, authsvc.AuthConfig{
		JWTSecret:        cfg.JWTSecret,
		AccessExpMinutes: cfg.JWTAccessExpMinutes,
		RefreshExpDays:   cfg.JWTRefreshExpDays,
	})
	authH := authhandler.NewAuthHandler(authService)

	// ── Tasks ─────────────────────────────────────────────────────────────────
	taskRepo := taskrepo.NewTaskRepository(db)
	taskService := tasksvc.NewTaskService(taskRepo, dispatcher, storageClient)
	taskH := taskhandler.NewTaskHandler(taskService)

	// ── Analytics ─────────────────────────────────────────────────────────────
	analyticsRepo := analyticsrepo.NewAnalyticsRepository(db)
	analyticsService := analyticssvc.NewAnalyticsService(analyticsRepo)
	analyticsH := analyticshandler.NewAnalyticsHandler(analyticsService)

	// ── AI ────────────────────────────────────────────────────────────────────
	parseService := aisvc.NewParseService(aisvc.ParseServiceConfig{
		APIKey:     cfg.AnthropicAPIKey,
		APIURL:     cfg.AnthropicAPIURL,
		Model:      cfg.AnthropicModel,
		APIVersion: cfg.AnthropicAPIVersion,
		MaxTokens:  cfg.AnthropicMaxTokens,
		Timeout:    time.Duration(cfg.AnthropicTimeoutSec) * time.Second,
	})
	aiH := aihandler.NewAIHandler(parseService)

	// ── Notifications ─────────────────────────────────────────────────────────
	deviceRepo := notifrepo.NewDeviceRepository(db)
	notifRepo := notifrepo.NewNotificationRepository(db)
	notifService := notifsvc.NewNotificationService(deviceRepo, notifRepo, notifsvc.NewStubPushSender())
	notifH := notifhandler.NewNotificationHandler(notifService)

	// ── Event subscriptions ───────────────────────────────────────────────────
	dispatcher.Register("task.created", analyticsService.OnTaskCreated)
	dispatcher.Register("task.completed", analyticsService.OnTaskCompleted)
	dispatcher.Register("task.archived", analyticsService.OnTaskArchived)
	dispatcher.Register("task.rescheduled", analyticsService.OnTaskRescheduled)
	dispatcher.Register("recurrence_instance.completed", analyticsService.OnRecurrenceInstanceCompleted)
	dispatcher.Register("task.overdue", notifService.OnTaskOverdue)

	// ── Echo ──────────────────────────────────────────────────────────────────
	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.Recover())
	e.HTTPErrorHandler = httpErrorHandler

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Stratix API")
	})

	v1 := e.Group("/v1")

	// Public
	authH.Register(v1.Group("/auth"))

	// Protected
	protected := v1.Group("", middleware.JWT(cfg.JWTSecret))
	taskH.Register(protected.Group("/tasks"))
	analyticsH.Register(protected.Group("/analytics"))
	notifH.Register(protected.Group("/notifications"))
	aiH.Register(protected.Group("/ai"), middleware.AIRateLimit(cfg.AIRateLimitRequests, cfg.AIRateLimitWindow))

	return &App{Server: e, db: db}, nil
}

func (a *App) Start(port string) error {
	if port == "" {
		port = "8080"
	}

	return a.Server.Start(":" + port)
}

func (a *App) Shutdown() {
	a.Server.Shutdown(context.Background())
	a.db.Close()
}

func waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
}

func httpErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		_ = c.JSON(appErr.Code, apperror.NewErrorResponse(appErr))
		return
	}

	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		msg := fmt.Sprintf("%v", echoErr.Message)
		_ = c.JSON(echoErr.Code, apperror.NewErrorResponse(apperror.BadRequest(msg)))
		return
	}

	_ = c.JSON(http.StatusInternalServerError, apperror.NewErrorResponse(apperror.Internal(err)))
}

func newDBPool(cfg config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DBUrl)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	poolCfg.MaxConns = cfg.DBMaxConns
	poolCfg.MinConns = cfg.DBMinConns
	poolCfg.MaxConnLifetime = cfg.DBMaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.DBMaxConnIdleTime

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
