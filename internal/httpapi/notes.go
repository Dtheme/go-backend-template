package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"example.com/go-backend-template/internal/note"
)

const maxBodyBytes = 16 << 10

type notesHandler struct {
	logger *slog.Logger
	svc    *note.Service
}

type createNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type noteResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func toNoteResponse(n note.Note) noteResponse {
	return noteResponse{ID: n.ID, Title: n.Title, Content: n.Content, CreatedAt: n.CreatedAt}
}

func (h *notesHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createNoteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", "invalid request body")
		return
	}
	n, err := h.svc.Create(r.Context(), note.CreateInput{Title: req.Title, Content: req.Content})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toNoteResponse(n))
}

func (h *notesHandler) get(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toNoteResponse(n))
}

func (h *notesHandler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *note.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, "invalid_argument", ve.Message)
	case errors.Is(err, note.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "note not found")
	default:
		h.logger.Error("note request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON value")
	}
	return nil
}
