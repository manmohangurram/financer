package services

import (
	"testing"

	"github.com/mohan9182/financer/api"
)

func TestEffectivePrice(t *testing.T) {
	cases := []struct {
		name string
		inst *api.InvestmentResponse
		want float32
	}{
		{"manual nav wins", &api.InvestmentResponse{ManualNav: 55.5, CurrentPrice: 60}, 55.5},
		{"no nav uses current", &api.InvestmentResponse{ManualNav: 0, CurrentPrice: 60}, 60},
		{"neither set", &api.InvestmentResponse{}, 0},
	}
	for _, tc := range cases {
		if got := effectivePrice(tc.inst); got != tc.want {
			t.Errorf("%s: effectivePrice = %v, want %v", tc.name, got, tc.want)
		}
	}
}