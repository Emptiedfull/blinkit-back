// Thin client over the existing Go API. Every function here maps 1:1 onto a
// handler already registered in main.go -- nothing new is expected server-side.
//
// Wire-format notes (see FINDINGS.md):
//  * Request bodies are snake_case, but responses built from models.* structs
//    carry Go field names (PascalCase) because those structs only have `db`
//    tags, no `json` tags. Normalizers below absorb that asymmetry.
//  * Errors are {"error":{"message":"..."}}, except UpdateCartItem which uses
//    http.Error and returns plain text.
//  * Several handlers reply 200 with an empty body.

const BASE = window.__API_BASE__ ?? "/api";

const TOKEN_KEY = "cc.auth";

export const session = {
  load() {
    try {
      return JSON.parse(localStorage.getItem(TOKEN_KEY)) || null;
    } catch {
      return null;
    }
  },
  save(auth) {
    localStorage.setItem(TOKEN_KEY, JSON.stringify(auth));
  },
  clear() {
    localStorage.removeItem(TOKEN_KEY);
  },
};

export class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

// The access token carries the only copy of the user's role: the login/signup
// response body omits it. Decoding the JWT payload client-side is the only way
// to know which navigation to show.
export function decodeClaims(token) {
  try {
    const [, payload] = token.split(".");
    const json = atob(payload.replace(/-/g, "+").replace(/_/g, "/"));
    return JSON.parse(json);
  } catch {
    return {};
  }
}

async function request(method, path, { body, auth = false, query } = {}) {
  const url = new URL(BASE + path, window.location.origin);
  if (query) {
    for (const [k, v] of Object.entries(query)) {
      if (v !== "" && v !== null && v !== undefined) url.searchParams.set(k, v);
    }
  }

  const headers = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (auth) {
    const s = session.load();
    if (!s?.accessToken) throw new ApiError(401, "You are signed out.");
    headers["Authorization"] = `Bearer ${s.accessToken}`;
  }

  const res = await fetch(url, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  const text = await res.text();
  let parsed = null;
  if (text.trim() !== "") {
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = null;
    }
  }

  if (!res.ok) {
    const msg =
      parsed?.error?.message ||
      (text.trim() !== "" ? text.trim() : `Request failed (${res.status})`);
    throw new ApiError(res.status, msg);
  }

  return parsed;
}

const num = (v) => (typeof v === "number" ? v : Number(v ?? 0));

function normItem(raw) {
  if (!raw) return null;
  return {
    id: raw.ID,
    sellerId: raw.SellerID,
    name: raw.Name,
    description: raw.Description,
    price: num(raw.Price),
    category: raw.Category,
    stock: num(raw.Stock),
    unit: raw.Unit,
    imageUrl: raw.ImageURL,
    averageRating: num(raw.AverageRating),
    createdAt: raw.CreatedAt,
  };
}

function normCartLine(raw) {
  return {
    itemId: raw.ItemID,
    sellerId: raw.SellerID,
    name: raw.Name,
    price: num(raw.Price),
    unit: raw.Unit,
    quantity: num(raw.Quantity),
  };
}

function normWallet(raw) {
  return {
    id: raw.ID,
    userId: raw.UserID,
    balance: num(raw.Balance),
    currency: raw.Currency,
  };
}

function normInventoryRow(raw) {
  return {
    id: raw.ID,
    name: raw.Name,
    price: num(raw.Price),
    stock: num(raw.Stock),
    lowStock: Boolean(raw.LowStock),
    unitsSold: num(raw.UnitsSold),
    revenue: num(raw.Revenue),
  };
}

function normSellerOrder(raw) {
  return {
    orderId: raw.ID,
    itemId: raw.ItemID,
    buyerId: raw.BuyerID,
    itemName: raw.ItemName,
    quantity: num(raw.Quantity),
    priceAtPurchase: num(raw.PriceATM),
    createdAt: raw.CreatedAt,
  };
}

function normAuth(raw) {
  const accessToken = raw.Accesstoken;
  const claims = decodeClaims(accessToken);
  return {
    accessToken,
    refreshToken: raw.RefreshToken,
    user: {
      id: raw.user?.id,
      email: raw.user?.email,
      name: raw.user?.name,
      // Role is absent from the response body; fall back to the JWT claim.
      role: claims.role || "",
    },
  };
}

export const api = {
  // --- auth: handler.HandleSignup / HandleLogin / Logout ---
  async signup({ email, name, role, password }) {
    return normAuth(
      await request("POST", "/auth/signup", {
        body: { email, name, role, password },
      }),
    );
  },

  async login({ email, password }) {
    return normAuth(
      await request("POST", "/auth/login", { body: { email, password } }),
    );
  },

  async logout(refreshToken) {
    await request("POST", "/auth/logout", {
      auth: true,
      body: { refresh_token: refreshToken },
    });
  },

  // --- wallet: handler.GetWallet / TopUpWallet ---
  async getWallet() {
    return normWallet(await request("GET", "/wallet", { auth: true }));
  },

  // TopUpWallet re-checks the body `id` against the token subject, so it has to
  // be sent even though the server already knows who is calling.
  async topUpWallet(userId, amount) {
    await request("POST", "/wallet/topup", {
      auth: true,
      body: { id: userId, amount: Math.trunc(amount) },
    });
  },

  // --- catalogue: handler.SearchItems / GetItem ---
  async searchItems(filters = {}) {
    const raw = await request("GET", "/items", {
      query: {
        q: filters.q,
        category: filters.category,
        min_price: filters.minPrice,
        max_price: filters.maxPrice,
        in_stock: filters.inStock ? "true" : "",
        sort: filters.sort,
      },
    });
    return (raw ?? []).map(normItem);
  },

  async getItem(id) {
    return normItem(await request("GET", `/items/${id}`));
  },

  // --- seller catalogue writes: handler.CreateItem / UpdateItem / DeleteItem ---
  async createItem(item) {
    return normItem(
      await request("POST", "/items", { auth: true, body: itemBody(item) }),
    );
  },

  async updateItem(id, item) {
    return normItem(
      await request("PATCH", `/items/${id}`, {
        auth: true,
        body: itemBody(item),
      }),
    );
  },

  async deleteItem(id) {
    await request("DELETE", `/items/${id}`, { auth: true });
  },

  // --- ratings: handler.RateItem ---
  async rateItem(id, rating, reviewText) {
    await request("POST", `/items/${id}/rate`, {
      auth: true,
      body: { rating: Number(rating), review_text: reviewText ?? "" },
    });
  },

  // --- cart: handler.ViewCart / AddCartItem / UpdateCartItem / RemoveCartItem / ClearCart ---
  async viewCart() {
    const raw = await request("GET", "/cart", { auth: true });
    return (raw ?? []).map(normCartLine);
  },

  async addCartItem(itemId, quantity) {
    await request("POST", "/cart/items", {
      auth: true,
      body: { item_id: itemId, quantity: Number(quantity) },
    });
  },

  async updateCartItem(itemId, quantity) {
    await request("PATCH", `/cart/items/${itemId}`, {
      auth: true,
      body: { quantity: Number(quantity) },
    });
  },

  async removeCartItem(itemId) {
    await request("DELETE", `/cart/items/${itemId}`, { auth: true });
  },

  async clearCart() {
    await request("DELETE", "/cart", { auth: true });
  },

  // --- checkout: handler.Checkout ---
  async checkout() {
    const raw = await request("POST", "/checkout", { auth: true });
    return raw?.order_id ?? null;
  },

  // --- seller dashboard: handler.SellerInventory / SellerOrders ---
  async sellerInventory() {
    const raw = await request("GET", "/seller/items", { auth: true });
    return (raw ?? []).map(normInventoryRow);
  },

  async sellerOrders() {
    const raw = await request("GET", "/seller/orders", { auth: true });
    return (raw ?? []).map(normSellerOrder);
  },
};

function itemBody(item) {
  return {
    name: item.name,
    description: item.description ?? "",
    price: Number(item.price),
    category: item.category ?? "",
    stock: Number(item.stock),
    unit: item.unit || "pcs",
    image_url: item.imageUrl ?? "",
  };
}
