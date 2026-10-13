package tools

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agennext/agent-chat/pkg/guard"
)

func TestHTTPInvokerCallsEndpoint(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write([]byte("echo:" + string(b)))
	}))
	defer srv.Close()

	inv := NewHTTPInvoker(map[string]Endpoint{"rag.retrieve": {URL: srv.URL}})
	out, err := inv.Invoke(context.Background(), "rag.retrieve", []byte("hello"))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if string(out.Data) != "echo:hello" {
		t.Fatalf("data = %q", out.Data)
	}
	// Tool output must be tagged untrusted so the loop screens it.
	if out.Origin != guard.OriginTool {
		t.Fatalf("origin = %q, want %q", out.Origin, guard.OriginTool)
	}
}

func TestHTTPInvokerUnknownCapabilityErrors(t *testing.T) {
	t.Parallel()
	inv := NewHTTPInvoker(nil)
	if _, err := inv.Invoke(context.Background(), "nope", nil); err == nil {
		t.Fatal("expected error for unconfigured capability (loop must escalate, not fabricate)")
	}
}

func TestHTTPInvokerHTTPErrorSurfaces(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()
	inv := NewHTTPInvoker(map[string]Endpoint{"c": {URL: srv.URL}})
	if _, err := inv.Invoke(context.Background(), "c", nil); err == nil {
		t.Fatal("expected error on 5xx from the tool endpoint")
	}
}

func TestHTTPInvokerCapsResponseSize(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 10_000))
	}))
	defer srv.Close()
	inv := NewHTTPInvoker(map[string]Endpoint{"c": {URL: srv.URL, MaxBytes: 100}})
	out, err := inv.Invoke(context.Background(), "c", nil)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if len(out.Data) != 100 {
		t.Fatalf("response not capped: got %d bytes", len(out.Data))
	}
}
