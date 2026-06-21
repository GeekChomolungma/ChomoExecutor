package binance

import (
	"fmt"
	"strings"

	futuresmodels "github.com/binance/binance-connector-go/clients/derivativestradingusdsfutures/src/restapi/models"
	spotmodels "github.com/binance/binance-connector-go/clients/spot/src/restapi/models"
)

func toSpotSide(side string) (spotmodels.NewOrderSideParameter, error) {
	switch strings.ToUpper(side) {
	case "BUY":
		return spotmodels.NewOrderSideParameterBuy, nil
	case "SELL":
		return spotmodels.NewOrderSideParameterSell, nil
	default:
		return "", fmt.Errorf("unknown spot side: %s", side)
	}
}

func toSpotType(orderType string) (spotmodels.NewOrderTypeParameter, error) {
	switch strings.ToUpper(orderType) {
	case "MARKET":
		return spotmodels.NewOrderTypeParameterMarket, nil
	case "LIMIT":
		return spotmodels.NewOrderTypeParameterLimit, nil
	default:
		return "", fmt.Errorf("unsupported spot order type: %s", orderType)
	}
}

func toFuturesSide(side string) (futuresmodels.NewAlgoOrderSideParameter, error) {
	switch strings.ToUpper(side) {
	case "BUY":
		return futuresmodels.NewAlgoOrderSideParameterBuy, nil
	case "SELL":
		return futuresmodels.NewAlgoOrderSideParameterSell, nil
	default:
		return "", fmt.Errorf("unknown futures side: %s", side)
	}
}

func toFuturesPositionSide(ps string) (futuresmodels.NewAlgoOrderPositionSideParameter, error) {
	switch strings.ToUpper(ps) {
	case "BOTH":
		return futuresmodels.NewAlgoOrderPositionSideParameterBoth, nil
	case "LONG":
		return futuresmodels.NewAlgoOrderPositionSideParameterLong, nil
	case "SHORT":
		return futuresmodels.NewAlgoOrderPositionSideParameterShort, nil
	default:
		return "", fmt.Errorf("unknown futures positionSide: %s", ps)
	}
}
