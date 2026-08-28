package models

import (
	"cc/internal/db"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Wallet struct {
	ID        uuid.UUID `db:"id"`
	UserID    uuid.UUID `db:"user_id"`
	Balance   float64   `db:"balance"`
	Currency  string    `db:"currency"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func createWallet(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO wallets (user_id) VALUES ($1)`, userID)
	return err
}

func GetWallet(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (Wallet, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, user_id, balance, currency, created_at, updated_at FROM wallets WHERE user_id=$1`, id)
	if err != nil {
		return Wallet{}, err
	}

	wallet, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Wallet])
	return wallet, err

}

func TopUpWallet(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, amount float64) error {

	err := db.DoTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		var balance float64
		if err := tx.QueryRow(ctx, `SELECT balance FROM wallets WHERE user_id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
			return err
		}
		newBalance := balance + amount

		if _, err := tx.Exec(ctx, `UPDATE wallets SET balance=$1, updated_at=now() WHERE user_id=$2`, newBalance, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO wallet_transactions (wallet_id, amount, type)
				 SELECT id, $1, 'topup' FROM wallets WHERE user_id=$2`, amount, userID)
		return err
	})
	if err != nil {
		return err
	}
	return nil
}
