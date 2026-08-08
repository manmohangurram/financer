package services

import "math"

// round2f rounds a monetary value to 2 decimal places to avoid float drift
// when amounts are stored and summed.
func round2f(v float32) float32 {
	return float32(math.Round(float64(v)*100) / 100)
}
