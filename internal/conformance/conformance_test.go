// Package conformance holds cross-cutting invariants — the platform's stance made
// executable. Unit tests prove each component; these prove properties of the whole
// that must not drift or corrode over time:
//
//   - Headless / zero-dependency: the core stays pure Go stdlib, forever.
//   - Deterministic: the same input yields the same output and the same trace —
//     unbiased and consistent by construction.
//
// If a future change erodes the stance (a dependency sneaks in, a nondeterministic
// path appears), one of these tests fails. That is the point.
package conformance

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agennext/agent-chat/pkg/capability"
	"github.com/agennext/agent-chat/pkg/event"
	"github.com/agennext/agent-chat/pkg/guard"
	"github.com/agennext/agent-chat/pkg/loop"
	"github.com/agennext/agent-chat/pkg/store"
)

const modulePath = "github.com/agennext/agent-chat"

// moduleRoot walks up from the test's working directory to the directory holding
// go.mod (the module root).
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from working directory")
		}
		dir = parent
	}
}

// TestZeroThirdPartyDependencies is the anti-corrosion guard for the headless
// promise: go.mod must declare no third-party modules, and no go.sum may exist
// (a go.sum means dependencies were locked, i.e. the promise already broke).
func TestZeroThirdPartyDependencies(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)

	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for i, line := range strings.Split(string(gomod), "\n") {
		s := strings.TrimSpace(line)
		// A populated require/replace introduces an external module.
		if strings.HasPrefix(s, "require ") || strings.HasPrefix(s, "require(") ||
			strings.HasPrefix(s, "replace ") {
			t.Errorf("go.mod:%d declares a dependency, breaking the zero-dep promise: %q", i+1, s)
		}
		// A bare `require (` block opener with entries below is equally a break.
		if s == "require (" {
			t.Errorf("go.mod:%d opens a require() block; the core must stay dependency-free", i+1)
		}
	}

	if _, err := os.Stat(filepath.Join(root, "go.sum")); err == nil {
		t.Error("go.sum exists; the dependency-free core must not lock any module")
	}
}

// TestNoThirdPartyImports is the belt-and-suspenders guard: every import in every
// Go file (including tests) must be either a standard-library package or a package
// of this module. Any other import is a third-party dependency sneaking in.
func TestNoThirdPartyImports(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip VCS, vendor, and node_modules trees.
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			t.Errorf("parse %s: %v", path, perr)
			return nil
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if isStdlib(p) || p == modulePath || strings.HasPrefix(p, modulePath+"/") {
				continue
			}
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s imports third-party package %q (only stdlib and %s are allowed)", rel, p, modulePath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// isStdlib reports whether an import path is a standard-library package. Stdlib
// paths never have a dot in their first segment (third-party hosts like
// github.com/... always do).
func isStdlib(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}

// --- determinism conformance ---

// fixedReasoner: retrieve once, then answer from the observation. Deterministic
// given the same state — no randomness, no clock, no external read.
type fixedReasoner struct{}

func (fixedReasoner) Reason(_ context.Context, s loop.State) (loop.Action, error) {
	if len(s.Scratch) == 0 {
		return loop.Action{
			Kind:       loop.ActionInvoke,
			Capability: "rag.retrieve",
			Input:      []byte("return policy"),
			Scope:      capability.Scope{Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/faq"}},
		}, nil
	}
	answer := "no answer"
	for _, o := range s.Scratch {
		if !o.Blocked && o.Output != "" {
			answer = "kb: " + o.Output
		}
	}
	return loop.Action{Kind: loop.ActionAnswer, Answer: answer}, nil
}

type fixedInvoker struct{}

func (fixedInvoker) Invoke(_ context.Context, _ string, _ []byte) (loop.Output, error) {
	return loop.Output{Data: []byte("returns within 30 days"), Origin: guard.OriginRetrieved}, nil
}

// newEngine builds an engine with fresh stores and a fixed clock, so two engines
// built identically must produce byte-identical results for identical input.
func newEngine(t *testing.T) *loop.Engine { return newEngineWith(t, fixedReasoner{}) }

// newEngineWith is newEngine parameterized by reasoner, for turns that do not
// resolve cleanly.
func newEngineWith(t *testing.T, r loop.Reasoner) *loop.Engine {
	t.Helper()
	reg := capability.NewRegistry()
	if err := reg.Register(capability.Contract{
		Name: "rag.retrieve", Version: "0.1.0", Provides: []string{"retrieve"},
		Scope:   capability.Scope{Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/*"}},
		Sandbox: capability.SandboxIsolated,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	fixed := time.Unix(1700000000, 0).UTC()
	return &loop.Engine{
		Reasoner: r, Invoker: fixedInvoker{}, Registry: reg,
		Screener: guard.NewHeuristicScreener(), Decider: guard.NewStaticDecider("rag.retrieve"),
		Ctx: store.NewMemContextStore(), Mem: store.NewMemMemoryStore(),
		Dedupe: loop.NewMemDeduper(), Budget: loop.DefaultBudget(),
		Now: func() time.Time { return fixed },
	}
}

func admitted() loop.AdmittedEvent {
	ev := event.New("evt-det-1", "channel/web", "chat.message.v1", "session-1",
		"acme", "user-1", "rag.retrieve", []byte("What is your return policy?"))
	return loop.AdmittedEvent{
		Event:       ev,
		Principal:   "user-1",
		Scope:       capability.Scope{Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/*"}},
		GuardPrompt: "guard",
	}
}

// TestCoreIsDeterministic proves the unbiased/consistent stance: two independent
// cores, the same input, must yield identical answer, control outcome, and trace.
func TestCoreIsDeterministic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	r1, err := newEngine(t).Run(ctx, admitted())
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	r2, err := newEngine(t).Run(ctx, admitted())
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}

	if !reflect.DeepEqual(r1, r2) {
		t.Fatalf("core is not deterministic:\n run1=%+v\n run2=%+v", r1, r2)
	}
	// Sanity: the run actually exercised the loop (answered, not a degenerate exit).
	if r1.StoppedBy != "answer" || r1.Answer == "" {
		t.Fatalf("expected a real answered turn, got StoppedBy=%q Answer=%q", r1.StoppedBy, r1.Answer)
	}
}

// --- no faked heartbeat: every turn offers a recovery path ---

// neverResolvesReasoner always invokes and never answers, so the turn can never
// resolve on its own. A turn like this must NOT be reported as a clean stop — it
// must take the recovery path (escalate to a human). A "done" with no answer and
// no escalation would be a faked heartbeat: health signalled with no path to heal.
type neverResolvesReasoner struct{}

func (neverResolvesReasoner) Reason(_ context.Context, _ loop.State) (loop.Action, error) {
	return loop.Action{
		Kind:       loop.ActionInvoke,
		Capability: "rag.retrieve",
		Input:      []byte("again"),
		Scope:      capability.Scope{Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/faq"}},
	}, nil
}

// TestNoFakedHeartbeatAlwaysOffersRecoveryPath enforces the rule: a turn either
// truly answers, or it escalates to a human — never a non-answer stop with no
// recovery path. Faking completion (a healthy-looking exit that cannot heal) is
// banned, and this proves the loop cannot produce one.
func TestNoFakedHeartbeatAlwaysOffersRecoveryPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// A turn that cannot resolve must escalate, not fake a clean finish.
	unresolved, err := newEngineWith(t, neverResolvesReasoner{}).Run(ctx, admitted())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if unresolved.StoppedBy == "answer" {
		t.Fatalf("a never-answering reasoner must not report StoppedBy=answer")
	}
	if !unresolved.Escalated {
		t.Fatalf("unresolved turn did not offer a recovery path (Escalated=false): %+v", unresolved)
	}

	// The invariant, both directions: Escalated <=> the turn did not truly answer.
	// There is no clean stop without a real answer, and no real answer that also
	// escalates. Heartbeat and recovery path are never decoupled.
	resolved, err := newEngine(t).Run(ctx, admitted())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resolved.Escalated != (resolved.StoppedBy != "answer") {
		t.Fatalf("recovery-path invariant violated: StoppedBy=%q Escalated=%v", resolved.StoppedBy, resolved.Escalated)
	}
	if unresolved.Escalated != (unresolved.StoppedBy != "answer") {
		t.Fatalf("recovery-path invariant violated: StoppedBy=%q Escalated=%v", unresolved.StoppedBy, unresolved.Escalated)
	}
}
