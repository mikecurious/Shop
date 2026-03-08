package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/michaelbrian/kiosk/internal/middleware"
	"github.com/michaelbrian/kiosk/internal/models"
	"github.com/michaelbrian/kiosk/internal/repository"
	"github.com/michaelbrian/kiosk/internal/services"
	"github.com/michaelbrian/kiosk/pkg/barcode"
)

type ProductHandler struct {
	productSvc *services.ProductService
}

func NewProductHandler(productSvc *services.ProductService) *ProductHandler {
	return &ProductHandler{productSvc: productSvc}
}

// --- Web handlers ---

func (h *ProductHandler) Index(c *gin.Context) {
	page, limit := paginationParams(c)
	search := c.Query("search")
	lowStock := c.Query("low_stock") == "1"
	catStr := c.Query("category_id")

	filter := repository.ProductFilter{
		Search:   search,
		LowStock: lowStock,
		Page:     page,
		Limit:    limit,
	}
	if catStr != "" {
		filter.CategoryID = &catStr
	}
	active := true
	filter.IsActive = &active

	products, total, err := h.productSvc.List(c.Request.Context(), filter)
	if err != nil {
		renderError(c, err)
		return
	}

	categories, _ := h.productSvc.GetCategories(c.Request.Context())
	claims := middleware.GetClaims(c)

	c.HTML(http.StatusOK, "inventory/index.html", gin.H{
		"title":      "Inventory",
		"products":   products,
		"pagination": newPagination(page, limit, total),
		"categories": categories,
		"filter": gin.H{
			"search":      search,
			"low_stock":   lowStock,
			"category_id": catStr,
		},
		"claims":        claims,
		"lowStockCount": countLowStock(products),
	})
}

func (h *ProductHandler) Show(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	product, err := h.productSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		renderError(c, err)
		return
	}

	// Generate barcode images
	barcodeImg, _ := barcode.GenerateCode128PNG(product.Barcode, 300, 80)
	qrImg, _ := barcode.GenerateQRPNG(product.Barcode, 150)

	// Stock history
	stockHistory, _, _ := h.productSvc.GetStockHistory(c.Request.Context(), id, 1, 10)

	c.HTML(http.StatusOK, "inventory/show.html", gin.H{
		"title":        "Product Details",
		"product":      product,
		"barcodeImg":   barcodeImg,
		"qrImg":        qrImg,
		"stockHistory": stockHistory,
		"claims":       middleware.GetClaims(c),
	})
}

func (h *ProductHandler) ShowCreate(c *gin.Context) {
	categories, _ := h.productSvc.GetCategories(c.Request.Context())
	c.HTML(http.StatusOK, "inventory/create.html", gin.H{
		"title":      "Add Product",
		"categories": categories,
		"claims":     middleware.GetClaims(c),
	})
}

func (h *ProductHandler) Create(c *gin.Context) {
	claims := middleware.GetClaims(c)
	categories, _ := h.productSvc.GetCategories(c.Request.Context())

	var req models.CreateProductRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "inventory/create.html", gin.H{
			"title":      "Add Product",
			"error":      err.Error(),
			"categories": categories,
			"claims":     claims,
		})
		return
	}

	createdBy := claims.UserID
	product, err := h.productSvc.Create(c.Request.Context(), req, createdBy)
	if err != nil {
		c.HTML(http.StatusBadRequest, "inventory/create.html", gin.H{
			"title":      "Add Product",
			"error":      err.Error(),
			"categories": categories,
			"req":        req,
			"claims":     claims,
		})
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/inventory/%s", product.ID))
}

func (h *ProductHandler) ShowEdit(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	product, err := h.productSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		renderError(c, err)
		return
	}

	categories, _ := h.productSvc.GetCategories(c.Request.Context())
	c.HTML(http.StatusOK, "inventory/edit.html", gin.H{
		"title":      "Edit Product",
		"product":    product,
		"categories": categories,
		"claims":     middleware.GetClaims(c),
	})
}

func (h *ProductHandler) Update(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	var req models.UpdateProductRequest
	if err := c.ShouldBind(&req); err != nil {
		renderError(c, err)
		return
	}

	product, err := h.productSvc.Update(c.Request.Context(), id, req)
	if err != nil {
		renderError(c, err)
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/inventory/%s?updated=1", product.ID))
}

func (h *ProductHandler) Delete(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	if err := h.productSvc.Delete(c.Request.Context(), id); err != nil {
		renderError(c, err)
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/inventory")
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusFound, "/inventory")
}

func (h *ProductHandler) AdjustStock(c *gin.Context) {
	claims := middleware.GetClaims(c)

	var req models.StockMovementRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdBy := claims.UserID
	if err := h.productSvc.AdjustStock(c.Request.Context(), req, createdBy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		product, _ := h.productSvc.GetByID(c.Request.Context(), req.ProductID)
		c.HTML(http.StatusOK, "partials/stock_badge.html", gin.H{"product": product})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "stock adjusted"})
}

func (h *ProductHandler) ImportCSV(c *gin.Context) {
	claims := middleware.GetClaims(c)

	file, _, err := c.Request.FormFile("csv_file")
	if err != nil {
		c.HTML(http.StatusBadRequest, "inventory/import.html", gin.H{
			"title":  "Import Products",
			"error":  "Please select a CSV file",
			"claims": claims,
		})
		return
	}
	defer file.Close()

	createdBy := claims.UserID
	imported, errs, err := h.productSvc.ImportCSV(c.Request.Context(), file, createdBy)
	if err != nil {
		renderError(c, err)
		return
	}

	c.HTML(http.StatusOK, "inventory/import.html", gin.H{
		"title":    "Import Products",
		"imported": imported,
		"errors":   errs,
		"claims":   claims,
	})
}

func (h *ProductHandler) ShowImport(c *gin.Context) {
	c.HTML(http.StatusOK, "inventory/import.html", gin.H{
		"title":  "Import Products",
		"claims": middleware.GetClaims(c),
	})
}

func (h *ProductHandler) GetBarcode(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	product, err := h.productSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	format := c.Query("format")
	if format == "qr" {
		img, err := barcode.GenerateQRPNG(product.Barcode, 200)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"image": img, "value": product.Barcode})
		return
	}

	img, err := barcode.GenerateCode128PNG(product.Barcode, 350, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"image": img, "value": product.Barcode})
}

// --- API handlers ---

func (h *ProductHandler) APIList(c *gin.Context) {
	page, limit := paginationParams(c)
	filter := repository.ProductFilter{
		Search: c.Query("search"),
		Page:   page,
		Limit:  limit,
	}
	active := true
	filter.IsActive = &active

	products, total, err := h.productSvc.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  products,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *ProductHandler) APICreate(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var req models.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product, err := h.productSvc.Create(c.Request.Context(), req, claims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, product)
}

func (h *ProductHandler) APIGet(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	product, err := h.productSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}
	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) APIUpdate(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	var req models.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	product, err := h.productSvc.Update(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) APIDelete(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.productSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *ProductHandler) APISearch(c *gin.Context) {
	q := c.Query("q")
	bc := c.Query("barcode")

	if bc != "" {
		product, err := h.productSvc.GetByBarcode(c.Request.Context(), bc)
		if err != nil || product == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusOK, product)
		return
	}

	filter := repository.ProductFilter{Search: q, Page: 1, Limit: 10}
	active := true
	filter.IsActive = &active
	products, _, err := h.productSvc.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, products)
}

// HTMX partial: product search results for sales page
func (h *ProductHandler) SearchPartial(c *gin.Context) {
	q := c.Query("q")
	filter := repository.ProductFilter{Search: q, Page: 1, Limit: 8}
	active := true
	filter.IsActive = &active

	products, _, _ := h.productSvc.List(c.Request.Context(), filter)
	c.HTML(http.StatusOK, "partials/product_search_results.html", gin.H{
		"products": products,
	})
}

// HTMX: low stock list
func (h *ProductHandler) LowStockPartial(c *gin.Context) {
	products, _ := h.productSvc.GetLowStock(c.Request.Context())
	c.HTML(http.StatusOK, "partials/low_stock_list.html", gin.H{
		"products": products,
	})
}

func renderError(c *gin.Context, err error) {
	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusBadRequest, "partials/error_toast.html", gin.H{"error": err.Error()})
		return
	}
	c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": err.Error()})
}

func countLowStock(products []*models.Product) int {
	count := 0
	for _, p := range products {
		if p.IsLowStock() {
			count++
		}
	}
	return count
}

// Helper: marshal to JSON string for template use
func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func formatFloat(f float64, decimals int) string {
	format := fmt.Sprintf("%%.%df", decimals)
	return fmt.Sprintf(format, f)
}

func percentOf(part, total float64) string {
	if total == 0 {
		return "0"
	}
	return strconv.FormatFloat((part/total)*100, 'f', 1, 64)
}
