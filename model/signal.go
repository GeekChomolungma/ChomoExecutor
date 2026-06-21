package model

// Signal is the raw webhook payload from TradingView (or any signal provider).
type Signal struct {
	Symbol         string `json:"symbol" binding:"required"`
	Side           string `json:"side" binding:"required"`           // "buy" | "sell"
	Exchange       string `json:"exchange"`                          // "binance" | "okx" | ..., defaults to "binance"
	MarketType     string `json:"marketType"`                        // "spot" | "futures", defaults to "futures"
	PositionSide   string `json:"positionSide"`                      // "BOTH" | "LONG" | "SHORT" (futures only)
	InvestmentType string `json:"investmentType" binding:"required"` // "notional_value" | "qty"
	Amount         string `json:"amount" binding:"required"`
	Price          string `json:"price" binding:"required"` // "market" | numeric string
	ReduceOnly     bool   `json:"reduceOnly"`               // futures only
	PositionMode   string `json:"positionMode"`             // "one_way_mode" | "hedge_mode" (futures only)
	SignalID       string `json:"signalId"`
	UID            string `json:"uid" binding:"required"`
}

// OrderRequest is the normalized, exchange-agnostic order struct.
// The executor builds this from a Signal; the exchange layer consumes it.
type OrderRequest struct {
	Symbol       string
	Side         string  // "BUY" | "SELL"
	PositionSide string  // "BOTH" | "LONG" | "SHORT"
	OrderType    string  // "MARKET" | "LIMIT"
	Quantity     float64 // already resolved from notional_value if needed
	Price        float64 // 0 for market orders
	ReduceOnly   bool
}

// OrderResult is the exchange-agnostic response after placing an order.
type OrderResult struct {
	OrderID   string
	Symbol    string
	Status    string
	FilledQty float64
	AvgPrice  float64
}
