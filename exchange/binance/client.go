package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/GeekChomolungma/ChomoExecutor/model"
)

const (
	spotBaseURL    = "https://api.binance.com"
	futuresBaseURL = "https://fapi.binance.com"
)

// SpotClient calls the Binance Spot REST API (/api/v3/...).
type SpotClient struct {
	baseClient
}

func NewSpotClient(apiKey, secretKey string) *SpotClient {
	return &SpotClient{newBaseClient(apiKey, secretKey, spotBaseURL)}
}

func (c *SpotClient) PlaceOrder(ctx context.Context, order *model.OrderRequest) (*model.OrderResult, error) {
	side, err := toSide(order.Side)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("symbol", order.Symbol)
	params.Set("side", side)
	params.Set("type", order.OrderType)
	params.Set("quantity", fmtQty(order.Quantity))
	params.Set("newOrderRespType", "RESULT")
	if order.OrderType == "LIMIT" {
		params.Set("price", fmtPrice(order.Price))
		params.Set("timeInForce", "GTC")
	}

	body, err := c.signedPOST(ctx, "/api/v3/order", params)
	if err != nil {
		return nil, fmt.Errorf("spot PlaceOrder: %w", err)
	}
	return parseOrderResult(body)
}

func (c *SpotClient) GetPrice(ctx context.Context, symbol string) (float64, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	body, err := c.publicGET(ctx, "/api/v3/ticker/price", params)
	if err != nil {
		return 0, fmt.Errorf("spot GetPrice: %w", err)
	}
	return parsePriceResponse(body)
}

func (c *SpotClient) GetBalance(ctx context.Context, asset string) (float64, error) {
	body, err := c.signedGET(ctx, "/api/v3/account", url.Values{})
	if err != nil {
		return 0, fmt.Errorf("spot GetBalance: %w", err)
	}

	var resp struct {
		Balances []struct {
			Asset string `json:"asset"`
			Free  string `json:"free"`
		} `json:"balances"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("spot GetBalance parse: %w", err)
	}
	for _, b := range resp.Balances {
		if b.Asset == asset {
			bal, _ := strconv.ParseFloat(b.Free, 64)
			return bal, nil
		}
	}
	return 0, fmt.Errorf("spot GetBalance: asset %s not found", asset)
}

// FuturesClient calls the Binance USDM Futures REST API (/fapi/v1/...).
type FuturesClient struct {
	baseClient
}

func NewFuturesClient(apiKey, secretKey string) *FuturesClient {
	return &FuturesClient{newBaseClient(apiKey, secretKey, futuresBaseURL)}
}

func (c *FuturesClient) PlaceOrder(ctx context.Context, order *model.OrderRequest) (*model.OrderResult, error) {
	side, err := toSide(order.Side)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("symbol", order.Symbol)
	params.Set("side", side)
	params.Set("positionSide", order.PositionSide)
	params.Set("type", order.OrderType)
	params.Set("quantity", fmtQty(order.Quantity))
	params.Set("newOrderRespType", "RESULT")
	if order.ReduceOnly {
		params.Set("reduceOnly", "true")
	}
	if order.OrderType == "LIMIT" {
		params.Set("price", fmtPrice(order.Price))
		params.Set("timeInForce", "GTC")
	}

	body, err := c.signedPOST(ctx, "/fapi/v1/order", params)
	if err != nil {
		return nil, fmt.Errorf("futures PlaceOrder: %w", err)
	}
	return parseOrderResult(body)
}

func (c *FuturesClient) GetPrice(ctx context.Context, symbol string) (float64, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	body, err := c.publicGET(ctx, "/fapi/v1/ticker/price", params)
	if err != nil {
		return 0, fmt.Errorf("futures GetPrice: %w", err)
	}
	return parsePriceResponse(body)
}

func (c *FuturesClient) GetBalance(ctx context.Context, asset string) (float64, error) {
	body, err := c.signedGET(ctx, "/fapi/v2/account", url.Values{})
	if err != nil {
		return 0, fmt.Errorf("futures GetBalance: %w", err)
	}

	var resp struct {
		Assets []struct {
			Asset            string `json:"asset"`
			AvailableBalance string `json:"availableBalance"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("futures GetBalance parse: %w", err)
	}
	for _, a := range resp.Assets {
		if a.Asset == asset {
			bal, _ := strconv.ParseFloat(a.AvailableBalance, 64)
			return bal, nil
		}
	}
	return 0, fmt.Errorf("futures GetBalance: asset %s not found", asset)
}

// --- shared response parsers ---

func parseOrderResult(body []byte) (*model.OrderResult, error) {
	var resp struct {
		OrderID     int64  `json:"orderId"`
		Symbol      string `json:"symbol"`
		Status      string `json:"status"`
		ExecutedQty string `json:"executedQty"`
		AvgPrice    string `json:"avgPrice"`
		Price       string `json:"price"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parseOrderResult: %w", err)
	}
	filledQty, _ := strconv.ParseFloat(resp.ExecutedQty, 64)
	// futures returns avgPrice; spot returns price
	rawPrice := resp.AvgPrice
	if rawPrice == "" || rawPrice == "0" {
		rawPrice = resp.Price
	}
	avgPrice, _ := strconv.ParseFloat(rawPrice, 64)
	return &model.OrderResult{
		OrderID:   strconv.FormatInt(resp.OrderID, 10),
		Symbol:    resp.Symbol,
		Status:    resp.Status,
		FilledQty: filledQty,
		AvgPrice:  avgPrice,
	}, nil
}

func parsePriceResponse(body []byte) (float64, error) {
	var resp struct {
		Price string `json:"price"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("parsePriceResponse: %w", err)
	}
	price, err := strconv.ParseFloat(resp.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("parsePriceResponse convert: %w", err)
	}
	return price, nil
}
