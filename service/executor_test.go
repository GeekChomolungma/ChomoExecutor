package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/GeekChomolungma/ChomoExecutor/exchange"
	"github.com/GeekChomolungma/ChomoExecutor/exchange/mock"
	"github.com/GeekChomolungma/ChomoExecutor/model"
	"github.com/GeekChomolungma/ChomoExecutor/service"
	"github.com/GeekChomolungma/ChomoExecutor/store"
)

// spyOrderStore records every SaveOrderResult call for assertion.
type spyOrderStore struct {
	records []*store.OrderRecord
	err     error
}

func (s *spyOrderStore) SaveOrderResult(_ context.Context, r *store.OrderRecord) error {
	s.records = append(s.records, r)
	return s.err
}

func newTestExecutor(spy store.OrderStore) *service.Executor {
	mockEx := mock.NewExchange(50000.0, 1000.0)
	exchanges := map[string]exchange.Exchange{
		service.RegisterKey("binance", "spot"):    mockEx,
		service.RegisterKey("binance", "futures"): mockEx,
	}
	return service.NewExecutor(exchanges, spy)
}

func baseSignal() *model.Signal {
	return &model.Signal{
		Symbol:         "BTCUSDT",
		Side:           "buy",
		Exchange:       "binance",
		MarketType:     "futures",
		PositionSide:   "LONG",
		InvestmentType: "qty",
		Amount:         "0.01",
		Price:          "market",
		UID:            "test-uid",
		SignalID:       "sig-001",
	}
}

func TestExecute_HappyPath(t *testing.T) {
	spy := &spyOrderStore{}
	executor := newTestExecutor(spy)

	result, err := executor.Execute(context.Background(), baseSignal())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OrderID != "MOCK-001" {
		t.Errorf("orderID: got %s, want MOCK-001", result.OrderID)
	}
	if len(spy.records) != 1 {
		t.Fatalf("persisted records: got %d, want 1", len(spy.records))
	}
	rec := spy.records[0]
	if rec.MarketType != "futures" {
		t.Errorf("record MarketType: got %s, want futures", rec.MarketType)
	}
	if rec.SignalID != "sig-001" {
		t.Errorf("record SignalID: got %s, want sig-001", rec.SignalID)
	}
	if rec.Quantity != 0.01 {
		t.Errorf("record Quantity: got %f, want 0.01", rec.Quantity)
	}
}

// A store failure must not be confused with an order failure —
// the order was already submitted to the exchange.
func TestExecute_PersistError_StillReturnsResult(t *testing.T) {
	spy := &spyOrderStore{err: errors.New("mongo down")}
	executor := newTestExecutor(spy)

	result, err := executor.Execute(context.Background(), baseSignal())
	if err != nil {
		t.Fatalf("store error must not propagate to caller: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result even when store fails")
	}
}

func TestExecute_NilStore_NoPanic(t *testing.T) {
	executor := newTestExecutor(nil)
	_, err := executor.Execute(context.Background(), baseSignal())
	if err != nil {
		t.Fatalf("unexpected error with nil store: %v", err)
	}
}

func TestExecute_UnknownExchange_ReturnsError(t *testing.T) {
	executor := newTestExecutor(&spyOrderStore{})
	sig := baseSignal()
	sig.Exchange = "okx"
	_, err := executor.Execute(context.Background(), sig)
	if err == nil {
		t.Fatal("expected error for unregistered exchange")
	}
}

func TestExecute_InvalidAmount_ReturnsError(t *testing.T) {
	executor := newTestExecutor(&spyOrderStore{})
	sig := baseSignal()
	sig.Amount = "not-a-number"
	_, err := executor.Execute(context.Background(), sig)
	if err == nil {
		t.Fatal("expected error for invalid amount")
	}
}

func TestExecute_NotionalValue_ResolvesQty(t *testing.T) {
	spy := &spyOrderStore{}
	executor := newTestExecutor(spy)
	sig := baseSignal()
	sig.InvestmentType = "notional_value"
	sig.Amount = "500" // 500 USDT / 50000 price = 0.01 BTC

	_, err := executor.Execute(context.Background(), sig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(spy.records))
	}
	if spy.records[0].Quantity != 0.01 {
		t.Errorf("resolved qty: got %f, want 0.01", spy.records[0].Quantity)
	}
}
