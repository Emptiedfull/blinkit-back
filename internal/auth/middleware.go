package auth

import (
	"cc/internal/httpx"
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

func (t *Issuer) Require(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		parts := strings.SplitN(header, " ", 2)

		if len(parts) != 2 || parts[0] != "Bearer" {
			httpx.WriteError(w, http.StatusUnauthorized, "missing auth token")
			return
		}

		claims, err := t.ValJWT(parts[1])
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid auth token")
			return
		}

		ctx := context.WithValue(r.Context(), "claims", claims)
		next(w, r.WithContext(ctx))
	}
}

func UserFromCtx(ctx context.Context) (uuid.UUID, bool) {
	claims, ok := ctx.Value("claims").(Claims)
	return claims.ID, ok

}

// type JsonErr struct {
// 	Code int    `json:"code"`
// 	Msg  string `json:"message"`
// }

// func HttpError(w http.ResponseWriter, message string, code int) {
// 	w.WriteHeader(code)
// 	JsonErr := JsonErr{
// 		Code: code,
// 		Msg:  message,
// 	}

// 	json.NewEncoder(w).Encode(JsonErr)

// }
