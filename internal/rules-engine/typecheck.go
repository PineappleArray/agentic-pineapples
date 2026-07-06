package rulesengine

import (
	"errors"
	"fmt"

	"github.com/PineappleArray/agentic-pineapples/internal/types"
)

// This will validate the rules to make sure they exist
func (rs *Ruleset) validate_type() error {
	if len(rs.Flags) == 0 {
		return errors.New("Ruleset has no flags")
	}
	for key, arr := range rs.Flags {
		if len(arr.Match) == 0 {
			return fmt.Errorf("flag %q has no match patterns", key)
		}
	}
	if len(rs.Rules) == 0 {
		return errors.New("ruleset has no rules")
	}
	for i := range rs.Rules {
		if err := rs.Rules[i].checkRule(); err != nil {
			return err
		}
	}

	return nil
}

// checkRule ensures a compiled rule is well-formed: required fields are set,
// its CEL program compiled successfully, and Outcome/Urgency match one of
// the enum consts defined in the types package.
func (r *CompiledRule) checkRule() error {
	if r.ID == "" {
		return errors.New("rule has empty id")
	}
	if r.Condition == "" {
		return fmt.Errorf("rule %q has empty condition", r.ID)
	}
	if r.Program == nil {
		return fmt.Errorf("rule %q has no compiled CEL program", r.ID)
	}

	switch r.Outcome {
	case types.OutcomeEscalate, types.OutcomeDraft, types.OutcomeAutoRespond, types.OutcomeIgnore:
	default:
		return fmt.Errorf("rule %q has invalid outcome %q", r.ID, r.Outcome)
	}

	switch r.Urgency {
	case "", types.PriorityP1, types.PriorityP2, types.PriorityP3, types.PriorityP4:
	default:
		return fmt.Errorf("rule %q has invalid urgency %q", r.ID, r.Urgency)
	}

	return nil
}

/*
Ruleset{
    Version: "2026-07-04.1",

    Flags: map[string]FlagDef{
        "refund_related": {
            Match: []string{"refund", "reimburse", "reimbursement", "money back", "my money", "chargeback", "charge back", "dispute the charge"},
        },
        "legal_threat": {
            Match: []string{"lawyer", "attorney", "legal action", "lawsuit", "small claims", "sue you", "suing", "report you to"},
        },
        // ... billing_related, data_loss, security_concern, churn_risk
    },

    Rules: []CompiledRule{
        {
            Rule: types.Rule{
                ID:        "escalate-legal-threat",
                Priority:  100,
                Condition: `event.text_flags.exists(f, f == "legal_threat")`,
                Outcome:   types.OutcomeEscalate,   // "escalate"
                Urgency:   types.PriorityP2,        // "p2"
                Version:   "",                      // per-rule version, unset here
            },
            Program: ,
        },
        {
            Rule: types.Rule{
                ID:        "default-catch-all",
                Priority:  0,
                Condition: `true`,
                Outcome:   types.OutcomeDraft,       // "draft"
                Urgency:   types.PriorityP3,         // "p3"
            },
            Program: ,
        },
        // ... 8 more, one per entry under `rules:` in rules.yaml
    },
}
*/
