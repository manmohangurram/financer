//! FIFO cost-basis math, mirroring Go's `services/fifo.go`.

#[derive(Debug, Clone, Copy)]
pub struct FifoLot {
    pub side: i64, // 1 buy, -1 sell
    pub quantity: f64,
    pub price: f64,
}

#[derive(Debug, Clone, Copy, Default)]
pub struct Position {
    pub quantity: f64,
    pub avg_cost: f64,
    pub cost_basis: f64,
    pub realized_pnl: f64,
}

/// Process buy/sell lots FIFO. A sell consumes the oldest remaining buy lots.
/// Realized P&L = sell proceeds - FIFO cost of sold units. Over-sells leak a
/// negative quantity (the caller rejects them).
pub fn compute_fifo(lots: &[FifoLot]) -> Position {
    let mut pos = Position::default();
    let mut buys: Vec<(f64, f64)> = Vec::new(); // (quantity, price)

    for lot in lots {
        if lot.side >= 0 {
            buys.push((lot.quantity, lot.price));
            continue;
        }
        let mut remaining = lot.quantity;
        while remaining > 0.0 && !buys.is_empty() {
            let (b_qty, b_price) = buys[0];
            if b_qty <= remaining {
                pos.realized_pnl += (lot.price - b_price) * b_qty;
                remaining -= b_qty;
                buys.remove(0);
            } else {
                pos.realized_pnl += (lot.price - b_price) * remaining;
                buys[0].0 -= remaining;
                remaining = 0.0;
            }
        }
        if remaining > 0.0 {
            pos.quantity -= remaining; // over-sell leaks negative
        }
    }

    for (b_qty, b_price) in buys {
        pos.quantity += b_qty;
        pos.cost_basis += b_qty * b_price;
    }
    if pos.quantity > 0.0 {
        pos.avg_cost = pos.cost_basis / pos.quantity;
    }
    pos
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fifo_buy_sell_consumes_oldest() {
        let pos = compute_fifo(&[
            FifoLot { side: 1, quantity: 10.0, price: 10.0 },
            FifoLot { side: 1, quantity: 10.0, price: 20.0 },
            FifoLot { side: -1, quantity: 15.0, price: 25.0 },
        ]);
        assert_eq!(pos.quantity, 5.0);
        assert_eq!(pos.avg_cost, 20.0); // remaining 5 from the 20-cost lot
        assert_eq!(pos.realized_pnl, (25.0 - 10.0) * 10.0 + (25.0 - 20.0) * 5.0);
    }

    #[test]
    fn fifo_partial_sell() {
        let pos = compute_fifo(&[
            FifoLot { side: 1, quantity: 10.0, price: 10.0 },
            FifoLot { side: -1, quantity: 4.0, price: 15.0 },
        ]);
        assert_eq!(pos.quantity, 6.0);
        assert_eq!(pos.realized_pnl, (15.0 - 10.0) * 4.0);
    }

    #[test]
    fn fifo_empty() {
        let pos = compute_fifo(&[]);
        assert_eq!(pos.quantity, 0.0);
        assert_eq!(pos.avg_cost, 0.0);
    }
}
