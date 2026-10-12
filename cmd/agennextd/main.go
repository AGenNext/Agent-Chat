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
	"github.com/agennext/agent-chat/pkg/authz"
	"github.com/agennext/agent-chat/pkg/capability"
	"github.com/agennext/agent-chat/pkg/chat"
	"github.com/agennext/agent-chat/pkg/edge"
	"github.com/agennext/agent-chat/pkg/event"
	"github.com/agennext/agent-chat/pkg/guard"
	"github.com/agennext/agent-chat/pkg/kernel"
	"github.com/agennext/agent-chat/pkg/loop"
	"github.com/agennext/agent-chat/pkg/reason"
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

// grantSpec is one relationship grant loaded from AGENNEXT_GRANTS.
type grantSpec struct {
	Principal  string   `json:"principal"`
	Tenant     string   `json:"tenant"`
	Capability string   `json:"capability"`
	Tenants    []string `json:"tenants"`
	Data       []string `json:"data"`
	Tools      []string `json:"tools"`
}

// buildAuthz builds the relationship-based authorizer that backs both the
// gate's authorization check and its scope derivation. It is deny-by-default
// and add-only. With AGENNEXT_GRANTS set to a JSON array of grants, those are
// loaded; otherwise a single demo grant is seeded so -demo and the default
// single-node run work. There is no permit-all path.
func buildAuthz() *authz.Store {
	s := authz.New()
	raw := os.Getenv("AGENNEXT_GRANTS")
	if raw == "" {
		// Default: the demo principal may retrieve from the acme knowledge base.
		s.Grant("user-1", "acme", "rag.retrieve", capability.Scope{
			Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/*"},
		})
		return s
	}
	var specs []grantSpec
	if err := json.Unmarshal([]byte(raw), &specs); err != nil {
		log.Fatalf("AGENNEXT_GRANTS: invalid JSON: %v", err)
	}
	for _, g := range specs {
		s.Grant(g.Principal, g.Tenant, g.Capability, capability.Scope{
			Tenants: g.Tenants, Data: g.Data, Tools: g.Tools,
		})
	}
	log.Printf("authz: %d relationship grant(s) loaded (deny-by-default)", len(specs))
	return s
}

// demoInvoker returns a benign knowledge-base passage.
type demoInvoker struct{}

func (demoInvoker) Invoke(_ context.Context, _ string, _ []byte) (loop.Output, error) {
	return loop.Output{
		Data:   []byte("returns are accepted within 30 days of delivery with proof of purchase."),
		Origin: guard.OriginRetrieved,
	}, nil
}

// mcpSpec is one capability→MCP-tool binding loaded from AGENNEXT_MCP.
type mcpSpec struct {
	URL  string `json:"url"`
	Tool string `json:"tool"`
}

// buildInvoker selects the ACT-step implementation. Tool output is always
// tagged untrusted and screened by the loop, whichever binding is used.
// Precedence:
//   - AGENNEXT_MCP (JSON map capability -> {"url","tool"}): call MCP tools over
//     JSON-RPC 2.0 (the interoperable edge tool surface);
//   - AGENNEXT_TOOLS (JSON map capability -> endpoint URL): call plain HTTP tools;
//   - otherwise: the in-process demo invoker.
func buildInvoker() loop.Invoker {
	if raw := os.Getenv("AGENNEXT_MCP"); raw != "" {
		var specs map[string]mcpSpec
		if err := json.Unmarshal([]byte(raw), &specs); err != nil {
			log.Fatalf("AGENNEXT_MCP: invalid JSON: %v", err)
		}
		servers := make(map[string]tools.MCPServer, len(specs))
		for name, s := range specs {
			servers[name] = tools.MCPServer{URL: s.URL, Tool: s.Tool}
		}
		log.Printf("mcp tool invoker: %d capability tool(s) configured", len(servers))
		return tools.NewMCPInvoker(servers)
	}
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

// buildStores selects the state surfaces. With AGENNEXT_STATE_DIR set, context
// and memory are persisted as JSON files under that directory and survive a
// restart (single-node durability, zero dependencies). Otherwise state is
// in-memory and lost on exit. The production binding is Valkey + PostgreSQL
// behind the same interfaces.
func buildStores() (store.ContextStore, store.MemoryStore) {
	dir := os.Getenv("AGENNEXT_STATE_DIR")
	if dir == "" {
		return store.NewMemContextStore(), store.NewMemMemoryStore()
	}
	log.Printf("durable state: file-backed stores under %s", dir)
	return store.NewFileContextStore(dir), store.NewFileMemoryStore(dir)
}

// policySpec is one allow-rule loaded from AGENNEXT_POLICY.
type policySpec struct {
	Tenant     string `json:"tenant"`
	Principal  string `json:"principal"`
	Capability string `json:"capability"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
}

// buildDecider builds the in-process policy decision point (the GUARD seam,
// OPA-shaped) shared by the gate and the loop. It is deny-by-default and
// add-only: a request is permitted only when a rule matches. With
// AGENNEXT_POLICY set to a JSON array of rules, those are loaded; otherwise a
// single rule permitting rag.retrieve invocation is seeded so the default run
// works. No request is permitted by absence of a rule.
func buildDecider() *guard.RuleDecider {
	raw := os.Getenv("AGENNEXT_POLICY")
	if raw == "" {
		return guard.NewRuleDecider(guard.Rule{
			Capability: "rag.retrieve", Action: "invoke", Reason: "default: rag.retrieve invoke",
		})
	}
	var specs []policySpec
	if err := json.Unmarshal([]byte(raw), &specs); err != nil {
		log.Fatalf("AGENNEXT_POLICY: invalid JSON: %v", err)
	}
	d := guard.NewRuleDecider()
	for _, p := range specs {
		d.Allow(guard.Rule{
			Tenant: p.Tenant, Principal: p.Principal, Capability: p.Capability,
			Action: p.Action, Reason: p.Reason,
		})
	}
	log.Printf("policy: %d allow-rule(s) loaded (deny-by-default)", len(specs))
	return d
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

	az := buildAuthz()
	decider := buildDecider()
	gate := &edge.Gate{
		Authn:    demoAuthn{},
		Authz:    az,
		Grants:   az,
		Decider:  decider,
		Screener: guard.NewHeuristicScreener(),
		Prompt:   guard.NewStaticPrompt(),
		Registry: reg,
	}
	ctxStore, memStore := buildStores()
	engine := &loop.Engine{
		Reasoner: reason.GroundedReasoner{Retrieve: "rag.retrieve", Preface: "Based on our knowledge base: "},
		Invoker:  buildInvoker(),
		Registry: reg,
		Screener: guard.NewHeuristicScreener(),
		Decider:  decider,
		Ctx:      ctxStore,
		Mem:      memStore,
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
