// package handlers endpoints
package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/alexkopcak/shortener/internal/config"
	"github.com/alexkopcak/shortener/internal/storage"
	"github.com/asaskevich/govalidator"
	"github.com/go-chi/chi/v5"

	"net/http/pprof"
)

// type Handler - handler class.
type (
	Handler struct {
		*chi.Mux
		dChannel chan *storage.DeletedShortURLValues
		Repo     storage.Storage
		Cfg      config.Config
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

// NewURLHandler create handler object and set handlers endpoints.
func NewURLHandler(repo storage.Storage, cfg config.Config, dChan chan *storage.DeletedShortURLValues) *Handler {
	h := &Handler{
		Mux:      chi.NewMux(),
		Repo:     repo,
		Cfg:      cfg,
		dChannel: dChan,
	}

	h.Mux.Use(h.authMiddlewareHandler)
	h.Mux.Use(gzipMiddlewareHandle)

	h.Mux.Get("/{idValue}", h.GetHandler())
	h.Mux.Get("/api/user/urls", h.GetAPIAllURLHandler())
	h.Mux.Get("/ping", h.Ping())
	h.Mux.Post("/", h.PostHandler())
	h.Mux.Post("/api/shorten", h.PostAPIHandler())
	h.Mux.Post("/api/shorten/batch", h.PostAPIBatchHandler())
	h.Mux.Delete("/api/user/urls", h.DeleteUserURLHandler())
	h.Mux.Get("/api/internal/stats", h.GetInternalStats())

	h.Mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	h.Mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	h.Mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	h.Mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	h.Mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	h.Mux.Handle("/debug/pprof/{cmd}", http.HandlerFunc(pprof.Index))

	h.Mux.MethodNotAllowed(h.MethodNotAllowed())
	h.Mux.NotFound(h.NotFound())

	return h
}

// DeleteUserURLHandler godoc
// @Summary delete user URLs based on params
// @Tags Storage
// @Accept json
// @Param shortURLs body string true
// @Success 202 {string} string
// @Failure 400 {string} string
// @Router /api/user/urls [delete]
func (h *Handler) DeleteUserURLHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, _ := ctx.Value(keyPrincipalID).(int32)

		var shortURLs []string

		if err := json.NewDecoder(r.Body).Decode(&shortURLs); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		deletedURLs := &storage.DeletedShortURLValues{
			ShortURLValues: shortURLs,
			UserIDValue:    userID,
		}

		h.dChannel <- deletedURLs

		w.WriteHeader(http.StatusAccepted)
	})
}

// Ping godoc
// @Summary simple test database connection
// @Tags Health
// @Success 200 {string} string
// @Failure 400 {string} string
// @Router /ping [get]
func (h *Handler) Ping() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.Repo.Ping(r.Context()) == nil {
			w.WriteHeader(http.StatusOK)
			return
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

func (h *Handler) decodeAuthCookie(cookie *http.Cookie) (int32, error) {
	if cookie == nil {
		return 0, http.ErrNoCookie
	}

	data, err := hex.DecodeString(cookie.Value)
	if err != nil {
		return 0, err
	}

	var id int32
	err = binary.Read(bytes.NewReader(data[:4]), binary.BigEndian, &id)
	if err != nil {
		return 0, err
	}

	hm := hmac.New(sha256.New, []byte(h.Cfg.SecretKey))
	hm.Write(data[:4])
	sign := hm.Sum(nil)
	if hmac.Equal(data[4:], sign) {
		return id, nil
	}
	return 0, http.ErrNoCookie
}

func (h *Handler) generateAuthCookie() (*http.Cookie, int32, error) {
	id := make([]byte, 4)

	_, err := rand.Read(id)
	if err != nil {
		return nil, 0, err
	}

	hm := hmac.New(sha256.New, []byte(h.Cfg.SecretKey))
	hm.Write(id)
	sign := hex.EncodeToString(append(id, hm.Sum(nil)...))

	var result int32
	err = binary.Read(bytes.NewReader(id), binary.BigEndian, &result)

	return &http.Cookie{
			Name:  h.Cfg.CookieAuthName,
			Value: sign,
		},
		result,
		err
}

func (h *Handler) authMiddlewareHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(h.Cfg.CookieAuthName)
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

// GetHandler godoc
// @Summary get short URL value
// @Tags Storage
// @Param idValue path string true "idValue"
// @Success 307 {string} string
// @Failure 400,410 {string} string
// @Router /{idValue} [get]
func (h *Handler) GetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idValue := chi.URLParam(r, "idValue")
		if idValue == "" {
			http.Error(w, "Bad request!", http.StatusBadRequest)
			return
		}
		longURLValue, err := h.Repo.GetURL(r.Context(), idValue)
		if err != nil {
			if errors.Is(err, storage.ErrNotExistRecord) {
				w.WriteHeader(http.StatusGone)
				return
			}
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

// GetAPIAllURLHandler godoc
// @Summary get short URL value
// @Tags Storage
// @Success 200,204 {string} string
// @Failure 400 {string} string
// @Router /api/user/urls [get]
func (h *Handler) GetAPIAllURLHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, _ := ctx.Value(keyPrincipalID).(int32)

		result, err := h.Repo.GetUserURL(r.Context(), h.Cfg.BaseURL, userID)
		if err != nil {
			http.Error(w, "Something went wrong!", http.StatusBadRequest)
			return
		}

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
	}
}

// PostAPIBatchHandler godoc
// @Summary add batch short URL values
// @Tags Storage
// @Accept json
// @Param batchrequest body storage.BatchRequestArray true "Batch request"
// @Success 201 {string} string
// @Failure 400 {array} storage.BatchRequest
// @Router /api/shorten/batch [post]
func (h *Handler) PostAPIBatchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, _ := ctx.Value(keyPrincipalID).(int32)

		bodyRaw, err := io.ReadAll(r.Body)
		if err != nil || len(bodyRaw) == 0 {
			http.Error(w, "Body are not contain URL!", http.StatusBadRequest)
			return
		}

		batchRequest := storage.BatchRequestArray{}

		if err = json.Unmarshal(bodyRaw, &batchRequest); err != nil {
			http.Error(w, "Bad request!", http.StatusBadRequest)
			return
		}

		for i := range batchRequest {
			ptr := &batchRequest[i]
			ptr.ShortURL = storage.ShortURLGenerator()
		}

		responseValue, err := h.Repo.PostAPIBatch(ctx, &batchRequest, h.Cfg.BaseURL, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		if err := json.NewEncoder(w).Encode(&responseValue); err != nil {
			http.Error(w, "Something went wrong!", http.StatusBadRequest)
			return
		}
	}
}

// PostAPIHandler godoc
// @Summary set short URL value
// @Tags Storage
// @Accept json
// @Param bodyraw body aliasRequest true "Alias request"
// @Success 201 {string} string
// @Failure 400,409 {string} string
// @Router /api/shorten [post]
func (h *Handler) PostAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, _ := ctx.Value(keyPrincipalID).(int32)

		bodyRaw, err := io.ReadAll(r.Body)
		if err != nil || len(bodyRaw) == 0 {
			http.Error(w, "Body are not contain URL!", http.StatusBadRequest)
			return
		}
		aliasRequest := &struct {
			LongURLValue string `json:"url,omitempty" valid:"url"`
		}{}

		if err = json.Unmarshal(bodyRaw, aliasRequest); err != nil {
			http.Error(w, "Bad request!", http.StatusBadRequest)
			return
		}

		_, err = govalidator.ValidateStruct(aliasRequest)
		if err != nil {
			http.Error(w, "Body are not contains URL value!", http.StatusBadRequest)
			return
		}

		requestValue, err := h.Repo.AddURL(r.Context(), aliasRequest.LongURLValue, storage.ShortURLGenerator(), userID)
		if err != nil {
			if !errors.Is(err, storage.ErrDuplicateRecord) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, storage.ErrDuplicateRecord) {
			w.WriteHeader(http.StatusConflict)
		} else {
			w.WriteHeader(http.StatusCreated)
		}

		responseValue := struct {
			ResultValue string `json:"result"`
		}{
			ResultValue: h.Cfg.BaseURL + "/" + requestValue,
		}
		if err := json.NewEncoder(w).Encode(&responseValue); err != nil {
			http.Error(w, "Something went wrong!", http.StatusBadRequest)
			return
		}
	}
}

// PostHandler godoc
// @Summary set short URL value
// @Tags Storage
// @Accept string
// @Param bodyraw body aliasRequest true "Alias request"
// @Success 201 {string} string
// @Failure 400,409 {string} string
// @Router /api/shorten [post]
func (h *Handler) PostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, _ := ctx.Value(keyPrincipalID).(int32)

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

		requestValue, err := h.Repo.AddURL(r.Context(), aliasRequest.LongURLValue, storage.ShortURLGenerator(), userID)
		if err != nil {
			if !errors.Is(err, storage.ErrDuplicateRecord) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if errors.Is(err, storage.ErrDuplicateRecord) {
			w.WriteHeader(http.StatusConflict)
		} else {
			w.WriteHeader(http.StatusCreated)
		}
		_, err = w.Write([]byte(h.Cfg.BaseURL + "/" + requestValue))
		if err != nil {
			http.Error(w, "Something went wrong!", http.StatusBadRequest)
			return
		}
	}
}

// GetInternalStats godoc
// @Summary get short URL value
// @Tags Storage
// @Success 200 {string} string
// @Failure 403 {string} string
// @Router /api/internal/stats [get]
func (h *Handler) GetInternalStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if xRealIP := r.Header.Get("X-Real-IP"); xRealIP == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}
}
