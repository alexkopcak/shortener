package handlers

import (
	"compress/gzip"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/alexkopcak/shortener/internal/storage"
	"github.com/asaskevich/govalidator"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
)

const (
	secretKey      = "We learn Go language"
	cookieAuthName = "shortenerId"
)

type Handler struct {
	*chi.Mux
	Repo    storage.Dictionary
	BaseURL string
}

// type gzipWriter struct {
// 	http.ResponseWriter
// 	Writer io.Writer
// }

// func (w gzipWriter) Write(b []byte) (int, error) {
// 	return w.Writer.Write(b)
// }

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

	//встроенный функционал
	h.Mux.Use(middleware.Compress(gzip.DefaultCompression, defaultCompressibleContentTypes...))
	// самописный миддлваре gzip
	//h.Mux.Use(gzipMiddlewareHandler)

	h.Mux.Use(authMiddlewareHandler)

	h.Mux.Get("/{idValue}", h.GetHandler())
	h.Mux.Get("/api/users/urls", h.GetAPIAllURLHandler())
	h.Mux.Post("/", h.PostHandler())
	h.Mux.Post("/api/shorten", h.PostAPIHandler())
	h.Mux.MethodNotAllowed(h.MethodNotAllowed())
	h.Mux.NotFound(h.NotFound())

	return h
}

func decodeAuthCookie(cookie *http.Cookie) (uint64, error) {
	if cookie == nil {
		return 0, http.ErrNoCookie
	}

	data, err := hex.DecodeString(cookie.Value)
	if err != nil {
		return 0, err
	}

	id := binary.BigEndian.Uint64(data[:8])

	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write(data[:8])
	sign := h.Sum(nil)
	if hmac.Equal(data[8:], sign) {
		return id, nil
	}
	return 0, http.ErrNoCookie
}

func generateAuthCookie() (*http.Cookie, error) {
	id := make([]byte, 8)

	_, err := rand.Read(id)
	if err != nil {
		return nil, err
	}

	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write(id)
	sign := hex.EncodeToString(append(id, h.Sum(nil)...))

	return &http.Cookie{
		Name:  cookieAuthName,
		Value: sign,
	}, nil
}

func authMiddlewareHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieAuthName)
		if err != nil && err != http.ErrNoCookie {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, err = decodeAuthCookie(cookie)
		if err != nil {
			cookie, err = generateAuthCookie()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.SetCookie(w, cookie)
		}
		next.ServeHTTP(w, r)
	})
}

// func gzipMiddlewareHandler(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
// 			next.ServeHTTP(w, r)
// 			return
// 		}
// 		gz, err := gzip.NewWriterLevel(w, gzip.DefaultCompression)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 			return
// 		}
// 		defer gz.Close()
// 		w.Header().Set("Content-Encoding", "gzip")
// 		next.ServeHTTP(gzipWriter{ResponseWriter: w, Writer: gz}, r)
// 	})
// }

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

func (h *Handler) GetAPIAllURLHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(h.Repo.Items) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		result := []struct {
			ShortURL    string `json:"short_url"`
			OriginalURL string `json:"original_url"`
		}{}
		for k, v := range h.Repo.Items {
			result = append(result,
				struct {
					ShortURL    string `json:"short_url"`
					OriginalURL string `json:"original_url"`
				}{
					h.BaseURL + "/" + k,
					v,
				})
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(&result); err != nil {
			http.Error(w, "Something went wrong!", http.StatusBadRequest)
			return
		}
	}
}

func (h *Handler) PostAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var reader io.Reader
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			reader = gz
			defer gz.Close()
		} else {
			reader = r.Body
			defer r.Body.Close()
		}

		bodyRaw, err := io.ReadAll(reader)
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
		var reader io.Reader
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			reader = gz
			defer gz.Close()
		} else {
			reader = r.Body
			defer r.Body.Close()
		}

		bodyRaw, err := io.ReadAll(reader)
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
