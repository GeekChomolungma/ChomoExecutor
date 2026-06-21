# ChomoExecutor

A generic trade execution module that receives upstream trading signals over HTTP and routes them to the appropriate exchange and market type.

Signals can come from any source — TradingView webhooks, Python ML models, or any system that can send an HTTP POST request. The execution layer is fully decoupled from the signal source.

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
     ┌─────────────────┐
     │service/executor │  Resolve qty, select exchange by
     │                 │  (exchange + marketType) registry key
     └────────┬────────┘
              │ model.OrderRequest
              ▼
     ┌─────────────────┐
     │exchange.Exchange│  Interface boundary — nothing above
     │   (interface)   │  this line knows which venue is used
     └────────┬────────┘
         ┌────┴────┐
         ▼         ▼
    binance/    binance/      ← add okx/, bybit/ here
    SpotClient  FuturesClient   without touching any other layer
```

### Design Principles

- The HTTP layer is source-agnostic — any caller that can POST JSON is a valid signal source.
- The executor routes by `exchange` + `marketType` via a registry map. Adding a new venue is a one-liner in `main.go`.
- The `exchange.Exchange` interface is the only boundary. Binance-specific code never leaks upward.
- Spot and Futures are separate clients under the same package — no conditional logic inside them.

## Directory Structure

```
├── main.go
├── config/             Environment variable loader
├── model/              Shared structs (Signal, OrderRequest, OrderResult)
├── exchange/
│   ├── interface.go    Exchange interface definition
│   ├── binance/
│   │   ├── base.go     Shared HMAC-SHA256 signing + HTTP helpers
│   │   ├── client.go   SpotClient + FuturesClient implementations
│   │   └── mapper.go   Field normalisation utilities
│   └── mock/           In-memory mock for local dev and tests
├── service/
│   └── executor.go     Core logic: qty resolution, exchange routing, order dispatch
└── api/
    ├── router.go
    └── handler/        HTTP handler (parse → auth → executor)
```

## Quick Start

### 1. Install dependencies

```bash
go mod tidy
```

### 2. Set environment variables

```bash
# Required for live trading (omit to run with the mock exchange)
export BINANCE_API_KEY=your_api_key
export BINANCE_SECRET_KEY=your_secret_key

# Optional
export AUTH_UID=bcb5ba0369fceede2427d939092c0db4b13f819873a6a0cb4e28ea4b32cc08a5
export PORT=8080
```

### 3. Run

```bash
go run main.go
```

When `BINANCE_API_KEY` is not set, the mock exchange is used for all venues — no real orders are placed.

### 4. Send a test signal

```bash
# Futures order (default when exchange/marketType are omitted)
curl -X POST http://localhost:8080/v1/signal \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "BTCUSDT",
    "side": "buy",
    "exchange": "binance",
    "marketType": "futures",
    "positionSide": "BOTH",
    "investmentType": "notional_value",
    "amount": "100",
    "price": "market",
    "reduceOnly": false,
    "positionMode": "one_way_mode",
    "signalId": "88bd83f5-b2e3-4d",
    "uid": "your_auth_uid"
  }'

# Spot order
curl -X POST http://localhost:8080/v1/signal \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "BTCUSDT",
    "side": "buy",
    "exchange": "binance",
    "marketType": "spot",
    "investmentType": "notional_value",
    "amount": "100",
    "price": "market",
    "uid": "your_auth_uid"
  }'
```

### 5. Health check

```bash
curl http://localhost:8080/health
```

## Signal Fields

| Field | Required | Description |
| --- | --- | --- |
| `symbol` | yes | Trading pair, e.g. `BTCUSDT` |
| `side` | yes | `buy` or `sell` |
| `exchange` | no | Target exchange. Default: `binance` |
| `marketType` | no | `spot` or `futures`. Default: `futures` |
| `investmentType` | yes | `notional_value` (USDT amount) or `qty` (base quantity) |
| `amount` | yes | Amount in the unit specified by `investmentType` |
| `price` | yes | `market` or a numeric limit price string |
| `positionSide` | no | `BOTH` / `LONG` / `SHORT`. Futures only |
| `reduceOnly` | no | `true` / `false`. Futures only |
| `uid` | yes | Auth token, must match `AUTH_UID` env var |

## Adding a New Exchange

1. Create `exchange/okx/client.go` implementing `exchange.Exchange`.
1. Register it in `main.go`:

```go
exchanges[service.RegisterKey("okx", "spot")] = okx.NewSpotClient(...)
```

1. Send signals with `"exchange": "okx"` — no other files need to change.

## Extension Points

- **Database layer**: inject a repository interface into `service/executor.go` to persist signals and order history — the layering keeps this entirely opt-in.
- **Multi-source auth**: extend `api/handler/signal.go` to support per-source API keys or HMAC verification without touching the executor or exchange layers.
