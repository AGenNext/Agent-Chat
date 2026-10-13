package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestFileContextStoreDurability proves the core promise: state written by one
// process instance is read back by a fresh instance over the same directory —
// i.e. it survives a restart.
func TestFileContextStoreDurability(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	const sid = "session-acme-1"

	writer := NewFileContextStore(dir)
	want := Context{Messages: []Message{
		{Role: "user", Content: "what is your return policy?"},
		{Role: "assistant", Content: "returns within 30 days"},
	}}
	if err := writer.Save(ctx, sid, want); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Fresh instance == simulated restart; no shared in-memory map.
	reader := NewFileContextStore(dir)
	got, err := reader.Load(ctx, sid)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Messages) != len(want.Messages) {
		t.Fatalf("messages: got %d, want %d", len(got.Messages), len(want.Messages))
	}
	for i, m := range want.Messages {
		if got.Messages[i].Role != m.Role || got.Messages[i].Content != m.Content {
			t.Errorf("message %d: got %+v, want %+v", i, got.Messages[i], m)
		}
	}
	if got.Updated.IsZero() {
		t.Error("Updated not set on save")
	}

	if err := reader.Clear(ctx, sid); err != nil {
		t.Fatalf("clear: %v", err)
	}
	after, err := NewFileContextStore(dir).Load(ctx, sid)
	if err != nil {
		t.Fatalf("load after clear: %v", err)
	}
	if len(after.Messages) != 0 {
		t.Errorf("expected empty after clear, got %d messages", len(after.Messages))
	}
}

// TestFileContextStoreAbsent: loading an unknown session yields the zero value,
// not an error (matches MemContextStore).
func TestFileContextStoreAbsent(t *testing.T) {
	got, err := NewFileContextStore(t.TempDir()).Load(context.Background(), "nope")
	if err != nil {
		t.Fatalf("load absent: %v", err)
	}
	if len(got.Messages) != 0 {
		t.Errorf("expected zero value, got %+v", got)
	}
}

// TestFileMemoryStoreDurability proves append-then-reload across a restart.
func TestFileMemoryStoreDurability(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	const sid = "session-acme-1"

	w := NewFileMemoryStore(dir)
	if err := w.Append(ctx, sid, MemoryItem{Key: "retrieval", Value: "first"}); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := w.Append(ctx, sid, MemoryItem{Key: "summary", Value: "second"}); err != nil {
		t.Fatalf("append 2: %v", err)
	}

	items, err := NewFileMemoryStore(dir).Load(ctx, sid)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items: got %d, want 2", len(items))
	}
	if items[0].Value != "first" || items[1].Value != "second" {
		t.Errorf("order not preserved: %+v", items)
	}
	if items[0].At.IsZero() {
		t.Error("At not stamped on append")
	}

	if err := NewFileMemoryStore(dir).Clear(ctx, sid); err != nil {
		t.Fatalf("clear: %v", err)
	}
	after, err := NewFileMemoryStore(dir).Load(ctx, sid)
	if err != nil {
		t.Fatalf("load after clear: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("expected empty after clear, got %d", len(after))
	}
}

// TestSafeNameNoTraversal: a caller-supplied session id containing path
// separators and ".." must never write outside the store directory. The file
// for a malicious id lands inside dir/context as a hashed name, nowhere else.
func TestSafeNameNoTraversal(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	evil := "../../../../etc/passwd"

	s := NewFileContextStore(dir)
	if err := s.Save(ctx, evil, Context{Messages: []Message{{Role: "user", Content: "x"}}}); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Walk the whole store dir: the only regular file written must be the single
	// hashed .json under <dir>/context — the "../etc/passwd" segments never
	// materialize as directories, proving the id cannot escape the root.
	var files []string
	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected exactly 1 file under store dir, got %d: %v", len(files), files)
	}
	if got := files[0]; filepath.Dir(got) != filepath.Join(dir, "context") || filepath.Ext(got) != ".json" {
		t.Errorf("file written outside <dir>/context or not hashed .json: %q", got)
	}

	// And it still round-trips by the same id.
	got, err := NewFileContextStore(dir).Load(ctx, evil)
	if err != nil || len(got.Messages) != 1 {
		t.Fatalf("round-trip by evil id failed: got %+v err %v", got, err)
	}
}
