package correlation

import (
	"context"
	"sync"
)

// Memory is an in-memory Store for tests. It ignores TTLs.
type Memory struct {
	mu      sync.Mutex
	records map[string]Record
}

// NewMemory builds an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{records: map[string]Record{}}
}

// Put records the correlation for id.
func (m *Memory) Put(_ context.Context, id string, rec Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[id] = rec
	return nil
}

// Get returns the correlation for id, or ErrNotFound.
func (m *Memory) Get(_ context.Context, id string) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	return rec, nil
}
