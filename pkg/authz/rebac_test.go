package authz

import (
	"context"
	"testing"

	"github.com/agennext/agent-chat/pkg/capability"
	"github.com/agennext/agent-chat/pkg/edge"
)

// Compile-time proof that the Store satisfies the gate's authority seams.
var (
	_ edge.Authorizer     = (*Store)(nil)
	_ edge.ScopeAuthority = (*Store)(nil)
)

func TestDenyByDefault(t *testing.T) {
	s := New()
	ctx := context.Background()

	ok, err := s.Authorize(ctx, "user-1", "acme", "rag.retrieve")
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if ok {
		t.Error("empty store must deny: start at zero")
	}
	granted, err := s.Granted(ctx, "user-1", "acme", "rag.retrieve")
	if err != nil {
		t.Fatalf("granted: %v", err)
	}
	if !granted.Empty() {
		t.Errorf("empty store must grant no scope, got %+v", granted)
	}
}

func TestGrantAuthorizes(t *testing.T) {
	ctx := context.Background()
	s := New().Grant("user-1", "acme", "rag.retrieve", capability.Scope{
		Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/*"},
	})

	ok, err := s.Authorize(ctx, "user-1", "acme", "rag.retrieve")
	if err != nil || !ok {
		t.Fatalf("granted principal must be authorized: ok=%v err=%v", ok, err)
	}

	// A different principal, tenant, or capability stays denied.
	for _, c := range []struct{ p, t, cap string }{
		{"user-2", "acme", "rag.retrieve"},
		{"user-1", "other", "rag.retrieve"},
		{"user-1", "acme", "rag.write"},
	} {
		if ok, _ := s.Authorize(ctx, c.p, c.t, c.cap); ok {
			t.Errorf("unexpected authorize for (%s,%s,%s)", c.p, c.t, c.cap)
		}
	}
}

func TestGrantedReturnsScope(t *testing.T) {
	ctx := context.Background()
	s := New().Grant("user-1", "acme", "rag.retrieve", capability.Scope{
		Tenants: []string{"acme"}, Data: []string{"tenant://acme/kb/faq"},
	})
	got, err := s.Granted(ctx, "user-1", "acme", "rag.retrieve")
	if err != nil {
		t.Fatalf("granted: %v", err)
	}
	if !got.Contains(capability.Scope{Data: []string{"tenant://acme/kb/faq"}}) {
		t.Errorf("granted scope missing the data glob: %+v", got)
	}
}

func TestWildcardGrantSide(t *testing.T) {
	ctx := context.Background()
	// "anyone in acme may invoke rag.retrieve".
	s := New().Grant("*", "acme", "rag.retrieve", capability.Scope{Tenants: []string{"acme"}})

	if ok, _ := s.Authorize(ctx, "anybody", "acme", "rag.retrieve"); !ok {
		t.Error("grant-side principal wildcard must authorize any principal")
	}
	// Wildcards are not honored on the request side: asking as "*" must not
	// slip past a concrete grant.
	concrete := New().Grant("user-1", "acme", "rag.retrieve", capability.Scope{Tenants: []string{"acme"}})
	if ok, _ := concrete.Authorize(ctx, "*", "acme", "rag.retrieve"); ok {
		t.Error("request-side wildcard must not match a concrete grant")
	}
}

func TestAddOnlyUnionsScope(t *testing.T) {
	ctx := context.Background()
	s := New().
		Grant("user-1", "acme", "rag.retrieve", capability.Scope{Data: []string{"tenant://acme/kb/faq"}}).
		Grant("user-1", "acme", "rag.retrieve", capability.Scope{Data: []string{"tenant://acme/kb/policy"}})

	got, err := s.Granted(ctx, "user-1", "acme", "rag.retrieve")
	if err != nil {
		t.Fatalf("granted: %v", err)
	}
	// Re-granting adds authority; it never shrinks. Both globs are present.
	for _, want := range []string{"tenant://acme/kb/faq", "tenant://acme/kb/policy"} {
		if !got.Contains(capability.Scope{Data: []string{want}}) {
			t.Errorf("union missing %q; got %+v", want, got)
		}
	}
}
