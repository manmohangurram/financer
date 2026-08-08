package services

import (
	"context"

	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/repository"
)

// TransferRuleService resolves a rule's "transfer to account" action: when a
// matching transaction exists in the target account (same amount, opposite
// type, within ±3 days), link it; otherwise create a counterpart and link.
type TransferRuleService struct {
	ruleRepo   *repository.RuleRepository
	txnRepo     *repository.TransactionRepository
	linkRepo    *repository.TransferRepository
	accountRepo *repository.AccountRepository
}

func NewTransferRuleService(ruleRepo *repository.RuleRepository, txnRepo *repository.TransactionRepository, linkRepo *repository.TransferRepository, accountRepo *repository.AccountRepository) *TransferRuleService {
	return &TransferRuleService{ruleRepo: ruleRepo, txnRepo: txnRepo, linkRepo: linkRepo, accountRepo: accountRepo}
}

// ApplyToTransactions runs the transfer actions of matching rules against the
// given transactions (used on create/import). Created counterparts are not
// re-processed. Returns counts of linked and created transactions.
func (s *TransferRuleService) ApplyToTransactions(ctx context.Context, txns []*api.TransactionResponse) (linked, created int, err error) {
	if len(txns) == 0 {
		return 0, 0, nil
	}
	userID, _ := ctx.Value(auth.UserIDKey).(string)
	rules, err := s.ruleRepo.ListForOverlay(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	for _, txn := range txns {
		for _, rule := range rules {
			if !matchRuleData(txnForMatchData{Name: txn.Name, Amount: float64(txn.Amount), Type: int(txn.Type), AccountID: txn.AccountId}, rule.Logic, rule.Conditions) {
				continue
			}
			for _, act := range rule.Actions {
				if act.SetTransferAccountId == "" {
					continue
				}
				l, c, err := s.resolve(ctx, userID, txn, act.SetTransferAccountId)
				if err != nil {
					return linked, created, err
				}
				if l {
					linked++
				}
				if c {
					created++
				}
			}
		}
	}
	return linked, created, nil
}

// RunRule runs one rule against the user's transactions. Rules with a
// transfer-to-account action also link/create counterparts; name/category
// actions are read-time overlays, so for those it only counts matches.
func (s *TransferRuleService) RunRule(ctx context.Context, ruleID string) (*api.RunRuleResponse, error) {
	userID, _ := ctx.Value(auth.UserIDKey).(string)
	rule, err := s.ruleRepo.GetByID(ctx, ruleID)
	if err != nil || rule == nil {
		return nil, NotFound("rule %s not found", ruleID)
	}

	var transferTarget string
	for _, a := range rule.Actions {
		if a.SetTransferAccountId != "" {
			transferTarget = a.SetTransferAccountId
		}
	}

	conds := repository.ConditionsToData(rule.Conditions)
	result, err := s.txnRepo.List(ctx, userID, repository.TxnListFilter{TxnType: api.TransactionType_DEBIT})
	if err != nil {
		return nil, err
	}

	matched, linked, created := 0, 0, 0
	for _, txn := range result.Transactions {
		if txn.LinkedTransferId != "" {
			continue
		}
		if !matchRuleData(txnForMatchData{Name: txn.Name, Amount: float64(txn.Amount), Type: int(txn.Type), AccountID: txn.AccountId}, repository.LogicString(rule.Logic), conds) {
			continue
		}
		matched++
		if transferTarget == "" {
			continue
		}
		l, c, err := s.resolve(ctx, userID, txn, transferTarget)
		if err != nil {
			return nil, err
		}
		if l {
			linked++
		}
		if c {
			created++
		}
	}

	return &api.RunRuleResponse{Matched: int32(matched), Linked: int32(linked), Created: int32(created)}, nil
}

func (s *TransferRuleService) resolve(ctx context.Context, userID string, txn *api.TransactionResponse, targetAccountID string) (bool, bool, error) {
	linked, created, _, err := resolveTransfer(ctx, userID, txn, targetAccountID, s.txnRepo, s.linkRepo, s.accountRepo)
	return linked, created, err
}
