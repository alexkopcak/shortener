package handlers

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexkopcak/shortener/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLHandler(t *testing.T) {
	type want struct {
		contentType string
		statusCode  int
		body        string
		location    string
	}

	tests := []struct {
		name   string
		target string
		body   string
		method string
		repo   storage.Dictionary
		want   want
	}{
		{
			name:   "post value and empty repo",
			target: "http://localhost:8080/",
			body:   "http://abc.test/abc/abd",
			method: http.MethodPost,
			repo: storage.Dictionary{
				Items: map[string]string{},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusCreated,
				body:        "http://localhost:8080/0",
				location:    "",
			},
		},
		{
			name:   "post value and repo",
			target: "http://localhost:8080/",
			body:   "http://abc2.test/",
			method: http.MethodPost,
			repo: storage.Dictionary{
				Items: map[string]string{"0": "http://abc.test/abc/abd"},
			},
			want: want{
				contentType: "text/plain; charset=utf-8",
				statusCode:  http.StatusCreated,
				body:        "http://localhost:8080/1",
				location:    "",
			},
		},
		{
			name:   "get value from repo",
			target: "http://localhost:8080/0",
			body:   "",
			method: http.MethodGet,
			repo: storage.Dictionary{
				Items: map[string]string{
					"0": "http://abc.test/abc/abd",
				},
			},
			want: want{
				contentType: "",
				statusCode:  http.StatusTemporaryRedirect,
				body:        "",
				location:    "http://abc.test/abc/abd",
			},
		},
		{
			name:   "get value from empty repo",
			target: "http://loaclhost:8080/0",
			body:   "",
			method: http.MethodGet,
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
			name:   "get with empty url",
			target: "http://localhost:8080/",
			body:   "",
			method: http.MethodGet,
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
			name:   "method not allowed",
			target: "http://localhost:8080/",
			body:   "",
			method: http.MethodConnect,
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
			name:   "method not allowed #2",
			target: "http://localhost:8080/0",
			body:   "",
			method: "abracadabra",
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
			name:   "bad URL",
			target: "http://localhost:8080/0/",
			body:   "",
			method: http.MethodGet,
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
			name:   "bad URL #2",
			target: "http://localhost:8080//",
			body:   "",
			method: http.MethodGet,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer([]byte(tt.body)))
			w := httptest.NewRecorder()
			h := http.Server{
				Handler: URLHandler(&tt.repo),
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

			if tt.method == http.MethodPost && result.StatusCode == http.StatusCreated {
				fmt.Println(string(requestResult))

				request2 := httptest.NewRequest(http.MethodGet, string(requestResult), nil)
				w2 := httptest.NewRecorder()
				h2 := http.Server{
					Handler: URLHandler(&tt.repo),
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
