package mail

import (
	"regexp"
	"testing"
)

// The prompt's worked examples must actually extract their own samples.
func TestPromptExamplesExtract(t *testing.T) {
	type ex struct {
		amount, debit, credit, account, merchant, date string
		body                                           string
	}
	examples := []ex{
		{
			amount:   `(?:(?:Rs\.?|INR|USD|EUR|GBP|AED|SGD)\s*|\p{Sc}\s*)?(?P<amount>[\d,]+(?:\.\d{1,2})?)`,
			debit:    `(?i)\bdebited\b`,
			credit:   `(?i)\bcredited\b`,
			account:  `(?i)account ending\s*(?P<account>\d{4})`,
			merchant: `(?i)towards VPA\s+\S+\s*\((?P<merchant>[^)]+)\)`,
			date:     `(?i)\bon\s+(?P<date>\d{1,2}[-/]\d{1,2}[-/]\d{2,4})`,
			body:     "Rs.2050.00 is debited from your account ending 7713 towards VPA ******8172@ybl (BALAJI WINES WALK  IN) on 12-11-26.",
		},
		{
			amount:   `(?:(?:INR|Rs\.?|USD|EUR|GBP|AED|SGD)\s*|\p{Sc}\s*)?(?P<amount>[\d,]+(?:\.\d{1,2})?)`,
			debit:    `(?i)\bspent\b|(?i)\bdebited\b`,
			credit:   `(?i)\bcredited\b`,
			account:  `(?i)card ending\s*(?:XX)?(?P<account>\d{4})`,
			merchant: `(?i)\bat\s+(?P<merchant>.+?)\s+on\s+`,
			date:     `(?i)\bon\s+(?P<date>\d{1,2}[-/. ][A-Za-z]{3,9}[-/. ]\d{4}|\d{1,2}[-/]\d{1,2}[-/]\d{2,4})`,
			body:     "INR 1,234.00 spent on your IDFC FIRST BANK Credit Card ending XX5678 at CAFE COFFEE DAY on 12 Nov 2026.",
		},
	}
	for i, e := range examples {
		spec := TemplateSpec{
			Name: "prompt example",
			Patterns: TemplatePatterns{
				Amount: e.amount, Debit: e.debit, Credit: e.credit,
				Account: e.account, Merchant: e.merchant, Date: e.date,
			},
		}
		tpl, err := CompileTemplate(spec)
		if err != nil {
			t.Fatalf("example %d compile: %v", i, err)
		}
		match, matched := tpl.Extract("", e.body)
		if !matched || match == nil || !match.Complete || match.Extraction == nil {
			t.Fatalf("example %d did not extract: %+v", i, match)
		}
		t.Logf("example %d: %+v", i, *match.Extraction)
	}
}

// Guards against a currency-only capture: the amount group must be numeric.
func TestPromptExampleAmountGroupIsNumeric(t *testing.T) {
	re := regexp.MustCompile(`(?:(?:Rs\.?|INR|USD)\s*|\p{Sc}\s*)?(?P<amount>[\d,]+(?:\.\d{1,2})?)`)
	for _, s := range []string{"Rs.2050.00 debited", "₹1,234.50 debited", "12.50 EUR debited"} {
		m := re.FindStringSubmatch(s)
		if m == nil {
			t.Fatalf("no match: %s", s)
		}
		if _, err := parseAmountValue(m[re.SubexpIndex("amount")]); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}
