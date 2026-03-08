package models

import (
	"time"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleStaff Role = "staff"
)

type User struct {
	ID           string     `bson:"_id" json:"id"`
	Email        string     `bson:"email" json:"email"`
	PasswordHash string     `bson:"password_hash" json:"-"`
	Name         string     `bson:"name" json:"name"`
	Role         Role       `bson:"role" json:"role"`
	IsActive     bool       `bson:"is_active" json:"is_active"`
	LastLoginAt  *time.Time `bson:"last_login_at,omitempty" json:"last_login_at"`
	CreatedAt    time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `bson:"updated_at" json:"updated_at"`
}

type Category struct {
	ID          string    `bson:"_id" json:"id"`
	Name        string    `bson:"name" json:"name"`
	Description string    `bson:"description" json:"description"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
}

type Product struct {
	ID            string    `bson:"_id" json:"id"`
	Name          string    `bson:"name" json:"name"`
	Description   string    `bson:"description" json:"description"`
	CategoryID    *string   `bson:"category_id,omitempty" json:"category_id"`
	CategoryName  string    `bson:"category_name,omitempty" json:"category_name"`
	SKU           string    `bson:"sku" json:"sku"`
	Barcode       string    `bson:"barcode" json:"barcode"`
	BuyingPrice   float64   `bson:"buying_price" json:"buying_price"`
	SellingPrice  float64   `bson:"selling_price" json:"selling_price"`
	Quantity      int       `bson:"quantity" json:"quantity"`
	ReorderLevel  int       `bson:"reorder_level" json:"reorder_level"`
	SupplierName  string    `bson:"supplier_name,omitempty" json:"supplier_name"`
	SupplierPhone string    `bson:"supplier_phone,omitempty" json:"supplier_phone"`
	ImageURL      string    `bson:"image_url,omitempty" json:"image_url"`
	IsActive      bool      `bson:"is_active" json:"is_active"`
	CreatedAt     time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time `bson:"updated_at" json:"updated_at"`
}

func (p *Product) IsLowStock() bool {
	return p.Quantity <= p.ReorderLevel
}

func (p *Product) Margin() float64 {
	if p.SellingPrice == 0 {
		return 0
	}
	return ((p.SellingPrice - p.BuyingPrice) / p.SellingPrice) * 100
}

type StockMovementType string

const (
	StockIn         StockMovementType = "in"
	StockOut        StockMovementType = "out"
	StockAdjustment StockMovementType = "adjustment"
)

type StockMovement struct {
	ID            string            `bson:"_id" json:"id"`
	ProductID     string            `bson:"product_id" json:"product_id"`
	ProductName   string            `bson:"product_name,omitempty" json:"product_name"`
	Type          StockMovementType `bson:"type" json:"type"`
	Quantity      int               `bson:"quantity" json:"quantity"`
	Reference     string            `bson:"reference,omitempty" json:"reference"`
	Notes         string            `bson:"notes,omitempty" json:"notes"`
	CreatedBy     string            `bson:"created_by" json:"created_by"`
	CreatedByName string            `bson:"created_by_name,omitempty" json:"created_by_name"`
	CreatedAt     time.Time         `bson:"created_at" json:"created_at"`
}

type PaymentMethod string

const (
	PaymentCash   PaymentMethod = "cash"
	PaymentMPesa  PaymentMethod = "mpesa"
	PaymentCard   PaymentMethod = "card"
	PaymentCredit PaymentMethod = "credit"
)

type Sale struct {
	ID               string        `bson:"_id" json:"id"`
	TotalAmount      float64       `bson:"total_amount" json:"total_amount"`
	DiscountType     string        `bson:"discount_type" json:"discount_type"`
	DiscountValue    float64       `bson:"discount_value" json:"discount_value"`
	DiscountAmount   float64       `bson:"discount_amount" json:"discount_amount"`
	NetAmount        float64       `bson:"net_amount" json:"net_amount"`
	TotalCOGS        float64       `bson:"total_cogs" json:"total_cogs"`
	PaymentMethod    PaymentMethod `bson:"payment_method" json:"payment_method"`
	PaymentReference string        `bson:"payment_reference,omitempty" json:"payment_reference"`
	CustomerName     string        `bson:"customer_name,omitempty" json:"customer_name"`
	CustomerPhone    string        `bson:"customer_phone,omitempty" json:"customer_phone"`
	Notes            string        `bson:"notes,omitempty" json:"notes"`
	CreatedBy        string        `bson:"created_by" json:"created_by"`
	CreatedByName    string        `bson:"created_by_name,omitempty" json:"created_by_name"`
	CreatedAt        time.Time     `bson:"created_at" json:"created_at"`
	Items            []SaleItem    `bson:"items" json:"items"`
}

type SaleItem struct {
	ID          string  `bson:"_id" json:"id"`
	SaleID      string  `bson:"sale_id" json:"sale_id"`
	ProductID   string  `bson:"product_id" json:"product_id"`
	ProductName string  `bson:"product_name" json:"product_name"`
	ProductSKU  string  `bson:"product_sku" json:"product_sku"`
	Quantity    int     `bson:"quantity" json:"quantity"`
	UnitPrice   float64 `bson:"unit_price" json:"unit_price"`
	BuyingPrice float64 `bson:"buying_price" json:"buying_price"`
	Subtotal    float64 `bson:"subtotal" json:"subtotal"`
}

type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentCompleted PaymentStatus = "completed"
	PaymentFailed    PaymentStatus = "failed"
	PaymentCancelled PaymentStatus = "cancelled"
)

type Payment struct {
	ID                string        `bson:"_id" json:"id"`
	SaleID            *string       `bson:"sale_id,omitempty" json:"sale_id"`
	MpesaReceipt      string        `bson:"mpesa_receipt,omitempty" json:"mpesa_receipt"`
	PhoneNumber       string        `bson:"phone_number" json:"phone_number"`
	Amount            float64       `bson:"amount" json:"amount"`
	Status            PaymentStatus `bson:"status" json:"status"`
	ResultCode        string        `bson:"result_code,omitempty" json:"result_code"`
	ResultDesc        string        `bson:"result_desc,omitempty" json:"result_desc"`
	CheckoutRequestID string        `bson:"checkout_request_id,omitempty" json:"checkout_request_id"`
	MerchantRequestID string        `bson:"merchant_request_id,omitempty" json:"merchant_request_id"`
	TransactionDate   time.Time     `bson:"transaction_date" json:"transaction_date"`
	CreatedAt         time.Time     `bson:"created_at" json:"created_at"`
}

type AlertType string
type AlertChannel string

const (
	AlertLowStock         AlertType = "low_stock"
	AlertDailySummary     AlertType = "daily_summary"
	AlertLargeTransaction AlertType = "large_transaction"
	AlertPaymentReceived  AlertType = "payment_received"

	AlertEmail    AlertChannel = "email"
	AlertWhatsApp AlertChannel = "whatsapp"
	AlertSMS      AlertChannel = "sms"
)

type Alert struct {
	ID        string       `bson:"_id" json:"id"`
	UserID    string       `bson:"user_id" json:"user_id"`
	Type      AlertType    `bson:"type" json:"type"`
	Channel   AlertChannel `bson:"channel" json:"channel"`
	Message   string       `bson:"message" json:"message"`
	Status    string       `bson:"status" json:"status"`
	SentAt    *time.Time   `bson:"sent_at,omitempty" json:"sent_at"`
	CreatedAt time.Time    `bson:"created_at" json:"created_at"`
}

type AlertPreference struct {
	ID        string       `bson:"_id" json:"id"`
	UserID    string       `bson:"user_id" json:"user_id"`
	AlertType AlertType    `bson:"alert_type" json:"alert_type"`
	Channel   AlertChannel `bson:"channel" json:"channel"`
	Enabled   bool         `bson:"enabled" json:"enabled"`
}

// --- Request/Response DTOs ---

type LoginRequest struct {
	Email    string `json:"email" form:"email" binding:"required,email"`
	Password string `json:"password" form:"password" binding:"required,min=6"`
}

type RegisterRequest struct {
	Name     string `json:"name" form:"name" binding:"required,min=2"`
	Email    string `json:"email" form:"email" binding:"required,email"`
	Password string `json:"password" form:"password" binding:"required,min=8"`
	Role     Role   `json:"role" form:"role"`
}

type CreateProductRequest struct {
	Name          string  `json:"name" form:"name" binding:"required"`
	Description   string  `json:"description" form:"description"`
	CategoryID    *string `json:"category_id" form:"category_id"`
	SKU           string  `json:"sku" form:"sku"`
	BuyingPrice   float64 `json:"buying_price" form:"buying_price" binding:"required,min=0"`
	SellingPrice  float64 `json:"selling_price" form:"selling_price" binding:"required,min=0"`
	Quantity      int     `json:"quantity" form:"quantity" binding:"min=0"`
	ReorderLevel  int     `json:"reorder_level" form:"reorder_level"`
	SupplierName  string  `json:"supplier_name" form:"supplier_name"`
	SupplierPhone string  `json:"supplier_phone" form:"supplier_phone"`
}

type UpdateProductRequest struct {
	Name          string  `json:"name" form:"name"`
	Description   string  `json:"description" form:"description"`
	CategoryID    *string `json:"category_id" form:"category_id"`
	BuyingPrice   float64 `json:"buying_price" form:"buying_price"`
	SellingPrice  float64 `json:"selling_price" form:"selling_price"`
	Quantity      int     `json:"quantity" form:"quantity"`
	ReorderLevel  int     `json:"reorder_level" form:"reorder_level"`
	SupplierName  string  `json:"supplier_name" form:"supplier_name"`
	SupplierPhone string  `json:"supplier_phone" form:"supplier_phone"`
}

type StockMovementRequest struct {
	ProductID string            `json:"product_id" form:"product_id" binding:"required"`
	Type      StockMovementType `json:"type" form:"type" binding:"required"`
	Quantity  int               `json:"quantity" form:"quantity" binding:"required,min=1"`
	Reference string            `json:"reference" form:"reference"`
	Notes     string            `json:"notes" form:"notes"`
}

type CreateSaleRequest struct {
	Items         []SaleItemRequest `json:"items" binding:"required,min=1"`
	DiscountType  string            `json:"discount_type" form:"discount_type"`
	DiscountValue float64           `json:"discount_value" form:"discount_value"`
	PaymentMethod PaymentMethod     `json:"payment_method" form:"payment_method" binding:"required"`
	CustomerName  string            `json:"customer_name" form:"customer_name"`
	CustomerPhone string            `json:"customer_phone" form:"customer_phone"`
	Notes         string            `json:"notes" form:"notes"`
}

type SaleItemRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

type DateRangeFilter struct {
	From     time.Time `form:"from"`
	To       time.Time `form:"to"`
	Category string    `form:"category"`
	Product  string    `form:"product"`
	Page     int       `form:"page"`
	Limit    int       `form:"limit"`
}

// --- Dashboard / Reports ---

type DashboardStats struct {
	TotalRevenue  float64 `json:"total_revenue"`
	TotalCOGS     float64 `json:"total_cogs"`
	GrossProfit   float64 `json:"gross_profit"`
	GrossMargin   float64 `json:"gross_margin"`
	TotalSales    int     `json:"total_sales"`
	LowStockCount int     `json:"low_stock_count"`
	TodayRevenue  float64 `json:"today_revenue"`
	TodaySales    int     `json:"today_sales"`
}

type PLSummary struct {
	Period      string  `json:"period"`
	Revenue     float64 `json:"revenue"`
	COGS        float64 `json:"cogs"`
	GrossProfit float64 `json:"gross_profit"`
	Margin      float64 `json:"margin"`
	SaleCount   int     `json:"sale_count"`
}

type TopProduct struct {
	ProductID    string  `json:"product_id"`
	ProductName  string  `json:"product_name"`
	SKU          string  `json:"sku"`
	TotalQty     int     `json:"total_qty"`
	TotalRevenue float64 `json:"total_revenue"`
	Margin       float64 `json:"margin"`
}

// Claims for JWT
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   Role   `json:"role"`
	Name   string `json:"name"`
}
