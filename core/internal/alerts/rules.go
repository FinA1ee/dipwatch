// Package alerts checks members' tracking rules after each source refresh and
// writes alerts (spec T1, T4, T5). Delivery is the notify job's work.
package alerts

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FinA1ee/dipwatch/core/internal/money"
)

// Rule types (T1).
const (
	RuleBelow       = "below"         // price below an amount
	RuleDrop        = "drop"          // dropped by a percentage since tracking started
	RuleBackInStock = "back_in_stock" // an offer that was out of stock is in stock again
	RuleNewSource   = "new_source"    // an offer from a source the product had none from
)

// Money is the API's amount type.
type Money = money.Money

// Rule is one tracking rule, as stored in watch_items.rules and sent by the API.
type Rule struct {
	Type    string `json:"type" enum:"below,drop,back_in_stock,new_source"`
	Below   *Money `json:"below,omitempty" doc:"For below: alert when an offer costs less than this"`
	Percent int    `json:"percent,omitempty" minimum:"0" maximum:"90" doc:"For drop: alert at this drop from the price when tracking started"`
}

// MaxRules keeps one tracker's rule list small.
const MaxRules = 8

// RuleError is a member's mistake in a rule list.
type RuleError string

func (e RuleError) Error() string { return string(e) }

// Validate checks rules from a member; every error is a RuleError.
func Validate(rules []Rule) error {
	if len(rules) == 0 {
		return RuleError("add at least one rule")
	}
	if len(rules) > MaxRules {
		return RuleError(fmt.Sprintf("at most %d rules", MaxRules))
	}
	for _, r := range rules {
		switch r.Type {
		case RuleBelow:
			if r.Below == nil || r.Below.Minor <= 0 || len(r.Below.Currency) != 3 {
				return RuleError("a below rule needs a positive amount and a currency")
			}
		case RuleDrop:
			if r.Percent < 1 || r.Percent > 90 {
				return RuleError("a drop rule needs a percentage from 1 to 90")
			}
		case RuleBackInStock, RuleNewSource:
		default:
			return RuleError(fmt.Sprintf("unknown rule %q", r.Type))
		}
	}
	return nil
}

// Parse reads stored rules; unknown entries are skipped rather than failing a refresh.
func Parse(raw []byte) []Rule {
	var rules []Rule
	_ = json.Unmarshal(raw, &rules)
	return rules
}

// Key names a rule in alerts.rule, so a changed threshold counts as a new rule (T5).
func (r Rule) Key() string {
	switch r.Type {
	case RuleBelow:
		if r.Below != nil {
			return fmt.Sprintf("below:%d:%s", r.Below.Minor, r.Below.Currency)
		}
	case RuleDrop:
		return fmt.Sprintf("drop:%d", r.Percent)
	}
	return r.Type
}

// Describe is the rule in words, for messages.
func (r Rule) Describe(format func(Money) string) string {
	switch r.Type {
	case RuleBelow:
		if r.Below != nil {
			return "price below " + format(*r.Below)
		}
	case RuleDrop:
		return fmt.Sprintf("price %d%% below where it was when you started tracking", r.Percent)
	case RuleBackInStock:
		return "back in stock"
	case RuleNewSource:
		return "offered by a new store"
	}
	return r.Type
}

// DescribeKey turns a stored rule key back into words, for messages.
func DescribeKey(key string, format func(Money) string) string {
	var r Rule
	var m Money
	switch {
	case strings.HasPrefix(key, "below:"):
		if _, err := fmt.Sscanf(key, "below:%d:%s", &m.Minor, &m.Currency); err == nil {
			r = Rule{Type: RuleBelow, Below: &m}
		}
	case strings.HasPrefix(key, "drop:"):
		r.Type = RuleDrop
		_, _ = fmt.Sscanf(key, "drop:%d", &r.Percent)
	default:
		r.Type = key
	}
	return r.Describe(format)
}
