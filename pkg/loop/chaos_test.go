package loop

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agennext/agent-chat/pkg/guard"
)

// Chaos / fault-injection: a misbehaving tool must never break the always-exit
// guarantee. Whatever the ACT step does — error, panic, or hang — the turn must
// terminate, escalate to a human, and bleed no internal detail.

// errInvoker always fails.
type errInvoker struct{}

func (errInvoker) Invoke(context.Context, string, []byte) (Output, error) {
	return Output{}, errors.New("upstream exploded")
}

// panicInvoker panics inside the ACT step.
type panicInvoker struct{}

func (panicInvoker) Invoke(context.Context, string, []byte) (Output, error) {
	panic("tool blew up")
}

// hangInvoker ignores ctx and blocks past the turn deadline. The fence must
// abandon it; the loop must not wait for it.
type hangInvoker struct{ d time.Duration }

func (h hangInvoker) Invoke(context.Context, string, []byte) (Output, error) {
	time.Sleep(h.d)
	return Output{Data: []byte("too late"), Origin: guard.OriginRetrieved}, nil
}

// assertSafeExit checks the always-exit invariants: escalated, a non-empty
// handoff that leaks nothing internal.
func assertSafeExit(t *testing.T, res Result) {
	t.Helper()
	if !res.Escalated {
		t.Error("a turn the tool could not complete must escalate to a human")
	}
	if res.Answer == "" {
		t.Error("escalation must still produce a safe exit message")
	}
	for _, bad := range []string{"panic", "exploded", "blew up", "deadline", "error:", "goroutine", "scope", "policy"} {
		if strings.Contains(strings.ToLower(res.Answer), bad) {
			t.Fatalf("handoff bled internal detail (%q): %q", bad, res.Answer)
		}
	}
}

func TestChaosInvokerError(t *testing.T) {
	t.Parallel()
	// Reasoner always invokes, never answers — so the only way out is escalation.
	r := &scriptedReasoner{steps: manyInvokes(100)}
	e := newEngine(t, r, errInvoker{}, "rag.retrieve")
	res, err := e.Run(context.Background(), admit(acmeScope()))
	if err != nil {
		t.Fatalf("run returned a hard error instead of escalating: %v", err)
	}
	assertSafeExit(t, res)
}

func TestChaosInvokerPanicDoesNotCrash(t *testing.T) {
	t.Parallel()
	r := &scriptedReasoner{steps: manyInvokes(100)}
	e := newEngine(t, r, panicInvoker{}, "rag.retrieve")
	// The call itself must not propagate the panic.
	res, err := e.Run(context.Background(), admit(acmeScope()))
	if err != nil {
		t.Fatalf("run returned a hard error instead of escalating: %v", err)
	}
	assertSafeExit(t, res)
}

func TestChaosInvokerHangIsBounded(t *testing.T) {
	t.Parallel()
	// One invoke that sleeps well past the turn's wall-clock budget.
	r := &scriptedReasoner{steps: manyInvokes(100)}
	e := newEngine(t, r, hangInvoker{d: 5 * time.Second}, "rag.retrieve")
	e.Budget = Budget{MaxIterations: 6, MaxTokens: 8000, Wall: 100 * time.Millisecond}

	done := make(chan Result, 1)
	start := time.Now()
	go func() {
		res, err := e.Run(context.Background(), admit(acmeScope()))
		if err != nil {
			t.Errorf("run returned a hard error instead of escalating: %v", err)
		}
		done <- res
	}()

	select {
	case res := <-done:
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("loop waited for the hung tool (%s); the fence did not bound it", elapsed)
		}
		assertSafeExit(t, res)
	case <-time.After(3 * time.Second):
		t.Fatal("loop hung on a tool that ignores ctx — the always-exit guarantee failed")
	}
}
