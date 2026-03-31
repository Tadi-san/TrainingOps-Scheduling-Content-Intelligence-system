package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"trainingops/internal/api/handlers"
	"trainingops/internal/api/middleware"
	"trainingops/internal/config"
	"trainingops/internal/repository"
	"trainingops/internal/repository/postgres"
	"trainingops/internal/security"
	"trainingops/internal/service"
)

type Server struct {
	echo          *echo.Echo
	startWorkers  func()
	stopWorkers   func()
	workerStarted bool
}

func NewServer(cfg config.Config, vault *security.Vault) *Server {
	e := echo.New()
	e.HideBanner = true
	e.EnableCORS(cfg.CORSOrigins)
	logger := slog.Default()
	e.Use(middleware.StructuredLogging(logger))

	var (
		userStore    repository.UserStore
		userCreator  repository.UserCreator
		sessions     repository.SessionStore
		bookingStore repository.BookingStore
		contentStore repository.ContentStore
		taskStore    repository.TaskStore
		storageMode  = "postgres"
	)

	if cfg.DatabaseURL == "" {
		panic("DATABASE_URL is required; in-memory fallback has been removed")
	}
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		panic(fmt.Errorf("failed to create postgres pool: %w", err))
	}
	if err := pool.Ping(context.Background()); err != nil {
		panic(fmt.Errorf("failed to connect to postgres: %w", err))
	}
	pgStore := postgres.NewStore(pool, vault)
	userStore = pgStore
	userCreator = pgStore
	sessions = pgStore
	bookingStore = pgStore
	contentStore = pgStore
	taskStore = pgStore
	e.Use(middleware.AuthGuard(cfg.JWTSigningKey, sessions, map[string]bool{
		"/healthz":          true,
		"/readyz":           true,
		"/v1/auth/register": true,
		"/v1/auth/login":    true,
		"/v1/setup/status":  true,
		"/v1/setup/tenant":  true,
	}))
	e.Use(middleware.TenantContext())
	e.Use(middleware.AuditContext())

	authService := service.NewAuthService(userStore, userCreator, sessions)
	setupService := service.NewSetupService(pgStore, authService)
	seedDefaultUsers(context.Background(), setupService)
	authHandler := &handlers.AuthHandler{
		Auth:     authService,
		Setup:    setupService,
		TokenKey: cfg.JWTSigningKey,
		TokenTTL: 24 * time.Hour,
	}
	setupHandler := &handlers.SetupHandler{Setup: setupService}
	contentService := service.NewContentService(contentStore, []byte("trainingops-share-secret"), cfg.StoragePath)
	contentHandler := &handlers.ContentHandler{Content: contentService}
	var auditStore service.AuditStore
	if pgStore != nil {
		auditStore = pgStore
	}
	auditRecorder := service.NewAuditRecorder(auditStore, logger)
	bookingHandler := &handlers.BookingHandler{Bookings: service.NewBookingService(bookingStore), Audit: auditRecorder}
	holdWorker := service.NewHoldExpiryWorker(pgStore, auditRecorder, logger, time.Minute)
	dashboardService := service.NewDashboardService(bookingStore)
	dashboardHandler := &handlers.DashboardHandler{Dashboard: dashboardService}
	analyticsHandler := &handlers.AnalyticsHandler{Engine: service.NewAnalyticsEngine(), Bookings: bookingStore}
	ingestionHandler := &handlers.IngestionHandler{Bot: service.NewIngestionBot(service.NewProxyPoolManager([]string{"proxy://127.0.0.1:8081", "proxy://127.0.0.1:8082"}), service.NewUserAgentRotator([]string{"TrainingOpsBot/1.0", "TrainingOpsOps/2.0", "TrainingOpsPreview/3.0"}), 42, pgStore)}
	reportingHandler := &handlers.ReportingHandler{Reports: service.NewReportingService(cfg.ReportsPath), Bookings: bookingStore}
	scheduleHandler := &handlers.ScheduleHandler{}
	if pg, ok := userStore.(*postgres.Store); ok {
		scheduleHandler.Store = pg
	}
	taskService := service.NewTaskService(taskStore)
	taskHandler := &handlers.TaskHandler{Tasks: taskService, Store: taskStore}
	adminHandler := &handlers.AdminHandler{Store: pgStore}
	learnerHandler := &handlers.LearnerHandler{Store: pgStore, Audit: auditRecorder}

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	readyStatus := "ready"

	e.GET("/readyz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":       readyStatus,
			"env":          cfg.AppEnv,
			"storage_mode": storageMode,
		})
	})

	e.POST("/v1/auth/register", authHandler.Register)
	e.POST("/v1/auth/login", authHandler.Login)
	e.POST("/v1/auth/logout", authHandler.Logout)
	e.GET("/v1/auth/session", authHandler.Session)
	e.GET("/v1/setup/status", setupHandler.Status)
	e.POST("/v1/setup/tenant", setupHandler.BootstrapTenant)
	e.GET("/v1/workspaces/:role/dashboard", dashboardHandler.Get)
	e.POST("/v1/bookings/hold", middleware.RequireRole("coordinator")(bookingHandler.Hold))
	e.GET("/v1/bookings/rooms", middleware.RequireRole("admin", "coordinator", "learner")(bookingHandler.ListRooms))
	e.GET("/v1/bookings/instructors", middleware.RequireRole("admin", "coordinator", "learner")(bookingHandler.ListInstructors))
	e.POST("/v1/bookings/confirm", middleware.RequireRole("coordinator")(bookingHandler.Confirm))
	e.POST("/v1/bookings/cancel", middleware.RequireRole("coordinator")(bookingHandler.Cancel))
	e.POST("/v1/bookings/reschedule", middleware.RequireRole("coordinator")(bookingHandler.Reschedule))
	e.PUT("/v1/bookings/reschedule", middleware.RequireRole("coordinator")(bookingHandler.Reschedule))
	e.POST("/v1/bookings/:id/check-in", middleware.RequireRole("instructor", "coordinator")(bookingHandler.CheckIn))
	e.POST("/v1/schedule/periods", middleware.RequireRole("admin", "coordinator")(scheduleHandler.CreateClassPeriod))
	e.GET("/v1/schedule/periods", middleware.RequireRole("admin", "coordinator")(scheduleHandler.ListClassPeriods))
	e.POST("/v1/schedule/class-periods", middleware.RequireRole("admin", "coordinator")(scheduleHandler.CreateClassPeriod))
	e.GET("/v1/schedule/class-periods", middleware.RequireRole("admin", "coordinator")(scheduleHandler.ListClassPeriods))
	e.PUT("/v1/schedule/class-periods/:id", middleware.RequireRole("admin", "coordinator")(scheduleHandler.UpdateClassPeriod))
	e.DELETE("/v1/schedule/class-periods/:id", middleware.RequireRole("admin", "coordinator")(scheduleHandler.DeleteClassPeriod))
	e.POST("/v1/schedule/blackout-dates", middleware.RequireRole("admin", "coordinator")(scheduleHandler.CreateBlackoutDate))
	e.GET("/v1/schedule/blackout-dates", middleware.RequireRole("admin", "coordinator")(scheduleHandler.ListBlackoutDates))
	e.DELETE("/v1/schedule/blackout-dates/:id", middleware.RequireRole("admin", "coordinator")(scheduleHandler.DeleteBlackoutDate))
	e.POST("/v1/schedule/conflicts", middleware.RequireRole("admin", "coordinator")(scheduleHandler.CheckConflicts))
	e.POST("/v1/uploads/start", middleware.RequireRole("coordinator", "instructor")(contentHandler.StartUpload))
	e.POST("/v1/uploads/chunk", middleware.RequireRole("coordinator", "instructor")(contentHandler.AppendChunk))
	e.POST("/v1/uploads/finalize", middleware.RequireRole("coordinator", "instructor")(contentHandler.FinalizeUpload))
	e.GET("/v1/content/search", middleware.RequireRole("coordinator", "instructor", "learner")(contentHandler.Search))
	e.GET("/v1/content/:id/versions", middleware.RequireRole("coordinator", "instructor", "learner")(contentHandler.Versions))
	e.POST("/v1/content/:id/share", middleware.RequireRole("coordinator", "instructor")(contentHandler.GenerateShare))
	e.GET("/v1/content/share/:token", contentHandler.DownloadShared)
	e.GET("/v1/learner/catalog", middleware.RequireRole("learner")(learnerHandler.Catalog))
	e.POST("/v1/learner/reserve", middleware.RequireRole("learner")(learnerHandler.Reserve))
	e.GET("/v1/learner/my-reservations", middleware.RequireRole("learner")(learnerHandler.MyReservations))
	e.GET("/v1/learner/download/:file_id", middleware.RequireRole("learner")(learnerHandler.DownloadApproved))
	e.GET("/v1/admin/tenants", middleware.RequireRole("admin")(adminHandler.ListTenants))
	e.PUT("/v1/admin/tenants/:id/policies", middleware.RequireRole("admin")(adminHandler.UpdateTenantPolicies))
	e.GET("/v1/admin/users", middleware.RequireRole("admin")(adminHandler.ListUsers))
	e.PUT("/v1/admin/users/:id/role", middleware.RequireRole("admin")(adminHandler.UpdateUserRole))
	e.GET("/v1/admin/permissions", middleware.RequireRole("admin")(adminHandler.ListPermissions))
	e.GET("/v1/admin/rooms", middleware.RequireRole("admin")(adminHandler.ListRooms))
	e.POST("/v1/admin/rooms", middleware.RequireRole("admin")(adminHandler.CreateRoom))
	e.PUT("/v1/admin/rooms/:id", middleware.RequireRole("admin")(adminHandler.UpdateRoom))
	e.POST("/v1/ingestion/run", middleware.RequireRole("admin", "coordinator")(ingestionHandler.Run))
	e.GET("/v1/analytics/cohorts", middleware.RequireRole("admin", "coordinator")(analyticsHandler.Cohorts))
	e.GET("/v1/analytics/anomalies", middleware.RequireRole("admin", "coordinator")(analyticsHandler.Anomalies))
	e.GET("/v1/reports/bookings.csv", middleware.RequireRole("admin")(reportingHandler.BookingsCSV))
	e.GET("/v1/reports/compliance.pdf", middleware.RequireRole("admin")(reportingHandler.CompliancePDF))
	e.GET("/v1/reports/download/:filename", middleware.RequireRole("admin")(reportingHandler.Download))
	e.POST("/v1/milestones/:id/tasks", middleware.RequireRole("coordinator")(taskHandler.Create))
	e.GET("/v1/milestones/:id/tasks", middleware.RequireRole("coordinator", "instructor")(taskHandler.ListByMilestone))
	e.PUT("/v1/tasks/:id", middleware.RequireRole("coordinator")(taskHandler.Update))
	e.DELETE("/v1/tasks/:id", middleware.RequireRole("coordinator")(taskHandler.Delete))
	e.POST("/v1/tasks/:id/dependencies", middleware.RequireRole("coordinator")(taskHandler.AddDependencies))

	server := &Server{echo: e}
	server.startWorkers = func() {
		if server.workerStarted {
			return
		}
		server.workerStarted = true
		workerCtx, cancelWorkers := context.WithCancel(context.Background())
		server.stopWorkers = cancelWorkers
		go holdWorker.Start(workerCtx)
	}
	return server
}

func (s *Server) Start(addr string) error {
	if s.startWorkers != nil {
		s.startWorkers()
	}
	defer func() {
		if s.stopWorkers != nil {
			s.stopWorkers()
		}
	}()
	return s.echo.Start(addr)
}

func seedDefaultUsers(ctx context.Context, setup *service.SetupService) {
	email := strings.TrimSpace(os.Getenv("ADMIN_SEED_EMAIL"))
	password := strings.TrimSpace(os.Getenv("ADMIN_SEED_PASSWORD"))
	if email == "" || password == "" {
		return
	}
	_, _ = setup.BootstrapTenant(ctx, "tenant-1", "tenant-1", email, "Default Admin", password, time.Now().UTC())
}

func postgresStoreOrNil(users repository.UserStore) *postgres.Store {
	store, ok := users.(*postgres.Store)
	if !ok {
		return nil
	}
	return store
}
