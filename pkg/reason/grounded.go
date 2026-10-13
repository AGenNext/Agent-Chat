// Package reason provides reasoner bindings for the agent loop's REASON step.
// The production binding is an LLM reached through the inference gateway
// (model-agnostic); this package holds the deterministic, inspectable reference
// reasoner that needs no model and no network — the zero-dependency default.
//
// GroundedReasoner answers ONLY from evidence. It encodes the platform's
// promise of intent, not assurance: it will retrieve, answer from what it
// retrieved, and — when it has nothing to ground an answer on — exit to a human
// rather than fabricate. It never invents a response.
package reason

import (
	"context"
	"strings"

	"github.com/agennext/agent-chat/pkg/loop"
	"github.com/agennext/agent-chat/pkg/store"
)

// Compile-time check: GroundedReasoner is a loop.Reasoner.
var _ loop.Reasoner = GroundedReasoner{}

// GroundedReasoner is a stateless, deterministic reasoner. Given a turn:
//   - if the scratch holds usable (non-blocked, non-empty) evidence, it answers
//     from that evidence and nothing else;
//   - otherwise, if a retrieval capability is configured and has not yet been
//     tried this turn, it invokes it within the turn's admitted scope;
//   - otherwise (retrieval tried and yielded nothing usable, or none is
//     configured) it escalates to a human instead of fabricating an answer.
//
// It keeps no state between turns; all turn state is read from loop.State, so a
// single instance is safe to share across the edge mesh.
type GroundedReasoner struct {
	// Retrieve is the capability invoked to gather evidence (e.g.
	// "rag.retrieve"). Empty disables retrieval: with no prior evidence the
	// reasoner escalates immediately.
	Retrieve string
	// Preface is prepended to a grounded answer so the answer states that it is
	// evidence-based (e.g. "Based on our knowledge base: "). Optional.
	Preface string
}

// Reason implements loop.Reasoner.
func (r GroundedReasoner) Reason(_ context.Context, s loop.State) (loop.Action, error) {
	// Answer from evidence if any usable observation is present.
	if evidence := usableEvidence(s.Scratch); len(evidence) > 0 {
		return loop.Action{
			Kind:   loop.ActionAnswer,
			Answer: r.Preface + strings.Join(evidence, " "),
		}, nil
	}

	// No evidence yet: retrieve once, within the admitted turn scope. Scope is
	// the turn's derived scope — never widened by the reasoner. The gate already
	// intersected it with the contract, so it stays within both bounds.
	if r.Retrieve != "" && !attempted(s.Scratch, r.Retrieve) {
		return loop.Action{
			Kind:       loop.ActionInvoke,
			Capability: r.Retrieve,
			Input:      []byte(lastUserMessage(s.History)),
			Scope:      s.Scope,
		}, nil
	}

	// Retrieval is exhausted or disabled and nothing could be grounded. Do not
	// fabricate — exit to a human. The detail is for the trace only.
	return loop.Action{
		Kind:   loop.ActionEscalate,
		Detail: "no grounded evidence for the request",
	}, nil
}

// usableEvidence returns the outputs of the non-blocked, non-empty observations,
// in order. Blocked or empty observations contribute nothing to an answer.
func usableEvidence(scratch []loop.Observation) []string {
	var out []string
	for _, o := range scratch {
		if !o.Blocked && o.Output != "" {
			out = append(out, o.Output)
		}
	}
	return out
}

// attempted reports whether capName has already been invoked this turn (an
// observation for it exists, blocked or not) — so retrieval is tried at most
// once before the reasoner gives up and escalates.
func attempted(scratch []loop.Observation, capName string) bool {
	for _, o := range scratch {
		if o.Capability == capName {
			return true
		}
	}
	return false
}

// lastUserMessage returns the most recent user message content, or "" if none.
func lastUserMessage(history []store.Message) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			return history[i].Content
		}
	}
	return ""
}
