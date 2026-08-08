package httpserver

import (
	"context"
	"net/http"

	"github.com/mohan9182/financer/api"
)

// --- rules ---

type reqRule struct {
	Id         string   `json:"id"`
	Name       string   `json:"name"`
	Priority   int32    `json:"priority"`
	Logic      any      `json:"logic"`
	Conditions []struct {
		MatchField any    `json:"matchField"`
		Operator   any    `json:"operator"`
		Pattern    string `json:"pattern"`
	} `json:"conditions"`
	Actions []*struct {
		SetName              string `json:"setName"`
		SetNameOp            any    `json:"setNameOp"`
		SetCategoryId        string `json:"setCategoryId"`
		SetTransferAccountId string `json:"setTransferAccountId"`
	} `json:"actions"`
}

func (a *API) ruleAction(in *reqRule) []*api.RuleAction {
	var out []*api.RuleAction
	for _, act := range in.Actions {
		op, _ := enumToValue[api.RuleActionOp](act.SetNameOp, "setNameOp")
		out = append(out, &api.RuleAction{
			SetName:              act.SetName,
			SetNameOp:            op,
			SetCategoryId:        act.SetCategoryId,
			SetTransferAccountId: act.SetTransferAccountId,
		})
	}
	return out
}

func (a *API) listRules(ctx context.Context, _ string, _ *http.Request) (any, error) {
	out, err := a.rule.ListRules(ctx, &api.ListRulesRequest{})
	if err != nil {
		return nil, err
	}
	items := make([]wireRule, 0, len(out.Rules))
	for _, al := range out.Rules {
		items = append(items, ruleWire(al))
	}
	return struct {
		Rules []wireRule `json:"rules"`
	}{Rules: items}, nil
}

func (a *API) createRule(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqRule
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	logic, err := toRuleLogic(in.Logic)
	if err != nil {
		return nil, bad("%v", err)
	}
	cond, err := toConditions(in.Conditions)
	if err != nil {
		return nil, bad("%v", err)
	}
	out, err := a.rule.CreateRule(ctx, &api.CreateRuleRequest{
		Name: in.Name, Priority: in.Priority, Logic: logic, Conditions: cond, Actions: a.ruleAction(&in),
	})
	if err != nil {
		return nil, err
	}
	return created(ruleWire(out)), nil
}

func (a *API) updateRule(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqRule
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	logic, err := toRuleLogic(in.Logic)
	if err != nil {
		return nil, bad("%v", err)
	}
	cond, err := toConditions(in.Conditions)
	if err != nil {
		return nil, bad("%v", err)
	}
	in.Id = r.PathValue("id")
	out, err := a.rule.UpdateRule(ctx, &api.UpdateRuleRequest{
		Id: in.Id, Name: in.Name, Priority: in.Priority, Logic: logic, Conditions: cond, Actions: a.ruleAction(&in),
	})
	if err != nil {
		return nil, err
	}
	return ruleWire(out), nil
}

func (a *API) deleteRule(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqRule
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	in.Id = r.PathValue("id")
	_, err := a.rule.DeleteRule(ctx, &api.DeleteRuleRequest{Id: in.Id})
	if err != nil {
		return nil, err
	}
	return noContent(), nil
}

type reqRulePreview struct {
	Logic      any `json:"logic"`
	Conditions []struct {
		MatchField any    `json:"matchField"`
		Operator   any    `json:"operator"`
		Pattern    string `json:"pattern"`
	} `json:"conditions"`
	Limit int32 `json:"limit"`
}

func (a *API) previewRule(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqRulePreview
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	logic, err := toRuleLogic(in.Logic)
	if err != nil {
		return nil, bad("%v", err)
	}
	cond, err := toConditions(in.Conditions)
	if err != nil {
		return nil, bad("%v", err)
	}
	out, err := a.rule.Preview(ctx, &api.PreviewRuleRequest{
		Logic: logic, Conditions: cond, Limit: in.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]wireTxn, 0, len(out))
	for _, t := range out {
		items = append(items, txnWire(t))
	}
	return struct {
		Transactions []wireTxn `json:"transactions"`
	}{Transactions: items}, nil
}

func (a *API) runRule(ctx context.Context, _ string, r *http.Request) (any, error) {
	out, err := a.transferRule.RunRule(ctx, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	return struct {
		Matched int32 `json:"matched"`
		Linked  int32 `json:"linked"`
		Created int32 `json:"created"`
	}{Matched: out.Matched, Linked: out.Linked, Created: out.Created}, nil
}
