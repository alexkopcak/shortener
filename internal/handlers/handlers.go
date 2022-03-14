package handlers

import (
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alexkopcak/shortener/internal/storage"
	"github.com/asaskevich/govalidator"
	"github.com/go-chi/chi/v5"
)

type (
	Handler struct {
		*chi.Mux
		Repo           storage.Dictionary
		BaseURL        string
		secretKey      string
		cookieAuthName string
	}

	key uint64
)

const (
	keyPrincipalID key = iota
)

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// var defaultCompressibleContentTypes = []string{
// 	"text/html",
// 	"text/css",
// 	"text/plain",
// 	"text/javascript",
// 	"application/javascript",
// 	"application/x-javascript",
// 	"application/json",
// 	"application/atom+xml",
// 	"application/rss+xml",
// 	"image/svg+xml",
// }

//func URLHandler(repo *storage.Dictionary, baseURL string, secretKey string, cookieAuthName string) *Handler {
func URLHandler(repo *storage.Dictionary, cfg Config) *Handler {
	h := &Handler{
		Mux:            chi.NewMux(),
		Repo:           *repo,
		BaseURL:        cfg.BaseURL,
		secretKey:      cfg.SecretKey,
		cookieAuthName: cfg.CookieAuthName,
	}

	h.Mux.Use(h.authMiddlewareHandler)
	h.Mux.Use(gzipMiddlewareHandle)
	//h.Mux.Use(middleware.Compress(gzip.DefaultCompression, defaultCompressibleContentTypes...))

	h.Mux.Get("/{idValue}", h.GetHandler())
	h.Mux.Get("/api/user/urls", h.GetAPIAllURLHandler())
	h.Mux.Get("/ping", h.GetPing(cfg))
	h.Mux.Post("/", h.PostHandler())
	h.Mux.Post("/api/shorten", h.PostAPIHandler())
	h.Mux.MethodNotAllowed(h.MethodNotAllowed())
	h.Mux.NotFound(h.NotFound())

	return h
}

func (h *Handler) GetPing(cfg Config) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.DB != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if cfg.DB.PingContext(ctx) != nil {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
}

func gzipMiddlewareHandle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gzr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			r.Body = gzr
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gzw, err := gzip.NewWriterLevel(w, gzip.DefaultCompression)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer gzw.Close()
		w.Header().Set("Content-Encoding", "gzip")
		next.ServeHTTP(gzipWriter{ResponseWriter: w, Writer: gzw}, r)
	})
}

func (h *Handler) decodeAuthCookie(cookie *http.Cookie) (uint64, error) {
	if cookie == nil {
		return 0, http.ErrNoCookie
	}

	data, err := hex.DecodeString(cookie.Value)
	if err != nil {
		return 0, err
	}

	id := binary.BigEndian.Uint64(data[:8])

	hm := hmac.New(sha256.New, []byte(h.secretKey))
	hm.Write(data[:8])
	sign := hm.Sum(nil)
	if hmac.Equal(data[8:], sign) {
		return id, nil
	}
	return 0, http.ErrNoCookie
}

func (h *Handler) generateAuthCookie() (*http.Cookie, uint64, error) {
	id := make([]byte, 8)

	_, err := rand.Read(id)
	if err != nil {
		return nil, 0, err
	}

	hm := hmac.New(sha256.New, []byte(h.secretKey))
	hm.Write(id)
	sign := hex.EncodeToString(append(id, hm.Sum(nil)...))

	return &http.Cookie{
			Name:  h.cookieAuthName,
			Value: sign,
		},
		binary.BigEndian.Uint64(id),
		nil
}

func (h *Handler) authMiddlewareHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(h.cookieAuthName)
		if err != nil && err != http.ErrNoCookie {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := h.decodeAuthCookie(cookie)
		if err != nil {
			cookie, id, err = h.generateAuthCookie()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.SetCookie(w, cookie)
		}
		ctx := context.WithValue(r.Context(), keyPrincipalID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
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

func (h *Handler) GetAPIAllURLHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, _ := ctx.Value(keyPrincipalID).(uint64)

		result := h.Repo.GetUserURL(h.BaseURL, userID)

		w.Header().Set("Content-Type", "application/json")

		if len(result) == 0 {
			w.WriteHeader(http.StatusNoContent)
			w.Write([]byte("[]"))
			return
		} else {
			w.WriteHeader(http.StatusOK)
		}

		if err := json.NewEncoder(w).Encode(&result); err != nil {
			http.Error(w, "Something went wrong!", http.StatusBadRequest)
			return
		}

		// if len(h.Repo.Items) == 0 {
		// 	w.WriteHeader(http.StatusNoContent)
		// 	w.Header().Set("Content-Type", "application/json")
		// 	w.Write([]byte("[]"))
		// 	return
		// }

		// result := []struct {
		// 	ShortURL    string `json:"short_url"`
		// 	OriginalURL string `json:"original_url"`
		// }{}
		// for k, v := range h.Repo.Items {
		// 	result = append(result,
		// 		struct {
		// 			ShortURL    string `json:"short_url"`
		// 			OriginalURL string `json:"original_url"`
		// 		}{
		// 			h.BaseURL + "/" + k,
		// 			v,
		// 		})
		// }
		// w.Header().Set("Content-Type", "application/json")
		// w.WriteHeader(http.StatusOK)

		// if err := json.NewEncoder(w).Encode(&result); err != nil {
		// 	http.Error(w, "Something went wrong!", http.StatusBadRequest)
		// 	return
		// }
	}
}

func (h *Handler) PostAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// var reader io.Reader
		// if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
		// 	gz, err := gzip.NewReader(r.Body)
		// 	if err != nil {
		// 		http.Error(w, err.Error(), http.StatusBadRequest)
		// 		return
		// 	}
		// 	reader = gz
		// 	defer gz.Close()
		// } else {
		// 	reader = r.Body
		// 	defer r.Body.Close()
		// }

		// bodyRaw, err := io.ReadAll(reader)
		ctx := r.Context()
		userID, _ := ctx.Value(keyPrincipalID).(uint64)

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

		requestValue, err := h.Repo.AddURL(aliasRequest.LongURLValue, userID)
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
		// var reader io.Reader
		// if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
		// 	gz, err := gzip.NewReader(r.Body)
		// 	if err != nil {
		// 		http.Error(w, err.Error(), http.StatusBadRequest)
		// 		return
		// 	}
		// 	reader = gz
		// 	defer gz.Close()
		// } else {
		// 	reader = r.Body
		// 	defer r.Body.Close()
		// }

		// bodyRaw, err := io.ReadAll(reader)
		ctx := r.Context()
		userID, _ := ctx.Value(keyPrincipalID).(uint64)

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

		requestValue, err := h.Repo.AddURL(aliasRequest.LongURLValue, userID)
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
