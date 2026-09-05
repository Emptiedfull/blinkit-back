package handlers

import (
	"cc/internal/auth"
	"cc/internal/httpx"
	"cc/internal/models"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

type rateItemRequest struct {
	Rating     int    `json:"rating"`
	ReviewText string `json:"review_text"`
}

func (h *ResHandler) RateItem(w http.ResponseWriter, r *http.Request) {
	userId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "no user found")
		return
	}

	itemId, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	var req rateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Rating < 1 || req.Rating > 5 {
		httpx.WriteError(w, http.StatusBadRequest, "rating must be between 1 and 5")
		return
	}

	err = models.RateItem(r.Context(), h.pool, userId, itemId, req.Rating, req.ReviewText)
	if err != nil {
		if errors.Is(err, models.ErrNotPurchased) {
			httpx.WriteError(w, http.StatusForbidden, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, "")
}
