package services

import (
	"context"

	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/repository"
)

type CategoryService struct {
	repo *repository.CategoryRepository
}

func NewCategoryService(repo *repository.CategoryRepository) *CategoryService {
	return &CategoryService{repo: repo}
}

func (s *CategoryService) ListCategories(ctx context.Context, msg *api.ListCategoriesRequest) (*api.ListCategoriesResponse, error) {
	userID := ctx.Value(auth.UserIDKey).(string)
	result, err := s.repo.List(ctx, userID, msg.PageSize, msg.PageToken)
	if err != nil {
		return nil, ServerError("%v", err)
	}

	return &api.ListCategoriesResponse{
		Categories:    result.Categories,
		NextPageToken: result.NextPageToken,
	}, nil
}

func (s *CategoryService) CreateCategories(ctx context.Context, msg *api.CreateCategoriesRequest) (*api.BulkOperationResponse, error) {
	userID := ctx.Value(auth.UserIDKey).(string)

	var inputs []repository.CreateCategoryInput
	for _, c := range msg.Categories {
		if c.Name == "" {
			continue
		}
		inputs = append(inputs, repository.CreateCategoryInput{
			Name:   c.Name,
			UserID: userID,
		})
	}

	_, errs := s.repo.Create(ctx, inputs)
	if len(errs) > 0 {
		return &api.BulkOperationResponse{
			Success:   false,
			Message:   "some categories failed",
			FailedIds: errStrings(errs),
		}, nil
	}

	return &api.BulkOperationResponse{
		Success: true,
		Message: "categories created successfully",
	}, nil
}

func (s *CategoryService) UpdateCategories(ctx context.Context, msg *api.UpdateCategoriesRequest) (*api.BulkOperationResponse, error) {
	var inputs []repository.UpdateCategoryInput
	for _, c := range msg.Categories {
		inputs = append(inputs, repository.UpdateCategoryInput{
			ID:   c.Id,
			Name: c.Name,
		})
	}

	errs := s.repo.Update(ctx, inputs)
	if len(errs) > 0 {
		return &api.BulkOperationResponse{
			Success:   false,
			Message:   "some updates failed",
			FailedIds: errStrings(errs),
		}, nil
	}

	return &api.BulkOperationResponse{
		Success: true,
		Message: "categories updated successfully",
	}, nil
}

func (s *CategoryService) DeleteCategories(ctx context.Context, msg *api.DeleteCategoriesRequest) (*api.BulkOperationResponse, error) {
	ids := msg.Ids
	if len(ids) == 0 {
		return nil, BadRequest("no ids provided")
	}

	errs := s.repo.Delete(ctx, ids)
	if len(errs) > 0 {
		return &api.BulkOperationResponse{
			Success:   false,
			Message:   "some deletions failed",
			FailedIds: errStrings(errs),
		}, nil
	}

	return &api.BulkOperationResponse{
		Success: true,
		Message: "categories deleted successfully",
	}, nil
}