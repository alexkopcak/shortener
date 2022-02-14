package handlers

import (
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Repositories interface {
	AddURL(string) string
	GetURL(string) string
}

type Handler struct {
	*chi.Mux
	Repo Repositories
}

func URLHandler(repo Repositories) *Handler {
	h := &Handler{
		Mux:  chi.NewMux(),
		Repo: repo,
	}
	h.Mux.Route("/", func(r chi.Router) {
		r.Get("/", h.GetHandler())
		r.Get("/{id}", h.GetHandler())
		r.Post("/", h.PostHandler())
	})
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

func (h *Handler) GetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestValue := r.URL.Path[len("/"):]
		if requestValue == "" || strings.Contains(requestValue, "/") {
			http.Error(w, "Empty URL", http.StatusBadRequest)
			return
		}

		longURLValue := h.Repo.GetURL(requestValue)
		if longURLValue == "" {
			http.Error(w, "There are no any short Urls", http.StatusBadRequest)
			return
		}

		w.Header().Set("Location", longURLValue)
		w.WriteHeader(http.StatusTemporaryRedirect) // 307
	}
}

func (h *Handler) PostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, "Bad request. POST allow only `/` ", http.StatusBadRequest)
			return
		}

		bodyRaw, err := io.ReadAll(r.Body)
		if err != nil || len(bodyRaw) == 0 {
			http.Error(w, "Body are not contain URL", http.StatusBadRequest)
			return
		}

		bodyString := string(bodyRaw)
		requestValue := h.Repo.AddURL(bodyString)
		shortURLValue := "http://localhost:8080/" + requestValue

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusCreated) // 201
		var byteArray = []byte(shortURLValue)
		_, err = w.Write(byteArray)
		if err != nil {
			http.Error(w, "Something went wrong", http.StatusBadRequest)
			return
		}
	}
}
