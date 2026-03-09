package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/michaelbrian/kiosk/internal/middleware"
	"github.com/michaelbrian/kiosk/internal/models"
	"github.com/michaelbrian/kiosk/internal/repository"
	"github.com/michaelbrian/kiosk/internal/services"
)

type SaleHandler struct {
	saleSvc    *services.SaleService
	productSvc *services.ProductService
	reportSvc  *services.ReportService
}

func NewSaleHandler(saleSvc *services.SaleService, productSvc *services.ProductService, reportSvc *services.ReportService) *SaleHandler {
	return &SaleHandler{saleSvc: saleSvc, productSvc: productSvc, reportSvc: reportSvc}
}

func (h *SaleHandler) Index(c *gin.Context) {
	page, limit := paginationParams(c)
	fromStr := c.Query("from")
	toStr := c.Query("to")

	filter := repository.SaleFilter{Page: page, Limit: limit}
	if fromStr != "" {
		filter.From, _ = time.Parse("2006-01-02", fromStr)
	}
	if toStr != "" {
		t, _ := time.Parse("2006-01-02", toStr)
		filter.To = t.Add(24*time.Hour - time.Second)
	}
	if filter.From.IsZero() {
		filter.From = time.Now().AddDate(0, 0, -30)
	}
	if filter.To.IsZero() {
		filter.To = time.Now()
	}

	sales, total, err := h.saleSvc.ListSales(c.Request.Context(), filter)
	if err != nil {
		renderError(c, err)
		return
	}

	c.HTML(http.StatusOK, "sales/index.html", gin.H{
		"title":      "Sales",
		"sales":      sales,
		"pagination": newPagination(page, limit, total),
		"filter":     filter,
		"claims":     middleware.GetClaims(c),
	})
}

func (h *SaleHandler) ShowPOS(c *gin.Context) {
	categories, _ := h.productSvc.GetCategories(c.Request.Context())
	c.HTML(http.StatusOK, "sales/pos.html", gin.H{
		"title":      "Point of Sale",
		"categories": categories,
		"claims":     middleware.GetClaims(c),
	})
}

func (h *SaleHandler) CreateSale(c *gin.Context) {
	claims := middleware.GetClaims(c)

	var req models.CreateSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sale, err := h.saleSvc.CreateSale(c.Request.Context(), req, claims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          sale.ID,
		"items":       sale.Items,
		"subtotal":    sale.TotalAmount,
		"total":       sale.NetAmount,
		"payment_method": string(sale.PaymentMethod),
		"customer_name":  sale.CustomerName,
		"created_at":     sale.CreatedAt,
		"receipt_url":    fmt.Sprintf("/sales/%s/receipt/pdf", sale.ID),
	})
}

func (h *SaleHandler) APIListSales(c *gin.Context) {
	page, limit := paginationParams(c)
	fromStr := c.Query("from")
	toStr := c.Query("to")

	filter := repository.SaleFilter{Page: page, Limit: limit}
	if fromStr != "" {
		filter.From, _ = time.Parse("2006-01-02", fromStr)
	}
	if toStr != "" {
		t, _ := time.Parse("2006-01-02", toStr)
		filter.To = t.Add(24*time.Hour - time.Second)
	}
	if filter.From.IsZero() {
		filter.From = time.Now().AddDate(0, 0, -30)
	}
	if filter.To.IsZero() {
		filter.To = time.Now()
	}

	sales, total, err := h.saleSvc.ListSales(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  sales,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *SaleHandler) ShowReceipt(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		renderError(c, fmt.Errorf("invalid sale ID"))
		return
	}

	sale, err := h.saleSvc.GetSale(c.Request.Context(), id)
	if err != nil {
		renderError(c, err)
		return
	}

	c.HTML(http.StatusOK, "sales/receipt.html", gin.H{
		"title":  "Receipt",
		"sale":   sale,
		"claims": middleware.GetClaims(c),
	})
}

func (h *SaleHandler) DownloadReceipt(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	sale, err := h.saleSvc.GetSale(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sale not found"})
		return
	}

	pdf, err := h.reportSvc.GenerateReceiptPDF(sale)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/pdf")
	suffix := sale.ID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=receipt-%s.pdf", suffix))
	c.Data(http.StatusOK, "application/pdf", pdf)
}

func (h *SaleHandler) GetSale(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	sale, err := h.saleSvc.GetSale(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sale not found"})
		return
	}
	c.JSON(http.StatusOK, sale)
}
