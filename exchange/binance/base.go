package binance

import "math"

// truncateToStep truncates qty to the nearest multiple of step (floor).
// This satisfies Binance's LOT_SIZE filter without rounding up.
func truncateToStep(qty, step float64) float64 {
	if step <= 0 {
		return qty
	}
	return math.Floor(qty/step) * step
}
