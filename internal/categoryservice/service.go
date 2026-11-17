// Package categoryservice implements category business logic.
package categoryservice

import (
	"fmt"
	"math"

	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/time"
)

const maxMonthlyBudget = 1_000_000_000

type Service struct {
	repo repo.CategoryRepo
}

func New(repo repo.CategoryRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(userID string, pageSize int64, pageToken string) ([]*model.CategoryResponse, string, error) {
	res, err := s.repo.List(userID, pageSize, pageToken)
	if err != nil {
		return nil, "", err
	}
	out := make([]*model.CategoryResponse, 0, len(res.Categories))
	for _, c := range res.Categories {
		out = append(out, &model.CategoryResponse{
			ID:            c.ID,
			Name:          c.Name,
			MonthlyBudget: c.MonthlyBudget,
			CreatedAt:     time.TSRFC3339(c.CreatedAt),
		})
	}
	return out, res.NextPageToken, nil
}

// SetBudget stores a category's recurring monthly limit. 0 clears it; the
// amount is rounded to paise and validated so bad input never reaches SQLite.
func (s *Service) SetBudget(userID, id string, budget float64) (*model.CategoryResponse, error) {
	normalised, err := normaliseBudget(budget)
	if err != nil {
		return nil, err
	}
	budget = normalised
	ok, err := s.repo.SetBudget(userID, id, budget)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.NotFound("category " + id + " not found")
	}
	row, err := s.repo.GetByID(userID, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errors.NotFound("category " + id + " not found")
	}
	return &model.CategoryResponse{
		ID:            row.ID,
		Name:          row.Name,
		MonthlyBudget: row.MonthlyBudget,
		CreatedAt:     time.TSRFC3339(row.CreatedAt),
	}, nil
}

func (s *Service) Create(userID string, inputs []repo.CategoryCreateInput) *model.BulkResult {
	clean := make([]repo.CategoryCreateInput, 0, len(inputs))
	for _, in := range inputs {
		if in.Name == "" {
			continue
		}
		budget, err := normaliseBudget(in.MonthlyBudget)
		if err != nil {
			return &model.BulkResult{Success: false, Message: err.Error()}
		}
		in.MonthlyBudget = budget
		clean = append(clean, in)
	}
	if _, err := s.repo.Create(userID, clean); err != nil {
		return &model.BulkResult{Success: false, Message: "some categories failed", FailedIDs: []string{err.Error()}}
	}
	return &model.BulkResult{Success: true, Message: "categories created successfully"}
}

func (s *Service) Update(userID string, inputs []repo.CategoryUpdateInput) *model.BulkResult {
	for i := range inputs {
		if inputs[i].MonthlyBudget == nil {
			continue
		}
		budget, err := normaliseBudget(*inputs[i].MonthlyBudget)
		if err != nil {
			return &model.BulkResult{Success: false, Message: err.Error()}
		}
		inputs[i].MonthlyBudget = &budget
	}
	errs, err := s.repo.Update(userID, inputs)
	if err != nil {
		return &model.BulkResult{Success: false, Message: "some updates failed", FailedIDs: []string{err.Error()}}
	}
	if len(errs) > 0 {
		return &model.BulkResult{Success: false, Message: "some updates failed", FailedIDs: errs}
	}
	return &model.BulkResult{Success: true, Message: "categories updated successfully"}
}

// normaliseBudget validates a monthly limit and rounds it to paise.
func normaliseBudget(budget float64) (float64, error) {
	if math.IsNaN(budget) || math.IsInf(budget, 0) {
		return 0, errors.BadRequest("monthlyBudget must be a number")
	}
	if budget < 0 {
		return 0, errors.BadRequest("monthlyBudget cannot be negative")
	}
	if budget > maxMonthlyBudget {
		return 0, errors.BadRequest(fmt.Sprintf("monthlyBudget must be at most %d", maxMonthlyBudget))
	}
	return math.Round(budget*100) / 100, nil
}

func (s *Service) Delete(userID string, ids []string) *model.BulkResult {
	if len(ids) == 0 {
		return &model.BulkResult{Success: false, Message: "no ids provided"} // handler maps to 400
	}
	errs, err := s.repo.Delete(userID, ids)
	if err != nil {
		return &model.BulkResult{Success: false, Message: "some deletions failed", FailedIDs: []string{err.Error()}}
	}
	if len(errs) > 0 {
		return &model.BulkResult{Success: false, Message: "some deletions failed", FailedIDs: errs}
	}
	return &model.BulkResult{Success: true, Message: "categories deleted successfully"}
}
