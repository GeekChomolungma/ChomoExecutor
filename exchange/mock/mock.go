package mock

import (
	"context"
	"fmt"
	"log"

	"github.com/GeekChomolungma/ChomoExecutor/model"
)

// Exchange is a fake exchange used in tests and local development.
// It logs every call and returns synthetic data without hitting any real API.
type Exchange struct {
	MockPrice   float64
	MockBalance float64
}

func NewExchange(mockPrice, mockBalance float64) *Exchange {
	return &Exchange{MockPrice: mockPrice, MockBalance: mockBalance}
}

func (m *Exchange) PlaceOrder(_ context.Context, order *model.OrderRequest) (*model.OrderResult, error) {
	log.Printf("[mock] PlaceOrder: %+v", order)
	return &model.OrderResult{
		OrderID:   "MOCK-001",
		Symbol:    order.Symbol,
		Status:    "FILLED",
		FilledQty: order.Quantity,
		AvgPrice:  m.MockPrice,
	}, nil
}

func (m *Exchange) GetPrice(_ context.Context, symbol string) (float64, error) {
	log.Printf("[mock] GetPrice: %s -> %.4f", symbol, m.MockPrice)
	if m.MockPrice <= 0 {
		return 0, fmt.Errorf("mock: price not configured")
	}
	return m.MockPrice, nil
}

func (m *Exchange) GetBalance(_ context.Context, asset string) (float64, error) {
	log.Printf("[mock] GetBalance: %s -> %.4f", asset, m.MockBalance)
	return m.MockBalance, nil
}
