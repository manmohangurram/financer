package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mohan9182/financer/api"
	"github.com/google/uuid"
)

type CatCursor struct {
	Name string `json:"n"`
	ID   string `json:"id"`
}

func encodeCatCursor(c CatCursor) string {
	b, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(b)
}

func decodeCatCursor(token string) (*CatCursor, error) {
	b, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	var c CatCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

type CategoryRepository struct {
	*BaseRepository
}

func NewCategoryRepository(base *BaseRepository) *CategoryRepository {
	return &CategoryRepository{BaseRepository: base}
}

func scanCategory(row scannable) (*api.CategoryResponse, error) {
	var cat api.CategoryResponse
	var createdAt time.Time
	err := row.Scan(&cat.Id, &cat.Name, &createdAt)
	if err != nil {
		return nil, err
	}
	cat.CreatedAt = createdAt
	return &cat, nil
}

type CreateCategoryInput struct {
	Name   string
	UserID string
}

type UpdateCategoryInput struct {
	ID   string
	Name string
}

func (r *CategoryRepository) Create(ctx context.Context, inputs []CreateCategoryInput) ([]*api.CategoryResponse, []error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	var results []*api.CategoryResponse
	var errors []error

	r.ExecInTx(ctx, func(tx *Tx) error {
		for _, input := range inputs {
			id := uuid.New().String()
			now := time.Now().UTC()
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO categories (id, name, created_at, user_id) VALUES (?, ?, ?, ?)`,
				id, input.Name, now, input.UserID,
			); err != nil {
				errors = append(errors, fmt.Errorf("failed to create category %q: %w", input.Name, err))
				continue
			}
			results = append(results, &api.CategoryResponse{
				Id:        id,
				Name:      input.Name,
				CreatedAt: now,
			})
		}
		if len(errors) > 0 {
			return fmt.Errorf("some categories failed")
		}
		return nil
	})

	return results, errors
}

func (r *CategoryRepository) Update(ctx context.Context, inputs []UpdateCategoryInput) []error {
	if len(inputs) == 0 {
		return nil
	}

	var errors []error

	r.ExecInTx(ctx, func(tx *Tx) error {
		for _, input := range inputs {
			result, err := tx.ExecContext(ctx,
				`UPDATE categories SET name = COALESCE(NULLIF(?, ''), name) WHERE id = ?`,
				input.Name, input.ID,
			)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to update %s: %w", input.ID, err))
				continue
			}
			rows, _ := result.RowsAffected()
			if rows == 0 {
				errors = append(errors, fmt.Errorf("category %s not found", input.ID))
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("some updates failed")
		}
		return nil
	})

	return errors
}

func (r *CategoryRepository) Delete(ctx context.Context, ids []string) []error {
	if len(ids) == 0 {
		return nil
	}

	var errors []error

	r.ExecInTx(ctx, func(tx *Tx) error {
		for _, id := range ids {
			if _, err := tx.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, id); err != nil {
				errors = append(errors, fmt.Errorf("failed to delete %s: %w", id, err))
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("some deletions failed")
		}
		return nil
	})

	return errors
}

type ListCatResult struct {
	Categories    []*api.CategoryResponse
	NextPageToken string
}

func (r *CategoryRepository) List(ctx context.Context, userID string, pageSize int32, pageToken string) (*ListCatResult, error) {
	query := `SELECT id, name, created_at FROM categories WHERE user_id = ?`
	var args []any
	args = append(args, userID)

	if pageToken != "" {
		cursor, err := decodeCatCursor(pageToken)
		if err == nil {
			query += " AND (name > ? OR (name = ? AND id > ?))"
			args = append(args, cursor.Name, cursor.Name, cursor.ID)
		}
	}

	query += " ORDER BY name ASC, id ASC"

	if pageSize > 0 {
		query += fmt.Sprintf(" LIMIT %d", pageSize+1)
	}

	results, err := QueryAll(ctx, r.readDB, query, scanCategory, args...)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}

	var nextToken string
	if pageSize > 0 && int32(len(results)) > pageSize {
		results = results[:pageSize]
		last := results[len(results)-1]
		nextToken = encodeCatCursor(CatCursor{Name: last.Name, ID: last.Id})
	}

	return &ListCatResult{Categories: results, NextPageToken: nextToken}, nil
}
