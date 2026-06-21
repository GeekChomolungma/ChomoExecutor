package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	futuresclient "github.com/binance/binance-connector-go/clients/derivativestradingusdsfutures"
	futuresmodels "github.com/binance/binance-connector-go/clients/derivativestradingusdsfutures/src/restapi/models"
	spotclient "github.com/binance/binance-connector-go/clients/spot"
	spotmodels "github.com/binance/binance-connector-go/clients/spot/src/restapi/models"
	"github.com/binance/binance-connector-go/common/v2/common"

	"github.com/GeekChomolungma/ChomoExecutor/model"
)

// SpotClient implements exchange.Exchange for Binance Spot via the official connector.
type SpotClient struct {
	api *spotclient.BinanceSpotClient
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

func (c *SpotClient) PlaceOrder(ctx context.Context, order *model.OrderRequest) (*model.OrderResult, error) {
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
		Quantity(float32(order.Quantity)).
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

	// MARKET orders return price="0"; derive avg from cummulative quote qty
	var avgPrice float64
	if p := d.GetPrice(); p != "" && p != "0" {
		avgPrice, _ = strconv.ParseFloat(p, 64)
	} else if filledQty > 0 {
		if cumQ, _ := strconv.ParseFloat(d.GetCummulativeQuoteQty(), 64); cumQ > 0 {
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
	api *futuresclient.BinanceDerivativesTradingUsdsFuturesClient
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

func (c *FuturesClient) PlaceOrder(ctx context.Context, order *model.OrderRequest) (*model.OrderResult, error) {
	side, err := toFuturesSide(order.Side)
	if err != nil {
		return nil, err
	}

	req := c.api.RestApi.TradeAPI.NewOrder(ctx).
		Symbol(order.Symbol).
		Side(side).
		Type(strings.ToUpper(order.OrderType)).
		Quantity(float32(order.Quantity)).
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
	rawPrice := d.GetAvgPrice()
	if rawPrice == "" || rawPrice == "0" {
		rawPrice = d.GetPrice()
	}
	avgPrice, _ := strconv.ParseFloat(rawPrice, 64)

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
