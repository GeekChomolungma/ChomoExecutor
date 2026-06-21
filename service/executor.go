package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/GeekChomolungma/ChomoExecutor/exchange"
	"github.com/GeekChomolungma/ChomoExecutor/model"
)

// Executor is the core business layer. It holds a registry of exchanges keyed
// by "venue_markettype" (e.g. "binance_futures", "okx_spot") so any combination
// can be added without touching this file.
type Executor struct {
	exchanges map[string]exchange.Exchange
}

func NewExecutor(exchanges map[string]exchange.Exchange) *Executor {
	return &Executor{exchanges: exchanges}
}

// RegisterKey builds the lookup key used in the exchange registry.
func RegisterKey(venue, marketType string) string {
	return strings.ToLower(venue) + "_" + strings.ToLower(marketType)
}

// Execute validates the signal, resolves quantity, then places the order.
func (e *Executor) Execute(ctx context.Context, sig *model.Signal) (*model.OrderResult, error) {
	ex, err := e.selectExchange(sig.Exchange, sig.MarketType)
	if err != nil {
		return nil, err
	}

	order, err := buildOrder(ctx, ex, sig)
	if err != nil {
		return nil, fmt.Errorf("executor build order: %w", err)
	}

	result, err := ex.PlaceOrder(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("executor place order: %w", err)
	}
	return result, nil
}

// selectExchange resolves venue + marketType to a concrete Exchange.
// Defaults: venue="binance", marketType="futures".
func (e *Executor) selectExchange(venue, marketType string) (exchange.Exchange, error) {
	if venue == "" {
		venue = "binance"
	}
	if marketType == "" {
		marketType = "futures"
	}
	key := RegisterKey(venue, marketType)
	ex, ok := e.exchanges[key]
	if !ok {
		return nil, fmt.Errorf("no exchange registered for %q (available: %v)", key, e.registeredKeys())
	}
	return ex, nil
}

func (e *Executor) registeredKeys() []string {
	keys := make([]string, 0, len(e.exchanges))
	for k := range e.exchanges {
		keys = append(keys, k)
	}
	return keys
}

// buildOrder converts a Signal into an exchange-agnostic OrderRequest.
func buildOrder(ctx context.Context, ex exchange.Exchange, sig *model.Signal) (*model.OrderRequest, error) {
	amount, err := strconv.ParseFloat(sig.Amount, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %s", sig.Amount)
	}

	qty, err := resolveQuantity(ctx, ex, sig.Symbol, sig.InvestmentType, amount)
	if err != nil {
		return nil, err
	}

	orderType, price, err := resolvePrice(sig.Price)
	if err != nil {
		return nil, err
	}

	return &model.OrderRequest{
		Symbol:       strings.ToUpper(sig.Symbol),
		Side:         strings.ToUpper(sig.Side),
		PositionSide: sig.PositionSide,
		OrderType:    orderType,
		Quantity:     qty,
		Price:        price,
		ReduceOnly:   sig.ReduceOnly,
	}, nil
}

func resolveQuantity(ctx context.Context, ex exchange.Exchange, symbol, investmentType string, amount float64) (float64, error) {
	switch investmentType {
	case "notional_value":
		price, err := ex.GetPrice(ctx, strings.ToUpper(symbol))
		if err != nil {
			return 0, fmt.Errorf("resolveQuantity: %w", err)
		}
		if price <= 0 {
			return 0, fmt.Errorf("resolveQuantity: invalid price %.4f for %s", price, symbol)
		}
		return amount / price, nil
	case "qty":
		return amount, nil
	default:
		return 0, fmt.Errorf("unknown investmentType: %s", investmentType)
	}
}

func resolvePrice(priceStr string) (string, float64, error) {
	if strings.ToLower(priceStr) == "market" {
		return "MARKET", 0, nil
	}
	p, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid price: %s", priceStr)
	}
	return "LIMIT", p, nil
}
