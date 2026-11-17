// Package accountservice implements account business logic.
package accountservice

import (
	"math"

	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/time"
)

const endingNumbersLen = 4

type Service struct {
	repo repo.AccountRepo
}

func New(repo repo.AccountRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(userID, bankName, nickname, accountType, endingNumbers string) (*model.AccountResponse, error) {
	if bankName == "" {
		return nil, errors.BadRequest("bank_name is required")
	}
	if err := validateEndingNumbers(endingNumbers); err != nil {
		return nil, err
	}
	row, err := s.repo.Create(userID, bankName, nickname, accountType, endingNumbers)
	if err != nil {
		return nil, err
	}
	return fromRow(row), nil
}

func (s *Service) Update(userID, id, bankName, nickname, accountType, endingNumbers string) (*model.AccountResponse, error) {
	if endingNumbers != "" {
		if err := validateEndingNumbers(endingNumbers); err != nil {
			return nil, err
		}
	}
	row, err := s.repo.Update(userID, id, bankName, nickname, accountType, endingNumbers)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errors.NotFound("account " + id + " not found")
	}
	return fromRow(row), nil
}

func (s *Service) Delete(userID, id string) error {
	ok, err := s.repo.Delete(userID, id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.NotFound("account " + id + " not found")
	}
	return nil
}

func (s *Service) List(userID string) ([]*model.AccountResponse, error) {
	rows, err := s.repo.List(userID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.AccountResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(r))
	}
	return out, nil
}

func (s *Service) SumBalance(userID string) (float64, error) {
	return s.repo.SumBalance(userID)
}

func (s *Service) SumTotals(userID string) (credit, debit float64, err error) {
	return s.repo.SumTotals(userID)
}

func validateEndingNumbers(v string) error {
	if len(v) != endingNumbersLen || !digitsOnly(v) {
		return errors.BadRequest("endingNumbers must be exactly 4 digits")
	}
	return nil
}

func digitsOnly(s string) bool {
	for _, b := range []byte(s) {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func fromRow(r *repo.AccountRow) *model.AccountResponse {
	return &model.AccountResponse{
		ID:            r.ID,
		BankName:      r.BankName,
		Nickname:      r.Nickname,
		Balance:       round2(r.Balance),
		AccountType:   model.AccountType(r.AccountType),
		EndingNumbers: r.EndingNumbers,
		CreatedAt:     time.TSRFC3339(r.CreatedAt),
	}
}
