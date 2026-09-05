package handlers

import (
	"cc/internal/auth"
	"cc/internal/httpx"
	"cc/internal/models"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ResHandler struct {
	pool   *pgxpool.Pool
	Issuer *auth.Issuer
}

type AuthResponse struct {
	Accesstoken  string
	RefreshToken string
	UserResponse `json:"user"`
}

type UserResponse struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name"`
}

func NewHandler(pool *pgxpool.Pool, Issuer *auth.Issuer) *ResHandler {
	return &ResHandler{
		pool:   pool,
		Issuer: Issuer,
	}
}

func (h *ResHandler) issueTokenAndRespond(w http.ResponseWriter, r *http.Request, user models.User) {
	token, err := h.Issuer.GenJWT(user.UserID, user.Role)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not issue token")
		return
	}

	refreshToken, err := auth.NewRefreshToken()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not issue token")
		return
	}

	err = models.StoreRefreshToken(r.Context(), h.pool, user.UserID, refreshToken, h.Issuer.RefreshToken())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not store token")
		return
	}

	res := AuthResponse{
		Accesstoken:  token,
		RefreshToken: refreshToken,
	}

	res.UserResponse = UserResponse{user.UserID, user.Email, user.UserName}
	httpx.WriteJSON(w, http.StatusOK, res)

}
