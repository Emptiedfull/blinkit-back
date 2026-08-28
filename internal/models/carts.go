package models

import (
	"cc/internal/db"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Item struct {
	ID          uuid.UUID `db:"id"`
	SellerID    string    `db:"seller_id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	Price       float64   `db:"price"`
	Category    string    `db:"category"`
	Stock       int       `db:"stock"`
	Unit        string    `db:"unit"`
	ImageURL    string    `db:"image_url"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type CartItem struct {
	ItemID   string  `db:"item_id"`
	Name     string  `db:"name"`
	Price    float64 `db:"price"`
	Unit     string  `db:"unit"`
	Quantity int     `db:"quantity"`
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
		`SELECT id, seller_id, name, description, price, category, stock, unit, image_url, created_at, updated_at
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
		`SELECT id, seller_id, name, description, price, category, stock, unit, image_url, created_at, updated_at
			 FROM items WHERE id=$1`, id)
	if err != nil {

		return Item{}, err
	}
	item, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Item])
	return item, err
}

func createCart(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO carts (user_id) VALUES ($1)`, userID)
	return err
}

func AddCartItem(ctx context.Context, pool *pgxpool.Pool, userId, itemId uuid.UUID, amount int) error {

	return db.DoTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		var stock int
		if err := tx.QueryRow(ctx, `SELECT stock FROM items WHERE id=$1 FOR UPDATE`, itemId).Scan(&stock); err != nil {
			return err
		}

		var existingQty int
		err := tx.QueryRow(ctx,
			`SELECT ci.quantity FROM cart_items ci JOIN carts c ON c.id = ci.cart_id
				 WHERE c.user_id=$1 AND ci.item_id=$2`, userId, itemId,
		).Scan(&existingQty)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if existingQty+amount > stock {
			return errors.New("stock exceeded")
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO cart_items (cart_id, item_id, quantity)
				 SELECT c.id, $1, $2 FROM carts c WHERE c.user_id=$3
				 ON CONFLICT (cart_id, item_id) DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity, updated_at = now()`,
			itemId, amount, userId)
		if err != nil {
			return errors.New("unable to add cart item")
		}
		return nil
	})

}

func UpdateCartItem(ctx context.Context, pool *pgxpool.Pool, userId, itemId uuid.UUID, Quantity int) error {
	return db.DoTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		var stock int
		if err := tx.QueryRow(ctx, `SELECT stock FROM items WHERE id=$1 FOR UPDATE`, itemId).Scan(&stock); err != nil {
			return err
		}
		if Quantity > stock {
			return errors.New("stock exceeded")
		}

		tag, err := tx.Exec(ctx,
			`UPDATE cart_items SET quantity=$1, updated_at=now()
				 WHERE item_id=$2 AND cart_id=(SELECT id FROM carts WHERE user_id=$3)`,
			Quantity, itemId, userId)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("no cart item")
		}
		return nil
	})
}

func ViewCart(ctx context.Context, pool *pgxpool.Pool, userId uuid.UUID) ([]CartItem, error) {
	rows, err := pool.Query(ctx,
		`SELECT i.id AS item_id, i.name, i.price, i.unit, ci.quantity
			 FROM cart_items ci
			 JOIN carts c ON c.id = ci.cart_id
			 JOIN items i ON i.id = ci.item_id
			 WHERE c.user_id=$1`, userId)
	if err != nil {

		return []CartItem{}, err
	}
	lines, err := pgx.CollectRows(rows, pgx.RowToStructByName[CartItem])
	if err != nil {
		return []CartItem{}, err
	}

	return lines, nil
}

func ClearCart(ctx context.Context, pool *pgxpool.Pool, userId uuid.UUID) error {
	_, err := pool.Exec(ctx,
		`DELETE FROM cart_items WHERE cart_id=(SELECT id FROM carts WHERE user_id=$1)`, userId)
	return err
}
