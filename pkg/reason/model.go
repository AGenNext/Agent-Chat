// ModelReasoner is the networked loop.Reasoner binding: it asks a language
// model, reached over an OpenAI-compatible chat-completions endpoint, to choose
// the next action. The same wire format is spoken by a LiteLLM gateway, vLLM,
// Ollama, and OpenAI itself, so one client serves them all — the uniform access
// layer, model-agnostic.
//
// It is self-fencing. A transport failure, a timeout, a malformed body, or a
// reply it cannot parse never crashes or stalls the loop: it fails safe to an
// escalate action, so a model outage degrades to a human handoff rather than a
// fabricated answer or a hang. The promise is intent, not assurance.
//
// The model is instructed to emit exactly one JSON action object:
//
//	{"action":"answer","answer":"..."}
//	{"action":"invoke","capability":"<name>","input":"<text>"}
//	{"action":"escalate","reason":"..."}
//
// An invoke is always scoped to the turn's admitted scope; the reasoner never
// widens authority. Pure standard library — net/http + encoding/json.
package reason

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agennext/agent-chat/pkg/capability"
	"github.com/agennext/agent-chat/pkg/loop"
)

// Compile-time check: ModelReasoner is a loop.Reasoner.
var _ loop.Reasoner = (*ModelReasoner)(nil)

// ModelReasoner calls an OpenAI-compatible chat-completions endpoint.
type ModelReasoner struct {
	// Endpoint is the full chat-completions URL (e.g.
	// https://gateway.internal/v1/chat/completions).
	Endpoint string
	// Model is the model identifier passed to the endpoint.
	Model string
	// APIKey, when set, is sent as a Bearer token.
	APIKey string
	// Capabilities the model may invoke. Listed in the prompt; an invoke of
	// anything else is still gated by the loop's GUARD step.
	Capabilities []string
	// Timeout bounds one model call. Default 20s.
	Timeout time.Duration
	// MaxBytes caps the response body. Default 1 MiB.
	MaxBytes int64

	client *http.Client
}

// NewModelReasoner builds a reasoner for the given endpoint/model.
func NewModelReasoner(endpoint, model, apiKey string, capabilities []string) *ModelReasoner {
	return &ModelReasoner{
		Endpoint:     endpoint,
		Model:        model,
		APIKey:       apiKey,
		Capabilities: append([]string(nil), capabilities...),
		client:       &http.Client{},
	}
}

// chat wire types (OpenAI-compatible subset).
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// modelAction is the JSON the model is asked to emit.
type modelAction struct {
	Action     string `json:"action"`
	Answer     string `json:"answer"`
	Capability string `json:"capability"`
	Input      string `json:"input"`
	Reason     string `json:"reason"`
}

// Reason implements loop.Reasoner. It never returns a non-nil error: every
// failure path resolves to an escalate action so the loop exits to a human
// cleanly (fail-safe). The Go error is kept in the signature for the interface.
func (m *ModelReasoner) Reason(ctx context.Context, s loop.State) (loop.Action, error) {
	act := m.reason(ctx, s)
	return act, nil
}

func (m *ModelReasoner) reason(ctx context.Context, s loop.State) (action loop.Action) {
	// Defensive: a panic anywhere below degrades to escalation, never a crash.
	defer func() {
		if r := recover(); r != nil {
			action = escalate(fmt.Sprintf("reasoner panicked: %v", r))
		}
	}()

	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	maxBytes := m.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}

	reqBody, err := json.Marshal(chatRequest{
		Model:       m.Model,
		Messages:    m.messages(s),
		Temperature: 0,
	})
	if err != nil {
		return escalate("encode request: " + err.Error())
	}

	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(rctx, http.MethodPost, m.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return escalate("build request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	if m.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.APIKey)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return escalate("transport: " + err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	raw := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	for int64(len(raw)) < maxBytes {
		n, rerr := resp.Body.Read(buf)
		raw = append(raw, buf[:n]...)
		if rerr != nil {
			break
		}
	}
	if resp.StatusCode >= 400 {
		return escalate(fmt.Sprintf("model endpoint status %d", resp.StatusCode))
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil || len(cr.Choices) == 0 {
		return escalate("decode model response")
	}
	return toAction(cr.Choices[0].Message.Content, s.Scope)
}

// messages builds the chat transcript: the guard prompt and the action
// protocol as system messages, the conversation history, and the observations
// gathered so far this turn.
func (m *ModelReasoner) messages(s loop.State) []chatMessage {
	var msgs []chatMessage
	if s.GuardPrompt != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: s.GuardPrompt})
	}
	msgs = append(msgs, chatMessage{Role: "system", Content: m.protocol()})

	for _, h := range s.History {
		role := h.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		msgs = append(msgs, chatMessage{Role: role, Content: h.Content})
	}

	if eb := evidenceBlock(s.Scratch); eb != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: eb})
	}
	return msgs
}

// protocol is the system instruction describing the JSON action contract.
func (m *ModelReasoner) protocol() string {
	var b strings.Builder
	b.WriteString("Respond with ONLY a single JSON object, no prose, no code fence.\n")
	b.WriteString("Choose one action:\n")
	b.WriteString(`{"action":"answer","answer":"<final reply, grounded only in provided evidence>"}` + "\n")
	b.WriteString(`{"action":"invoke","capability":"<name>","input":"<query>"}` + "\n")
	b.WriteString(`{"action":"escalate","reason":"<why you cannot answer>"}` + "\n")
	if len(m.Capabilities) > 0 {
		b.WriteString("Invokable capabilities: " + strings.Join(m.Capabilities, ", ") + ".\n")
	} else {
		b.WriteString("No capabilities are invokable; answer or escalate.\n")
	}
	b.WriteString("Answer only from evidence provided to you. If you lack grounds to answer, escalate — do not invent.")
	return b.String()
}

// evidenceBlock renders the usable observations as a system note for the model.
func evidenceBlock(scratch []loop.Observation) string {
	var lines []string
	for _, o := range scratch {
		if !o.Blocked && o.Output != "" {
			lines = append(lines, "- "+o.Output)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "Evidence gathered this turn:\n" + strings.Join(lines, "\n")
}

// toAction parses the model's JSON object into a loop.Action, scoping an invoke
// to the turn scope. Anything unparseable or unknown fails safe to escalate.
func toAction(content string, scope capability.Scope) loop.Action {
	obj := extractJSON(content)
	if obj == "" {
		return escalate("model did not return a JSON action")
	}
	var ma modelAction
	if err := json.Unmarshal([]byte(obj), &ma); err != nil {
		return escalate("model action not valid JSON")
	}
	switch strings.ToLower(strings.TrimSpace(ma.Action)) {
	case "answer":
		if ma.Answer == "" {
			return escalate("model returned an empty answer")
		}
		return loop.Action{Kind: loop.ActionAnswer, Answer: ma.Answer}
	case "invoke":
		if ma.Capability == "" {
			return escalate("model invoke named no capability")
		}
		return loop.Action{
			Kind:       loop.ActionInvoke,
			Capability: ma.Capability,
			Input:      []byte(ma.Input),
			Scope:      scope, // never widened by the reasoner
		}
	case "escalate":
		return escalate("model chose to escalate: " + ma.Reason)
	default:
		return escalate("model returned an unknown action")
	}
}

// extractJSON returns the first balanced top-level {...} object found in s, so a
// model that wraps the object in stray prose or a code fence is still parsed.
func extractJSON(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case esc:
			esc = false
		case c == '\\' && inStr:
			esc = true
		case c == '"':
			inStr = !inStr
		case inStr:
			// within a string literal: ignore braces
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// escalate builds a fail-safe escalate action. The detail is for the trace only
// (it never reaches the caller-facing answer).
func escalate(detail string) loop.Action {
	return loop.Action{Kind: loop.ActionEscalate, Detail: detail}
}
