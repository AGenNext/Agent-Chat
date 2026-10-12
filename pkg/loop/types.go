// Package loop implements the agent loop: the bounded, event-driven reason→act
// cycle that is the data-plane spine of AGenNext Chat. It enforces, inside the
// iteration, the platform's invariants: guard-before-act, screen untrusted tool
// output, a transactional turn, and at-least-once idempotency. See docs/LOOP.md.
package loop

import (
	"context"
	"sync"
	"time"

	"github.com/agennext/agent-chat/pkg/capability"
	"github.com/agennext/agent-chat/pkg/event"
	"github.com/agennext/agent-chat/pkg/guard"
	"github.com/agennext/agent-chat/pkg/store"
)

// AdmittedEvent is an event the Edge Gate has already authenticated, authorized,
// scope-checked, and L1-screened. The loop trusts only admitted events.
type AdmittedEvent struct {
	// Event is the original CloudEvent.
	Event event.Event
	// Principal is the authenticated caller.
	Principal string
	// Scope is the least-privilege authority granted for this turn.
	Scope capability.Scope
	// GuardPrompt is the system Guard Prompt to prepend before generation.
	GuardPrompt string
}

// SessionID returns the session identifier (the event subject).
func (a AdmittedEvent) SessionID() string { return a.Event.Subject }

// ActionKind is the kind of action the reasoner proposes.
type ActionKind string

const (
	// ActionAnswer ends the turn with a final response.
	ActionAnswer ActionKind = "answer"
	// ActionInvoke calls a capability (a tool/retrieval step).
	ActionInvoke ActionKind = "invoke"
	// ActionEscalate ends the turn by exiting to a human guard. The reasoner
	// chooses this when it cannot resolve the turn on grounded evidence rather
	// than emitting a fabricated answer — the promise is intent, not assurance,
	// and the agent always has an exit. Detail carries a non-bleeding reason for
	// the trace; the caller-facing answer stays the generic handoff.
	ActionEscalate ActionKind = "escalate"
)

// Action is the reasoner's proposed next step.
type Action struct {
	// Kind is answer or invoke.
	Kind ActionKind
	// Answer is the final response text (when Kind == ActionAnswer).
	Answer string
	// Detail is a non-bleeding reason recorded in the trace (e.g. with
	// Kind == ActionEscalate). It never reaches the caller-facing answer.
	Detail string
	// Capability is the capability to invoke (when Kind == ActionInvoke).
	Capability string
	// Input is the payload passed to the capability.
	Input []byte
	// Scope is the least-privilege authority this action requires.
	Scope capability.Scope
}

// Observation is the sanitized result of an ACT step fed back into REASON.
type Observation struct {
	// Capability that produced the observation.
	Capability string
	// Output is the sanitized capability output (empty when blocked).
	Output string
	// Blocked is true when the output was rejected (e.g. injection).
	Blocked bool
	// Reason explains a block (inspectability).
	Reason string
}

// State is the reasoner's view of the turn.
type State struct {
	Tenant      string
	Principal   string
	SessionID   string
	GuardPrompt string
	History     []store.Message
	Memory      []store.MemoryItem
	Scratch     []Observation
	Scope       capability.Scope
}

// Reasoner proposes the next action given the current turn state. The production
// binding is an LLM (model-agnostic, via the inference gateway); tests use
// deterministic reasoners.
type Reasoner interface {
	Reason(ctx context.Context, s State) (Action, error)
}

// Output is the result of invoking a capability, tagged with its trust origin.
type Output struct {
	// Data is the (untrusted) capability output.
	Data []byte
	// Origin is the trust origin used when screening the output.
	Origin guard.Origin
}

// Invoker executes a capability (the ACT step) and returns untrusted output.
type Invoker interface {
	Invoke(ctx context.Context, capName string, input []byte) (Output, error)
}

// Budget bounds a turn so the loop always terminates.
type Budget struct {
	// MaxIterations caps reason→act cycles.
	MaxIterations int
	// MaxTokens caps approximate tokens consumed.
	MaxTokens int
	// Wall caps wall-clock time for the turn.
	Wall time.Duration
}

// DefaultBudget is the platform default (6 iterations / 8k tokens / 30s).
func DefaultBudget() Budget {
	return Budget{MaxIterations: 6, MaxTokens: 8000, Wall: 30 * time.Second}
}

// TraceEntry is one inspectable step in a turn (no hidden logic).
type TraceEntry struct {
	Step       string
	Action     ActionKind
	Capability string
	Decision   string
	Detail     string
	At         time.Time
}

// Result is the outcome of a turn.
type Result struct {
	SessionID  string
	Answer     string
	Iterations int
	StoppedBy  string
	// Escalated is true when the agent could not resolve cleanly and the turn
	// exits OUT THROUGH THE EDGE to a human guard (the last exit) rather than
	// emitting a dead or partial result. The agent always has an exit.
	Escalated bool
	Trace     []TraceEntry
}

// Deduper provides at-least-once idempotency: a replayed event id returns the
// prior result instead of re-running side effects.
type Deduper interface {
	Lookup(id string) (Result, bool)
	Store(id string, r Result)
}

// MemDeduper is an in-memory Deduper.
type MemDeduper struct {
	mu sync.Mutex
	m  map[string]Result
}

// NewMemDeduper returns an empty in-memory deduper.
func NewMemDeduper() *MemDeduper { return &MemDeduper{m: make(map[string]Result)} }

// Lookup returns a previously stored result for id, if any.
func (d *MemDeduper) Lookup(id string) (Result, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r, ok := d.m[id]
	return r, ok
}

// Store records the result for id.
func (d *MemDeduper) Store(id string, r Result) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.m[id] = r
}
