package models

import (
	"cc/internal/db"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CartItem struct {
	ItemID   uuid.UUID `db:"item_id"`
	SellerID uuid.UUID `db:"seller_id"`
	Name     string    `db:"name"`
	Price    float64   `db:"price"`
	Unit     string    `db:"unit"`
	Quantity int       `db:"quantity"`
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

func RemoveCartItem(ctx context.Context, pool *pgxpool.Pool, userId, itemId uuid.UUID) error {
	tag, err := pool.Exec(ctx,
		`DELETE FROM cart_items WHERE item_id=$1 AND cart_id=(SELECT id FROM carts WHERE user_id=$2)`,
		itemId, userId)
	if err != nil {

		return err
	}
	if tag.RowsAffected() == 0 {

		return errors.New("No cart item found")
	}
	return nil
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

func CheckOut(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) (uuid.UUID, error) {
	var orderID uuid.UUID

	err := db.DoTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT i.id AS item_id, i.seller_id, i.price, ci.quantity
			 FROM cart_items ci
			 JOIN carts c ON c.id = ci.cart_id
			 JOIN items i ON i.id = ci.item_id
			 WHERE c.user_id = $1`,
			userID)

		if err != nil {
			return err
		}

		items, err := pgx.CollectRows(rows, pgx.RowToStructByName[CartItem])
		if err != nil {
			return err
		}

		if len(items) == 0 {
			return errors.New("Cart empty")
		}

		if err := tx.QueryRow(ctx,
			`INSERT INTO orders (user_id) VALUES ($1) RETURNING id`, userID,
		).Scan(&orderID); err != nil {
			return err
		}

		var total float64
		for _, item := range items {
			var stock int
			if err := tx.QueryRow(ctx, `SELECT stock FROM items WHERE id=$1 FOR UPDATE`, item.ItemID).Scan(&stock); err != nil {
				return err
			}

			if stock < item.Quantity {
				return errors.New("Out of stock")
			}

			_, err := tx.Exec(ctx,
				`UPDATE items SET stock = stock - $1, updated_at = now() WHERE id=$2`,
				item.Quantity, item.ItemID,
			)
			if err != nil {
				return err
			}

			if _, err := tx.Exec(ctx,
				`INSERT INTO order_items (order_id, item_id, seller_id, quantity, price_at_purchase)
				 VALUES ($1,$2,$3,$4,$5)`,
				orderID, item.ItemID, item.SellerID, item.Quantity, item.Price,
			); err != nil {
				return err
			}

			total += item.Price * float64(item.Quantity)

		}
		if err := DebitWallet(ctx, tx, userID, total); err != nil {
			return err
		}

		_, err = tx.Exec(ctx,
			`DELETE FROM cart_items WHERE cart_id=(SELECT id FROM carts WHERE user_id=$1)`, userID)
		return err

	})

	return orderID, err
}

func ClearCart(ctx context.Context, pool *pgxpool.Pool, userId uuid.UUID) error {
	_, err := pool.Exec(ctx,
		`DELETE FROM cart_items WHERE cart_id=(SELECT id FROM carts WHERE user_id=$1)`, userId)
	return err
}
