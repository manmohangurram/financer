package httpserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mohan9182/financer/api"
)

// --- accounts ---

type reqAccount struct {
	Id              string `json:"id"`
	BankName        string `json:"bankName"`
	AccountNickname string `json:"nickname"`
	AccountType     any    `json:"accountType"`
}

type wireAccount struct {
	Id              string  `json:"id"`
	BankName        string  `json:"bankName"`
	AccountNickname string  `json:"nickname"`
	AccountType     string  `json:"accountType"`
	Balance         float64 `json:"balance"`
	CreatedAt       string  `json:"createdAt"`
}

func accountWire(a *api.AccountResponse) wireAccount {
	return wireAccount{
		Id:              a.Id,
		BankName:        a.BankName,
		AccountNickname: a.AccountNickname,
		AccountType:     a.AccountType.String(),
		Balance:         cents(a.Balance),
		CreatedAt:       tsRFC3339(a.CreatedAt),
	}
}

// accountTypeValue maps a string name (frontend uses "ACCOUNT_TYPE_*") or a
// numeric value onto the enum.
func accountTypeValue(v any) (api.AccountType, error) {
	switch x := v.(type) {
	case nil:
		return 0, nil
	case string:
		if n, ok := api.AccountType_value[x]; ok {
			return api.AccountType(n), nil
		}
		return 0, fmt.Errorf("unknown account type %q", x)
	case float64:
		return api.AccountType(x), nil
	default:
		return 0, fmt.Errorf("invalid account type %v", v)
	}
}

func (a *API) createAccount(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqAccount
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	t, err := accountTypeValue(in.AccountType)
	if err != nil {
		return nil, bad("%v", err)
	}
	out, err := a.account.CreateAccount(ctx, &api.CreateAccountRequest{
		BankName: in.BankName, AccountNickname: in.AccountNickname, AccountType: t,
	})
	if err != nil {
		return nil, err
	}
	return accountWire(out), nil
}

func (a *API) updateAccount(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqAccount
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	t, err := accountTypeValue(in.AccountType)
	if err != nil {
		return nil, bad("%v", err)
	}
	in.Id = r.PathValue("id")
	out, err := a.account.UpdateAccount(ctx, &api.UpdateAccountRequest{
		Id: in.Id, BankName: in.BankName, AccountNickname: in.AccountNickname, AccountType: t,
	})
	if err != nil {
		return nil, err
	}
	return created(accountWire(out)), nil
}

func (a *API) deleteAccount(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqAccount
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	in.Id = r.PathValue("id")
	_, err := a.account.DeleteAccount(ctx, &api.DeleteAccountRequest{Id: in.Id})
	if err != nil {
		return nil, err
	}
	return noContent(), nil
}

func (a *API) listAccounts(ctx context.Context, _ string, _ *http.Request) (any, error) {
	out, err := a.account.ListAccounts(ctx, &api.ListAccountsRequest{})
	if err != nil {
		return nil, err
	}
	items := make([]wireAccount, 0, len(out.Accounts))
	for _, ac := range out.Accounts {
		items = append(items, accountWire(ac))
	}
	return struct {
		Accounts []wireAccount `json:"accounts"`
	}{Accounts: items}, nil
}
