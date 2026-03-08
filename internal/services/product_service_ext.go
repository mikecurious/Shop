package services

import (
	"context"

	"github.com/michaelbrian/kiosk/internal/models"
)

func (s *ProductService) GetStockHistory(ctx context.Context, productID string, page, limit int) ([]*models.StockMovement, int, error) {
	return s.stockRepo.List(ctx, &productID, limit, (page-1)*limit)
}
