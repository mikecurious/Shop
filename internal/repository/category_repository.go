package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/michaelbrian/kiosk/internal/models"
)

type CategoryRepository struct {
	db *DB
}

func NewCategoryRepository(db *DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) Create(ctx context.Context, c *models.Category) error {
	c.ID = uuid.New()
	c.CreatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO categories (id, name, description, created_at) VALUES ($1,$2,$3,$4)`,
		c.ID, c.Name, c.Description, c.CreatedAt)
	return err
}

func (r *CategoryRepository) List(ctx context.Context) ([]*models.Category, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, COALESCE(description,''), created_at FROM categories ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []*models.Category
	for rows.Next() {
		c := &models.Category{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (r *CategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, id)
	return err
}
