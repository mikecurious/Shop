package services

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/michaelbrian/kiosk/internal/models"
	"github.com/michaelbrian/kiosk/internal/repository"
	"github.com/michaelbrian/kiosk/pkg/barcode"
)

var (
	ErrProductNotFound = errors.New("product not found")
	ErrDuplicateSKU    = errors.New("SKU already exists")
)

type ProductService struct {
	productRepo  *repository.ProductRepository
	categoryRepo *repository.CategoryRepository
	stockRepo    *repository.StockRepository
}

func NewProductService(
	productRepo *repository.ProductRepository,
	categoryRepo *repository.CategoryRepository,
	stockRepo *repository.StockRepository,
) *ProductService {
	return &ProductService{
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
		stockRepo:    stockRepo,
	}
}

func (s *ProductService) Create(ctx context.Context, req models.CreateProductRequest, createdBy string) (*models.Product, error) {
	if req.SKU == "" {
		req.SKU = generateSKU(req.Name)
	}

	exists, err := s.productRepo.SKUExists(ctx, req.SKU, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		req.SKU = req.SKU + "-" + uuid.New().String()[:4]
	}

	bc, err := barcode.Generate()
	if err != nil {
		return nil, fmt.Errorf("generate barcode: %w", err)
	}

	p := &models.Product{
		Name:          req.Name,
		Description:   req.Description,
		CategoryID:    req.CategoryID,
		SKU:           req.SKU,
		Barcode:       bc,
		BuyingPrice:   req.BuyingPrice,
		SellingPrice:  req.SellingPrice,
		Quantity:      req.Quantity,
		ReorderLevel:  req.ReorderLevel,
		SupplierName:  req.SupplierName,
		SupplierPhone: req.SupplierPhone,
	}

	if err := s.productRepo.Create(ctx, p); err != nil {
		return nil, err
	}

	if req.Quantity > 0 {
		_ = s.stockRepo.Create(ctx, &models.StockMovement{
			ProductID: p.ID,
			Type:      models.StockIn,
			Quantity:  req.Quantity,
			Reference: "INITIAL",
			Notes:     "Initial stock on product creation",
			CreatedBy: createdBy,
		})
	}

	return p, nil
}

func (s *ProductService) GetByID(ctx context.Context, id string) (*models.Product, error) {
	p, err := s.productRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrProductNotFound
	}
	return p, nil
}

func (s *ProductService) GetByBarcode(ctx context.Context, bc string) (*models.Product, error) {
	return s.productRepo.GetByBarcode(ctx, bc)
}

func (s *ProductService) List(ctx context.Context, f repository.ProductFilter) ([]*models.Product, int, error) {
	return s.productRepo.List(ctx, f)
}

func (s *ProductService) Update(ctx context.Context, id string, req models.UpdateProductRequest) (*models.Product, error) {
	p, err := s.productRepo.GetByID(ctx, id)
	if err != nil || p == nil {
		return nil, ErrProductNotFound
	}

	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.CategoryID != nil {
		p.CategoryID = req.CategoryID
	}
	if req.BuyingPrice > 0 {
		p.BuyingPrice = req.BuyingPrice
	}
	if req.SellingPrice > 0 {
		p.SellingPrice = req.SellingPrice
	}
	if req.ReorderLevel >= 0 {
		p.ReorderLevel = req.ReorderLevel
	}
	if req.SupplierName != "" {
		p.SupplierName = req.SupplierName
	}
	if req.SupplierPhone != "" {
		p.SupplierPhone = req.SupplierPhone
	}

	if err := s.productRepo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProductService) Delete(ctx context.Context, id string) error {
	return s.productRepo.Delete(ctx, id)
}

func (s *ProductService) AdjustStock(ctx context.Context, req models.StockMovementRequest, createdBy string) error {
	p, err := s.productRepo.GetByID(ctx, req.ProductID)
	if err != nil || p == nil {
		return ErrProductNotFound
	}

	var delta int
	switch req.Type {
	case models.StockIn:
		delta = req.Quantity
	case models.StockOut:
		if p.Quantity < req.Quantity {
			return fmt.Errorf("insufficient stock: have %d, need %d", p.Quantity, req.Quantity)
		}
		delta = -req.Quantity
	case models.StockAdjustment:
		delta = req.Quantity - p.Quantity
	}

	if err := s.productRepo.UpdateQuantity(ctx, req.ProductID, delta); err != nil {
		return err
	}

	return s.stockRepo.Create(ctx, &models.StockMovement{
		ProductID: req.ProductID,
		Type:      req.Type,
		Quantity:  req.Quantity,
		Reference: req.Reference,
		Notes:     req.Notes,
		CreatedBy: createdBy,
	})
}

func (s *ProductService) GetLowStock(ctx context.Context) ([]*models.Product, error) {
	return s.productRepo.GetLowStock(ctx)
}

func (s *ProductService) ImportCSV(ctx context.Context, r io.Reader, createdBy string) (int, []string, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	if _, err := reader.Read(); err != nil {
		return 0, nil, fmt.Errorf("read header: %w", err)
	}

	var (
		imported int
		errs     []string
	)

	for lineNum := 2; ; lineNum++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("line %d: %v", lineNum, err))
			continue
		}
		if len(record) < 6 {
			errs = append(errs, fmt.Sprintf("line %d: insufficient columns", lineNum))
			continue
		}

		buyingPrice, _ := strconv.ParseFloat(strings.TrimSpace(record[4]), 64)
		sellingPrice, _ := strconv.ParseFloat(strings.TrimSpace(record[5]), 64)
		quantity := 0
		if len(record) > 6 {
			quantity, _ = strconv.Atoi(strings.TrimSpace(record[6]))
		}
		reorderLevel := 5
		if len(record) > 7 {
			reorderLevel, _ = strconv.Atoi(strings.TrimSpace(record[7]))
		}

		req := models.CreateProductRequest{
			Name:         strings.TrimSpace(record[0]),
			Description:  strings.TrimSpace(record[1]),
			SKU:          strings.TrimSpace(record[2]),
			BuyingPrice:  buyingPrice,
			SellingPrice: sellingPrice,
			Quantity:     quantity,
			ReorderLevel: reorderLevel,
		}
		if len(record) > 8 {
			req.SupplierName = strings.TrimSpace(record[8])
		}

		if _, err := s.Create(ctx, req, createdBy); err != nil {
			errs = append(errs, fmt.Sprintf("line %d (%s): %v", lineNum, req.Name, err))
			continue
		}
		imported++
	}

	return imported, errs, nil
}

func (s *ProductService) GetCategories(ctx context.Context) ([]*models.Category, error) {
	return s.categoryRepo.List(ctx)
}

func (s *ProductService) CreateCategory(ctx context.Context, name, description string) (*models.Category, error) {
	c := &models.Category{Name: name, Description: description}
	if err := s.categoryRepo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func generateSKU(name string) string {
	name = strings.ToUpper(strings.TrimSpace(name))
	words := strings.Fields(name)
	var prefix string
	for i, w := range words {
		if i >= 3 {
			break
		}
		if len(w) > 0 {
			prefix += string(w[0])
		}
	}
	if prefix == "" {
		prefix = "PRD"
	}
	return prefix + "-" + uuid.New().String()[:6]
}
