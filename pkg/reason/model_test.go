package reason

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agennext/agent-chat/pkg/capability"
	"github.com/agennext/agent-chat/pkg/loop"
	"github.com/agennext/agent-chat/pkg/store"
)

// modelServer returns an httptest server that replies with the given assistant
// content as an OpenAI-compatible chat completion.
func modelServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Requests must be well-formed chat-completions calls.
		var req chatRequest
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &req); err != nil || req.Model == "" || len(req.Messages) == 0 {
			t.Errorf("malformed chat request: %s", b)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := chatResponse{}
		resp.Choices = append(resp.Choices, struct {
			Message chatMessage `json:"message"`
		}{Message: chatMessage{Role: "assistant", Content: content}})
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func modelState(scratch ...loop.Observation) loop.State {
	return loop.State{
		Tenant:      "acme",
		Principal:   "user-1",
		GuardPrompt: "You are a bounded assistant.",
		History:     []store.Message{{Role: "user", Content: "what is your return policy?"}},
		Scope:       capability.Scope{Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/*"}},
		Scratch:     scratch,
	}
}

func TestModelAnswer(t *testing.T) {
	srv := modelServer(t, `{"action":"answer","answer":"Returns within 30 days."}`)
	defer srv.Close()
	r := NewModelReasoner(srv.URL, "gpt-x", "", []string{"rag.retrieve"})

	act, _ := r.Reason(context.Background(), modelState())
	if act.Kind != loop.ActionAnswer || act.Answer != "Returns within 30 days." {
		t.Fatalf("expected grounded answer, got %+v", act)
	}
}

func TestModelInvokeIsScoped(t *testing.T) {
	srv := modelServer(t, `{"action":"invoke","capability":"rag.retrieve","input":"return policy"}`)
	defer srv.Close()
	r := NewModelReasoner(srv.URL, "gpt-x", "", []string{"rag.retrieve"})

	st := modelState()
	act, _ := r.Reason(context.Background(), st)
	if act.Kind != loop.ActionInvoke || act.Capability != "rag.retrieve" {
		t.Fatalf("expected invoke, got %+v", act)
	}
	if string(act.Input) != "return policy" {
		t.Errorf("input: got %q", act.Input)
	}
	// The reasoner must not widen authority: requested scope ⊆ turn scope.
	if !st.Scope.Contains(act.Scope) {
		t.Errorf("requested scope %+v exceeds turn scope", act.Scope)
	}
}

func TestModelEscalate(t *testing.T) {
	srv := modelServer(t, `{"action":"escalate","reason":"insufficient evidence"}`)
	defer srv.Close()
	r := NewModelReasoner(srv.URL, "gpt-x", "", nil)

	act, _ := r.Reason(context.Background(), modelState())
	if act.Kind != loop.ActionEscalate {
		t.Fatalf("expected escalate, got %+v", act)
	}
	if act.Answer != "" {
		t.Errorf("escalate must carry no answer, got %q", act.Answer)
	}
}

func TestModelExtractsJSONFromProse(t *testing.T) {
	// A chatty model that wraps the object in a code fence and commentary.
	content := "Sure! Here is my decision:\n```json\n{\"action\":\"answer\",\"answer\":\"OK.\"}\n```\nHope that helps."
	srv := modelServer(t, content)
	defer srv.Close()
	r := NewModelReasoner(srv.URL, "gpt-x", "", nil)

	act, _ := r.Reason(context.Background(), modelState())
	if act.Kind != loop.ActionAnswer || act.Answer != "OK." {
		t.Fatalf("must extract the JSON object from prose, got %+v", act)
	}
}

func TestModelFailsSafeToEscalate(t *testing.T) {
	// Every failure mode must degrade to escalate, never a hard error or a hang.
	cases := map[string]http.HandlerFunc{
		"garbage body": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("not json at all"))
		},
		"5xx": func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "down", http.StatusInternalServerError)
		},
		"empty choices": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"choices":[]}`))
		},
		"no json action": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := chatResponse{}
			resp.Choices = append(resp.Choices, struct {
				Message chatMessage `json:"message"`
			}{Message: chatMessage{Content: "I think the answer is 30 days"}})
			_ = json.NewEncoder(w).Encode(resp)
		},
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(h)
			defer srv.Close()
			r := NewModelReasoner(srv.URL, "gpt-x", "", nil)
			act, err := r.Reason(context.Background(), modelState())
			if err != nil {
				t.Fatalf("Reason must never return a hard error, got %v", err)
			}
			if act.Kind != loop.ActionEscalate {
				t.Fatalf("%s must fail safe to escalate, got %+v", name, act)
			}
		})
	}
}

func TestModelUnreachableEndpointEscalates(t *testing.T) {
	// A closed server => transport error => escalate, not a crash or hang.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()
	r := NewModelReasoner(url, "gpt-x", "", nil)

	act, err := r.Reason(context.Background(), modelState())
	if err != nil {
		t.Fatalf("Reason must never return a hard error, got %v", err)
	}
	if act.Kind != loop.ActionEscalate {
		t.Fatalf("unreachable endpoint must escalate, got %+v", act)
	}
}
