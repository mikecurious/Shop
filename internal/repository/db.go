package repository

import (
	"context"
	"time"

	"github.com/michaelbrian/kiosk/internal/config"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type DB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func NewDB(cfg *config.MongoConfig) (*DB, error) {
	opts := options.Client().ApplyURI(cfg.URI).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(10 * time.Second)

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	db := &DB{
		Client:   client,
		Database: client.Database(cfg.DBName),
	}

	if err := db.EnsureIndexes(ctx); err != nil {
		return nil, err
	}

	log.Info().Str("db", cfg.DBName).Msg("mongodb connected")
	return db, nil
}

func (db *DB) Collection(name string) *mongo.Collection {
	return db.Database.Collection(name)
}

func (db *DB) EnsureIndexes(ctx context.Context) error {
	type spec struct {
		col   string
		model mongo.IndexModel
	}

	specs := []spec{
		{
			col: "users",
			model: mongo.IndexModel{
				Keys:    bson.D{{Key: "email", Value: 1}},
				Options: options.Index().SetUnique(true),
			},
		},
		{
			col: "categories",
			model: mongo.IndexModel{
				Keys:    bson.D{{Key: "name", Value: 1}},
				Options: options.Index().SetUnique(true),
			},
		},
		{
			col: "products",
			model: mongo.IndexModel{
				Keys:    bson.D{{Key: "sku", Value: 1}},
				Options: options.Index().SetUnique(true),
			},
		},
		{
			col: "products",
			model: mongo.IndexModel{
				Keys:    bson.D{{Key: "barcode", Value: 1}},
				Options: options.Index().SetUnique(true),
			},
		},
		{
			col: "products",
			model: mongo.IndexModel{
				Keys: bson.D{{Key: "category_id", Value: 1}},
			},
		},
		{
			col: "stock_movements",
			model: mongo.IndexModel{
				Keys: bson.D{{Key: "product_id", Value: 1}},
			},
		},
		{
			col: "stock_movements",
			model: mongo.IndexModel{
				Keys: bson.D{{Key: "created_at", Value: -1}},
			},
		},
		{
			col: "sales",
			model: mongo.IndexModel{
				Keys: bson.D{{Key: "created_at", Value: -1}},
			},
		},
		{
			col: "sales",
			model: mongo.IndexModel{
				Keys: bson.D{{Key: "payment_method", Value: 1}},
			},
		},
		{
			col: "payments",
			model: mongo.IndexModel{
				Keys: bson.D{{Key: "checkout_request_id", Value: 1}},
			},
		},
		{
			col: "payments",
			model: mongo.IndexModel{
				Keys: bson.D{{Key: "status", Value: 1}},
			},
		},
		{
			col: "alert_preferences",
			model: mongo.IndexModel{
				Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "alert_type", Value: 1}, {Key: "channel", Value: 1}},
				Options: options.Index().SetUnique(true),
			},
		},
		{
			col: "alerts",
			model: mongo.IndexModel{
				Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
			},
		},
	}

	for _, s := range specs {
		col := db.Database.Collection(s.col)
		if _, err := col.Indexes().CreateOne(ctx, s.model); err != nil {
			// Ignore errors from pre-existing identical indexes
			if !mongo.IsDuplicateKeyError(err) {
				log.Warn().Err(err).Str("collection", s.col).Msg("index creation warning")
			}
		}
	}

	return nil
}
