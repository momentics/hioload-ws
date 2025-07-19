// File: internal/session/store.go
// Package session
// Author: momentics <momentics@gmail.com>
//
// Sharded, thread-safe SessionManager for high concurrency.

package session

import (
	"hash/fnv"
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
)

// SessionManager defines operations on sessions.
type SessionManager interface {
	Create(id string) (Session, error)
	Get(id string) (Session, bool)
	Delete(id string)
	Range(func(Session))
}

// Session abstracts per-connection session state.
type Session interface {
	ID() string
	Context() api.Context
	Cancel()
	Done() <-chan struct{}
	Deadline() (time.Time, bool)
}

// sessionManager implements sharded storage for sessions.
type sessionManager struct {
	shards []*sessionShard
	mask   uint32
}

type sessionShard struct {
	mu       sync.RWMutex
	sessions map[string]*sessionImpl
}

// NewSessionManager constructs a sharded manager with shardCount shards.
func NewSessionManager(shardCount int) SessionManager {
	if shardCount <= 0 {
		shardCount = 16
	}
	// find power-of-two shards for bitmasking
	m := nextPowerOfTwo(uint32(shardCount))
	shards := make([]*sessionShard, m)
	for i := range shards {
		shards[i] = &sessionShard{sessions: make(map[string]*sessionImpl)}
	}
	return &sessionManager{shards: shards, mask: m - 1}
}

// shard picks the correct shard for a given id.
func (m *sessionManager) shard(id string) *sessionShard {
	h := fnv32(id)
	return m.shards[h&m.mask]
}

// Create returns existing or new session for id.
func (m *sessionManager) Create(id string) (Session, error) {
	sh := m.shard(id)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if s, ok := sh.sessions[id]; ok {
		return s, nil
	}
	s := newSession(id)
	sh.sessions[id] = s
	return s, nil
}

// Get fetches a session if present.
func (m *sessionManager) Get(id string) (Session, bool) {
	sh := m.shard(id)
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	s, ok := sh.sessions[id]
	return s, ok
}

// Delete cancels and removes the session.
func (m *sessionManager) Delete(id string) {
	sh := m.shard(id)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if s, ok := sh.sessions[id]; ok {
		s.Cancel()
		delete(sh.sessions, id)
	}
}

// Range applies fn to all sessions.
func (m *sessionManager) Range(fn func(Session)) {
	for _, sh := range m.shards {
		sh.mu.RLock()
		for _, s := range sh.sessions {
			fn(s)
		}
		sh.mu.RUnlock()
	}
}

// fnv32 hashes a string to uint32.
func fnv32(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// nextPowerOfTwo returns the next power-of-two >= v.
func nextPowerOfTwo(v uint32) uint32 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	return v
}
