package reason

import (
	"context"
	"testing"

	"github.com/agennext/agent-chat/pkg/capability"
	"github.com/agennext/agent-chat/pkg/loop"
	"github.com/agennext/agent-chat/pkg/store"
)

func state(scratch ...loop.Observation) loop.State {
	return loop.State{
		Tenant:    "acme",
		Principal: "user-1",
		SessionID: "s1",
		History:   []store.Message{{Role: "user", Content: "what is your return policy?"}},
		Scope:     capability.Scope{Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/*"}},
		Scratch:   scratch,
	}
}

func TestRetrievesFirstWithinScope(t *testing.T) {
	r := GroundedReasoner{Retrieve: "rag.retrieve", Preface: "KB: "}
	act, err := r.Reason(context.Background(), state())
	if err != nil {
		t.Fatalf("reason: %v", err)
	}
	if act.Kind != loop.ActionInvoke || act.Capability != "rag.retrieve" {
		t.Fatalf("expected invoke rag.retrieve, got %+v", act)
	}
	if string(act.Input) != "what is your return policy?" {
		t.Errorf("input should be the user's question, got %q", act.Input)
	}
	// The requested scope must not exceed the admitted turn scope.
	if !state().Scope.Contains(act.Scope) {
		t.Errorf("requested scope %+v exceeds turn scope", act.Scope)
	}
}

func TestAnswersFromEvidence(t *testing.T) {
	r := GroundedReasoner{Retrieve: "rag.retrieve", Preface: "KB: "}
	obs := loop.Observation{Capability: "rag.retrieve", Output: "returns within 30 days"}
	act, err := r.Reason(context.Background(), state(obs))
	if err != nil {
		t.Fatalf("reason: %v", err)
	}
	if act.Kind != loop.ActionAnswer {
		t.Fatalf("expected answer, got %+v", act)
	}
	if act.Answer != "KB: returns within 30 days" {
		t.Errorf("answer should be grounded in the evidence with preface, got %q", act.Answer)
	}
}

func TestEscalatesWhenRetrievalYieldsNothing(t *testing.T) {
	r := GroundedReasoner{Retrieve: "rag.retrieve"}
	// Retrieval was attempted but blocked — no usable evidence. The reasoner
	// must escalate, never fabricate.
	blocked := loop.Observation{Capability: "rag.retrieve", Blocked: true, Reason: "screened"}
	act, err := r.Reason(context.Background(), state(blocked))
	if err != nil {
		t.Fatalf("reason: %v", err)
	}
	if act.Kind != loop.ActionEscalate {
		t.Fatalf("expected escalate, got %+v", act)
	}
	if act.Answer != "" {
		t.Errorf("escalation must carry no fabricated answer, got %q", act.Answer)
	}
}

func TestEscalatesWhenNoRetrievalConfigured(t *testing.T) {
	r := GroundedReasoner{} // no Retrieve capability
	act, err := r.Reason(context.Background(), state())
	if err != nil {
		t.Fatalf("reason: %v", err)
	}
	if act.Kind != loop.ActionEscalate {
		t.Fatalf("with no retrieval and no evidence, must escalate; got %+v", act)
	}
}

func TestRetrievesAtMostOnce(t *testing.T) {
	r := GroundedReasoner{Retrieve: "rag.retrieve"}
	// An empty (not blocked) observation still counts as an attempt, so the
	// reasoner does not loop on retrieval — it escalates.
	empty := loop.Observation{Capability: "rag.retrieve", Output: ""}
	act, err := r.Reason(context.Background(), state(empty))
	if err != nil {
		t.Fatalf("reason: %v", err)
	}
	if act.Kind != loop.ActionEscalate {
		t.Fatalf("expected escalate after one empty retrieval, got %+v", act)
	}
}
