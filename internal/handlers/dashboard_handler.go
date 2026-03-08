package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/michaelbrian/kiosk/internal/middleware"
	"github.com/michaelbrian/kiosk/internal/services"
)

type DashboardHandler struct {
	saleSvc    *services.SaleService
	productSvc *services.ProductService
}

func NewDashboardHandler(saleSvc *services.SaleService, productSvc *services.ProductService) *DashboardHandler {
	return &DashboardHandler{saleSvc: saleSvc, productSvc: productSvc}
}

func (h *DashboardHandler) Index(c *gin.Context) {
	ctx := c.Request.Context()
	claims := middleware.GetClaims(c)

	stats, err := h.saleSvc.GetDashboardStats(ctx)
	if err != nil {
		renderError(c, err)
		return
	}

	// Last 30 days P&L for chart
	from := time.Now().AddDate(0, 0, -30)
	to := time.Now()
	plData, _ := h.saleSvc.GetPLSummary(ctx, from, to, "day")

	topProducts, _ := h.saleSvc.GetTopProducts(ctx, from, to, 5)
	lowStock, _ := h.productSvc.GetLowStock(ctx)

	c.HTML(http.StatusOK, "dashboard/index.html", gin.H{
		"title":       "Dashboard",
		"stats":       stats,
		"plData":      plData,
		"topProducts": topProducts,
		"lowStock":    lowStock,
		"claims":      claims,
	})
}

func (h *DashboardHandler) PLReport(c *gin.Context) {
	ctx := c.Request.Context()
	claims := middleware.GetClaims(c)

	fromStr := c.Query("from")
	toStr := c.Query("to")
	groupBy := c.DefaultQuery("group_by", "day")

	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()

	if fromStr != "" {
		from, _ = time.Parse("2006-01-02", fromStr)
	}
	if toStr != "" {
		t, _ := time.Parse("2006-01-02", toStr)
		to = t.Add(24*time.Hour - time.Second)
	}

	plData, err := h.saleSvc.GetPLSummary(ctx, from, to, groupBy)
	if err != nil {
		renderError(c, err)
		return
	}

	// Aggregate totals
	var totalRevenue, totalCOGS, totalProfit float64
	for _, p := range plData {
		totalRevenue += p.Revenue
		totalCOGS += p.COGS
		totalProfit += p.GrossProfit
	}

	topProducts, _ := h.saleSvc.GetTopProducts(ctx, from, to, 10)

	c.HTML(http.StatusOK, "dashboard/pl.html", gin.H{
		"title":         "P&L Report",
		"plData":        plData,
		"topProducts":   topProducts,
		"totalRevenue":  totalRevenue,
		"totalCOGS":     totalCOGS,
		"totalProfit":   totalProfit,
		"from":          from.Format("2006-01-02"),
		"to":            to.Format("2006-01-02"),
		"groupBy":       groupBy,
		"claims":        claims,
	})
}

// HTMX: refresh stats
func (h *DashboardHandler) StatsPartial(c *gin.Context) {
	stats, _ := h.saleSvc.GetDashboardStats(c.Request.Context())
	c.HTML(http.StatusOK, "partials/dashboard_stats.html", gin.H{"stats": stats})
}
