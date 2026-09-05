package handlers

import (
	"cc/internal/auth"
	"cc/internal/httpx"
	"cc/internal/models"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

type createItemRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Category    string  `json:"category"`
	Stock       int     `json:"stock"`
	Unit        string  `json:"unit"`
	ImageURL    string  `json:"image_url"`
}

type addCartItemRequest struct {
	ItemID   uuid.UUID `json:"item_id"`
	Quantity int       `json:"quantity"`
}

func (h *ResHandler) RemoveCartItem(w http.ResponseWriter, r *http.Request) {
	userId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "seller dosent exist")
		return
	}

	itemId, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "item not found")
		return
	}

	err = models.RemoveCartItem(r.Context(), h.pool, userId, itemId)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Unable to remove cart item: %v", err))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ResHandler) UpdateCartItem(w http.ResponseWriter, r *http.Request) {
	userId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "user not found")
		return
	}

	itemId, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "item not found")
		return
	}

	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Quantity <= 0 {
		http.Error(w, "positive quantity required", http.StatusBadRequest)
		return
	}

	err = models.UpdateCartItem(r.Context(), h.pool, userId, itemId, body.Quantity)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Unable to update cart: %v", err))
		return
	}
	w.WriteHeader(http.StatusOK)

}

func (h *ResHandler) ViewCart(w http.ResponseWriter, r *http.Request) {
	userId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "seller dosent exist")
		return
	}

	CartItems, err := models.ViewCart(r.Context(), h.pool, userId)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, fmt.Sprintf("unable to get cart: %v", err))
		return
	}

	httpx.WriteJSON(w, http.StatusOK, CartItems)
}

func (h *ResHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	userId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "seller dosent exist")
		return
	}

	orderID, err := models.CheckOut(r.Context(), h.pool, userId)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"order_id": orderID})

}

func (h *ResHandler) ClearCart(w http.ResponseWriter, r *http.Request) {
	userId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "seller dosent exist")
		return
	}

	err := models.ClearCart(r.Context(), h.pool, userId)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, fmt.Sprintf("unable to clear cart: %v", err))
		return
	}

	w.WriteHeader(http.StatusOK)
}
