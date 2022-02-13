package handlers

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexkopcak/shortener/internal/storage"
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
			name:   "append value at empty repo",
			target: "http://localhost:8080/",
			body:   "http://abc.test/abc/abd",
			method: http.MethodPost,
			repo: storage.Dictionary{
				Items:     map[string]int{},
				NextValue: 0,
			},
			want: want{
				contentType: "text/plain",
				statusCode:  http.StatusCreated,
				body:        "http://localhost:8080/0",
				location:    "",
			},
		},
		{
			name:   "get value from repo",
			target: "http://localhost:8080/0",
			body:   "",
			method: http.MethodGet,
			repo: storage.Dictionary{
				Items:     map[string]int{"http://abc.test/abc/abd": 0},
				NextValue: 1,
			},
			want: want{
				statusCode: http.StatusTemporaryRedirect,
				body:       "",
				location:   "http://abc.test/abc/abd",
			},
		},
		{
			name:   "get value from empty repo.",
			target: "http://loaclhost:8080/0",
			body:   "",
			method: http.MethodGet,
			repo: storage.Dictionary{
				Items:     map[string]int{},
				NextValue: 0,
			},
			want: want{
				statusCode: 400,
				body:       "There are no any short Urls\n",
				location:   "",
			},
		},
		{
			name:   "empty url test",
			target: "http://localhost:8080/",
			body:   "",
			method: http.MethodGet,
			repo: storage.Dictionary{
				Items:     map[string]int{},
				NextValue: 0,
			},
			want: want{
				statusCode: 400,
				body:       "Empty URL\n",
				location:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer([]byte(tt.body)))
			w := httptest.NewRecorder()
			h := http.HandlerFunc(URLHandler(&tt.repo))
			h.ServeHTTP(w, request)

			result := w.Result()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			assert.Equal(t, tt.want.location, result.Header.Get("Location"))

			requestResult, err := ioutil.ReadAll(result.Body)
			require.NoError(t, err)
			err = result.Body.Close()
			require.NoError(t, err)

			assert.Equal(t, tt.want.body, string(requestResult))

		})
	}
}
