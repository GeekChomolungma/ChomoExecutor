package store

import (
	"context"
	"time"

	"github.com/GeekChomolungma/ChomoExecutor/model"
)

// OrderStore is the persistence abstraction for order results.
// The executor depends only on this interface; concrete implementations live in sub-packages.
type OrderStore interface {
	SaveOrderResult(ctx context.Context, record *OrderRecord) error
}

// OrderRecord is the document persisted after each order execution.
// It combines the normalized result with enough signal context to reconstruct what triggered the order.
type OrderRecord struct {
	// Signal context
	SignalID     string    `bson:"signal_id"`
	UID          string    `bson:"uid"`
	Exchange     string    `bson:"exchange"`
	MarketType   string    `bson:"market_type"`
	Symbol       string    `bson:"symbol"`
	Side         string    `bson:"side"`
	PositionSide string    `bson:"position_side"`
	OrderType    string    `bson:"order_type"`
	Quantity     float64   `bson:"quantity"`
	Price        float64   `bson:"price"`
	CreatedAt    time.Time `bson:"created_at"`

	// Exchange response (normalized by the exchange layer)
	OrderID   string  `bson:"order_id"`
	Status    string  `bson:"status"`
	FilledQty float64 `bson:"filled_qty"`
	AvgPrice  float64 `bson:"avg_price"`
}

// NewOrderRecord merges a Signal and its OrderRequest + OrderResult into a single record.
func NewOrderRecord(sig *model.Signal, req *model.OrderRequest, result *model.OrderResult) *OrderRecord {
	return &OrderRecord{
		SignalID:     sig.SignalID,
		UID:          sig.UID,
		Exchange:     sig.Exchange,
		MarketType:   sig.MarketType,
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSide: req.PositionSide,
		OrderType:    req.OrderType,
		Quantity:     req.Quantity,
		Price:        req.Price,
		CreatedAt:    time.Now().UTC(),
		OrderID:      result.OrderID,
		Status:       result.Status,
		FilledQty:    result.FilledQty,
		AvgPrice:     result.AvgPrice,
	}
}
