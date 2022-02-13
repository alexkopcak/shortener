package handlers

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexkopcak/shortener/internal/storage"
)

//func (r *repo) AddURL(string) string {}
//func (r *repo) GetURL(string) string {}

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
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.target, bytes.NewBuffer([]byte(tt.body)))
			//fmt.Println("!!!", request)
			w := httptest.NewRecorder()
			h := http.HandlerFunc(URLHandler(&tt.repo))
			h.ServeHTTP(w, request)

			result := w.Result()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			//assert.Equal(t, tt.want.contentType, result.Header.Get("Content-Type"))
			for _, hdr := range result.Header {
				fmt.Println("!!!", hdr)
			}
			//fmt.Println("!!!", result.Header.Get("Content-Type"))

			requestResult, err := ioutil.ReadAll(result.Body)
			require.NoError(t, err)
			err = result.Body.Close()
			require.NoError(t, err)

			assert.Equal(t, tt.want.body, string(requestResult))

			// if got := URLHandler(tt.args.repo); !reflect.DeepEqual(got, tt.want) {
			// 	t.Errorf("URLHandler() = %v, want %v", got, tt.want)
			// }
		})
	}
}
