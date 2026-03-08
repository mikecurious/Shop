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

type ProductRepository struct {
	col    *mongo.Collection
	catCol *mongo.Collection
}

func NewProductRepository(db *DB) *ProductRepository {
	return &ProductRepository{
		col:    db.Collection("products"),
		catCol: db.Collection("categories"),
	}
}

type ProductFilter struct {
	Search     string
	CategoryID *string
	LowStock   bool
	IsActive   *bool
	Page       int
	Limit      int
}

func (r *ProductRepository) Create(ctx context.Context, p *models.Product) error {
	p.ID = uuid.New().String()
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	p.IsActive = true

	_, err := r.col.InsertOne(ctx, p)
	return err
}

func (r *ProductRepository) GetByID(ctx context.Context, id string) (*models.Product, error) {
	return r.findOne(ctx, bson.M{"_id": id})
}

func (r *ProductRepository) GetBySKU(ctx context.Context, sku string) (*models.Product, error) {
	return r.findOne(ctx, bson.M{"sku": sku})
}

func (r *ProductRepository) GetByBarcode(ctx context.Context, barcode string) (*models.Product, error) {
	return r.findOne(ctx, bson.M{"barcode": barcode})
}

func (r *ProductRepository) findOne(ctx context.Context, filter bson.M) (*models.Product, error) {
	p := &models.Product{}
	if err := r.col.FindOne(ctx, filter).Decode(p); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("find product: %w", err)
	}
	r.populateCategoryName(ctx, p)
	return p, nil
}

func (r *ProductRepository) populateCategoryName(ctx context.Context, p *models.Product) {
	if p.CategoryID == nil {
		return
	}
	cat := &models.Category{}
	if err := r.catCol.FindOne(ctx, bson.M{"_id": *p.CategoryID}).Decode(cat); err == nil {
		p.CategoryName = cat.Name
	}
}

func (r *ProductRepository) List(ctx context.Context, f ProductFilter) ([]*models.Product, int, error) {
	if f.Limit == 0 {
		f.Limit = 20
	}
	if f.Page == 0 {
		f.Page = 1
	}

	filter := bson.M{}

	if f.Search != "" {
		filter["$or"] = bson.A{
			bson.M{"name": bson.M{"$regex": f.Search, "$options": "i"}},
			bson.M{"sku": bson.M{"$regex": f.Search, "$options": "i"}},
			bson.M{"barcode": bson.M{"$regex": f.Search, "$options": "i"}},
		}
	}
	if f.CategoryID != nil {
		filter["category_id"] = *f.CategoryID
	}
	if f.IsActive != nil {
		filter["is_active"] = *f.IsActive
	}
	if f.LowStock {
		filter["$expr"] = bson.M{"$lte": bson.A{"$quantity", "$reorder_level"}}
	}

	total64, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(f.Limit)).
		SetSkip(int64((f.Page - 1) * f.Limit))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	defer cursor.Close(ctx)

	var products []*models.Product
	if err := cursor.All(ctx, &products); err != nil {
		return nil, 0, fmt.Errorf("decode products: %w", err)
	}

	// Populate category names
	for _, p := range products {
		r.populateCategoryName(ctx, p)
	}

	return products, int(total64), nil
}

func (r *ProductRepository) Update(ctx context.Context, p *models.Product) error {
	p.UpdatedAt = time.Now()
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": p.ID},
		bson.M{"$set": bson.M{
			"name":           p.Name,
			"description":    p.Description,
			"category_id":    p.CategoryID,
			"buying_price":   p.BuyingPrice,
			"selling_price":  p.SellingPrice,
			"quantity":       p.Quantity,
			"reorder_level":  p.ReorderLevel,
			"supplier_name":  p.SupplierName,
			"supplier_phone": p.SupplierPhone,
			"image_url":      p.ImageURL,
			"is_active":      p.IsActive,
			"updated_at":     p.UpdatedAt,
		}},
	)
	return err
}

func (r *ProductRepository) UpdateQuantity(ctx context.Context, id string, delta int) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{
			"$inc": bson.M{"quantity": delta},
			"$set": bson.M{"updated_at": time.Now()},
		},
	)
	return err
}

func (r *ProductRepository) Delete(ctx context.Context, id string) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"is_active": false, "updated_at": time.Now()}},
	)
	return err
}

func (r *ProductRepository) SKUExists(ctx context.Context, sku string, excludeID *string) (bool, error) {
	filter := bson.M{"sku": sku}
	if excludeID != nil {
		filter["_id"] = bson.M{"$ne": *excludeID}
	}
	count, err := r.col.CountDocuments(ctx, filter)
	return count > 0, err
}

func (r *ProductRepository) BarcodeExists(ctx context.Context, barcode string) (bool, error) {
	count, err := r.col.CountDocuments(ctx, bson.M{"barcode": barcode})
	return count > 0, err
}

func (r *ProductRepository) GetLowStock(ctx context.Context) ([]*models.Product, error) {
	filter := bson.M{
		"is_active": true,
		"$expr":     bson.M{"$lte": bson.A{"$quantity", "$reorder_level"}},
	}
	opts := options.Find().SetSort(bson.D{{Key: "quantity", Value: 1}})

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var products []*models.Product
	if err := cursor.All(ctx, &products); err != nil {
		return nil, err
	}
	for _, p := range products {
		r.populateCategoryName(ctx, p)
	}
	return products, nil
}
