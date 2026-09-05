package handlers

import (
	"cc/internal/auth"
	"cc/internal/httpx"
	"cc/internal/models"
	"net/http"
)

func (h *ResHandler) SellerInventory(w http.ResponseWriter, r *http.Request) {
	sellerId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "no user found")
		return
	}

	items, err := models.GetSellerInventory(r.Context(), h.pool, sellerId)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *ResHandler) SellerOrders(w http.ResponseWriter, r *http.Request) {
	sellerId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "no user found")
		return
	}

	orders, err := models.GetSellerOrders(r.Context(), h.pool, sellerId)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, orders)
}
