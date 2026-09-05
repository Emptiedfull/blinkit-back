package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Item struct {
	ID            uuid.UUID `db:"id"`
	SellerID      uuid.UUID `db:"seller_id"`
	Name          string    `db:"name"`
	Description   string    `db:"description"`
	Price         float64   `db:"price"`
	Category      string    `db:"category"`
	Stock         int       `db:"stock"`
	Unit          string    `db:"unit"`
	ImageURL      string    `db:"image_url"`
	AverageRating float64   `db:"average_rating"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type SortType string

const (
	price_asc  SortType = "price_asc"
	price_desc SortType = "price_desc"
	rating     SortType = "rating"
	age        SortType = "age"
)

type ItemFilter struct {
	Query    string
	Category string
	MinPrice *float64
	MaxPrice *float64
	InStock  bool
	Sort     SortType
}

func FilterItems(ctx context.Context, pool *pgxpool.Pool, filter ItemFilter) ([]Item, error) {

	base := `
		SELECT i.id, i.seller_id, i.name, i.description, i.price, i.category, i.stock, i.unit, i.image_url, i.created_at, i.updated_at,
		       COALESCE(AVG(r.rating), 0) AS average_rating
		FROM items i
		LEFT JOIN ratings r ON r.item_id = i.id
		WHERE 1=1
	`

	var args []any
	argN := 1

	if filter.Query != "" {
		base += fmt.Sprintf(" AND (i.name ILIKE $%d OR i.name %% $%d)", argN, argN+1)
		args = append(args, "%"+filter.Query+"%", filter.Query)
		argN += 2
	}

	if filter.Category != "" {
		base += fmt.Sprintf(" AND i.category = $%d", argN)
		args = append(args, filter.Category)
		argN++
	}

	if filter.MinPrice != nil {
		base += fmt.Sprintf(" AND i.price >= $%d", argN)
		args = append(args, *filter.MinPrice)
		argN++
	}

	if filter.MaxPrice != nil {
		base += fmt.Sprintf(" AND i.price <= $%d", argN)
		args = append(args, *filter.MaxPrice)
		argN++
	}
	if filter.InStock {
		base += " AND i.stock > 0"
	}

	base += " GROUP BY i.id"

	switch filter.Sort {
	case price_asc:
		base += " ORDER BY i.price ASC"
	case price_desc:
		base += " ORDER BY i.price DESC"
	case rating:
		base += " ORDER BY average_rating DESC"

	}

	rows, err := pool.Query(ctx, base, args...)
	if err != nil {
		return []Item{}, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[Item])

}

func CreateItem(ctx context.Context, pool *pgxpool.Pool, Name string, Description string, SellerID uuid.UUID, Price float64, Category string, Stock int, Unit string, ImageURL string) (Item, error) {
	rows, err := pool.Query(ctx,
		`INSERT INTO items (seller_id, name, description, price, category, stock, unit, image_url)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			 RETURNING id, seller_id, name, description, price, category, stock, unit, image_url, created_at, updated_at`,
		SellerID, Name, Description, Price, Category, Stock, Unit, ImageURL,
	)
	if err != nil {
		return Item{}, err
	}

	item, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Item])
	if err != nil {

		return Item{}, err
	}

	return item, nil
}

func ListItems(ctx context.Context, pool *pgxpool.Pool) ([]Item, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, seller_id, name, description, price, category, stock, unit, image_url, created_at, updated_at, COALESCE(AVG(r.rating), 0) AS average_rating
			 FROM items ORDER BY created_at DESC`)
	if err != nil {

		return []Item{}, nil
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Item])
	if err != nil {

		return []Item{}, nil
	}
	return items, nil
}

func GetItemByID(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (Item, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, seller_id, name, description, price, category, stock, unit, image_url, created_at, updated_at, COALESCE(AVG(r.rating), 0) AS average_rating
			 FROM items WHERE id=$1`, id)
	if err != nil {

		return Item{}, err
	}
	item, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Item])
	return item, err
}

func UpdateItem(ctx context.Context, pool *pgxpool.Pool, id, sellerID uuid.UUID, name, description string, price float64, category string, stock int, unit, imageURL string) (Item, error) {
	rows, err := pool.Query(ctx,
		`UPDATE items SET
		   name=$1, description=$2, price=$3, category=$4, stock=$5, unit=$6, image_url=$7, updated_at=now()
		 WHERE id=$8 AND seller_id=$9
		 RETURNING id, seller_id, name, description, price, category, stock, unit, image_url, created_at, updated_at`,
		name, description, price, category, stock, unit, imageURL, id, sellerID,
	)
	if err != nil {
		return Item{}, err
	}
	item, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Item])
	if err != nil {
		return Item{}, errors.New("item not found")
	}
	return item, nil
}

func DeleteItem(ctx context.Context, pool *pgxpool.Pool, id, sellerId uuid.UUID) error {
	tag, err := pool.Exec(ctx, `DELETE FROM items WHERE id=$1 AND seller_id=$2`, id, sellerId)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("item not found")
	}
	return nil
}
