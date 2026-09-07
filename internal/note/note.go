package note

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxTitleRunes   = 100
	maxContentRunes = 2000
)

var (
	ErrNotFound      = errors.New("note not found")
	ErrAlreadyExists = errors.New("note already exists")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type Note struct {
	ID        string
	Title     string
	Content   string
	CreatedAt time.Time
}

type CreateInput struct {
	Title   string
	Content string
}

type Repository interface {
	Save(ctx context.Context, n Note) error
	Find(ctx context.Context, id string) (Note, bool, error)
}

type Service struct {
	repo  Repository
	now   func() time.Time
	newID func() (string, error)
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, newID: newID}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Note, error) {
	title := strings.TrimSpace(in.Title)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleRunes {
		return Note{}, &ValidationError{Field: "title", Message: fmt.Sprintf("title must be 1-%d characters", maxTitleRunes)}
	}
	if utf8.RuneCountInString(in.Content) > maxContentRunes {
		return Note{}, &ValidationError{Field: "content", Message: fmt.Sprintf("content must be at most %d characters", maxContentRunes)}
	}

	id, err := s.newID()
	if err != nil {
		return Note{}, fmt.Errorf("generate note id: %w", err)
	}
	n := Note{ID: id, Title: title, Content: in.Content, CreatedAt: s.now().UTC()}
	if err := s.repo.Save(ctx, n); err != nil {
		return Note{}, fmt.Errorf("save note: %w", err)
	}
	return n, nil
}

func (s *Service) Get(ctx context.Context, id string) (Note, error) {
	n, ok, err := s.repo.Find(ctx, id)
	if err != nil {
		return Note{}, fmt.Errorf("find note: %w", err)
	}
	if !ok {
		return Note{}, ErrNotFound
	}
	return n, nil
}

func newID() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "n_" + hex.EncodeToString(b[:]), nil
}
