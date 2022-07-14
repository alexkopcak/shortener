package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/alexkopcak/shortener/internal/config"
	"github.com/alexkopcak/shortener/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	baseURL        = "http://localhost:8080"
	secretKey      = "secretKey"
	cookieAuthName = "id"
)

func TestURLHandler(t *testing.T) {

	type want struct {
		contentType string
		body        string
		location    string
		statusCode  int
	}

	tests := []struct {
		name     string
		target   string
		template string
		body     string
		method   string
		repo     storage.Dictionary
		want     want
	}{
		{
			name:     "post value and empty repo",
			target:   baseURL,
			template: "%s",
			body:     "http://abc.test/abc/abd",
			method:   http.MethodPost,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusCreated,
				body:        baseURL + "0",
				location:    "",
			},
		},
		{
			name:     "post value and repo",
			target:   baseURL,
			template: "%s",
			body:     "http://abc2.test/",
			method:   http.MethodPost,
			repo: storage.Dictionary{
				Items: map[string]string{"0": "http://abc.test/abc/abd"},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusCreated,
				body:        baseURL + "/1",
				location:    "",
			},
		},
		{
			name:     "get value from empty repo",
			target:   baseURL + "/0",
			template: "%s",
			body:     "",
			method:   http.MethodGet,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "There are no any short Urls\n",
				location:    "",
			},
		},
		{
			name:     "get with empty url",
			target:   baseURL,
			template: "%s",
			body:     "",
			method:   http.MethodGet,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Empty URL\n",
				location:    "",
			},
		},
		{
			name:     "method not allowed",
			target:   baseURL,
			template: "%s",
			body:     "",
			method:   http.MethodConnect,
			repo: storage.Dictionary{
				Items: map[string]string{
					"0": "http://abc.test/abc/abd",
				},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Bad request!\n",
				location:    "",
			},
		},
		{
			name:     "method not allowed #2",
			target:   baseURL + "/0",
			template: "%s",
			body:     "",
			method:   "abracadabra",
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Only GET and POST methods are supported!\n",
				location:    "",
			},
		},
		{
			name:     "bad URL",
			target:   baseURL + "/0/",
			template: "%s",
			body:     "",
			method:   http.MethodGet,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Bad request!\n",
				location:    "",
			},
		},
		{
			name:     "bad URL #2",
			target:   baseURL + "//",
			template: "%s",
			body:     "",
			method:   http.MethodGet,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Bad request!\n",
				location:    "",
			},
		},
		{
			name:     "body are not contains URL value",
			target:   baseURL,
			template: "%s",
			body:     "123",
			method:   http.MethodPost,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Body are not contains URL value!\n",
				location:    "",
			},
		},
		{
			name:     "body are not contains URL value #2",
			target:   baseURL + "/api/shorten",
			template: "{\"url\": \"%s\"}",
			body:     "123",
			method:   http.MethodPost,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Body are not contains URL value!\n",
				location:    "",
			},
		},
		{
			name:     "bad json",
			target:   baseURL + "/api/shorten",
			template: "%s",
			body:     "123",
			method:   http.MethodPost,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Bad request!\n",
				location:    "",
			},
		},
		{
			name:     "bad json #2",
			target:   baseURL + "/api/shorten",
			template: "{\"url\": %s}",
			body:     "http://abc/test",
			method:   http.MethodPost,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Bad request!\n",
				location:    "",
			},
		},
		{
			name:     "bad json #3",
			target:   baseURL + "/api/shorten",
			template: "{\"url\": \"%s}",
			body:     "http://abc/test",
			method:   http.MethodPost,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Bad request!\n",
				location:    "",
			},
		},
		{
			name:     "api post value and empty repo",
			target:   baseURL + "/api/shorten",
			template: "{\"url\": \"%s\"}",
			body:     "http://abc/test",
			method:   http.MethodPost,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "application/json",
				statusCode:  http.StatusCreated,
				body:        "",
				location:    "",
			},
		},
		{
			name:     "api post value and empty body",
			target:   baseURL + "/api/shorten",
			template: "{\"url\": \"%s\"}",
			body:     "",
			method:   http.MethodPost,
			repo: storage.Dictionary{
				Items:     map[string]string{},
				UserItems: map[int32][]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Body are not contain URL!",
				location:    "",
			},
		},
		{
			name:     "api post value and empty body and template",
			target:   baseURL + "/api/shorten",
			template: "",
			body:     "",
			method:   http.MethodPost,
			repo: storage.Dictionary{
				Items:     map[string]string{},
				UserItems: map[int32][]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "Body are not contain URL!",
				location:    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request *http.Request
			if tt.template == "" {
				request = httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer([]byte("")))
			} else {
				request = httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer([]byte(fmt.Sprintf(tt.template, tt.body))))
			}
			w := httptest.NewRecorder()

			dChan := make(chan *storage.DeletedShortURLValues)
			defer close(dChan)

			d, err := storage.NewDictionary(config.Config{}, &sync.WaitGroup{}, dChan)
			require.NoError(t, err)

			if len(tt.repo.Items) > 0 {
				for _, v := range tt.repo.Items {
					d.AddURL(request.Context(), v, storage.ShortURLGenerator(), 0)
				}
			}

			h := http.Server{
				Handler: NewURLHandler(d, config.Config{
					BaseURL:        baseURL,
					SecretKey:      secretKey,
					CookieAuthName: cookieAuthName,
				}, dChan),
			}
			h.Handler.ServeHTTP(w, request)
			result := w.Result()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			assert.Equal(t, tt.want.location, result.Header.Get("Location"))
			assert.Equal(t, tt.want.contentType, result.Header.Get("Content-Type"))

			requestResult, err := ioutil.ReadAll(result.Body)
			require.NoError(t, err)
			err = result.Body.Close()
			require.NoError(t, err)

			if tt.method == http.MethodGet && result.StatusCode == http.StatusTemporaryRedirect {
				assert.Equal(t, tt.want.body, string(requestResult))
			}

			if tt.method == http.MethodPost && result.StatusCode == http.StatusCreated {
				if strings.Contains(request.RequestURI, "/api/shorten") {
					aliasRequest := &struct {
						LongURLValue string `json:"result,omitempty"`
					}{}
					err = json.Unmarshal(requestResult, aliasRequest)
					require.NoError(t, err)
					requestResult = []byte(aliasRequest.LongURLValue)
				}

				request2 := httptest.NewRequest(http.MethodGet, string(requestResult), nil)
				w2 := httptest.NewRecorder()
				dChan := make(chan *storage.DeletedShortURLValues)
				defer close(dChan)

				h2 := http.Server{
					Handler: NewURLHandler(d, config.Config{
						BaseURL:        baseURL,
						SecretKey:      secretKey,
						CookieAuthName: cookieAuthName,
					}, dChan),
				}
				h2.Handler.ServeHTTP(w2, request2)
				result2 := w2.Result()
				assert.Equal(t, tt.body, result2.Header.Get("Location"))
				err = result2.Body.Close()
				require.NoError(t, err)
			}
		})
	}
}

func TestCookie(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		template string
		body     string
		method   string
		cookie   string
	}{
		{
			name:     "not empty cookie",
			target:   baseURL,
			template: "%s",
			body:     "http://abc.test/abc/abd",
			method:   http.MethodPost,
			cookie:   "",
		},
		{
			name:     "bad cookie value",
			target:   baseURL,
			template: "%s",
			body:     "http://abc.test/abc/abd",
			method:   http.MethodPost,
			cookie:   "id=id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer([]byte(fmt.Sprintf(tt.template, tt.body))))
			if tt.cookie != "" {
				request.AddCookie(&http.Cookie{Name: "id", Value: tt.cookie})
			}
			w := httptest.NewRecorder()
			dChan := make(chan *storage.DeletedShortURLValues)
			defer close(dChan)

			d, err := storage.NewDictionary(config.Config{}, &sync.WaitGroup{}, dChan)
			require.NoError(t, err)
			h := http.Server{
				Handler: NewURLHandler(d, config.Config{
					BaseURL:        baseURL,
					SecretKey:      secretKey,
					CookieAuthName: cookieAuthName,
				}, dChan),
			}
			h.Handler.ServeHTTP(w, request)
			result := w.Result()
			defer result.Body.Close()
			require.NotEmpty(t, result.Cookies(), "cookies field are empty")

			cookie := result.Cookies()

			request2 := httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer([]byte(fmt.Sprintf(tt.template, tt.body))))
			for _, v := range cookie {
				fmt.Printf("%v\n", v)
				request2.AddCookie(v)
			}

			h.Handler.ServeHTTP(w, request2)
			result = w.Result()

			require.EqualValues(t, cookie, result.Cookies())

			result.Body.Close()
		},
		)
	}
}

func TestHandler_DeleteUserURLHandler(t *testing.T) {
	type want struct {
		body       string
		statusCode int
	}

	tests := []struct {
		name     string
		target   string
		template string
		body     string
		method   string
		repo     storage.Dictionary
		want     want
	}{
		{
			name:     "delete User URL Handler",
			target:   baseURL + "/api/user/urls",
			template: "[\"short URL1\",\"short URL2\"]",
			method:   http.MethodDelete,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				statusCode: http.StatusAccepted,
				body:       "",
			},
		},
		{
			name:     "delete User URL Handler, bad body",
			target:   baseURL + "/api/user/urls",
			template: "[\"short URL1\",\"short URL2\"",
			method:   http.MethodDelete,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       "unexpected EOF\n",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer([]byte(tt.template)))
			w := httptest.NewRecorder()
			dChan := make(chan *storage.DeletedShortURLValues)
			defer close(dChan)
			d, err := storage.NewDictionary(config.Config{}, &sync.WaitGroup{}, dChan)
			require.NoError(t, err)

			h := http.Server{
				Handler: NewURLHandler(d, config.Config{
					BaseURL:        baseURL,
					SecretKey:      secretKey,
					CookieAuthName: cookieAuthName,
				}, dChan),
			}

			h.Handler.ServeHTTP(w, request)

			result := w.Result()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			requestResult, err := ioutil.ReadAll(result.Body)
			require.NoError(t, err)

			assert.Equal(t, tt.want.body, string(requestResult))
			err = result.Body.Close()
			require.NoError(t, err)
		})
	}
}

func TestHandler_Ping(t *testing.T) {
	type want struct {
		statusCode int
	}

	tests := []struct {
		name     string
		target   string
		template string
		method   string
		repo     storage.Dictionary
		want     want
	}{
		{
			name:     "ping test",
			target:   baseURL + "/ping",
			template: "/",
			method:   http.MethodGet,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				statusCode: http.StatusOK,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer([]byte(tt.template)))
			w := httptest.NewRecorder()
			dChan := make(chan *storage.DeletedShortURLValues)

			d, err := storage.NewDictionary(config.Config{}, &sync.WaitGroup{}, dChan)
			require.NoError(t, err)

			h := http.Server{
				Handler: NewURLHandler(d, config.Config{
					BaseURL:        baseURL,
					SecretKey:      secretKey,
					CookieAuthName: cookieAuthName,
				}, dChan),
			}

			h.Handler.ServeHTTP(w, request)

			close(dChan)

			result := w.Result()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			err = result.Body.Close()
			require.NoError(t, err)
		})
	}
}

func TestHandler_GetAPIAllURLHandler(t *testing.T) {
	type want struct {
		contentType string
		statusCode  int
	}

	tests := []struct {
		name        string
		target      string
		method      string
		originalURL string
		repo        storage.Dictionary
		want        want
	}{
		{
			name:        "get values from empty storage",
			target:      baseURL + "/api/user/urls",
			method:      http.MethodGet,
			originalURL: "",
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusNoContent,
			},
		},
		{
			name:        "get values from storage",
			target:      baseURL + "/api/user/urls",
			method:      http.MethodGet,
			originalURL: "http://original.url",
			repo: storage.Dictionary{
				Items:     map[string]string{},
				UserItems: map[int32][]string{},
			},
			want: want{
				contentType: "application/json",
				statusCode:  http.StatusOK,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addreq *http.Request
			var addw *httptest.ResponseRecorder
			if tt.originalURL != "" {
				addreq = httptest.NewRequest(http.MethodPost, baseURL, bytes.NewBuffer([]byte(tt.originalURL)))
				addw = httptest.NewRecorder()
			}

			request := httptest.NewRequest(tt.method, tt.target, nil)
			w := httptest.NewRecorder()

			dChan := make(chan *storage.DeletedShortURLValues)
			defer close(dChan)

			h := http.Server{
				Handler: NewURLHandler(&tt.repo, config.Config{
					BaseURL:        baseURL,
					SecretKey:      secretKey,
					CookieAuthName: cookieAuthName,
				}, dChan),
			}

			if tt.originalURL != "" {
				h.Handler.ServeHTTP(addw, addreq)
				cooks := addw.Result()
				for _, v := range cooks.Cookies() {
					request.AddCookie(v)
				}
				cooks.Body.Close()
			}

			h.Handler.ServeHTTP(w, request)
			result := w.Result()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			result.Body.Close()
		})
	}
}

func TestHandler_PostAPIBatchHandler(t *testing.T) {
	type want struct {
		contentType string
		body        string
		statusCode  int
	}

	tests := []struct {
		name   string
		target string
		method string
		repo   storage.Dictionary
		item   *storage.BatchRequestArray
		want   want
	}{
		{
			name:   "set batch values with api",
			target: baseURL + "/api/shorten/batch",
			method: http.MethodPost,
			repo:   storage.Dictionary{},
			item: &storage.BatchRequestArray{
				storage.BatchRequest{
					CorrelationID: "1",
					OriginalURL:   "http:\\test.tst",
				},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusCreated,
				body:        "[]",
			},
		},
		{
			name:   "set batch values with api empty body",
			target: baseURL + "/api/shorten/batch",
			method: http.MethodPost,
			repo:   storage.Dictionary{},
			item:   nil,
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusBadRequest,
				body:        "[]",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.item != nil {
				var err error
				body, err = json.Marshal(tt.item)
				require.NoError(t, err)
			} else {
				body = nil
			}

			request := httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			dChan := make(chan *storage.DeletedShortURLValues)
			defer close(dChan)
			cfg := config.Config{
				BaseURL:        baseURL,
				SecretKey:      secretKey,
				CookieAuthName: cookieAuthName,
			}

			d, err := storage.NewDictionary(cfg, &sync.WaitGroup{}, dChan)
			require.NoError(t, err)

			h := http.Server{
				Handler: NewURLHandler(d, cfg, dChan),
			}

			h.Handler.ServeHTTP(w, request)

			result := w.Result()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			require.NoError(t, err)

			err = result.Body.Close()
			require.NoError(t, err)

		})
	}
}

func TestHandler_GetInternalStats(t *testing.T) {
	type want struct {
		contentType string
		statusCode  int
	}

	tests := []struct {
		name          string
		realipRequest string
		trustedNet    string
		want          want
	}{
		{
			name:          "no x-real-ip",
			realipRequest: "",
			trustedNet:    "10.0.0.1/24",
			want: want{
				contentType: "",
				statusCode:  403,
			},
		},
		{
			name:          "no trusted network",
			realipRequest: "10.0.0.1",
			trustedNet:    "",
			want: want{
				contentType: "",
				statusCode:  403,
			},
		},
		{
			name:          "trusted network with ip",
			realipRequest: "10.0.0.1",
			trustedNet:    "10.0.0.0/8",
			want: want{
				contentType: "application/json",
				statusCode:  200,
			},
		},
		{
			name:          "bad trusted network with ip",
			realipRequest: "10.0.0.1",
			trustedNet:    "10.0.0.0",
			want: want{
				contentType: "",
				statusCode:  403,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, baseURL+"/api/internal/stats", nil)
			request.Header.Add("X-Real-IP", tt.realipRequest)
			w := httptest.NewRecorder()

			dChan := make(chan *storage.DeletedShortURLValues)
			defer close(dChan)

			cfg := config.Config{
				BaseURL:        baseURL,
				SecretKey:      secretKey,
				CookieAuthName: cookieAuthName,
				TrustedSubnet:  tt.trustedNet,
			}

			d, err := storage.NewDictionary(cfg, &sync.WaitGroup{}, dChan)
			require.NoError(t, err)

			h := http.Server{
				Handler: NewURLHandler(d, cfg, dChan),
			}

			h.Handler.ServeHTTP(w, request)

			result := w.Result()
			result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			assert.Equal(t, tt.want.contentType, result.Header.Get("Content-Type"))
		})
	}
}

func TestMiddleware_gzip(t *testing.T) {
	type want struct {
		contentType string
		statusCode  int
	}

	tests := []struct {
		name            string
		acceptEncoding  string
		contentEncoding string
		body            []byte
		want            want
	}{
		{
			name:            "bad gzip",
			contentEncoding: "gzip",
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  400,
			},
		},
		{
			name:           "accept gzip",
			acceptEncoding: "gzip",
			want: want{
				contentType: "",
				statusCode:  200,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, baseURL+"/ping", nil)
			request.Header.Add("Content-Encoding", tt.contentEncoding)
			request.Header.Add("Accept-Encoding", tt.acceptEncoding)
			w := httptest.NewRecorder()

			dChan := make(chan *storage.DeletedShortURLValues)
			defer close(dChan)

			cfg := config.Config{
				BaseURL:        baseURL,
				SecretKey:      secretKey,
				CookieAuthName: cookieAuthName,
			}

			d, err := storage.NewDictionary(cfg, &sync.WaitGroup{}, dChan)
			require.NoError(t, err)

			h := http.Server{
				Handler: NewURLHandler(d, cfg, dChan),
			}

			h.Handler.ServeHTTP(w, request)

			result := w.Result()
			result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			assert.Equal(t, tt.want.contentType, result.Header.Get("Content-Type"))
		})
	}

}
