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

type SaleRepository struct {
	col     *mongo.Collection
	prodCol *mongo.Collection
	userCol *mongo.Collection
}

func NewSaleRepository(db *DB) *SaleRepository {
	return &SaleRepository{
		col:     db.Collection("sales"),
		prodCol: db.Collection("products"),
		userCol: db.Collection("users"),
	}
}

type SaleFilter struct {
	From          time.Time
	To            time.Time
	PaymentMethod string
	CreatedBy     *string
	Page          int
	Limit         int
}

func (r *SaleRepository) Create(ctx context.Context, sale *models.Sale) error {
	sale.ID = uuid.New().String()
	sale.CreatedAt = time.Now()

	// Compute TotalCOGS and assign IDs to items
	var totalCOGS float64
	for i := range sale.Items {
		item := &sale.Items[i]
		item.ID = uuid.New().String()
		item.SaleID = sale.ID
		totalCOGS += item.BuyingPrice * float64(item.Quantity)
	}
	sale.TotalCOGS = totalCOGS

	// Populate creator name
	user := &models.User{}
	if err := r.userCol.FindOne(ctx, bson.M{"_id": sale.CreatedBy}).Decode(user); err == nil {
		sale.CreatedByName = user.Name
	}

	// Insert sale with embedded items
	if _, err := r.col.InsertOne(ctx, sale); err != nil {
		return fmt.Errorf("insert sale: %w", err)
	}

	// Decrement stock and record movements (best-effort, no distributed tx)
	for _, item := range sale.Items {
		_, _ = r.prodCol.UpdateOne(ctx,
			bson.M{"_id": item.ProductID},
			bson.M{
				"$inc": bson.M{"quantity": -item.Quantity},
				"$set": bson.M{"updated_at": sale.CreatedAt},
			},
		)

		movement := models.StockMovement{
			ID:        uuid.New().String(),
			ProductID: item.ProductID,
			Type:      models.StockOut,
			Quantity:  item.Quantity,
			Reference: fmt.Sprintf("SALE-%s", sale.ID[:8]),
			Notes:     "Sale transaction",
			CreatedBy: sale.CreatedBy,
			CreatedAt: sale.CreatedAt,
		}
		db := r.col.Database()
		_, _ = db.Collection("stock_movements").InsertOne(ctx, movement)
	}

	return nil
}

func (r *SaleRepository) GetByID(ctx context.Context, id string) (*models.Sale, error) {
	sale := &models.Sale{}
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(sale); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("get sale: %w", err)
	}

	// Populate creator name if missing
	if sale.CreatedByName == "" {
		user := &models.User{}
		if err := r.userCol.FindOne(ctx, bson.M{"_id": sale.CreatedBy}).Decode(user); err == nil {
			sale.CreatedByName = user.Name
		}
	}
	return sale, nil
}

func (r *SaleRepository) List(ctx context.Context, f SaleFilter) ([]*models.Sale, int, error) {
	if f.Limit == 0 {
		f.Limit = 20
	}
	if f.Page == 0 {
		f.Page = 1
	}

	filter := bson.M{}
	if !f.From.IsZero() || !f.To.IsZero() {
		dateRange := bson.M{}
		if !f.From.IsZero() {
			dateRange["$gte"] = f.From
		}
		if !f.To.IsZero() {
			dateRange["$lte"] = f.To
		}
		filter["created_at"] = dateRange
	}
	if f.PaymentMethod != "" {
		filter["payment_method"] = f.PaymentMethod
	}
	if f.CreatedBy != nil {
		filter["created_by"] = *f.CreatedBy
	}

	total64, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count sales: %w", err)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(f.Limit)).
		SetSkip(int64((f.Page - 1) * f.Limit))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list sales: %w", err)
	}
	defer cursor.Close(ctx)

	var sales []*models.Sale
	if err := cursor.All(ctx, &sales); err != nil {
		return nil, 0, fmt.Errorf("decode sales: %w", err)
	}
	return sales, int(total64), nil
}

func (r *SaleRepository) GetDashboardStats(ctx context.Context) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{}

	// All-time totals
	allTimePipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":     nil,
			"count":   bson.M{"$sum": 1},
			"revenue": bson.M{"$sum": "$net_amount"},
			"cogs":    bson.M{"$sum": "$total_cogs"},
		}}},
	}
	cursor, err := r.col.Aggregate(ctx, allTimePipeline)
	if err == nil {
		defer cursor.Close(ctx)
		var results []struct {
			Count   int     `bson:"count"`
			Revenue float64 `bson:"revenue"`
			COGS    float64 `bson:"cogs"`
		}
		if cursor.All(ctx, &results) == nil && len(results) > 0 {
			stats.TotalSales = results[0].Count
			stats.TotalRevenue = results[0].Revenue
			stats.TotalCOGS = results[0].COGS
		}
	}
	stats.GrossProfit = stats.TotalRevenue - stats.TotalCOGS
	if stats.TotalRevenue > 0 {
		stats.GrossMargin = (stats.GrossProfit / stats.TotalRevenue) * 100
	}

	// Today's stats
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"created_at": bson.M{"$gte": startOfDay}}}},
		{{Key: "$group", Value: bson.M{
			"_id":     nil,
			"count":   bson.M{"$sum": 1},
			"revenue": bson.M{"$sum": "$net_amount"},
		}}},
	}
	cursor2, err := r.col.Aggregate(ctx, todayPipeline)
	if err == nil {
		defer cursor2.Close(ctx)
		var todayResults []struct {
			Count   int     `bson:"count"`
			Revenue float64 `bson:"revenue"`
		}
		if cursor2.All(ctx, &todayResults) == nil && len(todayResults) > 0 {
			stats.TodaySales = todayResults[0].Count
			stats.TodayRevenue = todayResults[0].Revenue
		}
	}

	// Low stock count
	lowStockCount, _ := r.prodCol.CountDocuments(ctx, bson.M{
		"is_active": true,
		"$expr":     bson.M{"$lte": bson.A{"$quantity", "$reorder_level"}},
	})
	stats.LowStockCount = int(lowStockCount)

	return stats, nil
}

func (r *SaleRepository) GetPLSummary(ctx context.Context, from, to time.Time, groupBy string) ([]models.PLSummary, error) {
	var dateFormat string
	switch groupBy {
	case "week":
		dateFormat = "%G-W%V"
	case "month":
		dateFormat = "%Y-%m"
	default:
		dateFormat = "%Y-%m-%d"
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"created_at": bson.M{"$gte": from, "$lte": to}}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{"$dateToString": bson.M{
				"format": dateFormat,
				"date":   "$created_at",
			}},
			"revenue":    bson.M{"$sum": "$net_amount"},
			"cogs":       bson.M{"$sum": "$total_cogs"},
			"sale_count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("pl summary aggregate: %w", err)
	}
	defer cursor.Close(ctx)

	var raw []struct {
		ID        string  `bson:"_id"`
		Revenue   float64 `bson:"revenue"`
		COGS      float64 `bson:"cogs"`
		SaleCount int     `bson:"sale_count"`
	}
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}

	result := make([]models.PLSummary, 0, len(raw))
	for _, r := range raw {
		gp := r.Revenue - r.COGS
		margin := 0.0
		if r.Revenue > 0 {
			margin = (gp / r.Revenue) * 100
		}
		result = append(result, models.PLSummary{
			Period:      r.ID,
			Revenue:     r.Revenue,
			COGS:        r.COGS,
			GrossProfit: gp,
			Margin:      margin,
			SaleCount:   r.SaleCount,
		})
	}
	return result, nil
}

func (r *SaleRepository) GetTopProducts(ctx context.Context, from, to time.Time, limit int) ([]models.TopProduct, error) {
	if limit == 0 {
		limit = 10
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"created_at": bson.M{"$gte": from, "$lte": to}}}},
		{{Key: "$unwind", Value: "$items"}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"product_id":   "$items.product_id",
				"product_name": "$items.product_name",
				"product_sku":  "$items.product_sku",
			},
			"total_qty":     bson.M{"$sum": "$items.quantity"},
			"total_revenue": bson.M{"$sum": "$items.subtotal"},
			"total_cogs": bson.M{"$sum": bson.M{
				"$multiply": bson.A{"$items.buying_price", "$items.quantity"},
			}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "total_revenue", Value: -1}}}},
		{{Key: "$limit", Value: limit}},
	}

	cursor, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("top products aggregate: %w", err)
	}
	defer cursor.Close(ctx)

	var raw []struct {
		ID struct {
			ProductID   string `bson:"product_id"`
			ProductName string `bson:"product_name"`
			ProductSKU  string `bson:"product_sku"`
		} `bson:"_id"`
		TotalQty     int     `bson:"total_qty"`
		TotalRevenue float64 `bson:"total_revenue"`
		TotalCOGS    float64 `bson:"total_cogs"`
	}
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}

	result := make([]models.TopProduct, 0, len(raw))
	for _, r := range raw {
		margin := 0.0
		if r.TotalRevenue > 0 {
			margin = ((r.TotalRevenue - r.TotalCOGS) / r.TotalRevenue) * 100
		}
		result = append(result, models.TopProduct{
			ProductID:    r.ID.ProductID,
			ProductName:  r.ID.ProductName,
			SKU:          r.ID.ProductSKU,
			TotalQty:     r.TotalQty,
			TotalRevenue: r.TotalRevenue,
			Margin:       margin,
		})
	}
	return result, nil
}
