// Package authn is the identity-verification binding for the Edge Gate's first
// step. It is the pure-stdlib implementation behind edge.Authenticator; the
// production binding terminates mTLS/SPIFFE identity at the overlay, which this
// models with a shared-secret signed token.
//
// The rule it enforces: identity is never taken from the caller's claim. The
// caller presents a credential the platform itself signed; the authenticator
// verifies the signature and expiry and returns the principal FROM THE TOKEN,
// ignoring whatever principal the event asserts. A missing, forged, expired, or
// wrong-tenant token is denied — deny-by-default, so an unauthenticated caller
// gets no identity at all.
//
// Token format (compact, URL-safe, no padding):
//
//	base64url(payloadJSON) "." base64url(HMAC-SHA256(secret, base64url(payloadJSON)))
//
// payloadJSON = {"sub":<principal>,"ten":<tenant>,"exp":<unix seconds>}
//
// Pure standard library — crypto/hmac, crypto/sha256, crypto/subtle.
package authn

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agennext/agent-chat/pkg/event"
)

// Verification failures. All map to the gate's unauthenticated outcome.
var (
	// ErrNoCredential is returned when the event carries no credential.
	ErrNoCredential = errors.New("no credential")
	// ErrBadToken is returned when the token is malformed.
	ErrBadToken = errors.New("malformed token")
	// ErrBadSignature is returned when the HMAC does not verify.
	ErrBadSignature = errors.New("bad signature")
	// ErrExpired is returned when the token's expiry has passed.
	ErrExpired = errors.New("token expired")
	// ErrTenantMismatch is returned when the token is not for the event's tenant.
	ErrTenantMismatch = errors.New("token tenant mismatch")
)

var enc = base64.RawURLEncoding

// Claims are the verified contents of a token.
type Claims struct {
	Principal string `json:"sub"`
	Tenant    string `json:"ten"`
	Expiry    int64  `json:"exp"` // unix seconds
}

// Authenticator verifies signed credentials with a shared secret.
type Authenticator struct {
	secret []byte
	now    func() time.Time
}

// New returns an Authenticator keyed by secret. An empty secret is rejected so a
// zero-value key can never silently accept everything.
func New(secret string) (*Authenticator, error) {
	if secret == "" {
		return nil, errors.New("authn: empty secret")
	}
	return &Authenticator{secret: []byte(secret), now: func() time.Time { return time.Now().UTC() }}, nil
}

// Authenticate implements edge.Authenticator. It verifies the event's credential
// and returns the principal FROM THE TOKEN — never the event's claimed
// principal. The token's tenant must match the event's tenant.
func (a *Authenticator) Authenticate(_ context.Context, e event.Event) (string, error) {
	tok := e.Credential()
	if tok == "" {
		return "", ErrNoCredential
	}
	claims, err := a.Verify(tok)
	if err != nil {
		return "", err
	}
	if claims.Tenant != e.Tenant() {
		return "", fmt.Errorf("%w: token %q, event %q", ErrTenantMismatch, claims.Tenant, e.Tenant())
	}
	return claims.Principal, nil
}

// Sign mints a token for principal/tenant valid for ttl from now. It is used by
// trusted credential issuers (and tests); the daemon only ever verifies.
func (a *Authenticator) Sign(principal, tenant string, ttl time.Duration) (string, error) {
	payload, err := json.Marshal(Claims{
		Principal: principal,
		Tenant:    tenant,
		Expiry:    a.now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	p := enc.EncodeToString(payload)
	return p + "." + enc.EncodeToString(a.mac(p)), nil
}

// Verify checks a token's signature and expiry and returns its claims.
func (a *Authenticator) Verify(token string) (Claims, error) {
	p, sig, ok := strings.Cut(token, ".")
	if !ok || p == "" || sig == "" {
		return Claims{}, ErrBadToken
	}
	gotSig, err := enc.DecodeString(sig)
	if err != nil {
		return Claims{}, ErrBadToken
	}
	// Constant-time compare over the HMAC of the encoded payload.
	if subtle.ConstantTimeCompare(gotSig, a.mac(p)) != 1 {
		return Claims{}, ErrBadSignature
	}
	raw, err := enc.DecodeString(p)
	if err != nil {
		return Claims{}, ErrBadToken
	}
	var claims Claims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return Claims{}, ErrBadToken
	}
	if claims.Principal == "" || claims.Tenant == "" {
		return Claims{}, ErrBadToken
	}
	if a.now().Unix() >= claims.Expiry {
		return Claims{}, ErrExpired
	}
	return claims, nil
}

// mac computes HMAC-SHA256(secret, encodedPayload).
func (a *Authenticator) mac(encodedPayload string) []byte {
	h := hmac.New(sha256.New, a.secret)
	h.Write([]byte(encodedPayload))
	return h.Sum(nil)
}
