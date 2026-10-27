package session

import (
	"slices"
	"sync"
)

type MemoryStore struct {
	mu       sync.Mutex
	sessions map[string]Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: make(map[string]Session)}
}

func (s *MemoryStore) Create(game Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.sessions[game.ID]; exists {
		return ErrIDConflict
	}
	if s.sessions == nil {
		s.sessions = make(map[string]Session)
	}
	s.sessions[game.ID] = clone(game)
	return nil
}

func (s *MemoryStore) Get(id string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, exists := s.sessions[id]
	if !exists {
		return Session{}, ErrNotFound
	}
	return clone(game), nil
}

func (s *MemoryStore) Update(id string, update func(*Session) error) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, exists := s.sessions[id]
	if !exists {
		return Session{}, ErrNotFound
	}
	working := clone(game)
	if err := update(&working); err != nil {
		return Session{}, err
	}
	s.sessions[id] = clone(working)
	return working, nil
}

func clone(game Session) Session {
	game.History = slices.Clone(game.History)
	if game.OpenedAt != nil {
		openedAt := *game.OpenedAt
		game.OpenedAt = &openedAt
	}
	if game.Explanations != nil {
		explanations := *game.Explanations
		game.Explanations = &explanations
	}
	return game
}
