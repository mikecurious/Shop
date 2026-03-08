package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/michaelbrian/kiosk/internal/models"
	"github.com/michaelbrian/kiosk/internal/repository"
)

var ErrSaleNotFound = errors.New("sale not found")

type SaleService struct {
	saleRepo    *repository.SaleRepository
	productRepo *repository.ProductRepository
}

func NewSaleService(saleRepo *repository.SaleRepository, productRepo *repository.ProductRepository) *SaleService {
	return &SaleService{saleRepo: saleRepo, productRepo: productRepo}
}

func (s *SaleService) CreateSale(ctx context.Context, req models.CreateSaleRequest, createdBy string) (*models.Sale, error) {
	sale := &models.Sale{
		PaymentMethod:    req.PaymentMethod,
		PaymentReference: "",
		CustomerName:     req.CustomerName,
		CustomerPhone:    req.CustomerPhone,
		Notes:            req.Notes,
		CreatedBy:        createdBy,
		DiscountType:     req.DiscountType,
		DiscountValue:    req.DiscountValue,
	}

	var total float64
	items := make([]models.SaleItem, 0, len(req.Items))

	for _, itemReq := range req.Items {
		p, err := s.productRepo.GetByID(ctx, itemReq.ProductID)
		if err != nil || p == nil {
			return nil, fmt.Errorf("product %s not found", itemReq.ProductID)
		}
		if p.Quantity < itemReq.Quantity {
			return nil, fmt.Errorf("insufficient stock for %s: have %d, need %d",
				p.Name, p.Quantity, itemReq.Quantity)
		}

		subtotal := p.SellingPrice * float64(itemReq.Quantity)
		total += subtotal

		items = append(items, models.SaleItem{
			ProductID:   itemReq.ProductID,
			ProductName: p.Name,
			ProductSKU:  p.SKU,
			Quantity:    itemReq.Quantity,
			UnitPrice:   p.SellingPrice,
			BuyingPrice: p.BuyingPrice,
			Subtotal:    subtotal,
		})
	}

	sale.TotalAmount = total

	var discountAmount float64
	switch req.DiscountType {
	case "percentage":
		discountAmount = total * (req.DiscountValue / 100)
	case "fixed":
		discountAmount = req.DiscountValue
	}
	if discountAmount > total {
		discountAmount = total
	}
	sale.DiscountAmount = discountAmount
	sale.NetAmount = total - discountAmount
	sale.Items = items

	if err := s.saleRepo.Create(ctx, sale); err != nil {
		return nil, fmt.Errorf("create sale: %w", err)
	}
	return sale, nil
}

func (s *SaleService) GetSale(ctx context.Context, id string) (*models.Sale, error) {
	sale, err := s.saleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sale == nil {
		return nil, ErrSaleNotFound
	}
	return sale, nil
}

func (s *SaleService) ListSales(ctx context.Context, f repository.SaleFilter) ([]*models.Sale, int, error) {
	return s.saleRepo.List(ctx, f)
}

func (s *SaleService) GetDashboardStats(ctx context.Context) (*models.DashboardStats, error) {
	return s.saleRepo.GetDashboardStats(ctx)
}

func (s *SaleService) GetPLSummary(ctx context.Context, from, to time.Time, groupBy string) ([]models.PLSummary, error) {
	if from.IsZero() {
		from = time.Now().AddDate(0, -1, 0)
	}
	if to.IsZero() {
		to = time.Now()
	}
	return s.saleRepo.GetPLSummary(ctx, from, to, groupBy)
}

func (s *SaleService) GetTopProducts(ctx context.Context, from, to time.Time, limit int) ([]models.TopProduct, error) {
	if from.IsZero() {
		from = time.Now().AddDate(0, -1, 0)
	}
	if to.IsZero() {
		to = time.Now()
	}
	return s.saleRepo.GetTopProducts(ctx, from, to, limit)
}
