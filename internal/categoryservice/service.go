// Package categoryservice implements category business logic.
package categoryservice

import (
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/time"
)

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
			ID:        c.ID,
			Name:      c.Name,
			CreatedAt: time.TSRFC3339(c.CreatedAt),
		})
	}
	return out, res.NextPageToken, nil
}

func (s *Service) Create(userID string, names []string) *model.BulkResult {
	var inputs []repo.CategoryCreateInput
	for _, n := range names {
		if n == "" {
			continue
		}
		inputs = append(inputs, repo.CategoryCreateInput{Name: n})
	}
	if _, err := s.repo.Create(userID, inputs); err != nil {
		return &model.BulkResult{Success: false, Message: "some categories failed", FailedIDs: []string{err.Error()}}
	}
	return &model.BulkResult{Success: true, Message: "categories created successfully"}
}

func (s *Service) Update(userID string, pairs [][2]string) *model.BulkResult {
	var inputs []repo.CategoryUpdateInput
	for _, p := range pairs {
		inputs = append(inputs, repo.CategoryUpdateInput{ID: p[0], Name: p[1]})
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
