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
)

type (
	Handler struct {
		*chi.Mux
		Repo storage.Storage
		Cfg  config.Config
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

func URLHandler(repo storage.Storage, cfg config.Config) *Handler {
	h := &Handler{
		Mux:  chi.NewMux(),
		Repo: repo,
		Cfg:  cfg,
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
	h.Mux.MethodNotAllowed(h.MethodNotAllowed())
	h.Mux.NotFound(h.NotFound())

	return h
}

func (h *Handler) DeleteUserURLHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, _ := ctx.Value(keyPrincipalID).(int32)

		var shortURLs []string

		if err := json.NewDecoder(r.Body).Decode(&shortURLs); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		go func() {
			h.Repo.DeleteUserURL(context.Background(), shortURLs, userID)
		}()
		w.WriteHeader(http.StatusAccepted)
	})
}

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
	//	fmt.Printf("err %v\n", err)
	if err != nil {
		return 0, err
	}

	//	fmt.Printf("data[:4] %v\n", data)

	//id := int(binary.BigEndian.Uint32(data[:4]))
	var id int32
	err = binary.Read(bytes.NewReader(data[:4]), binary.BigEndian, &id)
	if err != nil {
		//		fmt.Printf("%v\n", err)

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
	//	fmt.Printf("%v\n", err)

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
		//		fmt.Println("auth middle ware")
		id, err := h.decodeAuthCookie(cookie)
		//		fmt.Printf("id: %v err: %v\n", id, err)
		if err != nil {
			cookie, id, err = h.generateAuthCookie()
			//			fmt.Printf("cookie: %v id: %v err: %v\n", cookie, id, err)
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
		longURLValue, deleted, err := h.Repo.GetURL(r.Context(), idValue)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if deleted {
			w.WriteHeader(http.StatusGone)
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

func (h *Handler) PostAPIBatchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, _ := ctx.Value(keyPrincipalID).(int32)

		bodyRaw, err := io.ReadAll(r.Body)
		if err != nil || len(bodyRaw) == 0 {
			http.Error(w, "Body are not contain URL!", http.StatusBadRequest)
			return
		}

		batchRequest := &storage.BatchRequestArray{}

		if err := json.Unmarshal(bodyRaw, batchRequest); err != nil {
			http.Error(w, "Bad request!", http.StatusBadRequest)
			return
		}

		responseValue, err := h.Repo.PostAPIBatch(ctx, batchRequest, h.Cfg.BaseURL, userID)
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

		if err := json.Unmarshal(bodyRaw, aliasRequest); err != nil {
			http.Error(w, "Bad request!", http.StatusBadRequest)
			return
		}

		_, err = govalidator.ValidateStruct(aliasRequest)
		if err != nil {
			http.Error(w, "Body are not contains URL value!", http.StatusBadRequest)
			return
		}

		requestValue, err := h.Repo.AddURL(r.Context(), aliasRequest.LongURLValue, userID)
		if err != nil {
			if !errors.Is(err, storage.ErrDuplicateRecord) {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			return
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

		//		fmt.Printf("userID: %v\n", userID)
		requestValue, err := h.Repo.AddURL(r.Context(), aliasRequest.LongURLValue, userID)
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
