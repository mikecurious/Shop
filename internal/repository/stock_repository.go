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

type StockRepository struct {
	col     *mongo.Collection
	userCol *mongo.Collection
	prodCol *mongo.Collection
}

func NewStockRepository(db *DB) *StockRepository {
	return &StockRepository{
		col:     db.Collection("stock_movements"),
		userCol: db.Collection("users"),
		prodCol: db.Collection("products"),
	}
}

func (r *StockRepository) Create(ctx context.Context, m *models.StockMovement) error {
	m.ID = uuid.New().String()
	m.CreatedAt = time.Now()
	_, err := r.col.InsertOne(ctx, m)
	return err
}

func (r *StockRepository) List(ctx context.Context, productID *string, limit, offset int) ([]*models.StockMovement, int, error) {
	if limit == 0 {
		limit = 20
	}

	filter := bson.M{}
	if productID != nil {
		filter["product_id"] = *productID
	}

	total64, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count stock movements: %w", err)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list stock movements: %w", err)
	}
	defer cursor.Close(ctx)

	var movements []*models.StockMovement
	if err := cursor.All(ctx, &movements); err != nil {
		return nil, 0, fmt.Errorf("decode stock movements: %w", err)
	}

	// Populate product and user names
	for _, m := range movements {
		if m.ProductName == "" {
			prod := &models.Product{}
			if err := r.prodCol.FindOne(ctx, bson.M{"_id": m.ProductID}).Decode(prod); err == nil {
				m.ProductName = prod.Name
			}
		}
		if m.CreatedByName == "" {
			user := &models.User{}
			if err := r.userCol.FindOne(ctx, bson.M{"_id": m.CreatedBy}).Decode(user); err == nil {
				m.CreatedByName = user.Name
			}
		}
	}

	return movements, int(total64), nil
}
