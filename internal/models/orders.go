package models

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	OrderID uuid.UUID `db:"id"`
	UserID  uuid.UUID `db:"user_id"`
	ItemID  uuid.UUID `db:"item_id"`

	created_at time.Time `db:"created_at"`
}
