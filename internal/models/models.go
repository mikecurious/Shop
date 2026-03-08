package models

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleStaff Role = "staff"
)

type User struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Email        string     `json:"email" db:"email"`
	PasswordHash string     `json:"-" db:"password_hash"`
	Name         string     `json:"name" db:"name"`
	Role         Role       `json:"role" db:"role"`
	IsActive     bool       `json:"is_active" db:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at" db:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

type Category struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type Product struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Name         string     `json:"name" db:"name"`
	Description  string     `json:"description" db:"description"`
	CategoryID   *uuid.UUID `json:"category_id" db:"category_id"`
	CategoryName string     `json:"category_name" db:"category_name"`
	SKU          string     `json:"sku" db:"sku"`
	Barcode      string     `json:"barcode" db:"barcode"`
	BuyingPrice  float64    `json:"buying_price" db:"buying_price"`
	SellingPrice float64    `json:"selling_price" db:"selling_price"`
	Quantity     int        `json:"quantity" db:"quantity"`
	ReorderLevel int        `json:"reorder_level" db:"reorder_level"`
	SupplierName string     `json:"supplier_name" db:"supplier_name"`
	SupplierPhone string    `json:"supplier_phone" db:"supplier_phone"`
	ImageURL     string     `json:"image_url" db:"image_url"`
	IsActive     bool       `json:"is_active" db:"is_active"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
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
	ID          uuid.UUID         `json:"id" db:"id"`
	ProductID   uuid.UUID         `json:"product_id" db:"product_id"`
	ProductName string            `json:"product_name" db:"product_name"`
	Type        StockMovementType `json:"type" db:"type"`
	Quantity    int               `json:"quantity" db:"quantity"`
	Reference   string            `json:"reference" db:"reference"`
	Notes       string            `json:"notes" db:"notes"`
	CreatedBy   uuid.UUID         `json:"created_by" db:"created_by"`
	CreatedByName string          `json:"created_by_name" db:"created_by_name"`
	CreatedAt   time.Time         `json:"created_at" db:"created_at"`
}

type PaymentMethod string

const (
	PaymentCash    PaymentMethod = "cash"
	PaymentMPesa   PaymentMethod = "mpesa"
	PaymentCard    PaymentMethod = "card"
	PaymentCredit  PaymentMethod = "credit"
)

type Sale struct {
	ID               uuid.UUID     `json:"id" db:"id"`
	TotalAmount      float64       `json:"total_amount" db:"total_amount"`
	DiscountType     string        `json:"discount_type" db:"discount_type"`
	DiscountValue    float64       `json:"discount_value" db:"discount_value"`
	DiscountAmount   float64       `json:"discount_amount" db:"discount_amount"`
	NetAmount        float64       `json:"net_amount" db:"net_amount"`
	PaymentMethod    PaymentMethod `json:"payment_method" db:"payment_method"`
	PaymentReference string        `json:"payment_reference" db:"payment_reference"`
	CustomerName     string        `json:"customer_name" db:"customer_name"`
	CustomerPhone    string        `json:"customer_phone" db:"customer_phone"`
	Notes            string        `json:"notes" db:"notes"`
	CreatedBy        uuid.UUID     `json:"created_by" db:"created_by"`
	CreatedByName    string        `json:"created_by_name" db:"created_by_name"`
	CreatedAt        time.Time     `json:"created_at" db:"created_at"`
	Items            []SaleItem    `json:"items"`
}

type SaleItem struct {
	ID          uuid.UUID `json:"id" db:"id"`
	SaleID      uuid.UUID `json:"sale_id" db:"sale_id"`
	ProductID   uuid.UUID `json:"product_id" db:"product_id"`
	ProductName string    `json:"product_name" db:"product_name"`
	ProductSKU  string    `json:"product_sku" db:"product_sku"`
	Quantity    int       `json:"quantity" db:"quantity"`
	UnitPrice   float64   `json:"unit_price" db:"unit_price"`
	BuyingPrice float64   `json:"buying_price" db:"buying_price"`
	Subtotal    float64   `json:"subtotal" db:"subtotal"`
}

type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentCompleted PaymentStatus = "completed"
	PaymentFailed    PaymentStatus = "failed"
	PaymentCancelled PaymentStatus = "cancelled"
)

type Payment struct {
	ID              uuid.UUID     `json:"id" db:"id"`
	SaleID          *uuid.UUID    `json:"sale_id" db:"sale_id"`
	MpesaReceipt    string        `json:"mpesa_receipt" db:"mpesa_receipt"`
	PhoneNumber     string        `json:"phone_number" db:"phone_number"`
	Amount          float64       `json:"amount" db:"amount"`
	Status          PaymentStatus `json:"status" db:"status"`
	ResultCode      string        `json:"result_code" db:"result_code"`
	ResultDesc      string        `json:"result_desc" db:"result_desc"`
	CheckoutRequestID string      `json:"checkout_request_id" db:"checkout_request_id"`
	MerchantRequestID string      `json:"merchant_request_id" db:"merchant_request_id"`
	TransactionDate time.Time     `json:"transaction_date" db:"transaction_date"`
	CreatedAt       time.Time     `json:"created_at" db:"created_at"`
}

type AlertType string
type AlertChannel string

const (
	AlertLowStock        AlertType = "low_stock"
	AlertDailySummary    AlertType = "daily_summary"
	AlertLargeTransaction AlertType = "large_transaction"
	AlertPaymentReceived AlertType = "payment_received"

	AlertEmail    AlertChannel = "email"
	AlertWhatsApp AlertChannel = "whatsapp" // retained for compatibility
	AlertSMS      AlertChannel = "sms"      // Celcom Africa
)

type Alert struct {
	ID        uuid.UUID    `json:"id" db:"id"`
	UserID    uuid.UUID    `json:"user_id" db:"user_id"`
	Type      AlertType    `json:"type" db:"type"`
	Channel   AlertChannel `json:"channel" db:"channel"`
	Message   string       `json:"message" db:"message"`
	SentAt    *time.Time   `json:"sent_at" db:"sent_at"`
	Status    string       `json:"status" db:"status"`
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
}

type AlertPreference struct {
	ID        uuid.UUID    `json:"id" db:"id"`
	UserID    uuid.UUID    `json:"user_id" db:"user_id"`
	AlertType AlertType    `json:"alert_type" db:"alert_type"`
	Channel   AlertChannel `json:"channel" db:"channel"`
	Enabled   bool         `json:"enabled" db:"enabled"`
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
	Name          string     `json:"name" form:"name" binding:"required"`
	Description   string     `json:"description" form:"description"`
	CategoryID    *uuid.UUID `json:"category_id" form:"category_id"`
	SKU           string     `json:"sku" form:"sku"`
	BuyingPrice   float64    `json:"buying_price" form:"buying_price" binding:"required,min=0"`
	SellingPrice  float64    `json:"selling_price" form:"selling_price" binding:"required,min=0"`
	Quantity      int        `json:"quantity" form:"quantity" binding:"min=0"`
	ReorderLevel  int        `json:"reorder_level" form:"reorder_level"`
	SupplierName  string     `json:"supplier_name" form:"supplier_name"`
	SupplierPhone string     `json:"supplier_phone" form:"supplier_phone"`
}

type UpdateProductRequest struct {
	Name          string     `json:"name" form:"name"`
	Description   string     `json:"description" form:"description"`
	CategoryID    *uuid.UUID `json:"category_id" form:"category_id"`
	BuyingPrice   float64    `json:"buying_price" form:"buying_price"`
	SellingPrice  float64    `json:"selling_price" form:"selling_price"`
	Quantity      int        `json:"quantity" form:"quantity"`
	ReorderLevel  int        `json:"reorder_level" form:"reorder_level"`
	SupplierName  string     `json:"supplier_name" form:"supplier_name"`
	SupplierPhone string     `json:"supplier_phone" form:"supplier_phone"`
}

type StockMovementRequest struct {
	ProductID uuid.UUID         `json:"product_id" form:"product_id" binding:"required"`
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
	ProductID uuid.UUID `json:"product_id" binding:"required"`
	Quantity  int       `json:"quantity" binding:"required,min=1"`
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
	TotalRevenue    float64 `json:"total_revenue"`
	TotalCOGS       float64 `json:"total_cogs"`
	GrossProfit     float64 `json:"gross_profit"`
	GrossMargin     float64 `json:"gross_margin"`
	TotalSales      int     `json:"total_sales"`
	LowStockCount   int     `json:"low_stock_count"`
	TodayRevenue    float64 `json:"today_revenue"`
	TodaySales      int     `json:"today_sales"`
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
	ProductID   uuid.UUID `json:"product_id"`
	ProductName string    `json:"product_name"`
	SKU         string    `json:"sku"`
	TotalQty    int       `json:"total_qty"`
	TotalRevenue float64  `json:"total_revenue"`
	Margin      float64   `json:"margin"`
}

// Claims for JWT
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   Role   `json:"role"`
	Name   string `json:"name"`
}
