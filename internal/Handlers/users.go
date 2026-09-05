package handlers

import (
	"cc/internal/auth"
	"cc/internal/db"
	"cc/internal/httpx"
	"cc/internal/models"
	"encoding/json"
	"fmt"
	"net/http"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SignUpRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

type LogOutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *ResHandler) HandleSignup(w http.ResponseWriter, r *http.Request) {
	var req SignUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" || req.Name == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	passwordHash, err := auth.HashPass(req.Password)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Unable to process password")
		return
	}

	if req.Role != "buyer" && req.Role != "seller" {
		httpx.WriteError(w, http.StatusBadRequest, "role must be 'buyer' or 'seller'")
		return
	}

	user, err := models.CreateUser(r.Context(), h.pool, req.Email, passwordHash, req.Name, req.Role)
	if err != nil {
		if db.IsUniqueViolation(err) {
			httpx.WriteError(w, http.StatusConflict, "user already exists")
			return
		}

		httpx.WriteError(w, http.StatusInternalServerError, "Unable to create user")
		return
	}

	h.issueTokenAndRespond(w, r, user)
}

func (h *ResHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "email and password required")
		return
	}

	user, err := models.GetUserByEmail(r.Context(), h.pool, req.Email)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "user not found")
		return
	}

	err = auth.ValPass(user.HashedPass, req.Password)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	h.issueTokenAndRespond(w, r, user)
}

func (h *ResHandler) Logout(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.UserFromCtx(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "not unauthorized")
	}

	var req LogOutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		httpx.WriteError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	err := models.RevokeRefreshToken(r.Context(), h.pool, id, req.RefreshToken)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Unable to logout: %v", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)

}
