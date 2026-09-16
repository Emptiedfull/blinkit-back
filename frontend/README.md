# Frontend

A dependency-free single-page frontend for the existing API. No build step, no
framework, no CDN — three files the browser loads directly.

| File | Role |
| --- | --- |
| [index.html](index.html) | Shell: top bar, view container, toast host |
| [api.js](api.js) | One function per handler in `internal/Handlers`, plus the normalizers that absorb the response format |
| [app.js](app.js) | Hash router and the views |
| [styles.css](styles.css) | Light/dark theme, responsive down to ~400px |
| [FINDINGS.md](FINDINGS.md) | Problems found in the existing code — flagged, not fixed |

No existing file was modified. The only addition outside this directory is
[cmd/web/main.go](../cmd/web/main.go), a ~70-line static server that
reverse-proxies `/api/*` to the API so the browser sees a single origin — the
API sends no CORS headers, so a cross-origin page cannot call it at all
(FINDINGS #3).

## Running

```sh
# 1. the API (as before)
DATABASE_URL=postgres://... go run .

# 2. the frontend
go run ./cmd/web            # http://localhost:8080, proxying /api -> :8070
```

Environment: `WEB_PORT` (8080), `API_URL` (`http://localhost:$PORT`, default
`:8070`), `WEB_DIR` (`frontend`).

To skip the proxy and point the page straight at an API that does send CORS
headers, set the base URL in the browser console:

```js
localStorage.setItem("cc.apiBase", "http://localhost:8070")
```

## What is wired up

Every route registered in [main.go](../main.go) is reachable from the UI:

| View | Endpoints |
| --- | --- |
| Sign in / sign up | `POST /auth/signup`, `POST /auth/login`, `POST /auth/logout` |
| Browse | `GET /items` with `q`, `category`, `min_price`, `max_price`, `in_stock`, `sort` |
| Item detail | `GET /items/{id}`, `POST /cart/items`, `POST /items/{id}/rate` |
| Cart | `GET /cart`, `PATCH /cart/items/{id}`, `DELETE /cart/items/{id}`, `DELETE /cart`, `POST /checkout` |
| Wallet | `GET /wallet`, `POST /wallet/topup` |
| My items (seller) | `GET /seller/items`, `POST /items`, `PATCH /items/{id}`, `DELETE /items/{id}` |
| Orders (seller) | `GET /seller/orders` |

`GET /items` is used for the catalogue because the `ListItems` route is
commented out in `main.go`; an unfiltered search is equivalent.

Navigation is driven by the `role` claim decoded from the access token, since
the login response body does not include it (FINDINGS #8).

## Notes on how it talks to the API

Handled in [api.js](api.js) because the API is inconsistent about them:

* Responses from `models.*` structs use Go field names (`ID`, `SellerID`,
  `PriceATM`) while requests are snake_case — normalizers convert at the edge,
  so views only ever see camelCase (FINDINGS #7).
* Errors are `{"error":{"message":…}}`, except `UpdateCartItem`, which uses
  `http.Error` and returns plain text. Both are unwrapped to a message.
* Several handlers reply `200` with an empty body, so responses are read as
  text and only parsed when non-empty.
* Any `401` clears the session and returns to sign-in: there is no refresh
  endpoint to recover the 15-minute access token (FINDINGS #4).

## Verifying without a database

The frontend was checked against a stub reproducing the handlers' exact wire
format. With `node` available:

```sh
go build ./...                  # API + cmd/web compile
node --input-type=module --check < frontend/api.js
node --input-type=module --check < frontend/app.js
```

Bear in mind that against the real API most endpoints currently fail for
reasons in [FINDINGS.md](FINDINGS.md) — signup and login included — so a
first-run smoke test will surface those before any UI issue.
