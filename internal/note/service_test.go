package note

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

var idPattern = regexp.MustCompile(`^n_[0-9a-f]{12}$`)

type fakeRepo struct {
	mu      sync.Mutex
	notes   map[string]Note
	saveErr error
	findErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{notes: map[string]Note{}}
}

func (r *fakeRepo) Save(_ context.Context, n Note) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notes[n.ID] = n
	return nil
}

func (r *fakeRepo) Find(_ context.Context, id string) (Note, bool, error) {
	if r.findErr != nil {
		return Note{}, false, r.findErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.notes[id]
	return n, ok, nil
}

func TestService_Create_ValidInput(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	local := time.Date(2026, 9, 7, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	svc.now = func() time.Time { return local }

	got, err := svc.Create(context.Background(), CreateInput{Title: "  买菜 \n", Content: "鸡蛋、牛奶"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !idPattern.MatchString(got.ID) {
		t.Errorf("ID = %q, want match %s", got.ID, idPattern)
	}
	if got.Title != "买菜" {
		t.Errorf("Title = %q, want trimmed %q", got.Title, "买菜")
	}
	if got.Content != "鸡蛋、牛奶" {
		t.Errorf("Content = %q, want %q", got.Content, "鸡蛋、牛奶")
	}
	if got.CreatedAt.Location() != time.UTC {
		t.Errorf("CreatedAt location = %v, want UTC", got.CreatedAt.Location())
	}
	if !got.CreatedAt.Equal(local) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, local)
	}
	if saved, ok := repo.notes[got.ID]; !ok || saved != got {
		t.Errorf("saved = %+v (ok=%v), want %+v", saved, ok, got)
	}
}

func TestService_Create_ContentDefaultsToEmpty(t *testing.T) {
	got, err := NewService(newFakeRepo()).Create(context.Background(), CreateInput{Title: "t"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Content != "" {
		t.Errorf("Content = %q, want empty", got.Content)
	}
}

func TestService_Create_Validation(t *testing.T) {
	const titleMsg = "title must be 1-100 characters"
	const contentMsg = "content must be at most 2000 characters"
	cases := []struct {
		name      string
		in        CreateInput
		wantField string
		wantMsg   string
	}{
		{name: "title missing", in: CreateInput{Content: "c"}, wantField: "title", wantMsg: titleMsg},
		{name: "title blank", in: CreateInput{Title: " \t\n "}, wantField: "title", wantMsg: titleMsg},
		{name: "title 101 ascii", in: CreateInput{Title: strings.Repeat("a", 101)}, wantField: "title", wantMsg: titleMsg},
		{name: "title 101 runes", in: CreateInput{Title: strings.Repeat("字", 101)}, wantField: "title", wantMsg: titleMsg},
		{name: "content 2001 runes", in: CreateInput{Title: "t", Content: strings.Repeat("字", 2001)}, wantField: "content", wantMsg: contentMsg},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			_, err := NewService(repo).Create(context.Background(), tc.in)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			if ve.Field != tc.wantField || ve.Message != tc.wantMsg {
				t.Errorf("ValidationError = {%q %q}, want {%q %q}", ve.Field, ve.Message, tc.wantField, tc.wantMsg)
			}
			if ve.Error() != tc.wantMsg {
				t.Errorf("Error() = %q, want %q", ve.Error(), tc.wantMsg)
			}
			if len(repo.notes) != 0 {
				t.Errorf("repo holds %d notes after rejected input, want 0", len(repo.notes))
			}
		})
	}
}

func TestService_Create_LengthCountsRunes(t *testing.T) {
	cases := []struct {
		name string
		in   CreateInput
	}{
		{name: "title 100 han", in: CreateInput{Title: strings.Repeat("字", 100)}},
		{name: "title 100 after trim", in: CreateInput{Title: "  " + strings.Repeat("a", 100) + "  "}},
		{name: "content 2000 han", in: CreateInput{Title: "t", Content: strings.Repeat("字", 2000)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewService(newFakeRepo()).Create(context.Background(), tc.in); err != nil {
				t.Fatalf("Create: %v, want nil", err)
			}
		})
	}
}

func TestService_Create_ConcurrentUniqueIDs(t *testing.T) {
	svc := NewService(newFakeRepo())
	const n = 100
	ids := make(chan string, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := svc.Create(context.Background(), CreateInput{Title: "t"})
			if err != nil {
				errs <- err
				return
			}
			ids <- got.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Errorf("Create: %v", err)
	}
	seen := make(map[string]bool, n)
	for id := range ids {
		if !idPattern.MatchString(id) {
			t.Errorf("ID = %q, want match %s", id, idPattern)
		}
		if seen[id] {
			t.Errorf("duplicate ID %q", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Fatalf("got %d distinct IDs, want %d", len(seen), n)
	}
}

func TestService_Create_IDGenerationFails(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	boom := errors.New("entropy exhausted")
	svc.newID = func() (string, error) { return "", boom }

	_, err := svc.Create(context.Background(), CreateInput{Title: "t"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
	var ve *ValidationError
	if errors.As(err, &ve) {
		t.Errorf("err = %v, must not be a ValidationError", err)
	}
	if len(repo.notes) != 0 {
		t.Errorf("repo holds %d notes, want 0", len(repo.notes))
	}
}

func TestService_Create_SaveFails(t *testing.T) {
	repo := newFakeRepo()
	repo.saveErr = errors.New("store down")

	_, err := NewService(repo).Create(context.Background(), CreateInput{Title: "t"})
	if !errors.Is(err, repo.saveErr) {
		t.Fatalf("err = %v, want wrapped %v", err, repo.saveErr)
	}
}

func TestService_Get(t *testing.T) {
	stored := Note{ID: "n_0123456789ab", Title: "t", Content: "c", CreatedAt: time.Date(2026, 9, 7, 2, 0, 0, 0, time.UTC)}
	repo := newFakeRepo()
	repo.notes[stored.ID] = stored
	svc := NewService(repo)

	t.Run("found", func(t *testing.T) {
		got, err := svc.Get(context.Background(), stored.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got != stored {
			t.Errorf("Get = %+v, want %+v", got, stored)
		}
	})
	t.Run("missing", func(t *testing.T) {
		if _, err := svc.Get(context.Background(), "n_000000000000"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})
	t.Run("case sensitive", func(t *testing.T) {
		if _, err := svc.Get(context.Background(), strings.ToUpper(stored.ID)); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})
	t.Run("repo error", func(t *testing.T) {
		broken := newFakeRepo()
		broken.findErr = errors.New("store down")
		_, err := NewService(broken).Get(context.Background(), stored.ID)
		if !errors.Is(err, broken.findErr) {
			t.Fatalf("err = %v, want wrapped %v", err, broken.findErr)
		}
		if errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, must not be ErrNotFound", err)
		}
	})
}

func TestService_Create_SaveAlreadyExists(t *testing.T) {
	repo := newFakeRepo()
	repo.saveErr = ErrAlreadyExists

	_, err := NewService(repo).Create(context.Background(), CreateInput{Title: "t"})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("err = %v, want wrapped ErrAlreadyExists", err)
	}
	if !strings.HasPrefix(err.Error(), "save note: ") {
		t.Errorf("err = %q, want prefix %q", err.Error(), "save note: ")
	}
	var ve *ValidationError
	if errors.As(err, &ve) {
		t.Errorf("err = %v, must not be a ValidationError", err)
	}
}
