package conformance

import (
	"context"
	"strconv"
	"testing"

	"github.com/agennext/agent-chat/pkg/capability"
	"github.com/agennext/agent-chat/pkg/event"
	"github.com/agennext/agent-chat/pkg/loop"
)

// admittedN builds a unique admitted event per iteration so the deduper does not
// short-circuit the turn (we want to measure the loop, not the replay cache).
func admittedN(i int) loop.AdmittedEvent {
	id := "evt-bench-" + strconv.Itoa(i)
	sess := "session-" + strconv.Itoa(i)
	ev := event.New(id, "channel/web", "chat.message.v1", sess,
		"acme", "user-1", "rag.retrieve", []byte("What is your return policy?"))
	return loop.AdmittedEvent{
		Event:       ev,
		Principal:   "user-1",
		Scope:       capability.Scope{Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/*"}},
		GuardPrompt: "guard",
	}
}

// BenchmarkCoreTurn measures a full retrieve-then-answer turn (reason → guard →
// act → screen → observe → persist → exit) on the in-memory bindings. This is the
// performance/efficiency baseline; report ns/op and allocs/op with -benchmem.
func BenchmarkCoreTurn(b *testing.B) {
	t := &testing.T{}
	eng := newEngine(t)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := eng.Run(ctx, admittedN(i))
		if err != nil {
			b.Fatalf("run: %v", err)
		}
		if res.StoppedBy != "answer" {
			b.Fatalf("expected answered turn, got %q", res.StoppedBy)
		}
	}
}
