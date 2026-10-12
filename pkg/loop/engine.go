package loop

import (
	"context"
	"time"

	"github.com/agennext/agent-chat/pkg/capability"
	"github.com/agennext/agent-chat/pkg/guard"
	"github.com/agennext/agent-chat/pkg/store"
)

// Engine runs the agent loop. It is stateless across turns (all turn state lives
// in the context/memory stores), which is what lets the platform scale
// horizontally across the edge mesh — build for billions.
type Engine struct {
	// Reasoner proposes actions (the model).
	Reasoner Reasoner
	// Invoker executes capabilities (the ACT step).
	Invoker Invoker
	// Registry holds the admitted capability contracts.
	Registry *capability.Registry
	// Screener screens untrusted tool/retrieval output (in-loop T2 defence).
	Screener guard.Screener
	// Decider is the in-process policy decision point (GUARD step).
	Decider guard.Decider
	// Ctx persists working context.
	Ctx store.ContextStore
	// Mem persists durable memory.
	Mem store.MemoryStore
	// Dedupe provides at-least-once idempotency.
	Dedupe Deduper
	// Budget bounds every turn.
	Budget Budget
	// Now returns the current time (injectable for tests).
	Now func() time.Time
}

func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now().UTC()
}

// estimateTokens is a coarse token estimate (~4 chars/token) used for budgeting.
func estimateTokens(s string) int { return (len(s) + 3) / 4 }

func (r *Result) trace(step string, a ActionKind, capName, decision, detail string, at time.Time) {
	r.Trace = append(r.Trace, TraceEntry{
		Step: step, Action: a, Capability: capName, Decision: decision, Detail: detail, At: at,
	})
}

// Run executes one turn for an admitted event and returns its result. The turn
// is transactional: context/memory deltas are buffered and committed once, at
// the end, so a mid-loop failure does not persist partial state.
func (e *Engine) Run(ctx context.Context, in AdmittedEvent) (Result, error) {
	// Idempotent replay: a redelivered event returns its prior result.
	if e.Dedupe != nil {
		if prior, ok := e.Dedupe.Lookup(in.Event.ID); ok {
			return prior, nil
		}
	}

	sid := in.SessionID()
	res := Result{SessionID: sid}

	// Bound the whole turn with a context deadline so the loop RESOLVES on any
	// node even if a single Reason/Invoke call hangs — termination is preemptive,
	// not merely cooperative between iterations. No knots.
	deadline := e.now().Add(e.Budget.Wall)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	// LOAD (step 2): hydrate buffered context + memory.
	bufCtx, err := e.Ctx.Load(ctx, sid)
	if err != nil {
		return res, err
	}
	mem, err := e.Mem.Load(ctx, sid)
	if err != nil {
		return res, err
	}
	bufCtx.Messages = append(bufCtx.Messages, store.Message{
		Role: "user", Content: string(in.Event.Data), At: e.now(),
	})

	state := State{
		Tenant:      in.Event.Tenant(),
		Principal:   in.Principal,
		SessionID:   sid,
		GuardPrompt: in.GuardPrompt,
		History:     bufCtx.Messages,
		Memory:      mem,
		Scope:       in.Scope,
	}

	var memDelta []store.MemoryItem
	tokens := 0

	for i := 0; ; i++ {
		if i >= e.Budget.MaxIterations {
			res.StoppedBy = "max_iterations"
			break
		}
		if e.now().After(deadline) {
			res.StoppedBy = "wall_clock"
			break
		}
		if tokens >= e.Budget.MaxTokens {
			res.StoppedBy = "token_budget"
			break
		}
		res.Iterations = i + 1

		// REASON (step 3).
		action, err := e.Reasoner.Reason(ctx, state)
		if err != nil {
			return res, err
		}
		tokens += estimateTokens(action.Answer) + estimateTokens(string(action.Input))
		res.trace("reason", action.Kind, action.Capability, "", "", e.now())

		if action.Kind == ActionAnswer {
			res.Answer = action.Answer
			res.StoppedBy = "answer"
			bufCtx.Messages = append(bufCtx.Messages, store.Message{
				Role: "assistant", Content: action.Answer, At: e.now(),
			})
			break
		}

		// The reasoner may choose to exit to a human rather than fabricate an
		// answer it cannot ground. This is a clean exit, not a failure: the EXIT
		// block below sets Escalated and the generic handoff message. The reason
		// is traced, never surfaced to the caller (must never bleed).
		if action.Kind == ActionEscalate {
			res.StoppedBy = "escalate:reason"
			res.trace("escalate", action.Kind, "", "reason", action.Detail, e.now())
			break
		}

		// GUARD (step 4): contract existence, scope, and policy — before ACT.
		obs, ok := e.guard(ctx, &res, state, action)
		if !ok {
			state.Scratch = append(state.Scratch, obs)
			continue
		}

		// ACT (step 5).
		out, err := e.Invoker.Invoke(ctx, action.Capability, action.Input)
		if err != nil {
			blocked := Observation{Capability: action.Capability, Blocked: true, Reason: "invoke error: " + err.Error()}
			state.Scratch = append(state.Scratch, blocked)
			res.trace("act", action.Kind, action.Capability, "error", err.Error(), e.now())
			continue
		}

		// SCREEN (step 6a): untrusted output must be screened before it can
		// influence REASON again — this closes the indirect-injection path (T2).
		if v := e.Screener.Screen(ctx, string(out.Data), out.Origin); v.Malicious {
			blocked := Observation{Capability: action.Capability, Blocked: true, Reason: v.Reason}
			state.Scratch = append(state.Scratch, blocked)
			res.trace("screen", action.Kind, action.Capability, "blocked", v.Reason, e.now())
			continue
		}

		// OBSERVE (step 6b): admit sanitized output and buffer a memory delta.
		observed := Observation{Capability: action.Capability, Output: string(out.Data)}
		state.Scratch = append(state.Scratch, observed)
		memDelta = append(memDelta, store.MemoryItem{Key: "retrieval", Value: string(out.Data), At: e.now()})
		tokens += estimateTokens(string(out.Data))
		res.trace("observe", action.Kind, action.Capability, "ok", "", e.now())
	}

	// EXIT: the agent ALWAYS has an exit. A clean answer exits normally; anything
	// else (budget bound or failure) escalates OUT THROUGH THE EDGE to a human
	// guard — the last exit — instead of emitting a dead or partial result. The
	// handoff message is generic so nothing internal bleeds out (must never bleed).
	exit := "answer"
	if res.StoppedBy != "answer" {
		res.Escalated = true
		exit = "escalate:human"
		if res.Answer == "" {
			res.Answer = humanHandoffMessage
		}
	}

	// SHIELD (exit-surface runtime): every surface has a runtime that ENFORCES
	// protection; this is the egress one. The outbound answer is screened before
	// it leaves — a leak signature is never emitted, it is redacted to the safe
	// handoff and escalated (must never bleed).
	if v := e.Screener.Screen(ctx, res.Answer, guard.OriginOutput); v.Malicious {
		res.Answer = humanHandoffMessage
		res.Escalated = true
		exit = "escalate:shield"
		res.trace("shield", "", "", "blocked", v.Reason, e.now())
	}

	// PERSIST (step 7): commit the transactional turn exactly once.
	if err := e.Ctx.Save(ctx, sid, bufCtx); err != nil {
		return res, err
	}
	for _, m := range memDelta {
		if err := e.Mem.Append(ctx, sid, m); err != nil {
			return res, err
		}
	}
	res.trace("persist", "", "", "ok", "", e.now())

	// EMIT (step 8): record for idempotent replay and return.
	if e.Dedupe != nil {
		e.Dedupe.Store(in.Event.ID, res)
	}
	res.trace("exit", "", "", exit, res.StoppedBy, e.now())
	return res, nil
}

// humanHandoffMessage is the generic last-exit response. It carries no internal
// detail (no stack, no policy text, no prompt) so the agent never bleeds.
const humanHandoffMessage = "I can't resolve this safely right now — connecting you to a person."

// guard performs the GUARD step. It returns a blocking Observation and false
// when the action must not proceed to ACT.
func (e *Engine) guard(ctx context.Context, res *Result, state State, action Action) (Observation, bool) {
	contract, ok := e.Registry.Get(action.Capability)
	if !ok {
		res.trace("guard", action.Kind, action.Capability, "deny", "unknown capability", e.now())
		return Observation{Capability: action.Capability, Blocked: true, Reason: "unknown capability"}, false
	}
	// Requested scope must be within both the contract's grant and the
	// admitted turn scope (defence in depth).
	if !contract.Scope.Contains(action.Scope) || !state.Scope.Contains(action.Scope) {
		res.trace("guard", action.Kind, action.Capability, "deny", "out of scope", e.now())
		return Observation{Capability: action.Capability, Blocked: true, Reason: "out of scope"}, false
	}
	dec := e.Decider.Decide(ctx, guard.Request{
		Tenant: state.Tenant, Principal: state.Principal, Capability: action.Capability, Action: "invoke",
	})
	if !dec.Allow {
		res.trace("guard", action.Kind, action.Capability, "deny", dec.Reason, e.now())
		return Observation{Capability: action.Capability, Blocked: true, Reason: "policy: " + dec.Reason}, false
	}
	res.trace("guard", action.Kind, action.Capability, "allow", dec.Reason, e.now())
	return Observation{}, true
}
