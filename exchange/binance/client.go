package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	futuresclient "github.com/binance/binance-connector-go/clients/derivativestradingusdsfutures"
	futuresmodels "github.com/binance/binance-connector-go/clients/derivativestradingusdsfutures/src/restapi/models"
	spotclient "github.com/binance/binance-connector-go/clients/spot"
	spotmodels "github.com/binance/binance-connector-go/clients/spot/src/restapi/models"
	"github.com/binance/binance-connector-go/common/v2/common"

	"github.com/GeekChomolungma/ChomoExecutor/model"
)

// SpotClient implements exchange.Exchange for Binance Spot via the official connector.
type SpotClient struct {
	api           *spotclient.BinanceSpotClient
	stepSizeCache sync.Map // symbol (string) -> float64
}

func NewSpotClient(apiKey, secretKey string) *SpotClient {
	cfg := common.NewConfigurationRestAPI(
		common.WithBasePath(common.SpotRestApiProdUrl),
		common.WithApiKey(apiKey),
		common.WithApiSecret(secretKey),
	)
	return &SpotClient{
		api: spotclient.NewBinanceSpotClient(spotclient.WithRestAPI(cfg)),
	}
}

// spotLotStep returns the LOT_SIZE stepSize for the symbol, fetching and caching on first call.
func (c *SpotClient) spotLotStep(ctx context.Context, symbol string) (float64, error) {
	if v, ok := c.stepSizeCache.Load(symbol); ok {
		return v.(float64), nil
	}
	resp, err := c.api.RestApi.GeneralAPI.ExchangeInfo(ctx).Symbol(symbol).Execute()
	if err != nil {
		return 0, fmt.Errorf("spot exchange info: %w", err)
	}
	for _, sym := range resp.Data.GetSymbols() {
		for _, f := range sym.GetFilters() {
			if f.LotSizeFilter == nil {
				continue
			}
			step, err := strconv.ParseFloat(f.LotSizeFilter.GetStepSize(), 64)
			if err != nil || step <= 0 {
				continue
			}
			c.stepSizeCache.Store(symbol, step)
			return step, nil
		}
	}
	return 0, fmt.Errorf("spot: LOT_SIZE filter not found for %s", symbol)
}

func (c *SpotClient) PlaceOrder(ctx context.Context, order *model.OrderRequest) (*model.OrderResult, error) {
	step, err := c.spotLotStep(ctx, order.Symbol)
	if err != nil {
		return nil, err
	}
	qty := truncateToStep(order.Quantity, step)

	side, err := toSpotSide(order.Side)
	if err != nil {
		return nil, err
	}
	orderType, err := toSpotType(order.OrderType)
	if err != nil {
		return nil, err
	}

	req := c.api.RestApi.TradeAPI.NewOrder(ctx).
		Symbol(order.Symbol).
		Side(side).
		Type(orderType).
		Quantity(float32(qty)).
		NewOrderRespType(spotmodels.NewOrderNewOrderRespTypeParameterResult)

	if order.OrderType == "LIMIT" {
		req = req.
			Price(float32(order.Price)).
			TimeInForce(spotmodels.NewOrderTimeInForceParameterGtc)
	}

	resp, err := req.Execute()
	if err != nil {
		return nil, fmt.Errorf("spot PlaceOrder: %w", err)
	}

	d := resp.Data
	filledQty, _ := strconv.ParseFloat(d.GetExecutedQty(), 64)

	// MARKET orders return price="0.00000"; derive avg from cummulativeQuoteQty/executedQty.
	avgPrice, _ := strconv.ParseFloat(d.GetPrice(), 64)
	if avgPrice == 0 && filledQty > 0 {
		cumQ, _ := strconv.ParseFloat(d.GetCummulativeQuoteQty(), 64)
		if cumQ > 0 {
			avgPrice = cumQ / filledQty
		}
	}

	return &model.OrderResult{
		OrderID:   strconv.FormatInt(d.GetOrderId(), 10),
		Symbol:    d.GetSymbol(),
		Status:    d.GetStatus(),
		FilledQty: filledQty,
		AvgPrice:  avgPrice,
	}, nil
}

func (c *SpotClient) GetPrice(ctx context.Context, symbol string) (float64, error) {
	resp, err := c.api.RestApi.MarketAPI.TickerPrice(ctx).Symbol(symbol).Execute()
	if err != nil {
		return 0, fmt.Errorf("spot GetPrice: %w", err)
	}

	r1 := resp.Data.TickerPriceResponse1
	if r1 == nil {
		return 0, fmt.Errorf("spot GetPrice: unexpected response type for symbol %s", symbol)
	}

	price, err := strconv.ParseFloat(r1.GetPrice(), 64)
	if err != nil {
		return 0, fmt.Errorf("spot GetPrice parse: %w", err)
	}
	return price, nil
}

func (c *SpotClient) GetBalance(ctx context.Context, asset string) (float64, error) {
	resp, err := c.api.RestApi.AccountAPI.GetAccount(ctx).Execute()
	if err != nil {
		return 0, fmt.Errorf("spot GetBalance: %w", err)
	}

	for _, b := range resp.Data.Balances {
		if b.GetAsset() == asset {
			bal, _ := strconv.ParseFloat(b.GetFree(), 64)
			return bal, nil
		}
	}
	return 0, fmt.Errorf("spot GetBalance: asset %s not found", asset)
}

// FuturesClient implements exchange.Exchange for Binance USDS-M Futures via the official connector.
type FuturesClient struct {
	api           *futuresclient.BinanceDerivativesTradingUsdsFuturesClient
	stepSizeCache sync.Map // symbol (string) -> float64
}

func NewFuturesClient(apiKey, secretKey string) *FuturesClient {
	cfg := common.NewConfigurationRestAPI(
		common.WithBasePath(common.DerivativesTradingUsdsFuturesRestApiProdUrl),
		common.WithApiKey(apiKey),
		common.WithApiSecret(secretKey),
	)
	return &FuturesClient{
		api: futuresclient.NewBinanceDerivativesTradingUsdsFuturesClient(futuresclient.WithRestAPI(cfg)),
	}
}

// futuresLotStep returns the LOT_SIZE stepSize for the symbol, fetching and caching on first call.
// Futures exchange info has no per-symbol filter on the request, so we fetch all and scan.
func (c *FuturesClient) futuresLotStep(ctx context.Context, symbol string) (float64, error) {
	if v, ok := c.stepSizeCache.Load(symbol); ok {
		return v.(float64), nil
	}
	resp, err := c.api.RestApi.MarketDataAPI.ExchangeInformation(ctx).Execute()
	if err != nil {
		return 0, fmt.Errorf("futures exchange info: %w", err)
	}
	for _, sym := range resp.Data.GetSymbols() {
		if sym.GetSymbol() != symbol {
			continue
		}
		for _, f := range sym.GetFilters() {
			if f.GetFilterType() != "LOT_SIZE" {
				continue
			}
			step, err := strconv.ParseFloat(f.GetStepSize(), 64)
			if err != nil || step <= 0 {
				continue
			}
			c.stepSizeCache.Store(symbol, step)
			return step, nil
		}
	}
	return 0, fmt.Errorf("futures: LOT_SIZE filter not found for %s", symbol)
}

func (c *FuturesClient) PlaceOrder(ctx context.Context, order *model.OrderRequest) (*model.OrderResult, error) {
	step, err := c.futuresLotStep(ctx, order.Symbol)
	if err != nil {
		return nil, err
	}
	qty := truncateToStep(order.Quantity, step)

	side, err := toFuturesSide(order.Side)
	if err != nil {
		return nil, err
	}

	req := c.api.RestApi.TradeAPI.NewOrder(ctx).
		Symbol(order.Symbol).
		Side(side).
		Type(strings.ToUpper(order.OrderType)).
		Quantity(float32(qty)).
		NewOrderRespType(futuresmodels.NewAlgoOrderNewOrderRespTypeParameterResult)

	if order.PositionSide != "" {
		ps, err := toFuturesPositionSide(order.PositionSide)
		if err != nil {
			return nil, err
		}
		req = req.PositionSide(ps)
	}
	if order.ReduceOnly {
		req = req.ReduceOnly("true")
	}
	if order.OrderType == "LIMIT" {
		req = req.
			Price(float32(order.Price)).
			TimeInForce(futuresmodels.NewAlgoOrderTimeInForceParameterGtc)
	}

	resp, err := req.Execute()
	if err != nil {
		return nil, fmt.Errorf("futures PlaceOrder: %w", err)
	}

	d := resp.Data
	filledQty, _ := strconv.ParseFloat(d.GetExecutedQty(), 64)

	// avgPrice is absent from the immediate PlaceOrder response for MARKET orders.
	// Query the order to get the definitive fill price.
	avgPrice, _ := strconv.ParseFloat(d.GetAvgPrice(), 64)
	if avgPrice == 0 && filledQty > 0 {
		if qResp, qErr := c.api.RestApi.TradeAPI.QueryOrder(ctx).
			Symbol(order.Symbol).
			OrderId(d.GetOrderId()).
			Execute(); qErr == nil && qResp != nil {
			avgPrice, _ = strconv.ParseFloat(qResp.Data.GetAvgPrice(), 64)
		}
	}

	return &model.OrderResult{
		OrderID:   strconv.FormatInt(d.GetOrderId(), 10),
		Symbol:    d.GetSymbol(),
		Status:    d.GetStatus(),
		FilledQty: filledQty,
		AvgPrice:  avgPrice,
	}, nil
}

func (c *FuturesClient) GetPrice(ctx context.Context, symbol string) (float64, error) {
	resp, err := c.api.RestApi.MarketDataAPI.SymbolPriceTickerV2(ctx).Symbol(symbol).Execute()
	if err != nil {
		return 0, fmt.Errorf("futures GetPrice: %w", err)
	}

	r1 := resp.Data.SymbolPriceTickerV2Response1
	if r1 == nil {
		return 0, fmt.Errorf("futures GetPrice: unexpected response type for symbol %s", symbol)
	}

	price, err := strconv.ParseFloat(r1.GetPrice(), 64)
	if err != nil {
		return 0, fmt.Errorf("futures GetPrice parse: %w", err)
	}
	return price, nil
}

func (c *FuturesClient) GetBalance(ctx context.Context, asset string) (float64, error) {
	resp, err := c.api.RestApi.AccountAPI.FuturesAccountBalanceV3(ctx).Execute()
	if err != nil {
		return 0, fmt.Errorf("futures GetBalance: %w", err)
	}

	for _, b := range resp.Data.Items {
		if b.GetAsset() == asset {
			bal, _ := strconv.ParseFloat(b.GetAvailableBalance(), 64)
			return bal, nil
		}
	}
	return 0, fmt.Errorf("futures GetBalance: asset %s not found", asset)
}
