package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mohan9182/financer/api"
)

type InvestmentRepository struct {
	*BaseRepository
}

func NewInvestmentRepository(base *BaseRepository) *InvestmentRepository {
	return &InvestmentRepository{BaseRepository: base}
}

const investmentCols = `id, user_id, symbol, name, investment_type, current_price, prev_close, last_quote_at, manual_nav, created_at`

func scanInvestment(row scannable) (*api.InvestmentResponse, error) {
	var inst api.InvestmentResponse
	var uid, sym, lq sql.NullString
	var cur, prev, nav sql.NullFloat64
	var itype int
	var createdAt time.Time
	err := row.Scan(&inst.Id, &uid, &sym, &inst.Name, &itype, &cur, &prev, &lq, &nav, &createdAt)
	if err != nil {
		return nil, err
	}
	inst.Symbol = sym.String
	inst.InvestmentType = api.InvestmentType(itype)
	inst.CurrentPrice = float32(cur.Float64)
	inst.PrevClose = float32(prev.Float64)
	inst.ManualNav = float32(nav.Float64)
	if lq.Valid {
		if t, err := time.Parse(time.RFC3339, lq.String); err == nil {
			inst.LastQuoteAt = t
		}
	}
	inst.CreatedAt = createdAt
	return &inst, nil
}

func (r *InvestmentRepository) CreateInvestment(ctx context.Context, userID string, symbol, name string, it api.InvestmentType, manualNav float32) (*api.InvestmentResponse, error) {
	id := uuid.New().String()
	_, err := r.ExecContext(ctx,
		`INSERT INTO investments (id, user_id, symbol, name, investment_type, current_price, prev_close, last_quote_at, manual_nav, created_at)
		 VALUES (?, ?, ?, ?, ?, NULL, NULL, NULL, ?, ?)`,
		id, userID, sql.NullString{String: symbol, Valid: symbol != ""}, name, int(it),
		sql.NullFloat64{Float64: float64(manualNav), Valid: manualNav != 0},
		time.Now().UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("inserting investment: %w", err)
	}
	return r.GetInvestment(ctx, id)
}

func (r *InvestmentRepository) GetInvestment(ctx context.Context, id string) (*api.InvestmentResponse, error) {
	return QueryOne(ctx, r.readDB,
		`SELECT `+investmentCols+` FROM investments WHERE id = ?`,
		scanInvestment, id,
	)
}

func (r *InvestmentRepository) ListInvestments(ctx context.Context, userID string) ([]*api.InvestmentResponse, error) {
	return QueryAll(ctx, r.readDB,
		`SELECT `+investmentCols+` FROM investments WHERE user_id = ? ORDER BY created_at DESC`,
		scanInvestment, userID,
	)
}

func (r *InvestmentRepository) UpdateInvestment(ctx context.Context, id string, symbol, name string, it api.InvestmentType, manualNav float32) (*api.InvestmentResponse, error) {
	var oldSym sql.NullString
	r.readDB.QueryRowContext(ctx, `SELECT symbol FROM investments WHERE id = ?`, id).Scan(&oldSym)

	var sym sql.NullString
	if symbol != "" {
		sym = sql.NullString{String: symbol, Valid: true}
	}
	_, err := r.ExecContext(ctx,
		`UPDATE investments SET symbol = COALESCE(?, symbol), name = ?, investment_type = ?, manual_nav = ? WHERE id = ?`,
		sym, name, int(it),
		sql.NullFloat64{Float64: float64(manualNav), Valid: manualNav != 0},
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating investment: %w", err)
	}
	// The cached price history belongs to the old symbol; drop it when the
	// symbol changed so it refetches under the new ticker.
	if symbol != "" && oldSym.String != symbol {
		if _, err := r.ExecContext(ctx, `DELETE FROM investment_price_history WHERE investment_id = ?`, id); err != nil {
			return nil, fmt.Errorf("clearing price history: %w", err)
		}
	}
	return r.GetInvestment(ctx, id)
}

func (r *InvestmentRepository) DeleteInvestment(ctx context.Context, id string) (bool, error) {
	if _, err := r.ExecContext(ctx, `DELETE FROM investment_price_history WHERE investment_id = ?`, id); err != nil {
		return false, fmt.Errorf("deleting investment: %w", err)
	}
	result, err := r.ExecContext(ctx, `DELETE FROM investments WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("deleting investment: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func scanLot(row scannable) (*api.LotResponse, error) {
	var lot api.LotResponse
	var side int
	var occurredAt, createdAt time.Time
	err := row.Scan(&lot.Id, &lot.InvestmentId, &side, &lot.Quantity, &lot.Price, &occurredAt, &createdAt)
	if err != nil {
		return nil, err
	}
	lot.Side = int32(side)
	lot.OccurredAt = occurredAt
	lot.CreatedAt = createdAt
	return &lot, nil
}

func (r *InvestmentRepository) ListLots(ctx context.Context, investmentID string) ([]*api.LotResponse, error) {
	return QueryAll(ctx, r.readDB,
		`SELECT id, investment_id, side, quantity, price, occurred_at, created_at FROM investment_lots WHERE investment_id = ? ORDER BY occurred_at ASC, created_at ASC`,
		scanLot, investmentID,
	)
}

func (r *InvestmentRepository) CreateLot(ctx context.Context, userID, investmentID string, side int32, quantity, price float32, occurredAt time.Time) (*api.LotResponse, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := r.ExecContext(ctx,
		`INSERT INTO investment_lots (id, user_id, investment_id, side, quantity, price, occurred_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, investmentID, side, quantity, price, occurredAt, now,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting lot: %w", err)
	}
	return &api.LotResponse{
		Id:           id,
		InvestmentId: investmentID,
		Side:         side,
		Quantity:     quantity,
		Price:        price,
		OccurredAt:   occurredAt,
		CreatedAt:    now,
	}, nil
}

func (r *InvestmentRepository) DeleteLot(ctx context.Context, id string) (bool, error) {
	result, err := r.ExecContext(ctx, `DELETE FROM investment_lots WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("deleting lot: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (r *InvestmentRepository) UpdateQuote(ctx context.Context, id string, currentPrice, prevClose float32) error {
	_, err := r.ExecContext(ctx,
		`UPDATE investments SET current_price = ?, prev_close = ?, last_quote_at = ? WHERE id = ?`,
		currentPrice, prevClose, time.Now().UTC(), id,
	)
	return err
}

func (r *InvestmentRepository) ListAllForRefresh(ctx context.Context) ([]*api.InvestmentResponse, error) {
	return QueryAll(ctx, r.readDB,
		`SELECT `+investmentCols+` FROM investments WHERE symbol IS NOT NULL AND symbol != ''`,
		scanInvestment,
	)
}

func (r *InvestmentRepository) UpsertPriceHistory(ctx context.Context, investmentID, rangeID string, ts []int64, closes []float64, fetchedAt int64) error {
	return r.ExecInTx(ctx, func(tx *Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM investment_price_history WHERE investment_id = ? AND range_id = ?`, investmentID, rangeID); err != nil {
			return err
		}
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO investment_price_history (investment_id, range_id, t, close, fetched_at) VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for i := range ts {
			if _, err := stmt.ExecContext(ctx, investmentID, rangeID, ts[i], closes[i], fetchedAt); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *InvestmentRepository) GetPriceHistory(ctx context.Context, investmentID, rangeID string) (ts []int64, closes []float64, lastFetched int64, err error) {
	rows, err := r.readDB.QueryContext(ctx,
		`SELECT t, close, fetched_at FROM investment_price_history WHERE investment_id = ? AND range_id = ? ORDER BY t`,
		investmentID, rangeID,
	)
	if err != nil {
		return nil, nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var t int64
		var c float64
		var fa int64
		if err := rows.Scan(&t, &c, &fa); err != nil {
			return nil, nil, 0, err
		}
		ts = append(ts, t)
		closes = append(closes, c)
		lastFetched = fa
	}
	return ts, closes, lastFetched, rows.Err()
}
