# API problems found while building the frontend

Flagged only — **nothing in this list has been fixed**, and no existing file was
modified. Line references are as of commit `a6738d6`.

The first four are blockers: the frontend calls these endpoints correctly and
they still cannot succeed. Everything below them is a working-but-fragile note.

## 1. Blocker — 8 of 13 row-scanning call sites can never succeed

Every query that scans into a struct uses `pgx.RowToStructByName`, which
requires the struct's fields and the row's columns to correspond **exactly, in
both directions**. Verified by running the real pgx scanner (v5.10.0) against
each query's exact column list: 8 of the 13 call sites fail deterministically —
on every call, not under load.

There are two distinct failure modes, and the difference matters because the
usual fix only addresses one of them:

**Mode A — a struct field has no matching column.** `computeNamedStructFields`
records it as `missingField` and `ScanRow` rejects the row with
`cannot find field <name> in returned row` (`rows.go:692`). Switching that call
site to `RowToStructByNameLax` fixes it; lax exists for exactly this case.

**Mode B — a column has no matching struct field.** `lookupNamedStructFields`
leaves that column's `path` nil and returns
`struct doesn't have corresponding row field <column>` (`rows.go:739`).
This check runs *before* the lax flag is consulted, so **`Lax` does not fix
mode B** — the names have to be reconciled.

| Call site | Endpoint | Mode | Error |
| --- | --- | --- | --- |
| `users.go:39` `CreateUser` | `POST /auth/signup` | A | `cannot find field role in returned row` |
| `users.go:64` `GetUserByEmail` | `POST /auth/login` | A | `cannot find field role in returned row` |
| `users.go:75` `GetUserByID` | *(uncalled)* | A | `cannot find field role in returned row` |
| `inventory.go:120` `CreateItem` | `POST /items` | A | `cannot find field average_rating in returned row` |
| `inventory.go:176` `UpdateItem` | `PATCH /items/{id}` | A | `cannot find field average_rating in returned row` |
| `carts.go:110` `ViewCart` | `GET /cart` | A | `cannot find field seller_id in returned row` |
| `carts.go:133` `CheckOut` | `POST /checkout` | A | `cannot find field name in returned row` (`unit` is also missing; only the first is reported) |
| `seller.go:68` `GetSellerOrders` | `GET /seller/orders` | **B** | `struct doesn't have corresponding row field order_id` |

The five that scan cleanly: `FilterItems` (`inventory.go:105`), `ListItems`
(`:141`), `GetItemByID` (`:161`), `GetSellerInventory` (`seller.go:40`) and
`GetWallet` (`wallets.go:35`).

### Cause per struct

* **`models.User`** declares `Role` (`users.go:19`, `db:"role"`), but none of
  the three user queries select `role` — `RETURNING` at `users.go:31`,
  `SELECT`s at `:59` and `:70`. Since signup and login are both here,
  **no account can be created or logged into**, which is what makes every
  authenticated route unreachable in practice. It is also the direct cause of
  #2.
* **`models.Item`** declares `AverageRating` (`inventory.go:24`,
  `db:"average_rating"`). The read queries synthesise that column with
  `COALESCE(AVG(r.rating), 0)`, so they pass; the two write paths return only
  real table columns (`RETURNING` at `:113` and `:170`) and fail. There is no
  `average_rating` column in `schema.sql` — the `db` tag makes a computed value
  look like stored one.
* **`models.CartItem`** declares six fields, and both its queries select a
  subset: `ViewCart` (`carts.go:101`) omits `seller_id`; `CheckOut`
  (`carts.go:122`) omits `name` and `unit`.
* **`models.SellerOrderItem`** is the mode-B case: the query (`seller.go:56`)
  returns `order_id` and `price_at_purchase`, while the struct asks for
  `db:"id"` (`seller.go:45`) and `db:"price_atm"` (`:50`). Both names differ in
  both directions, so two fields and two columns are simultaneously unmatched.
  Either alias the columns (`oi.order_id AS id`, `oi.price_at_purchase AS
  price_atm`) or retag the struct — `Lax` will not help here.

Note that `pool.Query` itself succeeds in all these cases; the error only
surfaces at `CollectRows`/`CollectOneRow`, which is why the handlers report
them as generic 500s and "not found" rather than anything about columns.

## 2. Blocker — the JWT role claim is always empty, so every role-gated route 403s

`CreateUser` and `GetUserByEmail` never select `role` (see above), so
`User.Role` is `""` when `issueTokenAndRespond` calls
`GenJWT(user.UserID, user.Role)` (`Handlers/helpers.go:46`). `RequireRole`
compares that empty string against the required role
(`auth/middleware.go:37-39`) and always rejects.

This permanently 403s `POST /wallet/topup`, `POST /items`,
`PATCH /items/{id}`, `DELETE /items/{id}`, `POST /items/{id}/rate`,
`POST /checkout`, `PATCH /cart/items/{id}`, `GET /seller/items` and
`GET /seller/orders` — i.e. every write except cart add/remove/clear. It is a
second, independent cause of failure from #1: fixing the scan alone would not
fix this, because `role` still needs to reach the token.

## 3. Blocker for any browser client — no CORS handling

No handler or middleware sets `Access-Control-Allow-Origin`, and no `OPTIONS`
route is registered in `main.go`. A page served from any other origin cannot
read a single response, and the `Content-Type: application/json` +
`Authorization` headers this client sends trigger a preflight that has nothing
to answer it.

Worked around without touching the API: `cmd/web` serves the frontend and
reverse-proxies `/api/*` to the API, so the browser only ever sees one origin.
That is a dev convenience, not a fix — a deployed frontend on its own domain
still needs real CORS support.

## 4. Blocker for long sessions — no token refresh endpoint

Refresh tokens are generated, hashed, stored and revoked
(`models.StoreRefreshToken`, `RevokeRefreshToken`), but nothing exchanges one
for a new access token — there is no `POST /auth/refresh` route. With
`accessTTL` at 15 minutes (`main.go:29`), users are hard-logged-out every 15
minutes and the stored refresh token is unusable for anything but logout. The
frontend therefore treats any 401 as "sign in again".

## 5. `errors.Is(ErrNotPurchased)` never matches

`models.RateItem` returns `errors.New("Item not purchased")`
(`ratings.go:38`) instead of the declared sentinel `ErrNotPurchased`
(`ratings.go:22`). The handler's `errors.Is(err, models.ErrNotPurchased)` check
(`Handlers/ratings.go:40`) is dead, so rating an unpurchased item returns 500
with a raw error string rather than the intended 403.

## 6. Two missing `return`s after writing an error response

* `Logout` (`Handlers/users.go:86`) — writes 401 then keeps going, so it
  decodes the body and calls `RevokeRefreshToken` with a zero UUID, appending a
  second response to an already-sent 401.
* `CreateItem` (`Handlers/inventory.go:29`) — same shape: writes 404, then
  proceeds to insert an item with a zero `seller_id`, which the FK on
  `items.seller_id` rejects as a 500.

Both are unreachable while the middleware always populates claims, but they are
one refactor away from being live.

## 7. Response bodies have no `json` tags

`models.Item`, `CartItem`, `Wallet`, `ItemSummary`, `SellerOrderItem` and
`User` carry only `db` tags, so `encoding/json` falls back to Go field names:
responses are `{"ID":…,"SellerID":…,"AverageRating":…,"PriceATM":…}` while
request bodies are snake_case (`item_id`, `image_url`, `review_text`). The
frontend absorbs this in the normalizers in [api.js](api.js), but the contract
is one rename away from silently breaking every client, and `PriceATM`
serialises as an opaque name.

## 8. Login and signup responses omit `role`

`AuthResponse.UserResponse` is only `{id, email, name}`
(`Handlers/helpers.go:22-26`), so a client cannot tell a buyer from a seller
from the response. The frontend base64-decodes the JWT payload to recover the
`role` claim (`decodeClaims` in [api.js](api.js)) — which only works because
the claim is readable client-side, and returns `""` today for the reason in #2.

## 9. Smaller notes

* **`POST /wallet/topup` requires the caller's own id in the body**
  (`Handlers/wallet.go:47-50`) and 403s when it mismatches. The token already
  identifies the user; the field is redundant and leaks the id into request
  bodies. It also replies `200` with an empty body rather than the new balance.
* **Top-up amounts are integers** (`Handlers/wallet.go:15`) while `balance` is
  `NUMERIC(12,2)`, so no one can add 10.50. The frontend truncates to match.
* **Inconsistent cart authorization** (`main.go:56-58`): add and remove use
  `Require` (any role) but update uses `RequireRole(buyer)`. A seller can put
  items in a cart and delete them, but not change a quantity.
* **`ListItems` swallows errors** (`inventory.go:139`, `:144`), returning
  `nil` error with an empty slice — a failed query is indistinguishable from an
  empty catalogue. Its route is commented out (`main.go:45`), so `SearchItems`
  with no filters is the de-facto list endpoint.
* **String context key** (`auth/middleware.go:29`): `context.WithValue(ctx,
  "claims", …)` uses a bare `string` rather than an unexported key type, so any
  other package writing `"claims"` silently collides.
* **Wrong status/message on several auth failures** — e.g. `RemoveCartItem`,
  `ViewCart`, `ClearCart` and `Checkout` return `404 "seller dosent exist"` for
  a missing user in context, which should be 401 and is not about sellers.
* **`GetItemByID` maps every error to 404** (`Handlers/inventory.go:88`), so a
  transient database failure is reported as a missing item.
* **Sellers are never paid** — `CheckOut` debits the buyer via `DebitWallet`
  (`carts.go:176`) and records `order_items`, but no seller wallet is credited
  and no `wallet_transactions` row is written for them.
* **No pagination on `GET /items`** — `FilterItems` has no `LIMIT`/`OFFSET`, so
  the catalogue grows unbounded in one response. The frontend renders whatever
  it receives.
* **`items.average_rating` is computed, not stored** — the column does not
  exist in `schema.sql`; `Item.AverageRating` is populated by
  `COALESCE(AVG(r.rating), 0)` in the read queries. That is fine, but the `db`
  tag makes it look like a column and is the direct cause of the write-path
  failures in #1.
