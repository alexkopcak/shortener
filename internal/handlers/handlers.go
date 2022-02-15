package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/alexkopcak/shortener/internal/storage"
	"github.com/go-chi/chi/v5"
)

type key int

const (
	serverAddr         = "http://localhost:8080/"
	keyPrincipalID key = iota
)

type Handler struct {
	*chi.Mux
	Repo storage.Dictionary
}

func URLHandler(repo *storage.Dictionary) *Handler {
	h := &Handler{
		Mux:  chi.NewMux(),
		Repo: *repo,
	}

	h.Mux.Route("/{idValue}", func(r chi.Router) {
		r.Use(h.IdContext)
		r.Get("/", h.GetHandler())
	})
	h.Mux.Post("/", h.PostHandler())
	h.Mux.MethodNotAllowed(h.MethodNotAllowed())
	h.Mux.NotFound(h.NotFound())

	return h
}

func (h *Handler) MethodNotAllowed() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Only GET and POST methods are supported!", http.StatusBadRequest)
	}
}

func (h *Handler) NotFound() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad request!", http.StatusBadRequest)
	}
}

func (h *Handler) IdContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idValue := chi.URLParam(r, "idValue")
		if idValue == "" ||
			strings.TrimSpace(idValue) == "" {
			http.Error(w, "Bad request!", http.StatusBadRequest)
			return
		}
		longURLValue := h.Repo.GetURL(idValue)
		if longURLValue == "" {
			http.Error(w, "There are no any short Urls", http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), keyPrincipalID, longURLValue)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) GetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		longURLValue, ok := ctx.Value(keyPrincipalID).(string)
		if !ok {
			http.Error(w, "Something went wrong", http.StatusBadRequest)
			return
		}
		w.Header().Set("Location", longURLValue)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}

func (h *Handler) PostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyRaw, err := io.ReadAll(r.Body)
		if err != nil || len(bodyRaw) == 0 {
			http.Error(w, "Body are not contain URL", http.StatusBadRequest)
			return
		}

		bodyString := string(bodyRaw)
		requestValue := h.Repo.AddURL(bodyString)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_, err = w.Write([]byte(serverAddr + requestValue))
		if err != nil {
			http.Error(w, "Something went wrong", http.StatusBadRequest)
			return
		}
	}
}
