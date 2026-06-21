package binance

import (
	"fmt"
	"strings"
)

func toSide(side string) (string, error) {
	switch strings.ToUpper(side) {
	case "BUY":
		return "BUY", nil
	case "SELL":
		return "SELL", nil
	default:
		return "", fmt.Errorf("unknown side: %s", side)
	}
}

func fmtQty(qty float64) string {
	return fmt.Sprintf("%.4f", qty)
}

func fmtPrice(price float64) string {
	return fmt.Sprintf("%.4f", price)
}
