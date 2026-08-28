package handlers

import (
	"cc/internal/auth"
	"cc/internal/httpx"
	"cc/internal/models"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

type TopUpRequest struct {
	ID     uuid.UUID `json:"id"`
	Amount int       `json:"amount"`
}

func (h *ResHandler) GetWallet(w http.ResponseWriter, r *http.Request) {

	userID, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "no user found")
		return
	}

	wallet, err := models.GetWallet(r.Context(), h.pool, userID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	}

	httpx.WriteJSON(w, http.StatusOK, wallet)

}

func (h *ResHandler) TopUpWallet(w http.ResponseWriter, r *http.Request) {
	var req TopUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid topup")
		return
	}

	if req.Amount <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, " Topup amount must be positive")
		return
	}

	err := models.TopUpWallet(r.Context(), h.pool, req.ID, float64(req.Amount))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

}
