package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/go-backend-template/internal/note"
)

var noteIDPattern = regexp.MustCompile(`^n_[0-9a-f]{12}$`)

// 无构建标签：单元层与集成层共用的响应形状与断言。
type noteBody struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

func postNote(h http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/notes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getNote(h http.Handler, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/v1/notes/"+id, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func mustCreateNote(t *testing.T, h http.Handler, body string) noteBody {
	t.Helper()
	rec := postNote(h, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/notes status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	return decodeNote(t, rec.Body.Bytes())
}

func decodeNote(t *testing.T, raw []byte) noteBody {
	t.Helper()
	var body noteBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("invalid JSON body %q: %v", raw, err)
	}
	return body
}

func assertErrorEnvelope(t *testing.T, status int, raw []byte, wantStatus int, wantCode string) errorBody {
	t.Helper()
	if status != wantStatus {
		t.Fatalf("status = %d, want %d (body %s)", status, wantStatus, raw)
	}
	var body errorResponse
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("invalid JSON body %q: %v", raw, err)
	}
	if body.Error.Code != wantCode {
		t.Errorf("error.code = %q, want %q", body.Error.Code, wantCode)
	}
	return body.Error
}

func TestCreateNote_Created(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantTitle   string
		wantContent string
	}{
		{name: "title and content", body: `{"title":"买菜","content":"鸡蛋、牛奶"}`, wantTitle: "买菜", wantContent: "鸡蛋、牛奶"},
		{name: "content omitted and title trimmed", body: `{"title":"  买菜  "}`, wantTitle: "买菜", wantContent: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postNote(newTestHandler(), tc.body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Errorf("Content-Type = %q", ct)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
				t.Fatalf("invalid JSON body: %v", err)
			}
			for _, key := range []string{"id", "title", "content", "created_at"} {
				if _, ok := fields[key]; !ok {
					t.Errorf("response missing field %q: %s", key, rec.Body.String())
				}
			}
			got := decodeNote(t, rec.Body.Bytes())
			if !noteIDPattern.MatchString(got.ID) {
				t.Errorf("id = %q, want match %s", got.ID, noteIDPattern)
			}
			if got.Title != tc.wantTitle || got.Content != tc.wantContent {
				t.Errorf("title/content = %q/%q, want %q/%q", got.Title, got.Content, tc.wantTitle, tc.wantContent)
			}
			if _, err := time.Parse(time.RFC3339Nano, got.CreatedAt); err != nil || !strings.HasSuffix(got.CreatedAt, "Z") {
				t.Errorf("created_at = %q, want RFC 3339 UTC (parse error: %v)", got.CreatedAt, err)
			}
		})
	}
}

func TestCreateNote_ValidationErrors(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantField string
	}{
		{name: "title missing", body: `{"content":"c"}`, wantField: "title"},
		{name: "title blank", body: `{"title":" \t "}`, wantField: "title"},
		{name: "title 101 chars", body: fmt.Sprintf(`{"title":%q}`, strings.Repeat("字", 101)), wantField: "title"},
		{name: "content 2001 chars", body: fmt.Sprintf(`{"title":"t","content":%q}`, strings.Repeat("字", 2001)), wantField: "content"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postNote(newTestHandler(), tc.body)
			e := assertErrorEnvelope(t, rec.Code, rec.Body.Bytes(), http.StatusBadRequest, "invalid_argument")
			if !strings.Contains(e.Message, tc.wantField) {
				t.Errorf("error.message = %q, want it to mention %q", e.Message, tc.wantField)
			}
		})
	}
}

func TestCreateNote_BadBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{"title":`},
		{name: "unknown field", body: `{"title":"t","tags":["x"]}`},
		{name: "not an object", body: `[]`},
		{name: "empty body", body: ``},
		{name: "over 16 KiB", body: fmt.Sprintf(`{"title":"t","content":%q}`, strings.Repeat("a", 17*1024))},
		{name: "trailing json value", body: `{"title":"t"}{"x":1}`},
		{name: "trailing garbage", body: `{"title":"t"} xyz`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postNote(newTestHandler(), tc.body)
			e := assertErrorEnvelope(t, rec.Code, rec.Body.Bytes(), http.StatusBadRequest, "invalid_argument")
			if e.Message != "invalid request body" {
				t.Errorf("error.message = %q, want %q", e.Message, "invalid request body")
			}
		})
	}
}

func TestGetNote_RoundTrip(t *testing.T) {
	h := newTestHandler()
	created := mustCreateNote(t, h, `{"title":"买菜","content":"鸡蛋、牛奶"}`)

	rec := getNote(h, created.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := decodeNote(t, rec.Body.Bytes()); got != created {
		t.Errorf("GET = %+v, want %+v", got, created)
	}
}

func TestGetNote_NotFound(t *testing.T) {
	h := newTestHandler()
	created := mustCreateNote(t, h, `{"title":"t"}`)
	cases := []struct {
		name string
		id   string
	}{
		{name: "unknown id", id: "n_000000000000"},
		{name: "case mismatch", id: strings.ToUpper(created.ID)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := getNote(h, tc.id)
			e := assertErrorEnvelope(t, rec.Code, rec.Body.Bytes(), http.StatusNotFound, "not_found")
			if e.Message != "note not found" {
				t.Errorf("error.message = %q, want %q", e.Message, "note not found")
			}
		})
	}
}

func TestNotes_MethodNotAllowed(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "delete note", method: http.MethodDelete, path: "/v1/notes/n_0123456789ab"},
		{name: "put notes", method: http.MethodPut, path: "/v1/notes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := doRequest(t, tc.method, tc.path); rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestCreateNote_ConcurrentUniqueIDs(t *testing.T) {
	h := newTestHandler()
	const n = 100
	ids := make(chan string, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := postNote(h, fmt.Sprintf(`{"title":"note %d"}`, i))
			if rec.Code != http.StatusCreated {
				errs <- fmt.Errorf("request %d: status = %d (body %s)", i, rec.Code, rec.Body.String())
				return
			}
			var body noteBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				errs <- fmt.Errorf("request %d: decode: %w", i, err)
				return
			}
			ids <- body.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	seen := make(map[string]bool, n)
	for id := range ids {
		if seen[id] {
			t.Errorf("duplicate id %q", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Fatalf("created %d distinct notes, want %d", len(seen), n)
	}
}

type failingRepo struct{ err error }

func (r failingRepo) Save(context.Context, note.Note) error { return r.err }

func (r failingRepo) Find(context.Context, string) (note.Note, bool, error) {
	return note.Note{}, false, r.err
}

func TestNotes_RepositoryFailureReturns500(t *testing.T) {
	h := NewHandler(testLogger(), note.NewService(failingRepo{err: errors.New("disk on fire")}))
	cases := []struct {
		name string
		do   func() *httptest.ResponseRecorder
	}{
		{name: "create", do: func() *httptest.ResponseRecorder { return postNote(h, `{"title":"t"}`) }},
		{name: "get", do: func() *httptest.ResponseRecorder { return getNote(h, "n_0123456789ab") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := tc.do()
			e := assertErrorEnvelope(t, rec.Code, rec.Body.Bytes(), http.StatusInternalServerError, "internal_error")
			if strings.Contains(e.Message, "disk on fire") {
				t.Errorf("error.message = %q leaks internal detail", e.Message)
			}
		})
	}
}
