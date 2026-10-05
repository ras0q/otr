package reghttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

func Handle(h func(w http.ResponseWriter, req *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			if herr, ok := errors.AsType[*HTTPError](err); ok {
				slog.ErrorContext(r.Context(), "HTTP error", "error", herr, slog.Int("status", herr.Status))
				http.Error(w, herr.Error(), herr.Status)
				return
			}

			slog.ErrorContext(r.Context(), "HTTP error", "error", err, slog.Int("status", http.StatusInternalServerError))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})
}

type HTTPError struct {
	Status int
	Err    error
}

func (e *HTTPError) Error() string {
	return e.Err.Error()
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}

func StatusError(status int, message string) *HTTPError {
	return &HTTPError{Status: status, Err: errors.New(message)}
}

func BadRequest(message string) error {
	return StatusError(http.StatusBadRequest, message)
}

func NotImplemented(message string) error {
	return StatusError(http.StatusNotImplemented, message)
}

func WriteJSON(w http.ResponseWriter, contentType string, v any) error {
	w.Header().Set("Content-Type", contentType)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	return nil
}
