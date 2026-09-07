package memstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"example.com/go-backend-template/internal/note"
)

func TestStore_SaveFind_RoundTrip(t *testing.T) {
	store := New()
	want := note.Note{ID: "n_0123456789ab", Title: "t", Content: "c", CreatedAt: time.Date(2026, 9, 7, 2, 0, 0, 0, time.UTC)}

	if err := store.Save(context.Background(), want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok, err := store.Find(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !ok {
		t.Fatalf("Find ok = false, want true")
	}
	if got != want {
		t.Errorf("Find = %+v, want %+v", got, want)
	}
}

func TestStore_Find_Missing(t *testing.T) {
	store := New()
	if err := store.Save(context.Background(), note.Note{ID: "n_0123456789ab"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cases := []struct {
		name string
		id   string
	}{
		{name: "unknown id", id: "n_000000000000"},
		{name: "case mismatch", id: "N_0123456789AB"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok, err := store.Find(context.Background(), tc.id)
			if err != nil {
				t.Fatalf("Find: %v", err)
			}
			if ok {
				t.Errorf("Find ok = true, want false (got %+v)", got)
			}
		})
	}
}

func TestStore_ConcurrentSave(t *testing.T) {
	store := New()
	const n = 100
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- store.Save(context.Background(), note.Note{ID: fmt.Sprintf("n_%012x", i), Title: fmt.Sprint(i)})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("Save: %v", err)
		}
	}
	for i := range n {
		id := fmt.Sprintf("n_%012x", i)
		got, ok, err := store.Find(context.Background(), id)
		if err != nil || !ok {
			t.Fatalf("Find(%q) = ok %v, err %v; want found", id, ok, err)
		}
		if got.Title != fmt.Sprint(i) {
			t.Errorf("Find(%q).Title = %q, want %q", id, got.Title, fmt.Sprint(i))
		}
	}
}

func TestStore_Save_DuplicateID(t *testing.T) {
	store := New()
	first := note.Note{ID: "n_0123456789ab", Title: "first", Content: "c1", CreatedAt: time.Date(2026, 9, 7, 2, 0, 0, 0, time.UTC)}
	if err := store.Save(context.Background(), first); err != nil {
		t.Fatalf("Save: %v", err)
	}

	err := store.Save(context.Background(), note.Note{ID: first.ID, Title: "second", Content: "c2"})
	if !errors.Is(err, note.ErrAlreadyExists) {
		t.Fatalf("second Save err = %v, want ErrAlreadyExists", err)
	}
	got, ok, err := store.Find(context.Background(), first.ID)
	if err != nil || !ok {
		t.Fatalf("Find = ok %v, err %v; want found", ok, err)
	}
	if got != first {
		t.Errorf("Find = %+v, want original %+v (must not be overwritten)", got, first)
	}
}
