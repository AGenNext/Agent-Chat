// Package authz is a relationship-based authorizer (ReBAC) for the Edge Gate.
// It is the in-process, zero-dependency binding behind edge.Authorizer and
// edge.ScopeAuthority; the production binding is OpenFGA, which this models.
//
// It enforces the platform's authority rules literally:
//   - Deny by default. An empty store authorizes nothing.
//   - Start at zero, add only. Authority is created by Grant; there is no
//     revoke and no deny-rule. Negative enforcement is deliberately absent —
//     a deny that overrides an allow is the loophole we refuse to create.
//   - Scope is derived, never caller-supplied. Granted returns the union of
//     the scopes a principal has been granted; the gate intersects that with
//     the capability contract to get the effective (least-privilege) scope.
//
// A grant is a tuple (principal, tenant, capability) → scope. Each field may be
// the wildcard "*" on the GRANT side to grant broadly (e.g. principal "*" =
// "anyone"); wildcards are never honored on the REQUEST side, so a caller
// cannot ask as "*".
package authz

import (
	"context"
	"sync"

	"github.com/agennext/agent-chat/pkg/capability"
)

// tuple identifies a grant: who, for which tenant, on which capability.
type tuple struct {
	principal  string
	tenant     string
	capability string
}

// Store is an add-only set of relationship grants. The zero value is not
// usable; construct with New. It is safe for concurrent use.
type Store struct {
	mu     sync.RWMutex
	grants map[tuple]capability.Scope
}

// New returns an empty authorizer: deny-by-default, authorizing nothing until
// Grant is called.
func New() *Store {
	return &Store{grants: make(map[tuple]capability.Scope)}
}

// Grant adds authority: principal may invoke capability for tenant, with scope.
// Any field may be "*" to grant broadly. Granting the same tuple again unions
// the new scope into the existing one (add-only; grants never shrink). It
// returns the receiver so grants can be chained at wiring time.
func (s *Store) Grant(principal, tenant, capName string, scope capability.Scope) *Store {
	k := tuple{principal: principal, tenant: tenant, capability: capName}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.grants[k]; ok {
		s.grants[k] = unionScope(existing, scope)
	} else {
		s.grants[k] = scope
	}
	return s
}

// matches reports whether a grant tuple covers a concrete request. Wildcards
// are honored only on the grant side; the request fields must be concrete.
func (k tuple) matches(principal, tenant, capName string) bool {
	return (k.principal == "*" || k.principal == principal) &&
		(k.tenant == "*" || k.tenant == tenant) &&
		(k.capability == "*" || k.capability == capName)
}

// Authorize reports whether principal may invoke capName for tenant. It is true
// only when a matching grant exists whose scope is non-empty. Implements
// edge.Authorizer. The error is always nil (an in-process check cannot fail);
// the signature matches the interface so a networked binding can report faults.
func (s *Store) Authorize(_ context.Context, principal, tenant, capName string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, scope := range s.grants {
		if k.matches(principal, tenant, capName) && !scope.Empty() {
			return true, nil
		}
	}
	return false, nil
}

// Granted returns the union of the scopes principal has been granted for capName
// and tenant — the principal's authority, before it is intersected with the
// capability contract. An empty scope (default-deny) when nothing matches.
// Implements edge.ScopeAuthority.
func (s *Store) Granted(_ context.Context, principal, tenant, capName string) (capability.Scope, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out capability.Scope
	for k, scope := range s.grants {
		if k.matches(principal, tenant, capName) {
			out = unionScope(out, scope)
		}
	}
	return out, nil
}

// unionScope merges two scopes: the authority permitted by EITHER. Used to
// accumulate multiple grants for the same principal (grants only ever add).
func unionScope(a, b capability.Scope) capability.Scope {
	return capability.Scope{
		Tenants: dedup(append(append([]string{}, a.Tenants...), b.Tenants...)),
		Data:    dedup(append(append([]string{}, a.Data...), b.Data...)),
		Tools:   dedup(append(append([]string{}, a.Tools...), b.Tools...)),
	}
}

func dedup(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	var out []string
	for _, x := range in {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
