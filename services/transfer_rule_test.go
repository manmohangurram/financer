package services

import (
	"context"
	"testing"

	"github.com/mohan9182/financer/api"
)

func TestTransferRuleResolveSkips(t *testing.T) {
	s := &TransferRuleService{}
	ctx := context.Background()

	// same account → no-op
	linked, created, err := s.resolve(ctx, "u1", &api.TransactionResponse{Id: "a", AccountId: "x", Type: api.TransactionType_DEBIT}, "x")
	if err != nil || linked || created {
		t.Fatalf("expected no-op for same account, got linked=%v created=%v err=%v", linked, created, err)
	}

	// already linked → no-op
	linked, created, err = s.resolve(ctx, "u1", &api.TransactionResponse{Id: "a", AccountId: "x", LinkedTransferId: "L1", Type: api.TransactionType_DEBIT}, "y")
	if err != nil || linked || created {
		t.Fatalf("expected no-op for already linked, got linked=%v created=%v err=%v", linked, created, err)
	}
}
