package handlers

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testDictionary struct {
	Items     map[string]int
	NextValue int
}

func (d *testDictionary) AddURL(value string) string {
	if d.Items == nil {
		d.Items = make(map[string]int)
	}
	sURLValue, ok := d.Items[value]
	if !ok {
		sURLValue = d.NextValue
		d.NextValue++
		d.Items[value] = sURLValue
	}

	return strconv.Itoa(sURLValue)
}

func (d *testDictionary) GetURL(shortValue string) string {
	shortURLValueString, err := strconv.Atoi(shortValue)

	if err != nil {
		return ""
	}

	for key, value := range d.Items {
		if value == shortURLValueString {
			return key
		}
	}
	return ""
}

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
		repo   testDictionary
		want   want
	}{
		{
			name:   "post value and empty repo",
			target: "http://localhost:8080/",
			body:   "http://abc.test/abc/abd",
			method: http.MethodPost,
			repo: testDictionary{
				Items:     map[string]int{},
				NextValue: 0,
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
			repo: testDictionary{
				Items:     map[string]int{"http://abc.test/abc/abd": 0},
				NextValue: 1,
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
			repo: testDictionary{
				Items:     map[string]int{"http://abc.test/abc/abd": 0},
				NextValue: 1,
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
			repo: testDictionary{
				Items:     map[string]int{},
				NextValue: 0,
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
			repo: testDictionary{
				Items:     map[string]int{},
				NextValue: 0,
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
			repo: testDictionary{
				Items:     map[string]int{"http://abc.test/abc/abd": 0},
				NextValue: 1,
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
			repo: testDictionary{
				Items:     map[string]int{"http://abc.test/abc/abd": 0},
				NextValue: 1,
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
			target: "http://localhost:8080//",
			body:   "",
			method: http.MethodGet,
			repo: testDictionary{
				Items:     map[string]int{"http://abc.test/abc/abd": 0},
				NextValue: 1,
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

			assert.Equal(t, tt.want.body, string(requestResult))

		})
	}
}
