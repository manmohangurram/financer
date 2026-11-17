package ruleservice

import (
	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/transferservice"
)

// TransferRuleService resolves a rule's SET_TRANSFER_ACCOUNT action at write
// time (create/import) and for "run now".
type TransferRuleService struct {
	rules    repo.RuleRepo
	txns     repo.TransactionRepo
	accts    repo.AccountRepo
	transfer *transferservice.Service
}

func NewTransferRuleService(rules repo.RuleRepo, txns repo.TransactionRepo, accts repo.AccountRepo, transfer *transferservice.Service) *TransferRuleService {
	return &TransferRuleService{rules: rules, txns: txns, accts: accts, transfer: transfer}
}

// RunRuleResponse reports a transfer-rule run.
type RunRuleResponse struct {
	Matched int64
	Linked  int64
	Created int64
}

// RunRule runs one rule against the user's debit, unlinked transactions.
func (t *TransferRuleService) RunRule(userID, ruleID string) (*RunRuleResponse, error) {
	rule, err := t.rules.GetByID(userID, ruleID)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, errors.NotFound("rule " + ruleID + " not found")
	}

	transferTarget := ""
	for _, act := range rule.Actions {
		if act.TransferAccountID != "" {
			transferTarget = act.TransferAccountID
		}
	}

	debit := string(model.TransactionTypeDebit)
	rows, err := t.txns.List(userID, &repo.TransactionListFilter{Type: &debit})
	if err != nil {
		return nil, err
	}

	resp := &RunRuleResponse{}
	for _, row := range rows.Rows {
		if row.LinkID != "" {
			continue
		}
		data := &TransactionMatchData{
			Name:      row.Txn.Name,
			Amount:    row.Txn.Amount,
			Type:      row.Txn.Type,
			AccountID: row.Txn.AccountID,
		}
		if !MatchRuleData(data, rule.Logic, rule.Conditions) {
			continue
		}
		resp.Matched++
		if transferTarget == "" {
			continue
		}
		outcome, err := t.transfer.ResolveTransfer(userID, row.Txn, transferTarget)
		if err != nil {
			return nil, err
		}
		if outcome.Linked {
			resp.Linked++
			if outcome.Created {
				resp.Created++
			}
		}
	}
	return resp, nil
}

// ApplyToTransactions applies matching rules' transfer actions to new rows.
func (t *TransferRuleService) ApplyToTransactions(userID string, txns []*repo.TransactionRow) (linked, created int64, err error) {
	if len(txns) == 0 {
		return 0, 0, nil
	}
	rules, err := t.rules.ListForOverlay(userID)
	if err != nil {
		return 0, 0, err
	}
	for _, txn := range txns {
		data := &TransactionMatchData{
			Name:      txn.Name,
			Amount:    txn.Amount,
			Type:      txn.Type,
			AccountID: txn.AccountID,
		}
		for _, rule := range rules {
			if !MatchRuleData(data, rule.Logic, rule.Conditions) {
				continue
			}
			for _, act := range rule.Actions {
				if act.TransferAccountID == "" {
					continue
				}
				outcome, err := t.transfer.ResolveTransfer(userID, txn, act.TransferAccountID)
				if err != nil {
					return linked, created, err
				}
				if outcome.Linked {
					linked++
					if outcome.Created {
						created++
					}
				}
			}
		}
	}
	return linked, created, nil
}
