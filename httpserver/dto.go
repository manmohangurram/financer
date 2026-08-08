package httpserver

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"time"

	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/services"
)

// dto.go holds shared wire helpers used across feature handler files.
// Each feature's request types, response types, and mappers live in its own
// handler file (e.g. account_handlers.go), not here.

func queryInt(q url.Values, key string) int32 {
	if v := q.Get(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(n)
		}
	}
	return 0
}

func queryFloat(q url.Values, key string) float32 {
	if v := q.Get(key); v != "" {
		if n, err := strconv.ParseFloat(v, 32); err == nil {
			return float32(n)
		}
	}
	return 0
}

func queryDate(q url.Values, key string) (time.Time, error) {
	if s := q.Get(key); s != "" {
		return time.Parse("2006-01-02", s)
	}
	return time.Time{}, nil
}

type wireBulk struct {
	Success   bool     `json:"success"`
	Message   string   `json:"message"`
	FailedIds []string `json:"failedIds"`
}

func bulkWire(b *api.BulkOperationResponse) wireBulk {
	return wireBulk{
		Success:   b.Success,
		Message:   b.Message,
		FailedIds: b.FailedIds,
	}
}

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
