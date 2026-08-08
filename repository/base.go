package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type scannable interface {
	Scan(dest ...any) error
}

type Queryable interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type RowQueryable interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Executable interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type BaseRepository struct {
	writeDB *sql.DB
	readDB  *sql.DB
}

func NewBaseRepository(writeDB, readDB *sql.DB) *BaseRepository {
	return &BaseRepository{writeDB: writeDB, readDB: readDB}
}

func (b *BaseRepository) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return b.writeDB.ExecContext(ctx, query, args...)
}

func (b *BaseRepository) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return b.readDB.QueryContext(ctx, query, args...)
}

func (b *BaseRepository) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return b.readDB.QueryRowContext(ctx, query, args...)
}

func (b *BaseRepository) BeginTx(ctx context.Context) (*Tx, error) {
	tx, err := b.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	return &Tx{Tx: tx}, nil
}

func (b *BaseRepository) ExecInTx(ctx context.Context, fn func(tx *Tx) error) error {
	tx, err := b.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}

type Tx struct {
	*sql.Tx
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.Tx.ExecContext(ctx, query, args...)
}

func (t *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.Tx.QueryContext(ctx, query, args...)
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.Tx.QueryRowContext(ctx, query, args...)
}

func QueryAll[T any](ctx context.Context, db Queryable, query string, mapper func(scannable) (*T, error), args ...any) ([]*T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*T
	for rows.Next() {
		item, err := mapper(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

func QueryOne[T any](ctx context.Context, db RowQueryable, query string, mapper func(scannable) (*T, error), args ...any) (*T, error) {
	row := db.QueryRowContext(ctx, query, args...)
	item, err := mapper(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func Exec(ctx context.Context, db Executable, query string, args ...any) (sql.Result, error) {
	return db.ExecContext(ctx, query, args...)
}
