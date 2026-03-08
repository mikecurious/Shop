package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/michaelbrian/kiosk/internal/models"
)

type SaleRepository struct {
	db *DB
}

func NewSaleRepository(db *DB) *SaleRepository {
	return &SaleRepository{db: db}
}

func (r *SaleRepository) Create(ctx context.Context, sale *models.Sale) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	sale.ID = uuid.New()
	sale.CreatedAt = time.Now()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO sales (id, total_amount, discount_type, discount_value, discount_amount,
		    net_amount, payment_method, payment_reference, customer_name, customer_phone,
		    notes, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, sale.ID, sale.TotalAmount, sale.DiscountType, sale.DiscountValue, sale.DiscountAmount,
		sale.NetAmount, sale.PaymentMethod, sale.PaymentReference,
		sale.CustomerName, sale.CustomerPhone, sale.Notes, sale.CreatedBy, sale.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert sale: %w", err)
	}

	for i := range sale.Items {
		item := &sale.Items[i]
		item.ID = uuid.New()
		item.SaleID = sale.ID
		_, err = tx.ExecContext(ctx, `
			INSERT INTO sale_items (id, sale_id, product_id, quantity, unit_price, buying_price, subtotal)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, item.ID, item.SaleID, item.ProductID, item.Quantity,
			item.UnitPrice, item.BuyingPrice, item.Subtotal)
		if err != nil {
			return fmt.Errorf("insert sale item: %w", err)
		}

		// Decrement stock
		_, err = tx.ExecContext(ctx,
			`UPDATE products SET quantity = quantity - $1, updated_at = NOW() WHERE id = $2`,
			item.Quantity, item.ProductID)
		if err != nil {
			return fmt.Errorf("update stock: %w", err)
		}

		// Record stock movement
		_, err = tx.ExecContext(ctx, `
			INSERT INTO stock_movements (id, product_id, type, quantity, reference, notes, created_by, created_at)
			VALUES ($1,$2,'out',$3,$4,'Sale transaction',$5,$6)
		`, uuid.New(), item.ProductID, item.Quantity,
			fmt.Sprintf("SALE-%s", sale.ID.String()[:8]),
			sale.CreatedBy, sale.CreatedAt)
		if err != nil {
			return fmt.Errorf("insert stock movement: %w", err)
		}
	}

	return tx.Commit()
}

func (r *SaleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Sale, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT s.id, s.total_amount, s.discount_type, s.discount_value, s.discount_amount,
		       s.net_amount, s.payment_method, s.payment_reference,
		       s.customer_name, s.customer_phone, s.notes,
		       s.created_by, COALESCE(u.name,'') as created_by_name, s.created_at
		FROM sales s
		LEFT JOIN users u ON u.id = s.created_by
		WHERE s.id = $1
	`, id)

	sale := &models.Sale{}
	err := row.Scan(
		&sale.ID, &sale.TotalAmount, &sale.DiscountType, &sale.DiscountValue, &sale.DiscountAmount,
		&sale.NetAmount, &sale.PaymentMethod, &sale.PaymentReference,
		&sale.CustomerName, &sale.CustomerPhone, &sale.Notes,
		&sale.CreatedBy, &sale.CreatedByName, &sale.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Load items
	rows, err := r.db.QueryContext(ctx, `
		SELECT si.id, si.sale_id, si.product_id, COALESCE(p.name,'') as product_name,
		       COALESCE(p.sku,'') as product_sku,
		       si.quantity, si.unit_price, si.buying_price, si.subtotal
		FROM sale_items si
		LEFT JOIN products p ON p.id = si.product_id
		WHERE si.sale_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.SaleItem
		if err := rows.Scan(
			&item.ID, &item.SaleID, &item.ProductID, &item.ProductName, &item.ProductSKU,
			&item.Quantity, &item.UnitPrice, &item.BuyingPrice, &item.Subtotal,
		); err != nil {
			return nil, err
		}
		sale.Items = append(sale.Items, item)
	}
	return sale, rows.Err()
}

type SaleFilter struct {
	From          time.Time
	To            time.Time
	PaymentMethod string
	CreatedBy     *uuid.UUID
	Page          int
	Limit         int
}

func (r *SaleRepository) List(ctx context.Context, f SaleFilter) ([]*models.Sale, int, error) {
	if f.Limit == 0 {
		f.Limit = 20
	}
	if f.Page == 0 {
		f.Page = 1
	}

	args := []any{}
	conditions := []string{}
	argIdx := 1

	if !f.From.IsZero() {
		conditions = append(conditions, fmt.Sprintf("s.created_at >= $%d", argIdx))
		args = append(args, f.From)
		argIdx++
	}
	if !f.To.IsZero() {
		conditions = append(conditions, fmt.Sprintf("s.created_at <= $%d", argIdx))
		args = append(args, f.To)
		argIdx++
	}
	if f.PaymentMethod != "" {
		conditions = append(conditions, fmt.Sprintf("s.payment_method = $%d", argIdx))
		args = append(args, f.PaymentMethod)
		argIdx++
	}
	if f.CreatedBy != nil {
		conditions = append(conditions, fmt.Sprintf("s.created_by = $%d", argIdx))
		args = append(args, *f.CreatedBy)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + joinConditions(conditions)
	}

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM sales s %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT s.id, s.total_amount, s.discount_type, s.discount_value, s.discount_amount,
		       s.net_amount, s.payment_method, s.payment_reference,
		       s.customer_name, s.customer_phone, s.notes,
		       s.created_by, COALESCE(u.name,'') as created_by_name, s.created_at
		FROM sales s
		LEFT JOIN users u ON u.id = s.created_by
		%s
		ORDER BY s.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1), append(args, f.Limit, (f.Page-1)*f.Limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var sales []*models.Sale
	for rows.Next() {
		s := &models.Sale{}
		if err := rows.Scan(
			&s.ID, &s.TotalAmount, &s.DiscountType, &s.DiscountValue, &s.DiscountAmount,
			&s.NetAmount, &s.PaymentMethod, &s.PaymentReference,
			&s.CustomerName, &s.CustomerPhone, &s.Notes,
			&s.CreatedBy, &s.CreatedByName, &s.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		sales = append(sales, s)
	}
	return sales, total, rows.Err()
}

func (r *SaleRepository) GetDashboardStats(ctx context.Context) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{}

	// All-time revenue, COGS, gross profit
	err := r.db.QueryRowContext(ctx, `
		SELECT
		    COALESCE(SUM(s.net_amount), 0) AS total_revenue,
		    COALESCE(SUM(si.buying_price * si.quantity), 0) AS total_cogs,
		    COUNT(DISTINCT s.id) AS total_sales
		FROM sales s
		JOIN sale_items si ON si.sale_id = s.id
	`).Scan(&stats.TotalRevenue, &stats.TotalCOGS, &stats.TotalSales)
	if err != nil {
		return nil, err
	}
	stats.GrossProfit = stats.TotalRevenue - stats.TotalCOGS
	if stats.TotalRevenue > 0 {
		stats.GrossMargin = (stats.GrossProfit / stats.TotalRevenue) * 100
	}

	// Today
	today := time.Now().Truncate(24 * time.Hour)
	err = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(net_amount),0), COUNT(*) FROM sales
		WHERE created_at >= $1
	`, today).Scan(&stats.TodayRevenue, &stats.TodaySales)
	if err != nil {
		return nil, err
	}

	// Low stock
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM products WHERE quantity <= reorder_level AND is_active = TRUE`,
	).Scan(&stats.LowStockCount)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *SaleRepository) GetPLSummary(ctx context.Context, from, to time.Time, groupBy string) ([]models.PLSummary, error) {
	var dateTrunc string
	switch groupBy {
	case "week":
		dateTrunc = "week"
	case "month":
		dateTrunc = "month"
	default:
		dateTrunc = "day"
	}

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT
		    TO_CHAR(DATE_TRUNC('%s', s.created_at), 'YYYY-MM-DD') as period,
		    COALESCE(SUM(s.net_amount), 0) as revenue,
		    COALESCE(SUM(si.buying_price * si.quantity), 0) as cogs,
		    COUNT(DISTINCT s.id) as sale_count
		FROM sales s
		JOIN sale_items si ON si.sale_id = s.id
		WHERE s.created_at BETWEEN $1 AND $2
		GROUP BY DATE_TRUNC('%s', s.created_at)
		ORDER BY period ASC
	`, dateTrunc, dateTrunc), from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.PLSummary
	for rows.Next() {
		var p models.PLSummary
		if err := rows.Scan(&p.Period, &p.Revenue, &p.COGS, &p.SaleCount); err != nil {
			return nil, err
		}
		p.GrossProfit = p.Revenue - p.COGS
		if p.Revenue > 0 {
			p.Margin = (p.GrossProfit / p.Revenue) * 100
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *SaleRepository) GetTopProducts(ctx context.Context, from, to time.Time, limit int) ([]models.TopProduct, error) {
	if limit == 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT si.product_id, COALESCE(p.name,'') as product_name, COALESCE(p.sku,'') as sku,
		       SUM(si.quantity) as total_qty,
		       SUM(si.subtotal) as total_revenue,
		       CASE WHEN SUM(si.subtotal) > 0
		            THEN (SUM(si.subtotal) - SUM(si.buying_price * si.quantity)) / SUM(si.subtotal) * 100
		            ELSE 0 END as margin
		FROM sale_items si
		LEFT JOIN products p ON p.id = si.product_id
		LEFT JOIN sales s ON s.id = si.sale_id
		WHERE s.created_at BETWEEN $1 AND $2
		GROUP BY si.product_id, p.name, p.sku
		ORDER BY total_revenue DESC
		LIMIT $3
	`, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.TopProduct
	for rows.Next() {
		var tp models.TopProduct
		if err := rows.Scan(&tp.ProductID, &tp.ProductName, &tp.SKU,
			&tp.TotalQty, &tp.TotalRevenue, &tp.Margin); err != nil {
			return nil, err
		}
		result = append(result, tp)
	}
	return result, rows.Err()
}

func joinConditions(conds []string) string {
	result := ""
	for i, c := range conds {
		if i > 0 {
			result += " AND "
		}
		result += c
	}
	return result
}
