package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
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

	// Seed default admin
	seedAdminUser(authSvc)

	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())

	allowedOrigins := []string{cfg.App.BaseURL, "https://api.shop.dominicatechnologies.com"}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.SetFuncMap(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a float64, b int) float64 { return a * float64(b) },
		"fmtMoney": func(f float64) string { return fmt.Sprintf("KES %.2f", f) },
		"fmtDate":  func(t time.Time) string { return t.Format("02 Jan 2006") },
		"fmtDateTime": func(t time.Time) string { return t.Format("02 Jan 2006 15:04") },
		"isAdmin":  func(role string) bool { return role == "admin" },
		"percent": func(a, b float64) string {
			if b == 0 {
				return "0%"
			}
			return fmt.Sprintf("%.1f%%", (a/b)*100)
		},
		"seq": func(n int) []int {
			result := make([]int, n)
			for i := range result {
				result[i] = i + 1
			}
			return result
		},
		"safeHTML": func(s string) template.HTML { return template.HTML(s) }, //nolint:gosec
	})
	r.LoadHTMLGlob("templates/**/*.html")
	r.Static("/static", "./static")

	// Handlers
	authH := handlers.NewAuthHandler(authSvc)
	productH := handlers.NewProductHandler(productSvc)
	saleH := handlers.NewSaleHandler(saleSvc, productSvc, reportSvc)
	dashH := handlers.NewDashboardHandler(saleSvc, productSvc)
	mpesaH := handlers.NewMPesaHandler(paymentRepo, mpesaClient)
	reportH := handlers.NewReportHandler(reportSvc, saleSvc)

	// Public routes
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/login") })
	r.GET("/login", authH.ShowLogin)
	r.POST("/login", middleware.RateLimit(cfg.RateLimit.Auth), authH.Login)
	r.GET("/logout", authH.Logout)
	r.POST("/api/mpesa/callback", mpesaH.Callback)

	// Protected web routes
	web := r.Group("/")
	web.Use(middleware.AuthRequired(authSvc))
	{
		web.GET("/dashboard", dashH.Index)
		web.GET("/dashboard/pl", dashH.PLReport)

		web.GET("/inventory", productH.Index)
		web.GET("/inventory/create", productH.ShowCreate)
		web.POST("/inventory/create", productH.Create)
		web.GET("/inventory/import", productH.ShowImport)
		web.POST("/inventory/import", productH.ImportCSV)
		web.GET("/inventory/:id", productH.Show)
		web.GET("/inventory/:id/edit", productH.ShowEdit)
		web.POST("/inventory/:id/edit", productH.Update)
		web.POST("/inventory/:id/delete", middleware.AdminRequired(), productH.Delete)
		web.POST("/inventory/stock/adjust", productH.AdjustStock)
		web.GET("/inventory/:id/barcode", productH.GetBarcode)

		web.GET("/sales", saleH.Index)
		web.GET("/sales/pos", saleH.ShowPOS)
		web.GET("/sales/:id/receipt", saleH.ShowReceipt)
		web.GET("/sales/:id/receipt/pdf", saleH.DownloadReceipt)

		web.GET("/payments", mpesaH.Index)

		web.GET("/reports", reportH.Index)
		web.GET("/reports/sales/csv", reportH.ExportSalesCSV)
		web.GET("/reports/inventory/csv", reportH.ExportInventoryCSV)
		web.GET("/reports/pl/csv", reportH.ExportPLCSV)
		web.GET("/reports/pl/pdf", reportH.ExportPLPDF)

		web.GET("/profile", authH.ShowProfile)
		web.POST("/profile/password", authH.ChangePassword)

		// HTMX partials
		web.GET("/partials/stats", dashH.StatsPartial)
		web.GET("/partials/products/search", productH.SearchPartial)
		web.GET("/partials/low-stock", productH.LowStockPartial)

		// Admin only
		admin := web.Group("/admin")
		admin.Use(middleware.AdminRequired())
		{
			admin.GET("/users", func(c *gin.Context) {
				users, err := authSvc.ListUsers(c.Request.Context())
				if err != nil {
					c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": err.Error()})
					return
				}
				c.HTML(http.StatusOK, "auth/users.html", gin.H{
					"title":  "User Management",
					"users":  users,
					"claims": middleware.GetClaims(c),
				})
			})
			admin.POST("/users", func(c *gin.Context) {
				var req models.RegisterRequest
				if err := c.ShouldBind(&req); err != nil {
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
	}

	// API routes
	api := r.Group("/api/v1")
	api.Use(middleware.APIAuthRequired(authSvc))
	api.Use(middleware.RateLimit(cfg.RateLimit.API))
	{
		api.POST("/auth/login", authH.APILogin)
		api.GET("/products", productH.APIList)
		api.POST("/products", productH.APICreate)
		api.GET("/products/search", productH.APISearch)
		api.GET("/products/:id", productH.APIGet)
		api.PUT("/products/:id", productH.APIUpdate)
		api.DELETE("/products/:id", middleware.AdminRequired(), productH.APIDelete)
		api.POST("/sales", saleH.CreateSale)
		api.GET("/sales/:id", saleH.GetSale)
		api.POST("/mpesa/stk-push", mpesaH.STKPush)
		api.GET("/mpesa/status/:checkout_id", mpesaH.CheckStatus)
		api.POST("/mpesa/link-sale", mpesaH.LinkToSale)
	}

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
