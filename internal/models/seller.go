package models

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const lowStockNum = 5

type ItemSummary struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	Price     float64   `db:"price"`
	Stock     int       `db:"stock"`
	LowStock  bool      `db:"low_stock"`
	UnitsSold int       `db:"units_sold"`
	Revenue   float64   `db:"revenue"`
}

func GetSellerInventory(ctx context.Context, pool *pgxpool.Pool, sellerID uuid.UUID) ([]ItemSummary, error) {
	rows, err := pool.Query(ctx,
		`SELECT i.id, i.name, i.price, i.stock,
		        (i.stock < $2) AS low_stock,
		        COALESCE(SUM(oi.quantity), 0) AS units_sold,
		        COALESCE(SUM(oi.quantity * oi.price_at_purchase), 0) AS revenue
		 FROM items i
		 LEFT JOIN order_items oi ON oi.item_id = i.id
		 WHERE i.seller_id = $1
		 GROUP BY i.id
		 ORDER BY i.created_at DESC`,
		sellerID, lowStockNum,
	)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[ItemSummary])

}

type SellerOrderItem struct {
	ID        uuid.UUID `db:"id"`
	ItemID    uuid.UUID `db:"item_id"`
	BuyerID   uuid.UUID `db:"buyer_id"`
	ItemName  string    `db:"item_name"`
	Quantity  int       `db:"quantity"`
	PriceATM  float64   `db:"price_atm"`
	CreatedAt time.Time `db:"created_at"`
}

func GetSellerOrders(ctx context.Context, pool *pgxpool.Pool, sellerID uuid.UUID) ([]SellerOrderItem, error) {
	rows, err := pool.Query(ctx,
		`SELECT oi.order_id, oi.item_id, i.name AS item_name, o.user_id AS buyer_id,
		        oi.quantity, oi.price_at_purchase, oi.created_at
		 FROM order_items oi
		 JOIN orders o ON o.id = oi.order_id
		 JOIN items i ON i.id = oi.item_id
		 WHERE oi.seller_id = $1
		 ORDER BY oi.created_at DESC`,
		sellerID,
	)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[SellerOrderItem])
}
