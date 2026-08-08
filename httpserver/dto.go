package httpserver

import (
	"fmt"
	"math"
	"time"

	"github.com/mohan9182/financer/services"
)

// dto.go holds shared wire helpers used across feature handler files.
// Each feature's request types, response types, and mappers live in its own
// handler file (e.g. account_handlers.go), not here.

func tsRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// cents rounds a money value to the nearest cent so float32 storage noise
// doesn't leak into the wire.
func cents(v float32) float64 {
	return math.Round(float64(v)*100) / 100
}

type wireOp struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func bad(format string, a ...any) error {
	return &services.APIError{Status: 400, Msg: fmt.Sprintf(format, a...)}
}
