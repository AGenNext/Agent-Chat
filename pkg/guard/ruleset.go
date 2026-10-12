package guard

import "context"

// Rule permits a class of requests. A request is allowed when it matches a
// rule: every non-empty field must equal the request's field, and the empty
// string "" is the wildcard "any" on the RULE side only (never the request
// side). Rules only ever permit — there is no deny-rule, so a later rule can
// never override an earlier allow. This is the platform's refusal of negative
// enforcement: authority is added, never subtracted.
type Rule struct {
	// Tenant the rule applies to ("" = any).
	Tenant string
	// Principal the rule applies to ("" = any).
	Principal string
	// Capability the rule permits ("" = any).
	Capability string
	// Action the rule permits ("" = any), e.g. "invoke".
	Action string
	// Reason is surfaced on a match (inspectability / no hidden logic).
	Reason string
}

// matches reports whether the rule covers a concrete request. Empty rule fields
// are wildcards; request fields are always concrete.
func (r Rule) matches(req Request) bool {
	return (r.Tenant == "" || r.Tenant == req.Tenant) &&
		(r.Principal == "" || r.Principal == req.Principal) &&
		(r.Capability == "" || r.Capability == req.Capability) &&
		(r.Action == "" || r.Action == req.Action)
}

// RuleDecider is a deny-by-default policy decision point: a request is allowed
// only when some rule permits it. It is the richer, in-process OPA-shaped
// binding behind Decider — matching on tenant, principal, capability, and
// action rather than capability name alone. The zero value denies everything;
// add authority with Allow.
type RuleDecider struct {
	rules []Rule
}

// NewRuleDecider returns a decider seeded with the given rules (optional).
func NewRuleDecider(rules ...Rule) *RuleDecider {
	return &RuleDecider{rules: append([]Rule(nil), rules...)}
}

// Allow appends a permitting rule and returns the decider, so rules can be
// chained at wiring time. Rules are add-only; there is no way to remove one or
// to add a denial.
func (d *RuleDecider) Allow(r Rule) *RuleDecider {
	d.rules = append(d.rules, r)
	return d
}

// Decide permits the request if any rule matches; otherwise it denies by
// default. The reason names the matched rule (or the default-deny) so the
// decision is inspectable.
func (d *RuleDecider) Decide(_ context.Context, req Request) Decision {
	for _, r := range d.rules {
		if r.matches(req) {
			reason := r.Reason
			if reason == "" {
				reason = "permitted by rule"
			}
			return Decision{Allow: true, Reason: reason}
		}
	}
	return Decision{Allow: false, Reason: "default-deny: no rule permits this request"}
}
