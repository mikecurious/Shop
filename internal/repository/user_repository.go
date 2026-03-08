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

type UserRepository struct {
	col *mongo.Collection
}

func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{col: db.Collection("users")}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	user.ID = uuid.New().String()
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := r.col.InsertOne(ctx, user)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	u := &models.User{}
	err := r.col.FindOne(ctx, bson.M{"email": email, "is_active": true}).Decode(u)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	u := &models.User{}
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(u)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (r *UserRepository) List(ctx context.Context) ([]*models.User, error) {
	cursor, err := r.col.Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer cursor.Close(ctx)

	var users []*models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}
	return users, nil
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, id string) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"last_login_at": time.Now()}},
	)
	return err
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id string, hash string) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"password_hash": hash, "updated_at": time.Now()}},
	)
	return err
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{
			"name":       user.Name,
			"role":       user.Role,
			"is_active":  user.IsActive,
			"updated_at": time.Now(),
		}},
	)
	return err
}

func (r *UserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	count, err := r.col.CountDocuments(ctx, bson.M{"email": email})
	return count > 0, err
}
