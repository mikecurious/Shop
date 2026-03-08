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

type CategoryRepository struct {
	col *mongo.Collection
}

func NewCategoryRepository(db *DB) *CategoryRepository {
	return &CategoryRepository{col: db.Collection("categories")}
}

func (r *CategoryRepository) Create(ctx context.Context, c *models.Category) error {
	c.ID = uuid.New().String()
	c.CreatedAt = time.Now()
	_, err := r.col.InsertOne(ctx, c)
	return err
}

func (r *CategoryRepository) List(ctx context.Context) ([]*models.Category, error) {
	cursor, err := r.col.Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer cursor.Close(ctx)

	var cats []*models.Category
	if err := cursor.All(ctx, &cats); err != nil {
		return nil, fmt.Errorf("decode categories: %w", err)
	}
	return cats, nil
}

func (r *CategoryRepository) Delete(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
