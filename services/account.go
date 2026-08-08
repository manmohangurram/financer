package services

import (
	"context"

	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/repository"
)

type AccountService struct {
	repo *repository.AccountRepository
}

func NewAccountService(repo *repository.AccountRepository) *AccountService {
	return &AccountService{repo: repo}
}

func (s *AccountService) CreateAccount(ctx context.Context, msg *api.CreateAccountRequest) (*api.AccountResponse, error) {
	if msg.BankName == "" {
		return nil, BadRequest("bank_name is required")
	}

	userID := ctx.Value(auth.UserIDKey).(string)
	acc, err := s.repo.Create(ctx, userID, msg.BankName, msg.AccountNickname, msg.AccountType)
	if err != nil {
		return nil, ServerError("%v", err)
	}

	return acc, nil
}

func (s *AccountService) UpdateAccount(ctx context.Context, msg *api.UpdateAccountRequest) (*api.AccountResponse, error) {
	acc, err := s.repo.Update(ctx, msg.Id, msg.BankName, msg.AccountNickname, msg.AccountType)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if acc == nil {
		return nil, NotFound("account %s not found", msg.Id)
	}

	return acc, nil
}

func (s *AccountService) DeleteAccount(ctx context.Context, msg *api.DeleteAccountRequest) (*api.OperationResponse, error) {
	deleted, err := s.repo.Delete(ctx, msg.Id)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if !deleted {
		return nil, NotFound("account %s not found", msg.Id)
	}

	return &api.OperationResponse{
		Success: true,
		Message: "account deleted",
	}, nil
}

func (s *AccountService) ListAccounts(ctx context.Context, _ *api.ListAccountsRequest) (*api.ListAccountsResponse, error) {
	userID := ctx.Value(auth.UserIDKey).(string)
	accounts, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, ServerError("%v", err)
	}

	return &api.ListAccountsResponse{Accounts: accounts}, nil
}