package services

import (
	"testing"
)

func TestComputeFIFO(t *testing.T) {
	cases := []struct {
		name         string
		lots         []FIFOLot
		wantQty      float64
		wantAvgCost  float64
		wantRealized float64
	}{
		{
			name: "buy then partial sell",
			lots: []FIFOLot{
				{Side: 1, Quantity: 10, Price: 100},
				{Side: -1, Quantity: 4, Price: 120},
			},
			wantQty:      6,
			wantAvgCost:  100,
			wantRealized: 80, // (120-100)*4
		},
		{
			name: "sell across two buy lots",
			lots: []FIFOLot{
				{Side: 1, Quantity: 5, Price: 50},
				{Side: 1, Quantity: 5, Price: 100},
				{Side: -1, Quantity: 7, Price: 90},
			},
			wantQty:      3,
			wantAvgCost:  100,
			wantRealized: 180, // (90-50)*5 + (90-100)*2
		},
		{
			name: "sell exactly at boundary",
			lots: []FIFOLot{
				{Side: 1, Quantity: 4, Price: 10},
				{Side: -1, Quantity: 4, Price: 15},
			},
			wantQty:      0,
			wantAvgCost:  0,
			wantRealized: 20,
		},
		{
			name: "unrealized gain",
			lots: []FIFOLot{
				{Side: 1, Quantity: 2, Price: 100},
				{Side: 1, Quantity: 2, Price: 120},
			},
			wantQty:      4,
			wantAvgCost:  110,
			wantRealized: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeFIFO(tc.lots)
			if float64(got.Quantity) != tc.wantQty {
				t.Errorf("quantity = %v, want %v", got.Quantity, tc.wantQty)
			}
			if float64(got.AvgCost) != tc.wantAvgCost {
				t.Errorf("avgCost = %v, want %v", got.AvgCost, tc.wantAvgCost)
			}
			if float64(got.RealizedPnl) != tc.wantRealized {
				t.Errorf("realized = %v, want %v", got.RealizedPnl, tc.wantRealized)
			}
		})
	}
}

func TestComputeFIFOOverSell(t *testing.T) {
	lots := []FIFOLot{
		{Side: 1, Quantity: 5, Price: 100},
		{Side: -1, Quantity: 7, Price: 110},
	}
	got := ComputeFIFO(lots)
	if got.Quantity != -2 {
		t.Errorf("quantity = %v, want -2 (over-sell leaks negative)", got.Quantity)
	}
}