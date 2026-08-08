package httpserver

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"time"

	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/services"
)

// dto.go holds shared wire helpers used across feature handler files.
// Each feature's request types, response types, and mappers live in its own
// handler file (e.g. account_handlers.go), not here.

func queryInt(q url.Values, key string) int32 {
	if v := q.Get(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(n)
		}
	}
	return 0
}

func queryFloat(q url.Values, key string) float32 {
	if v := q.Get(key); v != "" {
		if n, err := strconv.ParseFloat(v, 32); err == nil {
			return float32(n)
		}
	}
	return 0
}

func queryDate(q url.Values, key string) (time.Time, error) {
	if s := q.Get(key); s != "" {
		return time.Parse("2006-01-02", s)
	}
	return time.Time{}, nil
}

type wireBulk struct {
	Success   bool     `json:"success"`
	Message   string   `json:"message"`
	FailedIds []string `json:"failedIds,omitempty"`
	Skipped   int32    `json:"skipped,omitempty"`
}

func bulkWire(b *api.BulkOperationResponse) wireBulk {
	return wireBulk{
		Success:   b.Success,
		Message:   b.Message,
		FailedIds: b.FailedIds,
		Skipped:   b.Skipped,
	}
}

func tsRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// cents rounds a money value to the nearest cent so float32 storage noise
// doesn't leak into the wire.
func cents(v float32) float64 {
	return math.Round(float64(v)*100) / 100
}

func bad(format string, a ...any) error {
	return &services.APIError{Status: 400, Msg: fmt.Sprintf(format, a...)}
}

// --- categories ---

type wireCategory struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

func categoryWire(c *api.CategoryResponse) wireCategory {
	return wireCategory{
		Id:        c.Id,
		Name:      c.Name,
		CreatedAt: tsRFC3339(c.CreatedAt),
	}
}

// --- rules ---

type wireCondition struct {
	MatchField int32  `json:"matchField"`
	Operator   int32  `json:"operator"`
	Pattern    string `json:"pattern"`
}

type wireRuleAction struct {
	SetName              string `json:"setName"`
	SetNameOp            int32  `json:"setNameOp"`
	SetCategoryId        string `json:"setCategoryId"`
	SetTransferAccountId string `json:"setTransferAccountId"`
}

type wireRule struct {
	Id         string            `json:"id"`
	Name       string            `json:"name"`
	Priority   int32             `json:"priority"`
	Logic      int32             `json:"logic"`
	Conditions []wireCondition   `json:"conditions"`
	Actions    []wireRuleAction  `json:"actions"`
	CreatedAt  string            `json:"createdAt"`
}

func ruleWire(a *api.RuleResponse) wireRule {
	out := wireRule{
		Id:        a.Id,
		Name:      a.Name,
		Priority:  a.Priority,
		Logic:     int32(a.Logic),
		CreatedAt: tsRFC3339(a.CreatedAt),
	}
	for _, c := range a.Conditions {
		out.Conditions = append(out.Conditions, wireCondition{
			MatchField: int32(c.MatchField),
			Operator:   int32(c.Operator),
			Pattern:    c.Pattern,
		})
	}
	for _, act := range a.Actions {
		out.Actions = append(out.Actions, wireRuleAction{
			SetName:              act.SetName,
			SetNameOp:            int32(act.SetNameOp),
			SetCategoryId:        act.SetCategoryId,
			SetTransferAccountId: act.SetTransferAccountId,
		})
	}
	return out
}

// toRuleLogic accepts 1/2 (RULE_LOGIC_OR/AND) or its string name.
func toRuleLogic(v any) (api.RuleLogic, error) {
	switch x := v.(type) {
	case nil:
		return 0, fmt.Errorf("logic is required")
	case float64:
		return api.RuleLogic(x), nil
	case string:
		if n, ok := api.RuleLogic_value[x]; ok {
			return api.RuleLogic(n), nil
		}
		return 0, fmt.Errorf("unknown logic %q", x)
	default:
		return 0, fmt.Errorf("invalid logic %v", v)
	}
}

func enumToValue[T ~int32](v any, name string) (T, error) {
	switch x := v.(type) {
	case float64:
		return T(x), nil
	case string:
		var n int64
		if _, err := fmt.Sscanf(x, "%d", &n); err == nil {
			return T(n), nil
		}
	}
	return 0, fmt.Errorf("invalid %s %v", name, v)
}

func toConditions(in []struct {
	MatchField any    `json:"matchField"`
	Operator   any    `json:"operator"`
	Pattern    string `json:"pattern"`
}) ([]*api.RuleCondition, error) {
	out := make([]*api.RuleCondition, 0, len(in))
	for _, c := range in {
		mf, err := enumToValue[api.RuleMatchField](c.MatchField, "matchField")
		if err != nil {
			return nil, err
		}
		op, err := enumToValue[api.RuleMatchOperator](c.Operator, "operator")
		if err != nil {
			return nil, err
		}
		out = append(out, &api.RuleCondition{MatchField: mf, Operator: op, Pattern: c.Pattern})
	}
	return out, nil
}

// --- investments ---

type wireInvestment struct {
	Id             string  `json:"id"`
	Symbol         string  `json:"symbol"`
	Name           string  `json:"name"`
	InvestmentType string  `json:"investmentType"`
	Quantity       float64 `json:"quantity"`
	AvgCost        float64 `json:"avgCost"`
	CurrentPrice   float64 `json:"currentPrice"`
	CurrentValue   float64 `json:"currentValue"`
	UnrealizedPnl  float64 `json:"unrealizedPnl"`
	RealizedPnl    float64 `json:"realizedPnl"`
	ManualNav      float64 `json:"manualNav"`
	LastQuoteAt    string  `json:"lastQuoteAt"`
	CreatedAt      string  `json:"createdAt"`
}

func investmentWire(i *api.InvestmentResponse) wireInvestment {
	return wireInvestment{
		Id:             i.Id,
		Symbol:         i.Symbol,
		Name:           i.Name,
		InvestmentType: i.InvestmentType.String(),
		Quantity:       cents(i.Quantity),
		AvgCost:        cents(i.AvgCost),
		CurrentPrice:   cents(i.CurrentPrice),
		CurrentValue:   cents(i.CurrentValue),
		UnrealizedPnl:  cents(i.UnrealizedPnl),
		RealizedPnl:    cents(i.RealizedPnl),
		ManualNav:      cents(i.ManualNav),
		LastQuoteAt:    tsRFC3339(i.LastQuoteAt),
		CreatedAt:      tsRFC3339(i.CreatedAt),
	}
}

type wireLot struct {
	Id           string  `json:"id"`
	InvestmentId string  `json:"investmentId"`
	Side         int32   `json:"side"`
	Quantity     float64 `json:"quantity"`
	Price        float64 `json:"price"`
	OccurredAt   string  `json:"occurredAt"`
	CreatedAt    string  `json:"createdAt"`
}

func lotWire(l *api.LotResponse) wireLot {
	return wireLot{
		Id:           l.Id,
		InvestmentId: l.InvestmentId,
		Side:         l.Side,
		Quantity:     cents(l.Quantity),
		Price:        cents(l.Price),
		OccurredAt:   tsRFC3339(l.OccurredAt),
		CreatedAt:    tsRFC3339(l.CreatedAt),
	}
}

type wirePortfolioSummary struct {
	TotalInvested      float64 `json:"totalInvested"`
	TotalCurrentValue  float64 `json:"totalCurrentValue"`
	TotalUnrealizedPnl float64 `json:"totalUnrealizedPnl"`
	TotalRealizedPnl   float64 `json:"totalRealizedPnl"`
}

// investmentTypeValue maps "INVESTMENT_TYPE_STOCK"/"INVESTMENT_TYPE_MUTUAL_FUND"
// or a numeric value onto the enum.
func investmentTypeValue(v any) (api.InvestmentType, error) {
	switch x := v.(type) {
	case nil:
		return 0, nil
	case string:
		if n, ok := api.InvestmentType_value[x]; ok {
			return api.InvestmentType(n), nil
		}
		return 0, fmt.Errorf("unknown investment type %q", x)
	case float64:
		return api.InvestmentType(x), nil
	default:
		return 0, fmt.Errorf("invalid investment type %v", v)
	}
}

// lotSideValue accepts 1/-1 or "buy"/"sell".
func lotSideValue(v any) (int32, error) {
	switch x := v.(type) {
	case nil:
		return 0, fmt.Errorf("side is required")
	case float64:
		s := int32(x)
		if s != 1 && s != -1 {
			return 0, fmt.Errorf("side must be 1 or -1")
		}
		return s, nil
	case string:
		switch x {
		case "buy":
			return 1, nil
		case "sell":
			return -1, nil
		}
		return 0, fmt.Errorf("side must be buy or sell")
	default:
		return 0, fmt.Errorf("invalid side %v", v)
	}
}
