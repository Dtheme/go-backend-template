package httpapi

import (
	"log/slog"
	"net/http"

	"example.com/go-backend-template/internal/note"
)

func NewHandler(logger *slog.Logger, svc *note.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /v1/ping", handlePing)

	notes := &notesHandler{logger: logger, svc: svc}
	mux.HandleFunc("POST /v1/notes", notes.create)
	mux.HandleFunc("GET /v1/notes/{id}", notes.get)

	return withRecover(logger, withRequestLog(logger, mux))
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handlePing(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "pong"})
}
