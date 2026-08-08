package services

type FIFOLot struct {
	Side     int32   // 1 = buy, -1 = sell
	Quantity float32
	Price    float32
}

type Position struct {
	Quantity    float32 // remaining units
	AvgCost     float32 // per-unit cost of remaining units
	CostBasis   float32 // total cost of remaining units
	RealizedPnl float32
}

// ComputeFIFO processes buy/sell lots FIFO. A sell consumes the oldest
// remaining buy lots. Realized P&L = sell proceeds - FIFO cost of sold units.
// Over-sells leak a negative quantity (the caller rejects them).
func ComputeFIFO(lots []FIFOLot) Position {
	var pos Position
	type buyLot struct {
		quantity float32
		price    float32
	}
	var buys []buyLot

	for _, lot := range lots {
		if lot.Side >= 0 {
			buys = append(buys, buyLot{quantity: lot.Quantity, price: lot.Price})
			continue
		}
		remaining := lot.Quantity
		for remaining > 0 && len(buys) > 0 {
			if buys[0].quantity <= remaining {
				pos.RealizedPnl += (lot.Price - buys[0].price) * buys[0].quantity
				remaining -= buys[0].quantity
				buys = buys[1:]
			} else {
				pos.RealizedPnl += (lot.Price - buys[0].price) * remaining
				buys[0].quantity -= remaining
				remaining = 0
			}
		}
		if remaining > 0 {
			pos.Quantity -= remaining // over-sell leaks negative
		}
	}

	for _, b := range buys {
		pos.Quantity += b.quantity
		pos.CostBasis += b.quantity * b.price
	}
	if pos.Quantity > 0 {
		pos.AvgCost = pos.CostBasis / pos.Quantity
	}
	return pos
}