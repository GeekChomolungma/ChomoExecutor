package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/GeekChomolungma/ChomoExecutor/api"
	"github.com/GeekChomolungma/ChomoExecutor/api/handler"
	"github.com/GeekChomolungma/ChomoExecutor/exchange"
	"github.com/GeekChomolungma/ChomoExecutor/exchange/mock"
	"github.com/GeekChomolungma/ChomoExecutor/model"
	"github.com/GeekChomolungma/ChomoExecutor/service"
	"github.com/GeekChomolungma/ChomoExecutor/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type noopStore struct{}

func (n *noopStore) SaveOrderResult(_ context.Context, _ *store.OrderRecord) error { return nil }

func setupRouter(authUID string) http.Handler {
	mockEx := mock.NewExchange(50000.0, 1000.0)
	exchanges := map[string]exchange.Exchange{
		service.RegisterKey("binance", "spot"):    mockEx,
		service.RegisterKey("binance", "futures"): mockEx,
	}
	executor := service.NewExecutor(exchanges, &noopStore{})
	h := handler.NewSignalHandler(executor, authUID)
	return api.NewRouter(h)
}

func postSignal(router http.Handler, sig model.Signal) *httptest.ResponseRecorder {
	body, _ := json.Marshal(sig)
	req := httptest.NewRequest(http.MethodPost, "/v1/signal", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHandleSignal_ValidRequest_Returns200(t *testing.T) {
	router := setupRouter("uid-123")
	rec := postSignal(router, model.Signal{
		Symbol:         "BTCUSDT",
		Side:           "buy",
		Exchange:       "binance",
		MarketType:     "futures",
		InvestmentType: "qty",
		Amount:         "0.01",
		Price:          "market",
		UID:            "uid-123",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["orderId"] != "MOCK-001" {
		t.Errorf("orderId: got %v, want MOCK-001", resp["orderId"])
	}
	if resp["status"] != "FILLED" {
		t.Errorf("status: got %v, want FILLED", resp["status"])
	}
}

func TestHandleSignal_WrongUID_Returns401(t *testing.T) {
	router := setupRouter("uid-123")
	rec := postSignal(router, model.Signal{
		Symbol:         "BTCUSDT",
		Side:           "buy",
		InvestmentType: "qty",
		Amount:         "0.01",
		Price:          "market",
		UID:            "wrong-uid",
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
}

func TestHandleSignal_BadJSON_Returns400(t *testing.T) {
	router := setupRouter("uid-123")
	req := httptest.NewRequest(http.MethodPost, "/v1/signal", bytes.NewReader([]byte(`{invalid}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleSignal_MissingRequiredField_Returns400(t *testing.T) {
	router := setupRouter("uid-123")
	// Amount is required but omitted
	rec := postSignal(router, model.Signal{
		Symbol:         "BTCUSDT",
		Side:           "buy",
		InvestmentType: "qty",
		Price:          "market",
		UID:            "uid-123",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleSignal_UnknownExchange_Returns500(t *testing.T) {
	router := setupRouter("uid-123")
	rec := postSignal(router, model.Signal{
		Symbol:         "BTCUSDT",
		Side:           "buy",
		Exchange:       "okx",
		InvestmentType: "qty",
		Amount:         "0.01",
		Price:          "market",
		UID:            "uid-123",
	})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}
