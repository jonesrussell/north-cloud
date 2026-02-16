package classifier

import "context"

// Decision path constants shared by all hybrid classifiers.
const (
	decisionPathBothAgree    = "both_agree"
	decisionPathRuleOverride = "rule_override"
	decisionPathRulesOnly    = "rules_only"
	decisionPathMLOverride   = "ml_override"
	decisionPathMLUpgrade    = "ml_upgrade"
	decisionPathDefault      = "default"
)

// CallMLWithBodyLimit truncates body to maxChars and calls the given classify function.
// Use this so all optional ML classifiers share the same "truncate then call" flow;
// each classifier keeps its own client type and log message on error.
func CallMLWithBodyLimit[T any](
	ctx context.Context,
	title, body string,
	maxChars int,
	call func(context.Context, string, string) (*T, error),
) (*T, error) {
	if maxChars > 0 && len(body) > maxChars {
		body = body[:maxChars]
	}
	return call(ctx, title, body)
}
