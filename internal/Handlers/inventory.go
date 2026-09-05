package handlers

import (
	"cc/internal/auth"
	"cc/internal/db"
	"cc/internal/httpx"
	"cc/internal/models"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

type UpdateItemRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Category    string  `json:"category"`
	Stock       int     `json:"stock"`
	Unit        string  `json:"unit"`
	ImageURL    string  `json:"image_url"`
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

func (h *ResHandler) SearchItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := models.ItemFilter{
		Query:    q.Get("q"),
		Category: q.Get("category"),
		InStock:  q.Get("in_stock") == "true",
		Sort:     models.SortType(q.Get("sort")),
	}

	if v := q.Get("min_price"); v != "" {
		if p, err := strconv.ParseFloat(v, 64); err == nil {
			filter.MinPrice = &p
		}
	}
	if v := q.Get("max_price"); v != "" {
		if p, err := strconv.ParseFloat(v, 64); err == nil {
			filter.MaxPrice = &p
		}
	}

	items, err := models.FilterItems(r.Context(), h.pool, filter)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, items)

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

func (h *ResHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	sellerId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "no user found")
		return
	}

	itemId, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	var req UpdateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Price < 0 || req.Stock < 0 || req.Unit == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	item, err := models.UpdateItem(r.Context(), h.pool, itemId, sellerId, req.Name, req.Description, req.Price, req.Category, req.Stock, req.Unit, req.ImageURL)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "item not found")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, item)

}

func (h *ResHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	sellerId, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "no user found")
		return
	}

	itemId, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	if err := models.DeleteItem(r.Context(), h.pool, itemId, sellerId); err != nil {
		if db.IsForeignKeyViolation(err) {
			httpx.WriteError(w, http.StatusConflict, "item that has existing orders")
			return
		}
		httpx.WriteError(w, http.StatusNotFound, "item not found")
		return
	}

	w.WriteHeader(http.StatusOK)
}
