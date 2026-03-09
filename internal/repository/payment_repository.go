package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/michaelbrian/kiosk/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type PaymentRepository struct {
	col *mongo.Collection
}

func NewPaymentRepository(db *DB) *PaymentRepository {
	return &PaymentRepository{col: db.Collection("payments")}
}

func (r *PaymentRepository) Create(ctx context.Context, p *models.Payment) error {
	p.ID = uuid.New().String()
	p.CreatedAt = time.Now()
	_, err := r.col.InsertOne(ctx, p)
	return err
}

func (r *PaymentRepository) UpdateStatus(ctx context.Context, checkoutID, receipt, resultCode, resultDesc string, status models.PaymentStatus) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"checkout_request_id": checkoutID},
		bson.M{"$set": bson.M{
			"status":       status,
			"mpesa_receipt": receipt,
			"result_code":  resultCode,
			"result_desc":  resultDesc,
		}},
	)
	return err
}

func (r *PaymentRepository) GetByCheckoutID(ctx context.Context, checkoutID string) (*models.Payment, error) {
	p := &models.Payment{}
	if err := r.col.FindOne(ctx, bson.M{"checkout_request_id": checkoutID}).Decode(p); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
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

	filter := bson.M{}
	if !from.IsZero() || !to.IsZero() {
		dateRange := bson.M{}
		if !from.IsZero() {
			dateRange["$gte"] = from
		}
		if !to.IsZero() {
			dateRange["$lte"] = to
		}
		filter["created_at"] = dateRange
	}
	if status != "" {
		filter["status"] = status
	}

	total64, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count payments: %w", err)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64((page - 1) * limit))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list payments: %w", err)
	}
	defer cursor.Close(ctx)

	var payments []*models.Payment
	if err := cursor.All(ctx, &payments); err != nil {
		return nil, 0, fmt.Errorf("decode payments: %w", err)
	}
	return payments, int(total64), nil
}

func (r *PaymentRepository) LinkToSale(ctx context.Context, paymentID, saleID string) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": paymentID},
		bson.M{"$set": bson.M{"sale_id": saleID}},
	)
	return err
}
