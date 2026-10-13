package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agennext/agent-chat/pkg/authn"
	"github.com/agennext/agent-chat/pkg/capability"
	"github.com/agennext/agent-chat/pkg/chat"
	"github.com/agennext/agent-chat/pkg/edge"
	"github.com/agennext/agent-chat/pkg/guard"
	"github.com/agennext/agent-chat/pkg/loop"
	"github.com/agennext/agent-chat/pkg/store"
)

const testSecret = "server-auth-test-secret"

// echoPrincipalReasoner answers with the admitted principal, so a test can
// assert which identity the turn ran as.
type echoPrincipalReasoner struct{}

func (echoPrincipalReasoner) Reason(_ context.Context, s loop.State) (loop.Action, error) {
	return loop.Action{Kind: loop.ActionAnswer, Answer: "ran as " + s.Principal}, nil
}

// newAuthServer builds a server whose gate authenticates with signed tokens.
func newAuthServer(t *testing.T) http.Handler {
	t.Helper()
	a, err := authn.New(testSecret)
	if err != nil {
		t.Fatalf("authn: %v", err)
	}
	reg := capability.NewRegistry()
	if err := reg.Register(capability.Contract{
		Name: "rag.retrieve", Version: "0.1.0", Provides: []string{"retrieve"},
		Scope:   capability.Scope{Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/*"}},
		Sandbox: capability.SandboxIsolated,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	gate := &edge.Gate{
		Authn: a, Authz: okAuthz{}, Grants: okGrants{},
		Decider:  guard.NewStaticDecider("rag.retrieve"),
		Screener: guard.NewHeuristicScreener(),
		Prompt:   guard.NewStaticPrompt(),
		Registry: reg,
	}
	eng := &loop.Engine{
		Reasoner: echoPrincipalReasoner{},
		Registry: reg,
		Screener: guard.NewHeuristicScreener(),
		Decider:  guard.NewStaticDecider("rag.retrieve"),
		Ctx:      store.NewMemContextStore(),
		Mem:      store.NewMemMemoryStore(),
		Dedupe:   loop.NewMemDeduper(),
		Budget:   loop.DefaultBudget(),
	}
	return New(gate, chat.New(eng)).Handler()
}

func postAuth(t *testing.T, h http.Handler, body, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const authBody = `{"id":"e1","session":"s1","tenant":"acme","principal":"claimed","capability":"rag.retrieve","message":"hi"}`

func TestHTTPTokenAuthSucceeds(t *testing.T) {
	t.Parallel()
	h := newAuthServer(t)
	a, _ := authn.New(testSecret)
	tok, _ := a.Sign("real-user", "acme", time.Minute)

	rec := postAuth(t, h, authBody, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Identity must come from the verified token, not the body's "claimed".
	if resp.Answer != "ran as real-user" {
		t.Fatalf("turn ran as the wrong principal: %q", resp.Answer)
	}
}

func TestHTTPNoTokenIsUnauthorized(t *testing.T) {
	t.Parallel()
	h := newAuthServer(t)
	rec := postAuth(t, h, authBody, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing credential must be 401, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHTTPForgedTokenIsUnauthorized(t *testing.T) {
	t.Parallel()
	h := newAuthServer(t)
	// A token signed with the wrong secret must not authenticate.
	other, _ := authn.New("wrong-secret")
	forged, _ := other.Sign("real-user", "acme", time.Minute)

	rec := postAuth(t, h, authBody, forged)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("forged token must be 401, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestBearerTokenParsing(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Bearer abc":  "abc",
		"bearer abc":  "abc", // scheme is case-insensitive
		"Bearer  abc": "abc", // surrounding space trimmed
		"Basic abc":   "",
		"":            "",
		"Bearer":      "",
	}
	for header, want := range cases {
		if got := bearerToken(header); got != want {
			t.Errorf("bearerToken(%q) = %q, want %q", header, got, want)
		}
	}
}
