package handlers

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/alexkopcak/shortener/internal/storage"
	"github.com/asaskevich/govalidator"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	*chi.Mux
	Repo    storage.Dictionary
	BaseURL string
}

var defaultCompressibleContentTypes = []string{
	"text/html",
	"text/css",
	"text/plain",
	"text/javascript",
	"application/javascript",
	"application/x-javascript",
	"application/json",
	"application/atom+xml",
	"application/rss+xml",
	"image/svg+xml",
}

func URLHandler(repo *storage.Dictionary, baseURL string) *Handler {
	h := &Handler{
		Mux:     chi.NewMux(),
		Repo:    *repo,
		BaseURL: baseURL,
	}

	h.Mux.Use(middleware.Compress(gzip.DefaultCompression, defaultCompressibleContentTypes...))
	h.Mux.Use(postDecompressMiddleware)

	h.Mux.Get("/{idValue}", h.GetHandler())
	h.Mux.Post("/", h.PostHandler())
	h.Mux.Post("/api/shorten", h.PostAPIHandler())
	h.Mux.MethodNotAllowed(h.MethodNotAllowed())
	h.Mux.NotFound(h.NotFound())

	return h
}

func postDecompressMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//		fmt.Println("in middleware")
		if r.Method == http.MethodPost && strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer r.Body.Close()
			defer gz.Close()
			request, err := http.NewRequest(r.Method, r.URL.String(), gz)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			//fmt.Printf("%v", request)
			next.ServeHTTP(w, request)
		}
		next.ServeHTTP(w, r)
	})
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
		longURLValue, err := h.Repo.GetURL(idValue)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
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

		requestValue, err := h.Repo.AddURL(aliasRequest.LongURLValue)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		responseValue := struct {
			ResultValue string `json:"result"`
		}{
			ResultValue: h.BaseURL + "/" + requestValue,
		}
		if err := json.NewEncoder(w).Encode(&responseValue); err != nil {
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

		requestValue, err := h.Repo.AddURL(aliasRequest.LongURLValue)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_, err = w.Write([]byte(h.BaseURL + "/" + requestValue))
		if err != nil {
			http.Error(w, "Something went wrong!", http.StatusBadRequest)
			return
		}
	}
}
