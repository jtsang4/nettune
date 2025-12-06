package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jtsang4/nettune/internal/server/adapter"
	"github.com/jtsang4/nettune/internal/server/api/handlers"
	"github.com/jtsang4/nettune/internal/server/api/middleware"
	"github.com/jtsang4/nettune/internal/server/service"
	"github.com/jtsang4/nettune/internal/shared/config"
	"go.uber.org/zap"
)

// Server represents the HTTP API server
type Server struct {
	router          *gin.Engine
	httpServer      *http.Server
	config          *config.ServerConfig
	logger          *zap.Logger
	systemAdapter   *adapter.SystemAdapter
	profileService  *service.ProfileService
	snapshotService *service.SnapshotService
	historyService  *service.HistoryService
	applyService    *service.ApplyService
	probeService    *service.ProbeService
}

// NewServer creates a new HTTP API server
func NewServer(cfg *config.ServerConfig, logger *zap.Logger) (*Server, error) {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create system adapter
	systemAdapter := adapter.NewSystemAdapter(logger)

	// Create services
	profileService, err := service.NewProfileService(cfg.GetProfilesDir(), logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile service: %w", err)
	}

	snapshotService, err := service.NewSnapshotService(cfg.GetSnapshotsDir(), systemAdapter, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot service: %w", err)
	}

	historyService, err := service.NewHistoryService(cfg.GetHistoryDir(), logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create history service: %w", err)
	}

	applyService := service.NewApplyService(
		profileService,
		snapshotService,
		historyService,
		systemAdapter,
		logger,
	)

	probeService := service.NewProbeService(systemAdapter, logger)

	s := &Server{
		config:          cfg,
		logger:          logger,
		systemAdapter:   systemAdapter,
		profileService:  profileService,
		snapshotService: snapshotService,
		historyService:  historyService,
		applyService:    applyService,
		probeService:    probeService,
	}

	s.setupRouter()
	return s, nil
}

// setupRouter sets up the Gin router with all routes and middleware
func (s *Server) setupRouter() {
	router := gin.New()

	// Create rate limiter (100000 requests per minute with burst of 20000 for network testing)
	rateLimiter := middleware.NewRateLimiter(100000, 20000, time.Minute)

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(s.logger))
	router.Use(middleware.RateLimit(rateLimiter))
	router.Use(middleware.RequestSizeLimit(s.config.MaxBodyBytes))

	// Health check (no auth required)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// All other endpoints require authentication
	authorized := router.Group("/")
	authorized.Use(middleware.BearerAuth(s.config.APIKey))

	// Create handlers
	probeHandler := handlers.NewProbeHandler(s.probeService)
	profileHandler := handlers.NewProfileHandler(s.profileService)
	systemHandler := handlers.NewSystemHandler(s.snapshotService, s.applyService)

	// Probe endpoints
	probe := authorized.Group("/probe")
	{
		probe.GET("/echo", probeHandler.Echo)
		probe.GET("/download", probeHandler.Download)
		probe.POST("/upload", probeHandler.Upload)
		probe.GET("/info", probeHandler.Info)
	}

	// Profile endpoints
	profiles := authorized.Group("/profiles")
	{
		profiles.GET("", profileHandler.List)
		profiles.GET("/:id", profileHandler.Get)
	}

	// System endpoints
	sys := authorized.Group("/sys")
	{
		sys.POST("/snapshot", systemHandler.CreateSnapshot)
		sys.GET("/snapshot/:id", systemHandler.GetSnapshot)
		sys.GET("/snapshots", systemHandler.ListSnapshots)
		sys.POST("/apply", systemHandler.Apply)
		sys.POST("/rollback", systemHandler.Rollback)
		sys.GET("/status", systemHandler.Status)
	}

	s.router = router
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         s.config.Listen,
		Handler:      s.router,
		ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Second,
	}

	s.logger.Info("starting HTTP server",
		zap.String("listen", s.config.Listen))

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("stopping HTTP server")
	return s.httpServer.Shutdown(ctx)
}

// GetRouter returns the Gin router (for testing)
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}
