package guard

import (
	"context"
	"testing"
)

// RuleDecider must satisfy the Decider seam.
var _ Decider = (*RuleDecider)(nil)

func TestRuleDeciderDenyByDefault(t *testing.T) {
	d := NewRuleDecider()
	dec := d.Decide(context.Background(), Request{
		Tenant: "acme", Principal: "user-1", Capability: "rag.retrieve", Action: "invoke",
	})
	if dec.Allow {
		t.Error("empty ruleset must deny by default")
	}
	if dec.Reason == "" {
		t.Error("denial must carry a reason")
	}
}

func TestRuleDeciderAllowsMatch(t *testing.T) {
	d := NewRuleDecider().Allow(Rule{
		Tenant: "acme", Capability: "rag.retrieve", Action: "invoke", Reason: "acme kb read",
	})
	req := Request{Tenant: "acme", Principal: "anyone", Capability: "rag.retrieve", Action: "invoke"}
	dec := d.Decide(context.Background(), req)
	if !dec.Allow {
		t.Fatalf("matching rule must allow; got %+v", dec)
	}
	if dec.Reason != "acme kb read" {
		t.Errorf("reason: got %q, want the rule's reason", dec.Reason)
	}
}

func TestRuleDeciderFieldSpecificity(t *testing.T) {
	ctx := context.Background()
	d := NewRuleDecider().Allow(Rule{
		Tenant: "acme", Principal: "user-1", Capability: "rag.retrieve", Action: "invoke",
	})

	// Any field differing from the (non-empty) rule field denies.
	for _, req := range []Request{
		{Tenant: "other", Principal: "user-1", Capability: "rag.retrieve", Action: "invoke"},
		{Tenant: "acme", Principal: "user-2", Capability: "rag.retrieve", Action: "invoke"},
		{Tenant: "acme", Principal: "user-1", Capability: "rag.write", Action: "invoke"},
		{Tenant: "acme", Principal: "user-1", Capability: "rag.retrieve", Action: "delete"},
	} {
		if d.Decide(ctx, req).Allow {
			t.Errorf("unexpected allow for %+v", req)
		}
	}
}

func TestRuleDeciderWildcardRuleSide(t *testing.T) {
	ctx := context.Background()
	// Empty fields are wildcards: "any principal may invoke rag.retrieve in acme".
	d := NewRuleDecider().Allow(Rule{Tenant: "acme", Capability: "rag.retrieve", Action: "invoke"})

	if !d.Decide(ctx, Request{Tenant: "acme", Principal: "x", Capability: "rag.retrieve", Action: "invoke"}).Allow {
		t.Error("empty principal field must match any principal")
	}
	// Wildcard is rule-side only: an empty request field must not match a
	// concrete rule field.
	d2 := NewRuleDecider().Allow(Rule{Principal: "user-1", Capability: "rag.retrieve", Action: "invoke"})
	if d2.Decide(ctx, Request{Tenant: "acme", Principal: "", Capability: "rag.retrieve", Action: "invoke"}).Allow {
		t.Error("empty request principal must not match a concrete rule principal")
	}
}

func TestRuleDeciderAddOnly(t *testing.T) {
	ctx := context.Background()
	d := NewRuleDecider().
		Allow(Rule{Capability: "rag.retrieve", Action: "invoke"}).
		Allow(Rule{Capability: "rag.summarize", Action: "invoke"})

	for _, cap := range []string{"rag.retrieve", "rag.summarize"} {
		if !d.Decide(ctx, Request{Tenant: "acme", Capability: cap, Action: "invoke"}).Allow {
			t.Errorf("rule for %q must remain in effect after further Allow calls", cap)
		}
	}
}
