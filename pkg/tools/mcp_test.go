package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agennext/agent-chat/pkg/guard"
	"github.com/agennext/agent-chat/pkg/loop"
)

// MCPInvoker is a loop.Invoker.
var _ loop.Invoker = (*MCPInvoker)(nil)

// decodeCall reads the JSON-RPC tools/call envelope from a request body.
func decodeCall(t *testing.T, r *http.Request) rpcRequest {
	t.Helper()
	b, _ := io.ReadAll(r.Body)
	var req rpcRequest
	if err := json.Unmarshal(b, &req); err != nil {
		t.Fatalf("server: decode request: %v", err)
	}
	if req.JSONRPC != "2.0" || req.Method != "tools/call" {
		t.Fatalf("server: unexpected envelope: %+v", req)
	}
	return req
}

func writeResult(w http.ResponseWriter, id int, text string, isErr bool) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rpcResponse{
		JSONRPC: "2.0", ID: id,
		Result: &toolResult{Content: []contentBlock{{Type: "text", Text: text}}, IsError: isErr},
	})
}

func TestMCPInvokeSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeCall(t, r)
		if req.Params.Name != "search" {
			t.Errorf("tool name: got %q, want search", req.Params.Name)
		}
		// Plain-text input must have been wrapped as {"input": "..."}.
		var args map[string]string
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			t.Fatalf("arguments not an object: %v", err)
		}
		if args["input"] != "return policy?" {
			t.Errorf("wrapped argument: got %q", args["input"])
		}
		writeResult(w, req.ID, "returns within 30 days", false)
	}))
	defer srv.Close()

	inv := NewMCPInvoker(map[string]MCPServer{"kb.search": {URL: srv.URL, Tool: "search"}})
	out, err := inv.Invoke(context.Background(), "kb.search", []byte("return policy?"))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if string(out.Data) != "returns within 30 days" {
		t.Errorf("data: got %q", out.Data)
	}
	if out.Origin != guard.OriginTool {
		t.Errorf("MCP output must be tagged OriginTool (untrusted), got %v", out.Origin)
	}
}

func TestMCPPassesThroughJSONArguments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeCall(t, r)
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			t.Fatalf("arguments: %v", err)
		}
		if args["q"] != "policy" {
			t.Errorf("JSON arguments must pass through unchanged, got %+v", args)
		}
		writeResult(w, req.ID, "ok", false)
	}))
	defer srv.Close()

	inv := NewMCPInvoker(map[string]MCPServer{"kb.search": {URL: srv.URL, Tool: "search"}})
	if _, err := inv.Invoke(context.Background(), "kb.search", []byte(`{"q":"policy"}`)); err != nil {
		t.Fatalf("invoke: %v", err)
	}
}

func TestMCPUnknownCapability(t *testing.T) {
	inv := NewMCPInvoker(map[string]MCPServer{})
	if _, err := inv.Invoke(context.Background(), "missing", []byte("x")); err == nil {
		t.Fatal("unknown capability must error, so the loop escalates")
	}
}

func TestMCPJSONRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeCall(t, r)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rpcResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &rpcError{Code: -32602, Message: "invalid params"},
		})
	}))
	defer srv.Close()

	inv := NewMCPInvoker(map[string]MCPServer{"kb.search": {URL: srv.URL, Tool: "search"}})
	if _, err := inv.Invoke(context.Background(), "kb.search", []byte("x")); err == nil {
		t.Fatal("a JSON-RPC error must surface as a Go error")
	}
}

func TestMCPToolIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeCall(t, r)
		writeResult(w, req.ID, "upstream down", true) // isError = true
	}))
	defer srv.Close()

	inv := NewMCPInvoker(map[string]MCPServer{"kb.search": {URL: srv.URL, Tool: "search"}})
	if _, err := inv.Invoke(context.Background(), "kb.search", []byte("x")); err == nil {
		t.Fatal("an MCP tool-level error (isError) must surface as a Go error")
	}
}

func TestMCPHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()

	inv := NewMCPInvoker(map[string]MCPServer{"kb.search": {URL: srv.URL, Tool: "search"}})
	if _, err := inv.Invoke(context.Background(), "kb.search", []byte("x")); err == nil {
		t.Fatal("an HTTP 5xx must surface as a Go error")
	}
}
