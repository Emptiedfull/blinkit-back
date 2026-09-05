package models

import (
	"cc/internal/db"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	UserID     uuid.UUID `db:"id"`
	UserName   string    `db:"name"`
	Role       string    `db:"role"`
	Email      string    `db:"email"`
	HashedPass string    `db:"password_hash"`
	CreatedAt  time.Time `db:"created_at"`
}

func CreateUser(ctx context.Context, pool *pgxpool.Pool, email, hashpassword, name, role string) (User, error) {

	var u User
	err := db.DoTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`INSERT INTO users (email, password_hash, name,role) VALUES ($1,$2,$3,$4)
			 RETURNING id, email, password_hash, name, created_at`,
			email, hashpassword, name, role,
		)

		if err != nil {
			return err
		}

		u, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[User])
		if err != nil {
			return err
		}

		if err := createWallet(ctx, tx, u.UserID.String()); err != nil {
			return err
		}
		return createCart(ctx, tx, u.UserID.String())
	})

	if err != nil {
		return User{}, err
	}

	return u, nil
}

func GetUserByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (User, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, email, password_hash, name, created_at FROM users WHERE email=$1`, email,
	)
	if err != nil {
		return User{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[User])

}

func GetUserByID(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (User, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, email, password_hash, name, created_at FROM users WHERE id=$1`, id,
	)
	if err != nil {
		return User{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[User])

}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func StoreRefreshToken(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, token string, ttl time.Duration) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, hashToken(token), time.Now().Add(ttl),
	)
	return err
}

func RevokeRefreshToken(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, token string) error {
	tag, err := pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now()
		 WHERE token_hash = $1 AND user_id = $2 AND revoked_at IS NULL AND expires_at > now()`,
		hashToken(token), userID,
	)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("no user found")
	}
	return nil
}
