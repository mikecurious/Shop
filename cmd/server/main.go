package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/michaelbrian/kiosk/internal/config"
	"github.com/michaelbrian/kiosk/internal/handlers"
	"github.com/michaelbrian/kiosk/internal/middleware"
	"github.com/michaelbrian/kiosk/internal/models"
	"github.com/michaelbrian/kiosk/internal/repository"
	"github.com/michaelbrian/kiosk/internal/services"
	"github.com/michaelbrian/kiosk/pkg/mpesa"
	"github.com/michaelbrian/kiosk/pkg/notifications"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if cfg.Log.Format == "pretty" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	}
	level, _ := zerolog.ParseLevel(cfg.Log.Level)
	zerolog.SetGlobalLevel(level)

	db, err := repository.NewDB(&cfg.Mongo)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}

	// Repositories
	userRepo := repository.NewUserRepository(db)
	productRepo := repository.NewProductRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	stockRepo := repository.NewStockRepository(db)
	saleRepo := repository.NewSaleRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)

	// Services
	authSvc := services.NewAuthService(userRepo, cfg)
	productSvc := services.NewProductService(productRepo, categoryRepo, stockRepo)
	saleSvc := services.NewSaleService(saleRepo, productRepo)
	reportSvc := services.NewReportService(saleRepo, productRepo)

	emailSvc := notifications.NewEmailService(&cfg.Email)
	smsSvc := notifications.NewSMSService(&cfg.Celcom)
	notifSvc := services.NewNotificationService(db, emailSvc, smsSvc, productRepo, userRepo)

	mpesaClient := mpesa.NewClient(&cfg.MPesa)

	seedAdminUser(authSvc)

	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())

	allowedOrigins := []string{
		cfg.App.BaseURL,
		"https://api.shop.dominicatechnologies.com",
		"https://a6c27ba7-da19-4a1b-a30a-66b313a19446.lovableproject.com",
		"https://id-preview--a6c27ba7-da19-4a1b-a30a-66b313a19446.lovable.app",
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Handlers
	authH := handlers.NewAuthHandler(authSvc)
	productH := handlers.NewProductHandler(productSvc)
	saleH := handlers.NewSaleHandler(saleSvc, productSvc, reportSvc)
	dashH := handlers.NewDashboardHandler(saleSvc, productSvc)
	mpesaH := handlers.NewMPesaHandler(paymentRepo, mpesaClient)
	reportH := handlers.NewReportHandler(reportSvc, saleSvc)

	// React SPA static assets
	r.Static("/assets", "./dist/assets")
	r.StaticFile("/favicon.ico", "./dist/favicon.ico")
	r.StaticFile("/robots.txt", "./dist/robots.txt")

	// M-Pesa callback (no auth — external service)
	r.POST("/api/mpesa/callback", mpesaH.Callback)

	// Cookie-auth downloads (report CSVs/PDFs opened via window.open)
	downloads := r.Group("/")
	downloads.Use(middleware.AuthRequired(authSvc))
	{
		downloads.GET("/logout", authH.Logout)
		downloads.GET("/reports/sales/csv", reportH.ExportSalesCSV)
		downloads.GET("/reports/inventory/csv", reportH.ExportInventoryCSV)
		downloads.GET("/reports/pl/csv", reportH.ExportPLCSV)
		downloads.GET("/reports/pl/pdf", reportH.ExportPLPDF)
		downloads.GET("/sales/:id/receipt/pdf", saleH.DownloadReceipt)
	}

	// Public API routes (no auth)
	r.POST("/api/v1/auth/login", middleware.RateLimit(cfg.RateLimit.Auth), authH.APILogin)

	// API routes — Bearer token auth
	api := r.Group("/api/v1")
	api.Use(middleware.APIAuthRequired(authSvc))
	api.Use(middleware.RateLimit(cfg.RateLimit.API))
	{
		api.GET("/stats", dashH.APIStats)

		api.GET("/products", productH.APIList)
		api.POST("/products", productH.APICreate)
		api.GET("/products/search", productH.APISearch)
		api.GET("/products/:id", productH.APIGet)
		api.PUT("/products/:id", productH.APIUpdate)
		api.DELETE("/products/:id", middleware.AdminRequired(), productH.APIDelete)

		api.GET("/sales", saleH.APIListSales)
		api.POST("/sales", saleH.CreateSale)
		api.GET("/sales/:id", saleH.GetSale)

		api.POST("/mpesa/stk-push", mpesaH.STKPush)
		api.GET("/mpesa/status/:checkout_id", mpesaH.CheckStatus)
		api.POST("/mpesa/link-sale", mpesaH.LinkToSale)

		// Admin
		api.GET("/users", middleware.AdminRequired(), func(c *gin.Context) {
			users, err := authSvc.ListUsers(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, users)
		})
		api.POST("/users", middleware.AdminRequired(), func(c *gin.Context) {
			var req models.RegisterRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			user, err := authSvc.Register(c.Request.Context(), req)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, user)
		})
	}

	// SPA fallback — serve index.html for all non-API routes
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.File("./dist/index.html")
	})

	go runDailyJobs(notifSvc, saleSvc)

	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.App.Port).Msg("server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("forced shutdown")
	}
}

func seedAdminUser(authSvc *services.AuthService) {
	_, err := authSvc.Register(context.Background(), models.RegisterRequest{
		Name:     "Administrator",
		Email:    "admin@kiosk.local",
		Password: "Admin@1234",
		Role:     models.RoleAdmin,
	})
	if err == nil {
		log.Info().Msg("default admin created: admin@kiosk.local / Admin@1234")
	}
}

func runDailyJobs(notifSvc *services.NotificationService, saleSvc *services.SaleService) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		ctx := context.Background()
		_ = notifSvc.SendLowStockAlerts(ctx)
		if stats, err := saleSvc.GetDashboardStats(ctx); err == nil {
			_ = notifSvc.SendDailySummary(ctx, stats)
		}
	}
}
