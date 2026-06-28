# ChomoExecutor

A generic trade execution module that receives upstream trading signals over HTTP, routes them to the appropriate exchange and market type, and persists every order result to MongoDB.

Signals can come from any source — TradingView webhooks, Python ML models, or any system that can send an HTTP POST request. The execution layer is fully decoupled from both the signal source and the storage backend.

## Architecture

```
Any Signal Source (TradingView / Python ML / etc.)
              │  HTTP POST /v1/signal
              ▼
     ┌─────────────────┐
     │   api/handler   │  Parse JSON, authenticate UID
     └────────┬────────┘
              │ model.Signal
              ▼
     ┌─────────────────────────────────────────┐
     │           service/executor              │
     │  Resolve qty · select exchange · route  │
     └──────┬──────────────────────┬───────────┘
            │ model.OrderRequest   │ store.OrderRecord
            ▼                      ▼
   ┌─────────────────┐    ┌──────────────────────┐
   │exchange.Exchange│    │  store.OrderStore    │
   │   (interface)   │    │    (interface)       │
   └────────┬────────┘    └──────────┬───────────┘
       ┌────┴────┐                   │
       ▼         ▼                   ▼
  binance/    binance/         store/mongo
  SpotClient  FuturesClient    (collection routed
                                by marketType)
```

**Two parallel interface boundaries** keep every concrete dependency out of the business layer:

| Interface | Abstraction | Concrete implementations |
|---|---|---|
| `exchange.Exchange` | Where to trade | `binance.SpotClient`, `binance.FuturesClient`, `mock.Exchange` |
| `store.OrderStore` | Where to persist | `mongostore` (spot → `order_results_spot`, futures → `order_results_futures`) |

Both are injected at startup in `main.go`. Adding a new exchange or swapping the database requires zero changes to the executor.

### Design Principles

- The HTTP layer is source-agnostic — any caller that can POST JSON is a valid signal source.
- The executor routes by `exchange` + `marketType` via a registry map. Adding a new venue is a one-liner in `main.go`.
- Neither interface boundary (`Exchange` or `OrderStore`) leaks implementation details upward.
- Persistence is **best-effort**: a MongoDB write failure is logged but does not fail the order response. The order was already submitted to the exchange; returning an error could trigger a retry and cause duplicate orders.
- Spot and Futures are separate clients — no conditional logic inside them. Each handles its own LOT_SIZE precision and average fill price extraction.

## Directory Structure

```
├── main.go
├── config/             Environment variable loader
├── model/              Shared structs (Signal, OrderRequest, OrderResult)
├── exchange/
│   ├── interface.go    Exchange interface definition
│   ├── binance/
│   │   ├── base.go     Shared utilities (LOT_SIZE step truncation)
│   │   ├── client.go   SpotClient + FuturesClient implementations
│   │   └── mapper.go   Field normalisation (side, orderType, positionSide)
│   └── mock/           In-memory mock for local dev and tests
├── store/
│   ├── store.go        OrderStore interface + OrderRecord document struct
│   └── mongo/
│       └── mongo.go    MongoDB implementation, collection routing by marketType
├── service/
│   └── executor.go     Core logic: qty resolution, exchange routing, order dispatch, persistence
└── api/
    ├── router.go
    └── handler/        HTTP handler (parse → auth → executor)
```

## Quick Start

### 1. Set environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `BINANCE_API_KEY` | For live trading | — | Binance API key |
| `BINANCE_SECRET_KEY` | For live trading | — | Binance secret key |
| `AUTH_UID` | Recommended | — | Shared secret; every webhook payload must carry this value |
| `MONGODB_URI` | Optional | — | MongoDB connection URI. Omit to disable persistence |
| `HOST` | Optional | `127.0.0.1` | Bind address. Set to `0.0.0.0` to expose externally |
| `PORT` | Optional | `8080` | Listen port |

When `BINANCE_API_KEY` is not set, the mock exchange is used for all venues — no real orders are placed.

### 2. Run

```bash
go run main.go
```

### 3. Send a test signal

**Futures — open long (100 USDT notional):**
```bash
curl -X POST http://localhost:8080/v1/signal \
  -H "Content-Type: application/json" \
  -d '{
    "symbol":         "BTCUSDT",
    "side":           "buy",
    "exchange":       "binance",
    "marketType":     "futures",
    "investmentType": "notional_value",
    "amount":         "100",
    "price":          "market",
    "uid":            "your_auth_uid"
  }'
```

**Futures — close position safely (reduceOnly prevents accidental reversal):**
```bash
curl -X POST http://localhost:8080/v1/signal \
  -H "Content-Type: application/json" \
  -d '{
    "symbol":         "BTCUSDT",
    "side":           "sell",
    "exchange":       "binance",
    "marketType":     "futures",
    "investmentType": "notional_value",
    "amount":         "100",
    "price":          "market",
    "reduceOnly":     true,
    "uid":            "your_auth_uid"
  }'
```

**Spot — buy 20 USDT worth of BTC:**
```bash
curl -X POST http://localhost:8080/v1/signal \
  -H "Content-Type: application/json" \
  -d '{
    "symbol":         "BTCUSDT",
    "side":           "buy",
    "exchange":       "binance",
    "marketType":     "spot",
    "investmentType": "notional_value",
    "amount":         "20",
    "price":          "market",
    "uid":            "your_auth_uid"
  }'
```

### 4. Health check

```bash
curl http://localhost:8080/health
```

## Signal Fields

| Field | Required | Description |
|---|---|---|
| `symbol` | yes | Trading pair, e.g. `BTCUSDT` |
| `side` | yes | `buy` or `sell` |
| `exchange` | no | Target exchange. Default: `binance` |
| `marketType` | no | `spot` or `futures`. Default: `futures` |
| `investmentType` | yes | `notional_value` (spend N USDT) or `qty` (trade N units directly) |
| `amount` | yes | Amount in the unit specified by `investmentType` |
| `price` | yes | `market` or a numeric limit price string |
| `positionSide` | no | Futures only. `BOTH` for One-way Mode (default), `LONG`/`SHORT` for Hedge Mode |
| `reduceOnly` | no | Futures only. `true` = close-only, prevents accidental position reversal |
| `signalId` | no | Optional trace ID persisted with the order record |
| `uid` | yes | Auth token, must match `AUTH_UID` env var |

### Futures position mode notes

Binance Futures supports two position modes. The mode is set once on the account level:

- **One-way Mode** (default): a symbol can only hold one direction at a time. Use `positionSide: "BOTH"` or omit the field entirely. Closing a position requires `reduceOnly: true` to prevent accidental reversal when the order size exceeds the current holding.
- **Hedge Mode**: long and short can coexist on the same symbol. Use `positionSide: "LONG"` or `"SHORT"` explicitly.

### investmentType behaviour

| Value | What `amount` means | How quantity is resolved |
|---|---|---|
| `notional_value` | USDT to spend | `qty = amount / current_price` (price fetched live) |
| `qty` | Base asset units | Used directly, no conversion |

Quantity is automatically truncated to the exchange's LOT_SIZE step size before submission (fetched once per symbol and cached in-process).

## Order Persistence

Every successfully placed order is persisted to MongoDB as an `OrderRecord` document. The collection is routed by `marketType`:

| marketType | Collection |
|---|---|
| `spot` | `order_results_spot` |
| `futures` | `order_results_futures` |
| other | `order_results_unknown` |

**Document fields:**

| Field | Source |
|---|---|
| `signal_id`, `uid`, `exchange`, `market_type` | Signal context |
| `symbol`, `side`, `order_type`, `quantity`, `price` | Resolved order request |
| `order_id`, `status`, `filled_qty`, `avg_price` | Exchange response |
| `created_at` | Server time (UTC) at execution |

The `avg_price` for MARKET orders is derived from cumulative quote / executed quantity (spot), or via a follow-up order query (futures), since the immediate placement response does not include a fill price.

Set `MONGODB_URI` to enable persistence. Leave it unset to run without a database — the service behaves identically, only omitting the write step.

## Testing

```bash
# Unit tests + HTTP integration tests (no external dependencies)
go test ./service/... ./api/...

# MongoDB integration tests (requires Docker)
go test -tags integration ./store/mongo/...
```

The MongoDB integration tests spin up a real `mongo:7` container via testcontainers-go, verify collection routing, and assert document content. They are gated behind the `integration` build tag so `go test ./...` stays dependency-free.

## Adding a New Exchange

1. Create `exchange/okx/client.go` implementing `exchange.Exchange`.
2. Register it in `main.go`:

```go
exchanges[service.RegisterKey("okx", "spot")] = okx.NewSpotClient(...)
```

3. Send signals with `"exchange": "okx"` — no other files need to change.

## Adding a New Storage Backend

1. Create `store/postgres/postgres.go` implementing `store.OrderStore`.
2. Wire it in `main.go` instead of (or alongside) the MongoDB store.

The executor has no knowledge of the concrete backend — the swap is entirely contained in `main.go`.
