// Package cursor encodes opaque keyset-pagination tokens. The payload is a JSON
// array of the ordering columns; callers own the meaning of each part.
package cursor

import (
	"encoding/base64"
	"encoding/json"

	"github.com/mohan9182/financer/internal/errors"
)

// Encode turns the ordering values of one row into a token.
func Encode(values ...string) string {
	payload, _ := json.Marshal(values)
	return base64.RawURLEncoding.EncodeToString(payload)
}

// Decode returns the ordering values, or nil for an empty token.
func Decode(token string) ([]string, error) {
	if token == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, errors.BadRequest("invalid pageToken")
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || len(values) == 0 {
		return nil, errors.BadRequest("invalid pageToken")
	}
	return values, nil
}
