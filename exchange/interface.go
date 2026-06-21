package exchange

import (
	"context"

	"github.com/GeekChomolungma/ChomoExecutor/model"
)

// Exchange is the single abstraction boundary between business logic and any
// concrete trading venue. The service layer only depends on this interface.
type Exchange interface {
	// PlaceOrder submits a normalized order and returns the venue's response.
	PlaceOrder(ctx context.Context, order *model.OrderRequest) (*model.OrderResult, error)

	// GetPrice returns the latest mark/last price for the given symbol.
	// Used by the executor to convert notional_value to quantity.
	GetPrice(ctx context.Context, symbol string) (float64, error)

	// GetBalance returns the available balance for the given asset (e.g. "USDT").
	GetBalance(ctx context.Context, asset string) (float64, error)
}
