// Command agennextd is the headless AGenNext Chat daemon. By default it serves
// the HTTP API (the edge transport) wrapping the chat core; with -demo it runs a
// single turn and prints the inspectable trace.
//
// Production bindings (NATS, OPA, OpenFGA, KServe, PostgreSQL) replace the
// in-memory stubs behind the same interfaces; the gRPC transport (proto Chat
// service) is the release-gated alternative to this stdlib HTTP server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agennext/agent-chat/channels/mattermost"
	"github.com/agennext/agent-chat/channels/slack"
	"github.com/agennext/agent-chat/pkg/capability"
	"github.com/agennext/agent-chat/pkg/chat"
	"github.com/agennext/agent-chat/pkg/edge"
	"github.com/agennext/agent-chat/pkg/event"
	"github.com/agennext/agent-chat/pkg/guard"
	"github.com/agennext/agent-chat/pkg/kernel"
	"github.com/agennext/agent-chat/pkg/loop"
	"github.com/agennext/agent-chat/pkg/server"
	"github.com/agennext/agent-chat/pkg/store"
	"github.com/agennext/agent-chat/pkg/tools"
)

// demoAuthn accepts the principal carried on the event (single-node stand-in for
// mTLS/SPIFFE identity termination at the overlay).
type demoAuthn struct{}

func (demoAuthn) Authenticate(_ context.Context, e event.Event) (string, error) {
	return e.Principal(), nil
}

// demoAuthz permits the demo tenant/principal (stand-in for OpenFGA).
type demoAuthz struct{}

func (demoAuthz) Authorize(_ context.Context, _, _, _ string) (bool, error) { return true, nil }

// demoGrants returns the scope the demo principal is granted (stand-in for
// OpenFGA-derived grants). Scope is derived here, never supplied by the caller.
type demoGrants struct{}

func (demoGrants) Granted(_ context.Context, _, _, _ string) (capability.Scope, error) {
	return capability.Scope{Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/*"}}, nil
}

// demoReasoner: retrieve once, then answer from the observation.
type demoReasoner struct{ used bool }

func (r *demoReasoner) Reason(_ context.Context, s loop.State) (loop.Action, error) {
	if !r.used {
		r.used = true
		return loop.Action{
			Kind:       loop.ActionInvoke,
			Capability: "rag.retrieve",
			Input:      []byte("return policy"),
			Scope:      capability.Scope{Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/faq"}},
		}, nil
	}
	answer := "I could not find an answer."
	for _, o := range s.Scratch {
		if !o.Blocked && o.Output != "" {
			answer = "Based on our knowledge base: " + o.Output
		}
	}
	return loop.Action{Kind: loop.ActionAnswer, Answer: answer}, nil
}

// demoInvoker returns a benign knowledge-base passage.
type demoInvoker struct{}

func (demoInvoker) Invoke(_ context.Context, _ string, _ []byte) (loop.Output, error) {
	return loop.Output{
		Data:   []byte("returns are accepted within 30 days of delivery with proof of purchase."),
		Origin: guard.OriginRetrieved,
	}, nil
}

// buildInvoker selects the ACT-step implementation. With AGENNEXT_TOOLS set to a
// JSON map of capability -> endpoint URL (e.g. {"rag.retrieve":"http://tool:9000"}),
// it dispatches to real HTTP tools; their output is tagged untrusted and screened
// by the loop. Otherwise it falls back to the in-process demo invoker.
func buildInvoker() loop.Invoker {
	raw := os.Getenv("AGENNEXT_TOOLS")
	if raw == "" {
		return demoInvoker{}
	}
	var urls map[string]string
	if err := json.Unmarshal([]byte(raw), &urls); err != nil {
		log.Fatalf("AGENNEXT_TOOLS: invalid JSON: %v", err)
	}
	eps := make(map[string]tools.Endpoint, len(urls))
	for name, url := range urls {
		eps[name] = tools.Endpoint{URL: url}
	}
	log.Printf("http tool invoker: %d capability endpoint(s) configured", len(eps))
	return tools.NewHTTPInvoker(eps)
}

// build wires the kernel-admitted contract, the entry gate, and the chat core.
func build() (*edge.Gate, *chat.Core) {
	k := kernel.New()
	if _, failures := k.Reconcile([]capability.Contract{{
		Name:       "rag.retrieve",
		Version:    "0.1.0",
		Provides:   []string{"retrieve(query) -> passages"},
		Scope:      capability.Scope{Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/*"}},
		Sandbox:    capability.SandboxIsolated,
		Idempotent: true,
	}}); len(failures) > 0 {
		for name, err := range failures {
			log.Fatalf("admit %s: %v", name, err)
		}
	}
	reg := k.Registry

	gate := &edge.Gate{
		Authn:    demoAuthn{},
		Authz:    demoAuthz{},
		Grants:   demoGrants{},
		Decider:  guard.NewStaticDecider("rag.retrieve"),
		Screener: guard.NewHeuristicScreener(),
		Prompt:   guard.NewStaticPrompt(),
		Registry: reg,
	}
	engine := &loop.Engine{
		Reasoner: &demoReasoner{},
		Invoker:  buildInvoker(),
		Registry: reg,
		Screener: guard.NewHeuristicScreener(),
		Decider:  guard.NewStaticDecider("rag.retrieve"),
		Ctx:      store.NewMemContextStore(),
		Mem:      store.NewMemMemoryStore(),
		Dedupe:   loop.NewMemDeduper(),
		Budget:   loop.DefaultBudget(),
	}
	return gate, chat.New(engine)
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	demo := flag.Bool("demo", false, "run one turn and exit (print the trace) instead of serving")
	flag.Parse()

	gate, core := build()

	if *demo {
		runDemo(gate, core)
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/", server.New(gate, core).Handler())

	// Peer platforms attach at the edge. Mount the Slack channel when configured;
	// it terminates Slack, then crosses into the core only through the gate.
	if sec := os.Getenv("SLACK_SIGNING_SECRET"); sec != "" {
		br := &slack.Bridge{
			Adapter: slack.Adapter{SigningSecret: sec, Tenant: "acme", Capability: "rag.retrieve"},
			Gate:    gate,
			Core:    core,
			Post:    slack.PostMessage(os.Getenv("SLACK_BOT_TOKEN")),
		}
		mux.HandleFunc("POST /slack/events", br.Events)
		log.Print("slack channel mounted at POST /slack/events")
	}

	// Mattermost is the open-source peer platform. Mount it when configured; the
	// outgoing-webhook token is verified, then the post crosses into the core
	// only through the gate, and the answer returns as the synchronous reply.
	if tok := os.Getenv("MATTERMOST_TOKEN"); tok != "" {
		br := &mattermost.Bridge{
			Adapter: mattermost.Adapter{Token: tok, Tenant: "acme", Capability: "rag.retrieve"},
			Gate:    gate,
			Core:    core,
		}
		mux.HandleFunc("POST /mattermost/hooks", br.Hook)
		log.Print("mattermost channel mounted at POST /mattermost/hooks")
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("agennextd serving on %s (GET /healthz, POST /v1/chat)", *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM (must not bleed connections).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Print("agennextd stopped")
}

// runDemo executes a single turn end-to-end and prints the inspectable trace.
func runDemo(gate *edge.Gate, core *chat.Core) {
	ctx := context.Background()
	in := event.New("evt-1", "channel/web", "chat.message.v1", "session-acme-1",
		"acme", "user-1", "rag.retrieve", []byte("What is your return policy?"))

	admitted, err := gate.Admit(ctx, in)
	if err != nil {
		log.Fatalf("gate denied: %v", err)
	}
	res, err := core.Run(ctx, admitted)
	if err != nil {
		log.Fatalf("loop error: %v", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{
		"session": res.SessionID, "answer": res.Answer, "iterations": res.Iterations,
		"stopped_by": res.StoppedBy, "escalated": res.Escalated, "trace": res.Trace,
	}); err != nil {
		log.Fatalf("encode: %v", err)
	}
}
