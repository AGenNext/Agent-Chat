package authn

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agennext/agent-chat/pkg/edge"
	"github.com/agennext/agent-chat/pkg/event"
)

// Authenticator satisfies the gate's authn seam.
var _ edge.Authenticator = (*Authenticator)(nil)

func mustNew(t *testing.T, secret string) *Authenticator {
	t.Helper()
	a, err := New(secret)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	return a
}

func evtWith(tenant, claimedPrincipal, credential string) event.Event {
	e := event.New("e1", "web", "chat.message.v1", "sess-1", tenant, claimedPrincipal, "rag.retrieve", []byte("hi"))
	if credential != "" {
		e.Extensions[event.ExtCredential] = credential
	}
	return e
}

func TestEmptySecretRejected(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("an empty secret must be rejected, never silently accept")
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	a := mustNew(t, "s3cr3t")
	tok, err := a.Sign("user-1", "acme", time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	principal, err := a.Authenticate(context.Background(), evtWith("acme", "user-1", tok))
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if principal != "user-1" {
		t.Errorf("principal: got %q, want user-1", principal)
	}
}

func TestIdentityComesFromTokenNotEvent(t *testing.T) {
	a := mustNew(t, "s3cr3t")
	tok, _ := a.Sign("real-user", "acme", time.Hour)
	// The event claims a different principal; the verified identity must win.
	principal, err := a.Authenticate(context.Background(), evtWith("acme", "attacker", tok))
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if principal != "real-user" {
		t.Errorf("identity must come from the token, got %q", principal)
	}
}

func TestNoCredentialDenied(t *testing.T) {
	a := mustNew(t, "s3cr3t")
	_, err := a.Authenticate(context.Background(), evtWith("acme", "user-1", ""))
	if !errors.Is(err, ErrNoCredential) {
		t.Fatalf("missing credential must deny, got %v", err)
	}
}

func TestForgedSignatureDenied(t *testing.T) {
	a := mustNew(t, "s3cr3t")
	tok, _ := a.Sign("user-1", "acme", time.Hour)
	// A token signed with a different secret must not verify under a.
	other := mustNew(t, "different")
	forged, _ := other.Sign("user-1", "acme", time.Hour)

	if _, err := a.Verify(forged); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("foreign-signed token must fail signature check, got %v", err)
	}
	// Tampering with the payload must also fail.
	tampered := "YWFh" + tok[4:]
	if _, err := a.Verify(tampered); err == nil {
		t.Fatal("tampered token must not verify")
	}
}

func TestExpiredDenied(t *testing.T) {
	a := mustNew(t, "s3cr3t")
	// Mint a token that expired an hour ago by moving the clock back for Sign.
	a.now = func() time.Time { return time.Now().UTC().Add(-2 * time.Hour) }
	tok, _ := a.Sign("user-1", "acme", time.Hour) // exp = now-2h+1h = now-1h
	a.now = func() time.Time { return time.Now().UTC() }

	if _, err := a.Verify(tok); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired token must deny, got %v", err)
	}
}

func TestTenantMismatchDenied(t *testing.T) {
	a := mustNew(t, "s3cr3t")
	tok, _ := a.Sign("user-1", "acme", time.Hour)
	// Present an acme token on a beta event.
	_, err := a.Authenticate(context.Background(), evtWith("beta", "user-1", tok))
	if !errors.Is(err, ErrTenantMismatch) {
		t.Fatalf("cross-tenant token must deny, got %v", err)
	}
}

func TestMalformedDenied(t *testing.T) {
	a := mustNew(t, "s3cr3t")
	for _, bad := range []string{"", "nodot", "a.", ".b", "!!.!!"} {
		if _, err := a.Verify(bad); err == nil {
			t.Errorf("malformed token %q must not verify", bad)
		}
	}
}
