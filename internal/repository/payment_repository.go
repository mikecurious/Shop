package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/michaelbrian/kiosk/internal/models"
)

type PaymentRepository struct {
	db *DB
}

func NewPaymentRepository(db *DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(ctx context.Context, p *models.Payment) error {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO payments (id, sale_id, mpesa_receipt, phone_number, amount, status,
		    result_code, result_desc, checkout_request_id, merchant_request_id,
		    transaction_date, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, p.ID, p.SaleID, p.MpesaReceipt, p.PhoneNumber, p.Amount, p.Status,
		p.ResultCode, p.ResultDesc, p.CheckoutRequestID, p.MerchantRequestID,
		p.TransactionDate, p.CreatedAt)
	return err
}

func (r *PaymentRepository) UpdateStatus(ctx context.Context, checkoutID, receipt, resultCode, resultDesc string, status models.PaymentStatus) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE payments SET status = $1, mpesa_receipt = $2, result_code = $3, result_desc = $4
		WHERE checkout_request_id = $5
	`, status, receipt, resultCode, resultDesc, checkoutID)
	return err
}

func (r *PaymentRepository) GetByCheckoutID(ctx context.Context, checkoutID string) (*models.Payment, error) {
	p := &models.Payment{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, sale_id, mpesa_receipt, phone_number, amount, status,
		       result_code, result_desc, checkout_request_id, merchant_request_id,
		       transaction_date, created_at
		FROM payments WHERE checkout_request_id = $1
	`, checkoutID).Scan(
		&p.ID, &p.SaleID, &p.MpesaReceipt, &p.PhoneNumber, &p.Amount, &p.Status,
		&p.ResultCode, &p.ResultDesc, &p.CheckoutRequestID, &p.MerchantRequestID,
		&p.TransactionDate, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *PaymentRepository) List(ctx context.Context, from, to time.Time, status string, page, limit int) ([]*models.Payment, int, error) {
	if limit == 0 {
		limit = 20
	}
	if page == 0 {
		page = 1
	}

	args := []any{}
	conditions := []string{}
	argIdx := 1

	if !from.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, from)
		argIdx++
	}
	if !to.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, to)
		argIdx++
	}
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + joinConditions(conditions)
	}

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM payments %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, sale_id, mpesa_receipt, phone_number, amount, status,
		       result_code, result_desc, checkout_request_id, merchant_request_id,
		       transaction_date, created_at
		FROM payments %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1), append(args, limit, (page-1)*limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var payments []*models.Payment
	for rows.Next() {
		p := &models.Payment{}
		if err := rows.Scan(
			&p.ID, &p.SaleID, &p.MpesaReceipt, &p.PhoneNumber, &p.Amount, &p.Status,
			&p.ResultCode, &p.ResultDesc, &p.CheckoutRequestID, &p.MerchantRequestID,
			&p.TransactionDate, &p.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		payments = append(payments, p)
	}
	return payments, total, rows.Err()
}

func (r *PaymentRepository) LinkToSale(ctx context.Context, paymentID, saleID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE payments SET sale_id = $1 WHERE id = $2`, saleID, paymentID)
	return err
}
