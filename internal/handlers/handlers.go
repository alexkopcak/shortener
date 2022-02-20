package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/alexkopcak/shortener/internal/storage"
	"github.com/asaskevich/govalidator"
	"github.com/go-chi/chi/v5"
)

const (
	serverAddr = "http://localhost:8080/"
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

	h.Mux.Get("/{idValue}", h.GetHandler())
	h.Mux.Post("/", h.PostHandler())
	h.Mux.Post("/api/shorten", h.PostAPIHandler())
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
		idValue := chi.URLParam(r, "idValue")
		if idValue == "" {
			http.Error(w, "Bad request!", http.StatusBadRequest)
			return
		}
		longURLValue := h.Repo.GetURL(idValue)
		if longURLValue == "" {
			http.Error(w, "There are no any short Urls!", http.StatusBadRequest)
			return
		}
		w.Header().Set("Location", longURLValue)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}

func (h *Handler) PostAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyRaw, err := io.ReadAll(r.Body)
		if err != nil || len(bodyRaw) == 0 {
			http.Error(w, "Body are not contain URL!", http.StatusBadRequest)
			return
		}
		aliasRequest := &struct {
			LongURLValue string `json:"url,omitempty" valid:"url"`
		}{}

		if err := json.Unmarshal(bodyRaw, aliasRequest); err != nil {
			http.Error(w, "Bad request!", http.StatusBadRequest)
			return
		}

		_, err = govalidator.ValidateStruct(aliasRequest)
		if err != nil {
			http.Error(w, "Body are not contains URL value!", http.StatusBadRequest)
			return
		}

		requestValue := h.Repo.AddURL(aliasRequest.LongURLValue)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		responseValue := struct {
			ResultValue string `json:"result"`
		}{
			ResultValue: serverAddr + requestValue,
		}
		responseValueRaw, err := json.Marshal(responseValue)
		if err != nil {
			http.Error(w, "JSON marshaling error!", http.StatusBadRequest)
			return
		}
		_, err = w.Write(responseValueRaw)
		if err != nil {
			http.Error(w, "Something went wrong!", http.StatusBadRequest)
			return
		}
	}
}

func (h *Handler) PostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyRaw, err := io.ReadAll(r.Body)
		if err != nil || len(bodyRaw) == 0 {
			http.Error(w, "Body are not contain URL!", http.StatusBadRequest)
			return
		}

		aliasRequest := &struct {
			LongURLValue string `valid:"url"`
		}{
			LongURLValue: string(bodyRaw),
		}
		_, err = govalidator.ValidateStruct(aliasRequest)
		if err != nil {
			http.Error(w, "Body are not contains URL value!", http.StatusBadRequest)
			return
		}

		requestValue := h.Repo.AddURL(aliasRequest.LongURLValue)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_, err = w.Write([]byte(serverAddr + requestValue))
		if err != nil {
			http.Error(w, "Something went wrong!", http.StatusBadRequest)
			return
		}
	}
}
