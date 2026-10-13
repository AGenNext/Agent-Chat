// File-backed ContextStore and MemoryStore: session state persisted as JSON files
// under a base directory, surviving process restarts. Writes are atomic (temp file
// + fsync + rename) so a crash mid-write never corrupts state. Session IDs are
// hashed to safe, fixed filenames (no path traversal from caller-supplied subjects).
//
// Pure standard library — no embedded database, no third-party deps (the headless,
// zero-dependency promise holds). For single-node / small deployments; the
// production binding remains Valkey (context) + Postgres (memory).
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Compile-time checks: the file stores satisfy the interfaces.
var (
	_ ContextStore = (*FileContextStore)(nil)
	_ MemoryStore  = (*FileMemoryStore)(nil)
)

// safeName hashes a (possibly caller-supplied) session id into a filesystem-safe,
// fixed-length name — no "/", "..", or other traversal from the subject.
func safeName(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(sum[:]) + ".json"
}

// atomicWriteJSON writes v to path atomically (temp + fsync + rename).
func atomicWriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// readJSON loads path into v. Returns (false, nil) when the file is absent/empty.
func readJSON(path string, v any) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(data) == 0 {
		return false, nil
	}
	return true, json.Unmarshal(data, v)
}

// FileContextStore persists working context to <dir>/context/<hash>.json.
type FileContextStore struct {
	mu  sync.Mutex
	dir string
}

// NewFileContextStore returns a context store rooted at dir.
func NewFileContextStore(dir string) *FileContextStore {
	return &FileContextStore{dir: filepath.Join(dir, "context")}
}

func (s *FileContextStore) path(id string) string { return filepath.Join(s.dir, safeName(id)) }

// Load returns the stored context for sessionID (zero value if absent).
func (s *FileContextStore) Load(_ context.Context, sessionID string) (Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var c Context
	if _, err := readJSON(s.path(sessionID), &c); err != nil {
		return Context{}, err
	}
	return c, nil
}

// Save atomically replaces the stored context for sessionID.
func (s *FileContextStore) Save(_ context.Context, sessionID string, c Context) error {
	c.Updated = time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	return atomicWriteJSON(s.path(sessionID), c)
}

// Clear removes the stored context for sessionID.
func (s *FileContextStore) Clear(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path(sessionID)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// FileMemoryStore persists durable memory to <dir>/memory/<hash>.json.
type FileMemoryStore struct {
	mu  sync.Mutex
	dir string
}

// NewFileMemoryStore returns a memory store rooted at dir.
func NewFileMemoryStore(dir string) *FileMemoryStore {
	return &FileMemoryStore{dir: filepath.Join(dir, "memory")}
}

func (s *FileMemoryStore) path(id string) string { return filepath.Join(s.dir, safeName(id)) }

// Load returns the durable memory items for sessionID.
func (s *FileMemoryStore) Load(_ context.Context, sessionID string) ([]MemoryItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var items []MemoryItem
	if _, err := readJSON(s.path(sessionID), &items); err != nil {
		return nil, err
	}
	return items, nil
}

// Append adds one durable memory item for sessionID (read-modify-write, atomic).
func (s *FileMemoryStore) Append(_ context.Context, sessionID string, item MemoryItem) error {
	if item.At.IsZero() {
		item.At = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var items []MemoryItem
	if _, err := readJSON(s.path(sessionID), &items); err != nil {
		return err
	}
	items = append(items, item)
	return atomicWriteJSON(s.path(sessionID), items)
}

// Clear removes all durable memory for sessionID.
func (s *FileMemoryStore) Clear(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path(sessionID)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
