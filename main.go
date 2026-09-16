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

const buyer = "buyer"
const seller = "seller"

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

	mux.HandleFunc("POST /api/auth/signup", handler.HandleSignup)
	mux.HandleFunc("POST /api/auth/login", handler.HandleLogin)
	mux.HandleFunc("POST /api/auth/logout", issuer.Require(handler.Logout))

	mux.HandleFunc("GET /api/wallet", issuer.Require(handler.GetWallet))
	mux.HandleFunc("POST /api/wallet/topup", issuer.RequireRole(buyer, handler.TopUpWallet))

	mux.HandleFunc("POST /api/items", issuer.RequireRole(seller, handler.CreateItem))
	mux.HandleFunc("PATCH /api/items/{id}", issuer.RequireRole(seller, handler.UpdateItem))
	mux.HandleFunc("DELETE /api/items/{id}", issuer.RequireRole(seller, handler.DeleteItem))

	mux.HandleFunc("POST /api/items/{id}/rate", issuer.RequireRole(buyer, handler.RateItem))

	// mux.HandleFunc("GET /items", handler.ListItems)
	mux.HandleFunc("GET /api/items", handler.SearchItems)
	mux.HandleFunc("GET /api/items/{id}", handler.GetItem)

	mux.HandleFunc("POST /api/checkout", issuer.RequireRole(buyer, handler.Checkout))

	mux.HandleFunc("GET /api/seller/items", issuer.RequireRole("seller", handler.SellerInventory))
	mux.HandleFunc("GET /api/seller/orders", issuer.RequireRole("seller", handler.SellerOrders))

	mux.HandleFunc("GET /api/cart", issuer.Require(handler.ViewCart))
	mux.HandleFunc("POST /api/cart/items", issuer.Require(handler.AddCartItem))
	mux.HandleFunc("PATCH /api/cart/items/{id}", issuer.RequireRole(buyer, handler.UpdateCartItem))
	mux.HandleFunc("DELETE /api/cart/items/{id}", issuer.Require(handler.RemoveCartItem))
	mux.HandleFunc("DELETE /api/cart", issuer.Require(handler.ClearCart))

	mux.Handle("/", http.FileServer(http.Dir("./frontend")))

	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}

}
