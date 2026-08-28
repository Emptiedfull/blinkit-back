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
	Quantity int       `json:"quantitiy"`
}

func (h *ResHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
	sellerId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "seller dosent exist")
	}

	var req createItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Description == "" || req.Price == 0 || req.Stock == 0 || req.Unit == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	item, err := models.CreateItem(r.Context(), h.pool, req.Name, req.Description, sellerId, req.Price, req.Category, req.Stock, req.Unit, req.ImageURL)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("unable to create item: %v", err))
		return
	}

	httpx.WriteJSON(w, http.StatusOK, item)

}

func (h *ResHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	items, err := models.ListItems(r.Context(), h.pool)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *ResHandler) GetItem(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "invalid item ID")
		return
	}
	item, err := models.GetItemByID(r.Context(), h.pool, id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "item not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, item)
}

func (h *ResHandler) AddCartItem(w http.ResponseWriter, r *http.Request) {
	userId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "seller dosent exist")
		return
	}

	var req addCartItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ItemID.String() == "" || req.Quantity <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	err := models.AddCartItem(r.Context(), h.pool, userId, req.ItemID, req.Quantity)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
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
