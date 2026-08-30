package mcp

import (
	"sync"

	"github.com/yougile-mcp/internal/config"
)

// sessionState — пер-сессионное состояние агента (issue #4).
// В daemon-режиме каждая MCP-сессия (= один подключённый агент) держит
// собственный режим и имя, вместо общего режима в config.json.
type sessionState struct {
	Name string      // имя из clientInfo handshake (идентичность агента)
	Mode config.Mode // пер-сессионный режим; "" — override отсутствует
}

// SessionRegistry — потокобезопасный реестр состояний сессий.
type SessionRegistry struct {
	mu      sync.RWMutex
	states  map[string]sessionState
	changes map[string]int // количество set_mode на сессию (для диагностики/тестов)
}

func newSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		states:  make(map[string]sessionState),
		changes: make(map[string]int),
	}
}

// Remember фиксирует сессию с именем агента из handshake (идемпотентно).
func (r *SessionRegistry) Remember(sessionID, agentName string) {
	if sessionID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.states[sessionID] // сохранить mode, если уже был
	st.Name = agentName
	r.states[sessionID] = st
}

// SetMode устанавливает пер-сессионный режим.
func (r *SessionRegistry) SetMode(sessionID string, mode config.Mode) {
	if sessionID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.states[sessionID]
	st.Mode = mode
	r.states[sessionID] = st
	r.changes[sessionID]++
}

// Get возвращает состояние сессии.
func (r *SessionRegistry) Get(sessionID string) (sessionState, bool) {
	if sessionID == "" {
		return sessionState{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	st, ok := r.states[sessionID]
	return st, ok
}

// Forget удаляет сессию (disconnect).
func (r *SessionRegistry) Forget(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.states, sessionID)
	delete(r.changes, sessionID)
}
