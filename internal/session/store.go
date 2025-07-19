// Package session
// Author: momentics <momentics@gmail.com>
//
// Lock-free sharded session storage manager.
// Scales for high concurrency and session-churn under high-load.

package session

import (
	"sync"
)

type sessionManager struct {
	shards []*sessionShard
	N      int
}

type sessionShard struct {
	mu       sync.RWMutex
	sessions map[string]*sessionImpl
}

func NewSessionManager(shardCount int) SessionManager {
	mgr := &sessionManager{
		N:      shardCount,
		shards: make([]*sessionShard, shardCount),
	}
	for i := 0; i < shardCount; i++ {
		mgr.shards[i] = &sessionShard{
			sessions: make(map[string]*sessionImpl),
		}
	}
	return mgr
}

func (m *sessionManager) shard(id string) *sessionShard {
	hash := fnv32(id)
	idx := int(hash % uint32(m.N))
	return m.shards[idx]
}

func (m *sessionManager) Create(id string) (Session, error) {
	s := m.shard(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.sessions[id]; ok {
		return existing, nil
	}
	newSess := newSession(id)
	s.sessions[id] = newSess
	return newSess, nil
}

func (m *sessionManager) Get(id string) (Session, bool) {
	s := m.shard(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

func (m *sessionManager) Delete(id string) {
	s := m.shard(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.Cancel()
		delete(s.sessions, id)
	}
}

func (m *sessionManager) Range(fn func(Session)) {
	for _, shard := range m.shards {
		shard.mu.RLock()
		for _, session := range shard.sessions {
			fn(session)
		}
		shard.mu.RUnlock()
	}
}

func fnv32(key string) uint32 {
	const prime32 = 16777619
	var hash uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		hash *= prime32
		hash ^= uint32(key[i])
	}
	return hash
}
