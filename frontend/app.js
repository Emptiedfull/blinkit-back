import { api, session, ApiError } from "./api.js";

/* ------------------------------------------------------------------ *
 * small DOM + formatting helpers
 * ------------------------------------------------------------------ */

const view = document.getElementById("view");
const navEl = document.getElementById("nav");
const whoamiEl = document.getElementById("whoami");
const toastsEl = document.getElementById("toasts");

function h(tag, props = {}, ...children) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(props)) {
    if (v === null || v === undefined || v === false) continue;
    if (k === "class") node.className = v;
    else if (k === "html") node.innerHTML = v;
    else if (k.startsWith("on") && typeof v === "function")
      node.addEventListener(k.slice(2).toLowerCase(), v);
    else if (k === "value") node.value = v;
    else if (k === "checked") node.checked = Boolean(v);
    else node.setAttribute(k, v);
  }
  for (const c of children.flat()) {
    if (c === null || c === undefined || c === false) continue;
    node.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return node;
}

let currency = "INR";
const money = (n) => `${currency} ${Number(n ?? 0).toFixed(2)}`;
const shortId = (id) => (id ? String(id).slice(0, 8) : "—");
const date = (s) => (s ? new Date(s).toLocaleString() : "—");

function stars(avg) {
  const r = Math.round(Number(avg) || 0);
  return r > 0 ? "★".repeat(r) + "☆".repeat(5 - r) : "unrated";
}

function toast(message, isError = false) {
  const t = h("div", { class: `toast${isError ? " err" : ""}` }, message);
  toastsEl.append(t);
  setTimeout(() => t.remove(), isError ? 6000 : 3000);
}

// Wraps every handler call: turns ApiError into a toast and signs the user out
// when the 15-minute access token has expired (there is no refresh endpoint).
async function guard(fn, { onError } = {}) {
  try {
    return await fn();
  } catch (err) {
    const msg = err instanceof ApiError ? err.message : String(err);
    toast(msg, true);
    if (err instanceof ApiError && err.status === 401) {
      session.clear();
      state.auth = null;
      go("#/login");
    }
    onError?.(err);
    return undefined;
  }
}

function field(label, input) {
  return h("div", { class: "field" }, h("label", {}, label), input);
}

function empty(text) {
  return h("div", { class: "empty" }, text);
}

function loading() {
  return h("p", { class: "muted" }, "Loading…");
}

/* ------------------------------------------------------------------ *
 * state + routing
 * ------------------------------------------------------------------ */

const state = {
  auth: session.load(),
  filters: { q: "", category: "", minPrice: "", maxPrice: "", inStock: false, sort: "" },
};

const isSeller = () => state.auth?.user?.role === "seller";
const isBuyer = () => state.auth?.user?.role === "buyer";
const signedIn = () => Boolean(state.auth?.accessToken);

function go(hash) {
  if (window.location.hash === hash) render();
  else window.location.hash = hash;
}

const routes = [
  { pattern: /^#?\/?$/, view: browseView, nav: "#/browse" },
  { pattern: /^#\/browse$/, view: browseView, nav: "#/browse" },
  { pattern: /^#\/items\/([0-9a-fA-F-]+)$/, view: itemView, nav: "#/browse" },
  { pattern: /^#\/cart$/, view: cartView, nav: "#/cart" },
  { pattern: /^#\/wallet$/, view: walletView, nav: "#/wallet" },
  { pattern: /^#\/seller\/items$/, view: sellerItemsView, nav: "#/seller/items" },
  { pattern: /^#\/seller\/orders$/, view: sellerOrdersView, nav: "#/seller/orders" },
  { pattern: /^#\/login$/, view: authView, nav: "#/login" },
];

function render() {
  const hash = window.location.hash || "#/browse";
  const route = routes.find((r) => r.pattern.test(hash)) ?? routes[0];
  const params = hash.match(route.pattern)?.slice(1) ?? [];

  renderChrome(route.nav);
  view.replaceChildren(loading());
  Promise.resolve(route.view(...params)).catch((err) =>
    view.replaceChildren(empty(`Could not render this page: ${err.message}`)),
  );
}

function renderChrome(current) {
  const links = [["Browse", "#/browse"]];
  if (signedIn()) {
    links.push(["Cart", "#/cart"], ["Wallet", "#/wallet"]);
    if (isSeller()) {
      links.push(["My items", "#/seller/items"], ["Orders", "#/seller/orders"]);
    }
  }

  navEl.replaceChildren(
    ...links.map(([label, hash]) =>
      h(
        "button",
        {
          "aria-current": hash === current ? "page" : null,
          onClick: () => go(hash),
        },
        label,
      ),
    ),
  );

  if (signedIn()) {
    const u = state.auth.user;
    whoamiEl.replaceChildren(
      h("span", {}, u.name || u.email || "signed in"),
      u.role ? h("span", { class: "pill" }, u.role) : h("span", { class: "pill" }, "no role"),
      h("button", { class: "btn ghost sm", onClick: signOut }, "Sign out"),
    );
  } else {
    whoamiEl.replaceChildren(
      h("button", { class: "btn sm", onClick: () => go("#/login") }, "Sign in"),
    );
  }
}

async function signOut() {
  // Best-effort: the handler revokes the refresh token, then we drop local state
  // regardless of the outcome (nothing can revoke the access token server-side).
  try {
    await api.logout(state.auth.refreshToken);
  } catch {
    /* token already expired or revoked -- fall through */
  }
  session.clear();
  state.auth = null;
  toast("Signed out.");
  go("#/browse");
}

function requireAuth() {
  if (signedIn()) return false;
  view.replaceChildren(
    h(
      "div",
      { class: "stack" },
      h("h1", {}, "Sign in required"),
      h("p", { class: "sub" }, "This page needs an account."),
      h("div", {}, h("button", { class: "btn", onClick: () => go("#/login") }, "Go to sign in")),
    ),
  );
  return true;
}

/* ------------------------------------------------------------------ *
 * auth view  --  POST /auth/signup, POST /auth/login
 * ------------------------------------------------------------------ */

function authView() {
  let mode = "login";

  const container = h("div", {});

  function paint() {
    const isLogin = mode === "login";

    const email = h("input", { type: "email", autocomplete: "email", required: true });
    const password = h("input", { type: "password", autocomplete: "current-password", required: true });
    const name = h("input", { type: "text", autocomplete: "name" });
    const role = h(
      "select",
      {},
      h("option", { value: "buyer" }, "Buyer"),
      h("option", { value: "seller" }, "Seller"),
    );

    const submit = h("button", { class: "btn", type: "submit" }, isLogin ? "Sign in" : "Create account");

    const form = h(
      "form",
      {
        class: "stack",
        onSubmit: async (e) => {
          e.preventDefault();
          submit.disabled = true;
          const auth = await guard(() =>
            isLogin
              ? api.login({ email: email.value.trim(), password: password.value })
              : api.signup({
                  email: email.value.trim(),
                  password: password.value,
                  name: name.value.trim(),
                  role: role.value,
                }),
          );
          submit.disabled = false;
          if (!auth) return;

          state.auth = auth;
          session.save(auth);
          toast(`Welcome, ${auth.user.name || auth.user.email}.`);
          go(auth.user.role === "seller" ? "#/seller/items" : "#/browse");
        },
      },
      field("Email", email),
      !isLogin && field("Name", name),
      !isLogin && field("Account type", role),
      field("Password", password),
      h("div", { class: "row" }, submit),
    );

    container.replaceChildren(
      h(
        "div",
        { class: "stack", style: "max-width:420px;margin:0 auto" },
        h("div", {}, h("h1", {}, isLogin ? "Sign in" : "Create an account"), h(
          "p",
          { class: "sub" },
          isLogin
            ? "Use the credentials you signed up with."
            : "Buyers shop and rate; sellers list items and track orders.",
        )),
        h("div", { class: "card" }, form),
        h(
          "p",
          { class: "muted", style: "text-align:center;font-size:13px" },
          isLogin ? "No account yet? " : "Already registered? ",
          h(
            "a",
            {
              href: "#",
              onClick: (e) => {
                e.preventDefault();
                mode = isLogin ? "signup" : "login";
                paint();
              },
            },
            isLogin ? "Create one" : "Sign in",
          ),
        ),
      ),
    );
  }

  paint();
  view.replaceChildren(container);
}

/* ------------------------------------------------------------------ *
 * browse view  --  GET /items  (handler.SearchItems)
 * ------------------------------------------------------------------ */

async function browseView() {
  const results = h("div", {});

  const q = h("input", { type: "search", placeholder: "Search items…", value: state.filters.q });
  const category = h("input", { type: "text", placeholder: "Any", value: state.filters.category });
  const minPrice = h("input", { type: "number", min: "0", step: "0.01", placeholder: "0", value: state.filters.minPrice });
  const maxPrice = h("input", { type: "number", min: "0", step: "0.01", placeholder: "∞", value: state.filters.maxPrice });
  const inStock = h("input", { type: "checkbox", checked: state.filters.inStock });
  const sort = h(
    "select",
    {},
    h("option", { value: "" }, "Relevance"),
    h("option", { value: "price_asc" }, "Price: low to high"),
    h("option", { value: "price_desc" }, "Price: high to low"),
    h("option", { value: "rating" }, "Highest rated"),
    h("option", { value: "age" }, "Newest"),
  );
  sort.value = state.filters.sort;

  async function load() {
    state.filters = {
      q: q.value.trim(),
      category: category.value.trim(),
      minPrice: minPrice.value,
      maxPrice: maxPrice.value,
      inStock: inStock.checked,
      sort: sort.value,
    };
    results.replaceChildren(loading());
    const items = await guard(() => api.searchItems(state.filters));
    if (!items) {
      results.replaceChildren(empty("Could not load the catalogue."));
      return;
    }
    results.replaceChildren(
      items.length === 0
        ? empty("No items match these filters.")
        : h("div", { class: "grid" }, ...items.map(itemCard)),
    );
  }

  const form = h(
    "form",
    {
      class: "stack",
      onSubmit: (e) => {
        e.preventDefault();
        load();
      },
    },
    h("div", { class: "row" }, field("Search", q), field("Category", category), field("Sort", sort)),
    h(
      "div",
      { class: "row" },
      field("Min price", minPrice),
      field("Max price", maxPrice),
      h("label", { class: "check", style: "flex:1 1 150px" }, inStock, "In stock only"),
      h("button", { class: "btn", type: "submit" }, "Apply"),
      h(
        "button",
        {
          class: "btn ghost",
          type: "button",
          onClick: () => {
            q.value = category.value = minPrice.value = maxPrice.value = "";
            inStock.checked = false;
            sort.value = "";
            load();
          },
        },
        "Reset",
      ),
    ),
  );

  view.replaceChildren(
    h(
      "div",
      { class: "stack" },
      h("div", {}, h("h1", {}, "Browse"), h("p", { class: "sub" }, "Everything listed by every seller.")),
      h("div", { class: "card" }, form),
      results,
    ),
  );

  await load();
}

function itemCard(item) {
  const soldOut = item.stock <= 0;
  return h(
    "article",
    { class: "item" },
    h(
      "div",
      { class: "thumb" },
      item.imageUrl
        ? h("img", { src: item.imageUrl, alt: item.name, loading: "lazy", onError: (e) => e.target.remove() })
        : "no image",
    ),
    h(
      "div",
      { class: "body" },
      h("div", { class: "name" }, item.name),
      h("div", { class: "price mono" }, money(item.price), h("span", { class: "muted" }, ` / ${item.unit}`)),
      h(
        "div",
        { class: "meta" },
        h("span", { class: "stars" }, stars(item.averageRating)),
        item.category ? h("span", { class: "tag" }, item.category) : null,
        soldOut ? h("span", { class: "tag out" }, "out of stock") : h("span", {}, `${item.stock} left`),
      ),
      h(
        "div",
        { class: "actions" },
        h("button", { class: "btn sm", onClick: () => go(`#/items/${item.id}`) }, "View"),
        signedIn() && !soldOut
          ? h(
              "button",
              {
                class: "btn ghost sm",
                onClick: async (e) => {
                  e.target.disabled = true;
                  const ok = await guard(() => api.addCartItem(item.id, 1));
                  e.target.disabled = false;
                  if (ok !== undefined) toast(`Added ${item.name} to cart.`);
                },
              },
              "Add to cart",
            )
          : null,
      ),
    ),
  );
}

/* ------------------------------------------------------------------ *
 * item detail  --  GET /items/{id}, POST /cart/items, POST /items/{id}/rate
 * ------------------------------------------------------------------ */

async function itemView(id) {
  const item = await guard(() => api.getItem(id));
  if (!item) {
    view.replaceChildren(empty("That item could not be loaded."));
    return;
  }

  const qty = h("input", { type: "number", min: "1", max: String(Math.max(item.stock, 1)), value: "1" });
  const addBtn = h(
    "button",
    {
      class: "btn",
      disabled: item.stock <= 0,
      onClick: async () => {
        addBtn.disabled = true;
        const ok = await guard(() => api.addCartItem(item.id, Number(qty.value)));
        addBtn.disabled = item.stock <= 0;
        if (ok !== undefined) toast("Added to cart.");
      },
    },
    item.stock > 0 ? "Add to cart" : "Out of stock",
  );

  const buyPanel = signedIn()
    ? h(
        "div",
        { class: "card stack" },
        h("h2", {}, "Purchase"),
        h("div", { class: "row" }, field("Quantity", qty), addBtn),
        h(
          "p",
          { class: "muted", style: "font-size:13px;margin:0" },
          `${item.stock} ${item.unit} available.`,
        ),
      )
    : h(
        "div",
        { class: "card stack" },
        h("h2", {}, "Purchase"),
        h("p", { class: "sub", style: "margin:0" }, "Sign in to add this to your cart."),
        h("div", {}, h("button", { class: "btn", onClick: () => go("#/login") }, "Sign in")),
      );

  view.replaceChildren(
    h(
      "div",
      { class: "stack" },
      h(
        "div",
        { class: "spread" },
        h("div", {}, h("h1", {}, item.name), h("p", { class: "sub", style: "margin:0" }, item.category || "uncategorised")),
        h("button", { class: "btn ghost sm", onClick: () => go("#/browse") }, "← Back"),
      ),
      h(
        "div",
        { class: "two-col" },
        h(
          "div",
          { class: "stack" },
          item.imageUrl
            ? h("img", { class: "hero-img", src: item.imageUrl, alt: item.name, onError: (e) => e.target.remove() })
            : null,
          h(
            "div",
            { class: "card stack" },
            h("h2", {}, "Description"),
            h("p", { style: "margin:0" }, item.description || h("span", { class: "muted" }, "No description provided.")),
          ),
          ratingPanel(item),
        ),
        h(
          "div",
          { class: "stack" },
          h(
            "div",
            { class: "card stack" },
            h("div", { class: "stat" }, h("span", { class: "value" }, money(item.price)), h("span", { class: "label" }, `per ${item.unit}`)),
            h("div", { class: "spread" }, h("span", { class: "stars" }, stars(item.averageRating)), h("span", { class: "muted", style: "font-size:13px" }, item.averageRating > 0 ? item.averageRating.toFixed(2) : "")),
            h("div", { class: "muted", style: "font-size:13px" }, "Seller ", h("code", { class: "id" }, shortId(item.sellerId))),
          ),
          buyPanel,
        ),
      ),
    ),
  );
}

// Rating is buyer-only and the handler rejects items the caller never bought.
function ratingPanel(item) {
  if (!signedIn()) return null;

  const rating = h(
    "select",
    {},
    ...[5, 4, 3, 2, 1].map((n) => h("option", { value: String(n) }, `${n} ★`)),
  );
  const review = h("textarea", { placeholder: "Optional review" });
  const submit = h("button", { class: "btn", type: "submit" }, "Submit rating");

  return h(
    "form",
    {
      class: "card stack",
      onSubmit: async (e) => {
        e.preventDefault();
        submit.disabled = true;
        const ok = await guard(() => api.rateItem(item.id, rating.value, review.value.trim()));
        submit.disabled = false;
        if (ok !== undefined) {
          toast("Rating saved.");
          review.value = "";
          render();
        }
      },
    },
    h("h2", {}, "Rate this item"),
    h("p", { class: "sub", style: "margin:0" }, "Only available once you have purchased it."),
    h("div", { class: "row" }, field("Score", rating)),
    field("Review", review),
    h("div", {}, submit),
  );
}

/* ------------------------------------------------------------------ *
 * cart  --  GET /cart, PATCH & DELETE /cart/items/{id}, DELETE /cart, POST /checkout
 * ------------------------------------------------------------------ */

async function cartView() {
  if (requireAuth()) return;

  const lines = await guard(() => api.viewCart());
  if (!lines) {
    view.replaceChildren(empty("Could not load your cart."));
    return;
  }

  const total = lines.reduce((sum, l) => sum + l.price * l.quantity, 0);

  const body = h(
    "tbody",
    {},
    ...lines.map((line) => {
      const qty = h("input", { type: "number", min: "1", value: String(line.quantity), style: "width:80px" });
      return h(
        "tr",
        {},
        h("td", {}, h("div", {}, line.name), h("code", { class: "id" }, shortId(line.itemId))),
        h("td", { class: "num mono" }, money(line.price)),
        h(
          "td",
          {},
          h(
            "div",
            { class: "row", style: "gap:6px;align-items:center" },
            qty,
            h(
              "button",
              {
                class: "btn ghost sm",
                onClick: async () => {
                  const ok = await guard(() => api.updateCartItem(line.itemId, Number(qty.value)));
                  if (ok !== undefined) {
                    toast("Quantity updated.");
                    render();
                  }
                },
              },
              "Update",
            ),
          ),
        ),
        h("td", { class: "num mono" }, money(line.price * line.quantity)),
        h(
          "td",
          { class: "num" },
          h(
            "button",
            {
              class: "btn danger sm",
              onClick: async () => {
                const ok = await guard(() => api.removeCartItem(line.itemId));
                if (ok !== undefined) {
                  toast("Item removed.");
                  render();
                }
              },
            },
            "Remove",
          ),
        ),
      );
    }),
  );

  const checkoutBtn = h(
    "button",
    {
      class: "btn",
      disabled: lines.length === 0,
      onClick: async () => {
        checkoutBtn.disabled = true;
        const orderId = await guard(() => api.checkout());
        checkoutBtn.disabled = false;
        if (orderId !== undefined) {
          toast(`Order ${shortId(orderId)} placed.`);
          render();
        }
      },
    },
    "Checkout",
  );

  view.replaceChildren(
    h(
      "div",
      { class: "stack" },
      h("div", {}, h("h1", {}, "Cart"), h("p", { class: "sub" }, "Checkout debits your wallet and reduces stock.")),
      lines.length === 0
        ? empty("Your cart is empty.")
        : h(
            "div",
            { class: "card stack" },
            h(
              "div",
              { class: "table-wrap" },
              h(
                "table",
                {},
                h(
                  "thead",
                  {},
                  h(
                    "tr",
                    {},
                    h("th", {}, "Item"),
                    h("th", { class: "num" }, "Price"),
                    h("th", {}, "Quantity"),
                    h("th", { class: "num" }, "Line total"),
                    h("th", {}),
                  ),
                ),
                body,
              ),
            ),
            h(
              "div",
              { class: "spread" },
              h("div", { class: "stat" }, h("span", { class: "value" }, money(total)), h("span", { class: "label" }, "Cart total")),
              h(
                "div",
                { class: "row" },
                h(
                  "button",
                  {
                    class: "btn danger",
                    onClick: async () => {
                      const ok = await guard(() => api.clearCart());
                      if (ok !== undefined) {
                        toast("Cart cleared.");
                        render();
                      }
                    },
                  },
                  "Clear cart",
                ),
                checkoutBtn,
              ),
            ),
          ),
    ),
  );
}

/* ------------------------------------------------------------------ *
 * wallet  --  GET /wallet, POST /wallet/topup
 * ------------------------------------------------------------------ */

async function walletView() {
  if (requireAuth()) return;

  const wallet = await guard(() => api.getWallet());
  if (!wallet) {
    view.replaceChildren(empty("Could not load your wallet."));
    return;
  }
  currency = wallet.currency || currency;

  // The top-up handler takes an int amount and is buyer-only.
  const amount = h("input", { type: "number", min: "1", step: "1", value: "500" });
  const submit = h("button", { class: "btn", type: "submit" }, "Top up");

  const topUp = h(
    "form",
    {
      class: "card stack",
      onSubmit: async (e) => {
        e.preventDefault();
        submit.disabled = true;
        const ok = await guard(() => api.topUpWallet(state.auth.user.id, Number(amount.value)));
        submit.disabled = false;
        if (ok !== undefined) {
          toast(`Added ${money(Math.trunc(Number(amount.value)))}.`);
          render();
        }
      },
    },
    h("h2", {}, "Add funds"),
    h("p", { class: "sub", style: "margin:0" }, "Whole amounts only — the endpoint takes an integer."),
    h("div", { class: "row" }, field(`Amount (${wallet.currency})`, amount), submit),
  );

  view.replaceChildren(
    h(
      "div",
      { class: "stack" },
      h("div", {}, h("h1", {}, "Wallet"), h("p", { class: "sub" }, "Balance is drawn down at checkout.")),
      h(
        "div",
        { class: "two-col" },
        h(
          "div",
          { class: "card" },
          h(
            "div",
            { class: "stat" },
            h("span", { class: "value" }, money(wallet.balance)),
            h("span", { class: "label" }, `available · ${wallet.currency}`),
          ),
        ),
        isBuyer() || !state.auth.user.role
          ? topUp
          : h(
              "div",
              { class: "card" },
              h("h2", {}, "Add funds"),
              h("p", { class: "sub", style: "margin:0" }, "Top-ups are restricted to buyer accounts."),
            ),
      ),
    ),
  );
}

/* ------------------------------------------------------------------ *
 * seller items  --  GET /seller/items, POST /items, PATCH & DELETE /items/{id}
 * ------------------------------------------------------------------ */

async function sellerItemsView() {
  if (requireAuth()) return;

  const rows = await guard(() => api.sellerInventory());
  if (!rows) {
    view.replaceChildren(empty("Could not load your inventory."));
    return;
  }

  const totals = rows.reduce(
    (acc, r) => ({ revenue: acc.revenue + r.revenue, sold: acc.sold + r.unitsSold }),
    { revenue: 0, sold: 0 },
  );

  view.replaceChildren(
    h(
      "div",
      { class: "stack" },
      h("div", {}, h("h1", {}, "My items"), h("p", { class: "sub" }, "Your listings, stock levels and lifetime sales.")),
      h(
        "div",
        { class: "grid" },
        h("div", { class: "card stat" }, h("span", { class: "value" }, String(rows.length)), h("span", { class: "label" }, "Listings")),
        h("div", { class: "card stat" }, h("span", { class: "value" }, String(totals.sold)), h("span", { class: "label" }, "Units sold")),
        h("div", { class: "card stat" }, h("span", { class: "value" }, money(totals.revenue)), h("span", { class: "label" }, "Revenue")),
      ),
      itemEditor(),
      rows.length === 0
        ? empty("You have not listed anything yet.")
        : h(
            "div",
            { class: "card table-wrap" },
            h(
              "table",
              {},
              h(
                "thead",
                {},
                h(
                  "tr",
                  {},
                  h("th", {}, "Item"),
                  h("th", { class: "num" }, "Price"),
                  h("th", { class: "num" }, "Stock"),
                  h("th", { class: "num" }, "Sold"),
                  h("th", { class: "num" }, "Revenue"),
                  h("th", {}),
                ),
              ),
              h("tbody", {}, ...rows.map(inventoryRow)),
            ),
          ),
    ),
  );
}

function inventoryRow(row) {
  return h(
    "tr",
    {},
    h(
      "td",
      {},
      h("div", {}, row.name),
      h("code", { class: "id" }, shortId(row.id)),
      row.stock <= 0
        ? h("span", { class: "tag out" }, " out of stock")
        : row.lowStock
          ? h("span", { class: "tag low" }, " low stock")
          : null,
    ),
    h("td", { class: "num mono" }, money(row.price)),
    h("td", { class: "num mono" }, String(row.stock)),
    h("td", { class: "num mono" }, String(row.unitsSold)),
    h("td", { class: "num mono" }, money(row.revenue)),
    h(
      "td",
      { class: "num" },
      h(
        "div",
        { class: "row", style: "justify-content:flex-end;gap:6px" },
        h("button", { class: "btn ghost sm", onClick: () => go(`#/items/${row.id}`) }, "View"),
        h(
          "button",
          {
            class: "btn ghost sm",
            onClick: async () => {
              // The inventory summary omits description/unit/category, and the
              // update handler replaces every column, so refetch the full item.
              const full = await guard(() => api.getItem(row.id));
              if (full) openEditor(full);
            },
          },
          "Edit",
        ),
        h(
          "button",
          {
            class: "btn danger sm",
            onClick: async () => {
              if (!confirm(`Delete "${row.name}"? This cannot be undone.`)) return;
              const ok = await guard(() => api.deleteItem(row.id));
              if (ok !== undefined) {
                toast("Item deleted.");
                render();
              }
            },
          },
          "Delete",
        ),
      ),
    ),
  );
}

let editorTarget = null;

function openEditor(item) {
  editorTarget = item;
  render();
  // render() rebuilds the page; scroll the freshly-built form into view.
  requestAnimationFrame(() => document.getElementById("item-editor")?.scrollIntoView({ behavior: "smooth", block: "center" }));
}

// One form for both create (POST /items) and update (PATCH /items/{id}).
// Both handlers require name, price, stock and unit to be non-empty.
function itemEditor() {
  const editing = editorTarget;

  const name = h("input", { type: "text", value: editing?.name ?? "", required: true });
  const price = h("input", { type: "number", min: "0", step: "0.01", value: editing ? String(editing.price) : "", required: true });
  const stock = h("input", { type: "number", min: "0", step: "1", value: editing ? String(editing.stock) : "", required: true });
  const unit = h("input", { type: "text", value: editing?.unit ?? "pcs", required: true });
  const category = h("input", { type: "text", value: editing?.category ?? "" });
  const imageUrl = h("input", { type: "url", value: editing?.imageUrl ?? "", placeholder: "https://…" });
  const description = h("textarea", {}, editing?.description ?? "");

  const submit = h("button", { class: "btn", type: "submit" }, editing ? "Save changes" : "Create item");

  return h(
    "form",
    {
      id: "item-editor",
      class: "card stack",
      onSubmit: async (e) => {
        e.preventDefault();
        const payload = {
          name: name.value.trim(),
          price: price.value,
          stock: stock.value,
          unit: unit.value.trim(),
          category: category.value.trim(),
          imageUrl: imageUrl.value.trim(),
          description: description.value.trim(),
        };
        submit.disabled = true;
        const saved = await guard(() =>
          editing ? api.updateItem(editing.id, payload) : api.createItem(payload),
        );
        submit.disabled = false;
        if (!saved) return;
        toast(editing ? "Item updated." : "Item created.");
        editorTarget = null;
        render();
      },
    },
    h(
      "div",
      { class: "spread" },
      h("h2", { style: "margin:0" }, editing ? `Edit “${editing.name}”` : "New listing"),
      editing
        ? h(
            "button",
            {
              class: "btn ghost sm",
              type: "button",
              onClick: () => {
                editorTarget = null;
                render();
              },
            },
            "Cancel",
          )
        : null,
    ),
    h("div", { class: "row" }, field("Name", name), field("Category", category)),
    h("div", { class: "row" }, field("Price", price), field("Stock", stock), field("Unit", unit)),
    field("Image URL", imageUrl),
    field("Description", description),
    h("div", {}, submit),
  );
}

/* ------------------------------------------------------------------ *
 * seller orders  --  GET /seller/orders
 * ------------------------------------------------------------------ */

async function sellerOrdersView() {
  if (requireAuth()) return;

  const orders = await guard(() => api.sellerOrders());
  if (!orders) {
    view.replaceChildren(empty("Could not load your orders."));
    return;
  }

  const revenue = orders.reduce((sum, o) => sum + o.priceAtPurchase * o.quantity, 0);

  view.replaceChildren(
    h(
      "div",
      { class: "stack" },
      h("div", {}, h("h1", {}, "Orders"), h("p", { class: "sub" }, "Every line item sold from your listings.")),
      h(
        "div",
        { class: "grid" },
        h("div", { class: "card stat" }, h("span", { class: "value" }, String(orders.length)), h("span", { class: "label" }, "Order lines")),
        h("div", { class: "card stat" }, h("span", { class: "value" }, money(revenue)), h("span", { class: "label" }, "Gross value")),
      ),
      orders.length === 0
        ? empty("Nothing sold yet.")
        : h(
            "div",
            { class: "card table-wrap" },
            h(
              "table",
              {},
              h(
                "thead",
                {},
                h(
                  "tr",
                  {},
                  h("th", {}, "When"),
                  h("th", {}, "Item"),
                  h("th", {}, "Buyer"),
                  h("th", { class: "num" }, "Qty"),
                  h("th", { class: "num" }, "Unit price"),
                  h("th", { class: "num" }, "Total"),
                ),
              ),
              h(
                "tbody",
                {},
                ...orders.map((o) =>
                  h(
                    "tr",
                    {},
                    h("td", {}, date(o.createdAt)),
                    h("td", {}, h("div", {}, o.itemName), h("code", { class: "id" }, shortId(o.orderId))),
                    h("td", {}, h("code", { class: "id" }, shortId(o.buyerId))),
                    h("td", { class: "num mono" }, String(o.quantity)),
                    h("td", { class: "num mono" }, money(o.priceAtPurchase)),
                    h("td", { class: "num mono" }, money(o.priceAtPurchase * o.quantity)),
                  ),
                ),
              ),
            ),
          ),
    ),
  );
}

/* ------------------------------------------------------------------ *
 * boot
 * ------------------------------------------------------------------ */

window.addEventListener("hashchange", () => {
  editorTarget = null;
  render();
});
render();
