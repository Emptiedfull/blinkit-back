package main

import (
	handlers "cc/internal/Handlers"
	"cc/internal/auth"
	"cc/internal/config"
	"cc/internal/db"
	"context"
	"log"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()
	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	db, err := db.NewDB(context.Background(), cfg.DBUrl)
	if err != nil {
		panic(err)
	}

	issuer := auth.NewIssuer([]byte(cfg.JWTSecret), 15*time.Minute, 7*24*time.Hour)

	handler := handlers.NewHandler(db, issuer)

	mux.HandleFunc("POST /auth/signup", handler.HandleSignup)
	mux.HandleFunc("POST /auth/login", handler.HandleLogin)
	mux.HandleFunc("POST /auth/logout", issuer.Require(handler.Logout))

	mux.HandleFunc("GET /wallet", issuer.Require(handler.GetWallet))
	mux.HandleFunc("POST /wallet/topup", handler.TopUpWallet)

	mux.HandleFunc("POST /items", issuer.Require(handler.CreateItem))

	mux.HandleFunc("GET /items", handler.ListItems)
	mux.HandleFunc("GET /items/{id}", handler.GetItem)

	mux.HandleFunc("GET /cart", issuer.Require(handler.ViewCart))
	mux.HandleFunc("POST /cart/items", issuer.Require(handler.AddCartItem))
	mux.HandleFunc("PATCH /cart/items/{id}", issuer.Require(handler.UpdateCartItem))
	mux.HandleFunc("DELETE /cart/items/{id}", issuer.Require(handler.RemoveCartItem))
	mux.HandleFunc("DELETE /cart", issuer.Require(handler.ClearCart))

	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}

}
