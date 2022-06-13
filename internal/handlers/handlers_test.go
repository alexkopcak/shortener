package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer([]byte(fmt.Sprintf(tt.template, tt.body))))
			w := httptest.NewRecorder()
			d, err := storage.NewDictionary(config.Config{})
			require.NoError(t, err)

			if len(tt.repo.Items) > 0 {
				for _, v := range tt.repo.Items {
					d.AddURL(request.Context(), v, 0)
				}
			}
			dChan := make(chan *storage.DeletedShortURLValues)

			h := http.Server{
				Handler: URLHandler(d, config.Config{
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

				h2 := http.Server{
					Handler: URLHandler(d, config.Config{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer([]byte(fmt.Sprintf(tt.template, tt.body))))
			w := httptest.NewRecorder()
			d, err := storage.NewDictionary(config.Config{})
			dChan := make(chan *storage.DeletedShortURLValues)
			require.NoError(t, err)
			h := http.Server{
				Handler: URLHandler(d, config.Config{
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
			d, err := storage.NewDictionary(config.Config{})
			require.NoError(t, err)

			dChan := make(chan *storage.DeletedShortURLValues, 1)

			h := http.Server{
				Handler: URLHandler(d, config.Config{
					BaseURL:        baseURL,
					SecretKey:      secretKey,
					CookieAuthName: cookieAuthName,
				}, dChan),
			}

			h.Handler.ServeHTTP(w, request)

			close(dChan)

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
