package memstore

import (
	"context"
	"sync"

	"example.com/go-backend-template/internal/note"
)

type Store struct {
	mu    sync.RWMutex
	notes map[string]note.Note
}

var _ note.Repository = (*Store)(nil)

func New() *Store {
	return &Store{notes: make(map[string]note.Note)}
}

func (s *Store) Save(_ context.Context, n note.Note) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.notes[n.ID]; exists {
		return note.ErrAlreadyExists
	}
	s.notes[n.ID] = n
	return nil
}

func (s *Store) Find(_ context.Context, id string) (note.Note, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.notes[id]
	return n, ok, nil
}
