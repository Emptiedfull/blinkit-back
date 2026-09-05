package models

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Rating struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	ItemID    uuid.UUID `json:"item_id"`
	Rating    int       `json:"rating"`
	Review    string    `json:"review"`
	CreatedAt time.Time `json:"created_at"`
	UpdateAt  time.Time `json:"updated_at"`
}

var ErrNotPurchased = errors.New("Item not purchased")

func RateItem(ctx context.Context, pool *pgxpool.Pool, userID, itemID uuid.UUID, rating int, review string) error {
	var purchased bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM order_items oi
		   JOIN orders o ON o.id = oi.order_id
		   WHERE o.user_id = $1 AND oi.item_id = $2
		 )`,
		userID, itemID,
	).Scan(&purchased); err != nil {
		return err
	}

	if !purchased {
		return errors.New("Item not purchased")
	}

	_, err := pool.Query(ctx,
		`INSERT INTO ratings (user_id, item_id, rating, review_text)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (user_id, item_id)
		 DO UPDATE SET rating = EXCLUDED.rating, review_text = EXCLUDED.review_text, updated_at = now()
		`,
		userID, itemID, rating, review,
	)
	if err != nil {
		return err
	}
	return nil
}
