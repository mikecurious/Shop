package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/michaelbrian/kiosk/internal/models"
)

type ProductRepository struct {
	db *DB
}

func NewProductRepository(db *DB) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) Create(ctx context.Context, p *models.Product) error {
	query := `
		INSERT INTO products (id, name, description, category_id, sku, barcode, buying_price,
		    selling_price, quantity, reorder_level, supplier_name, supplier_phone, image_url,
		    is_active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	p.IsActive = true

	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.Name, p.Description, p.CategoryID, p.SKU, p.Barcode,
		p.BuyingPrice, p.SellingPrice, p.Quantity, p.ReorderLevel,
		p.SupplierName, p.SupplierPhone, p.ImageURL,
		p.IsActive, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	query := `
		SELECT p.id, p.name, p.description, p.category_id, COALESCE(c.name,'') as category_name,
		       p.sku, p.barcode, p.buying_price, p.selling_price, p.quantity, p.reorder_level,
		       p.supplier_name, p.supplier_phone, p.image_url, p.is_active, p.created_at, p.updated_at
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanProduct(row)
}

func (r *ProductRepository) GetBySKU(ctx context.Context, sku string) (*models.Product, error) {
	query := `
		SELECT p.id, p.name, p.description, p.category_id, COALESCE(c.name,'') as category_name,
		       p.sku, p.barcode, p.buying_price, p.selling_price, p.quantity, p.reorder_level,
		       p.supplier_name, p.supplier_phone, p.image_url, p.is_active, p.created_at, p.updated_at
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.sku = $1
	`
	row := r.db.QueryRowContext(ctx, query, sku)
	return scanProduct(row)
}

func (r *ProductRepository) GetByBarcode(ctx context.Context, barcode string) (*models.Product, error) {
	query := `
		SELECT p.id, p.name, p.description, p.category_id, COALESCE(c.name,'') as category_name,
		       p.sku, p.barcode, p.buying_price, p.selling_price, p.quantity, p.reorder_level,
		       p.supplier_name, p.supplier_phone, p.image_url, p.is_active, p.created_at, p.updated_at
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.barcode = $1
	`
	row := r.db.QueryRowContext(ctx, query, barcode)
	return scanProduct(row)
}

type ProductFilter struct {
	Search     string
	CategoryID *uuid.UUID
	LowStock   bool
	IsActive   *bool
	Page       int
	Limit      int
}

func (r *ProductRepository) List(ctx context.Context, f ProductFilter) ([]*models.Product, int, error) {
	if f.Limit == 0 {
		f.Limit = 20
	}
	if f.Page == 0 {
		f.Page = 1
	}

	args := []any{}
	argIdx := 1
	conditions := []string{}

	if f.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(p.name ILIKE $%d OR p.sku ILIKE $%d OR p.barcode ILIKE $%d)",
			argIdx, argIdx+1, argIdx+2))
		term := "%" + f.Search + "%"
		args = append(args, term, term, term)
		argIdx += 3
	}

	if f.CategoryID != nil {
		conditions = append(conditions, fmt.Sprintf("p.category_id = $%d", argIdx))
		args = append(args, *f.CategoryID)
		argIdx++
	}

	if f.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("p.is_active = $%d", argIdx))
		args = append(args, *f.IsActive)
		argIdx++
	}

	if f.LowStock {
		conditions = append(conditions, "p.quantity <= p.reorder_level")
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM products p %s`, where)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	listQuery := fmt.Sprintf(`
		SELECT p.id, p.name, p.description, p.category_id, COALESCE(c.name,'') as category_name,
		       p.sku, p.barcode, p.buying_price, p.selling_price, p.quantity, p.reorder_level,
		       p.supplier_name, p.supplier_phone, p.image_url, p.is_active, p.created_at, p.updated_at
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		%s
		ORDER BY p.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	var products []*models.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	return products, total, rows.Err()
}

func (r *ProductRepository) Update(ctx context.Context, p *models.Product) error {
	query := `
		UPDATE products SET name=$1, description=$2, category_id=$3, buying_price=$4,
		    selling_price=$5, quantity=$6, reorder_level=$7, supplier_name=$8,
		    supplier_phone=$9, image_url=$10, is_active=$11, updated_at=NOW()
		WHERE id = $12
	`
	_, err := r.db.ExecContext(ctx, query,
		p.Name, p.Description, p.CategoryID, p.BuyingPrice,
		p.SellingPrice, p.Quantity, p.ReorderLevel,
		p.SupplierName, p.SupplierPhone, p.ImageURL, p.IsActive, p.ID,
	)
	return err
}

func (r *ProductRepository) UpdateQuantity(ctx context.Context, id uuid.UUID, delta int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE products SET quantity = quantity + $1, updated_at = NOW() WHERE id = $2`,
		delta, id)
	return err
}

func (r *ProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE products SET is_active = FALSE, updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *ProductRepository) SKUExists(ctx context.Context, sku string, excludeID *uuid.UUID) (bool, error) {
	var count int
	var err error
	if excludeID != nil {
		err = r.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM products WHERE sku = $1 AND id != $2`, sku, *excludeID).Scan(&count)
	} else {
		err = r.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM products WHERE sku = $1`, sku).Scan(&count)
	}
	return count > 0, err
}

func (r *ProductRepository) BarcodeExists(ctx context.Context, barcode string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM products WHERE barcode = $1`, barcode).Scan(&count)
	return count > 0, err
}

func (r *ProductRepository) GetLowStock(ctx context.Context) ([]*models.Product, error) {
	query := `
		SELECT p.id, p.name, p.description, p.category_id, COALESCE(c.name,'') as category_name,
		       p.sku, p.barcode, p.buying_price, p.selling_price, p.quantity, p.reorder_level,
		       p.supplier_name, p.supplier_phone, p.image_url, p.is_active, p.created_at, p.updated_at
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.quantity <= p.reorder_level AND p.is_active = TRUE
		ORDER BY p.quantity ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*models.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func scanProduct(s scanner) (*models.Product, error) {
	p := &models.Product{}
	var supplierName, supplierPhone, imageURL sql.NullString
	err := s.Scan(
		&p.ID, &p.Name, &p.Description, &p.CategoryID, &p.CategoryName,
		&p.SKU, &p.Barcode, &p.BuyingPrice, &p.SellingPrice,
		&p.Quantity, &p.ReorderLevel,
		&supplierName, &supplierPhone, &imageURL,
		&p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan product: %w", err)
	}
	p.SupplierName = supplierName.String
	p.SupplierPhone = supplierPhone.String
	p.ImageURL = imageURL.String
	return p, nil
}
