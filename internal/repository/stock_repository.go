package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/michaelbrian/kiosk/internal/models"
)

type StockRepository struct {
	db *DB
}

func NewStockRepository(db *DB) *StockRepository {
	return &StockRepository{db: db}
}

func (r *StockRepository) Create(ctx context.Context, m *models.StockMovement) error {
	query := `
		INSERT INTO stock_movements (id, product_id, type, quantity, reference, notes, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	m.ID = uuid.New()
	m.CreatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, query,
		m.ID, m.ProductID, m.Type, m.Quantity, m.Reference, m.Notes, m.CreatedBy, m.CreatedAt)
	return err
}

func (r *StockRepository) List(ctx context.Context, productID *uuid.UUID, limit, offset int) ([]*models.StockMovement, int, error) {
	if limit == 0 {
		limit = 20
	}
	args := []any{}
	argIdx := 1
	where := ""

	if productID != nil {
		where = fmt.Sprintf("WHERE sm.product_id = $%d", argIdx)
		args = append(args, *productID)
		argIdx++
	}

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM stock_movements sm %s`, where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT sm.id, sm.product_id, COALESCE(p.name,'') as product_name,
		       sm.type, sm.quantity, sm.reference, sm.notes,
		       sm.created_by, COALESCE(u.name,'') as created_by_name, sm.created_at
		FROM stock_movements sm
		LEFT JOIN products p ON p.id = sm.product_id
		LEFT JOIN users u ON u.id = sm.created_by
		%s
		ORDER BY sm.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var movements []*models.StockMovement
	for rows.Next() {
		m := &models.StockMovement{}
		if err := rows.Scan(
			&m.ID, &m.ProductID, &m.ProductName,
			&m.Type, &m.Quantity, &m.Reference, &m.Notes,
			&m.CreatedBy, &m.CreatedByName, &m.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		movements = append(movements, m)
	}
	return movements, total, rows.Err()
}
