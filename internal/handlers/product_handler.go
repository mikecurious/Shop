package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/michaelbrian/kiosk/internal/middleware"
	"github.com/michaelbrian/kiosk/internal/models"
	"github.com/michaelbrian/kiosk/internal/repository"
	"github.com/michaelbrian/kiosk/internal/services"
	"github.com/michaelbrian/kiosk/pkg/barcode"
)

const maxProductPhotoSize = 5 << 20 // 5MB

var allowedPhotoTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// saveProductPhoto reads an optional "photo" file field from a multipart
// form, validates its size and content type, and saves it under
// ./static/uploads/products/. Returns "" with a nil error when no file
// was submitted (photo uploads are optional on create and edit).
func saveProductPhoto(c *gin.Context) (string, error) {
	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return "", nil
		}
		return "", fmt.Errorf("read photo upload: %w", err)
	}
	defer file.Close()

	if header.Size > maxProductPhotoSize {
		return "", fmt.Errorf("photo must be smaller than 5MB")
	}

	sniff := make([]byte, 512)
	n, _ := file.Read(sniff)
	contentType := http.DetectContentType(sniff[:n])
	ext, ok := allowedPhotoTypes[contentType]
	if !ok {
		return "", fmt.Errorf("unsupported image type %q (use JPEG, PNG, WEBP, or GIF)", contentType)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("read photo upload: %w", err)
	}

	filename := uuid.New().String() + ext
	dst, err := os.Create(filepath.Join("static", "uploads", "products", filename))
	if err != nil {
		return "", fmt.Errorf("save photo: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("save photo: %w", err)
	}

	return "/static/uploads/products/" + filename, nil
}

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
	c.HTML(http.StatusOK, "inventory/create.html", withCSRF(c, gin.H{
		"title":      "Add Product",
		"categories": categories,
		"claims":     middleware.GetClaims(c),
	}))
}

func (h *ProductHandler) Create(c *gin.Context) {
	claims := middleware.GetClaims(c)
	categories, _ := h.productSvc.GetCategories(c.Request.Context())

	var req models.CreateProductRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "inventory/create.html", withCSRF(c, gin.H{
			"title":      "Add Product",
			"error":      err.Error(),
			"categories": categories,
			"claims":     claims,
		}))
		return
	}

	imageURL, err := saveProductPhoto(c)
	if err != nil {
		c.HTML(http.StatusBadRequest, "inventory/create.html", withCSRF(c, gin.H{
			"title":      "Add Product",
			"error":      err.Error(),
			"categories": categories,
			"req":        req,
			"claims":     claims,
		}))
		return
	}
	req.ImageURL = imageURL

	createdBy := claims.UserID
	product, err := h.productSvc.Create(c.Request.Context(), req, createdBy)
	if err != nil {
		c.HTML(http.StatusBadRequest, "inventory/create.html", withCSRF(c, gin.H{
			"title":      "Add Product",
			"error":      err.Error(),
			"categories": categories,
			"req":        req,
			"claims":     claims,
		}))
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/staff/inventory/%s", product.ID))
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
	c.HTML(http.StatusOK, "inventory/edit.html", withCSRF(c, gin.H{
		"title":      "Edit Product",
		"product":    product,
		"categories": categories,
		"claims":     middleware.GetClaims(c),
	}))
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

	imageURL, err := saveProductPhoto(c)
	if err != nil {
		product, _ := h.productSvc.GetByID(c.Request.Context(), id)
		categories, _ := h.productSvc.GetCategories(c.Request.Context())
		c.HTML(http.StatusBadRequest, "inventory/edit.html", withCSRF(c, gin.H{
			"title":      "Edit Product",
			"error":      err.Error(),
			"product":    product,
			"categories": categories,
			"claims":     middleware.GetClaims(c),
		}))
		return
	}
	req.ImageURL = imageURL

	product, err := h.productSvc.Update(c.Request.Context(), id, req)
	if err != nil {
		renderError(c, err)
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/staff/inventory/%s?updated=1", product.ID))
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
		c.Header("HX-Redirect", "/staff/inventory")
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusFound, "/staff/inventory")
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
		c.HTML(http.StatusBadRequest, "inventory/import.html", withCSRF(c, gin.H{
			"title":  "Import Products",
			"error":  "Please select a CSV file",
			"claims": claims,
		}))
		return
	}
	defer file.Close()

	createdBy := claims.UserID
	imported, errs, err := h.productSvc.ImportCSV(c.Request.Context(), file, createdBy)
	if err != nil {
		renderError(c, err)
		return
	}

	c.HTML(http.StatusOK, "inventory/import.html", withCSRF(c, gin.H{
		"title":    "Import Products",
		"imported": imported,
		"errors":   errs,
		"claims":   claims,
	}))
}

func (h *ProductHandler) ShowImport(c *gin.Context) {
	c.HTML(http.StatusOK, "inventory/import.html", withCSRF(c, gin.H{
		"title":  "Import Products",
		"claims": middleware.GetClaims(c),
	}))
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

// apiProduct maps backend model fields to the names the React frontend expects.
type apiProduct struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	SKU           string  `json:"sku"`
	Barcode       string  `json:"barcode"`
	Category      string  `json:"category"`
	CategoryID    *string `json:"category_id"`
	Price         float64 `json:"price"`
	Cost          float64 `json:"cost"`
	Stock         int     `json:"stock"`
	MinStock      int     `json:"min_stock"`
	SupplierName  string  `json:"supplier_name"`
	SupplierPhone string  `json:"supplier_phone"`
	ImageURL      string  `json:"image_url"`
	IsActive      bool    `json:"is_active"`
}

func toAPIProduct(p *models.Product) apiProduct {
	return apiProduct{
		ID:            p.ID,
		Name:          p.Name,
		Description:   p.Description,
		SKU:           p.SKU,
		Barcode:       p.Barcode,
		Category:      p.CategoryName,
		CategoryID:    p.CategoryID,
		Price:         p.SellingPrice,
		Cost:          p.BuyingPrice,
		Stock:         p.Quantity,
		MinStock:      p.ReorderLevel,
		SupplierName:  p.SupplierName,
		SupplierPhone: p.SupplierPhone,
		ImageURL:      p.ImageURL,
		IsActive:      p.IsActive,
	}
}

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

	out := make([]apiProduct, len(products))
	for i, p := range products {
		out[i] = toAPIProduct(p)
	}

	c.JSON(http.StatusOK, gin.H{
		"products": out,
		"total":    total,
		"page":     page,
		"per_page": limit,
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
	c.JSON(http.StatusCreated, toAPIProduct(product))
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
	c.JSON(http.StatusOK, toAPIProduct(product))
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
	c.JSON(http.StatusOK, toAPIProduct(product))
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
		c.JSON(http.StatusOK, toAPIProduct(product))
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
	out := make([]apiProduct, len(products))
	for i, p := range products {
		out[i] = toAPIProduct(p)
	}
	c.JSON(http.StatusOK, out)
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
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

