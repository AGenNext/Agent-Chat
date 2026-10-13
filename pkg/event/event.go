// Package event defines the platform's event envelope and bus.
//
// Events conform to the CloudEvents 1.0 core attribute model so that channel
// adapters, the Edge Gate, and the agent loop speak a single, standard wire
// shape. The type is implemented in pure stdlib (no third-party SDK) to keep
// the supply chain trivially auditable; see docs/SUPPLY_CHAIN.md.
package event

import (
	"errors"
	"fmt"
	"time"
)

// SpecVersion is the CloudEvents spec version this envelope conforms to.
const SpecVersion = "1.0"

// Platform extension attribute keys carried in Event.Extensions.
const (
	// ExtTenant scopes everything downstream to a single tenant.
	ExtTenant = "tenant"
	// ExtPrincipal identifies the caller for authorization checks.
	ExtPrincipal = "principal"
	// ExtCapability names the capability the event is addressed to.
	ExtCapability = "capability"
	// ExtCredential carries the caller's signed credential (e.g. a bearer
	// token) for the authenticator to verify. It is never trusted as identity
	// on its own — the authenticator derives the principal from it.
	ExtCredential = "credential"
)

// Event is a CloudEvents 1.0 envelope plus platform extension attributes.
type Event struct {
	// SpecVersion is the CloudEvents spec version ("1.0").
	SpecVersion string
	// ID uniquely identifies the event within its Source. Used for
	// at-least-once idempotency in the agent loop.
	ID string
	// Source identifies the context in which the event occurred (a channel).
	Source string
	// Type describes the kind of event (e.g. "chat.message.v1").
	Type string
	// Subject is the addressed subject; the platform uses it as the session id.
	Subject string
	// Time is the event timestamp.
	Time time.Time
	// DataContentType is the RFC 2046 media type of Data.
	DataContentType string
	// Data is the (untrusted) event payload.
	Data []byte
	// Extensions carries platform attributes (tenant, principal, capability).
	Extensions map[string]string
}

// New builds a well-formed event with the platform extension attributes set.
func New(id, source, typ, subject, tenant, principal, capability string, data []byte) Event {
	return Event{
		SpecVersion:     SpecVersion,
		ID:              id,
		Source:          source,
		Type:            typ,
		Subject:         subject,
		Time:            time.Now().UTC(),
		DataContentType: "application/json",
		Data:            data,
		Extensions: map[string]string{
			ExtTenant:     tenant,
			ExtPrincipal:  principal,
			ExtCapability: capability,
		},
	}
}

// ErrInvalidEvent is returned by Validate for a malformed envelope.
var ErrInvalidEvent = errors.New("invalid event")

// Validate checks the required CloudEvents and platform attributes.
func (e Event) Validate() error {
	switch {
	case e.SpecVersion != SpecVersion:
		return fmt.Errorf("%w: specversion must be %q", ErrInvalidEvent, SpecVersion)
	case e.ID == "":
		return fmt.Errorf("%w: id is required", ErrInvalidEvent)
	case e.Source == "":
		return fmt.Errorf("%w: source is required", ErrInvalidEvent)
	case e.Type == "":
		return fmt.Errorf("%w: type is required", ErrInvalidEvent)
	case e.Tenant() == "":
		return fmt.Errorf("%w: tenant extension is required", ErrInvalidEvent)
	}
	return nil
}

// Tenant returns the tenant extension attribute.
func (e Event) Tenant() string { return e.Extensions[ExtTenant] }

// Principal returns the principal extension attribute.
func (e Event) Principal() string { return e.Extensions[ExtPrincipal] }

// Capability returns the capability extension attribute.
func (e Event) Capability() string { return e.Extensions[ExtCapability] }

// Credential returns the caller's signed credential extension attribute, if any.
func (e Event) Credential() string { return e.Extensions[ExtCredential] }
