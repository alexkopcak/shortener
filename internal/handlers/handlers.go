package handlers

import (
	"io"
	"net/http"
	"strings"
)

type Repositories interface {
	AddURL(string) string
	GetURL(string) string
}

func URLHandler(repo Repositories) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			{
				requestValue := r.URL.Path[len("/"):]
				//fmt.Println(requestValue)
				if requestValue == "" || strings.Contains(requestValue, "/") {
					http.Error(w, "Empty URL", http.StatusBadRequest)
					return
				}

				longURLValue := repo.GetURL(requestValue)
				//fmt.Println(longURLValue)
				if longURLValue == "" {
					http.Error(w, "There are no any short Urls", http.StatusBadRequest)
					return
				}

				//fmt.Println("GET", requestValue, longURLValue)

				w.Header().Set("Location", longURLValue)
				w.WriteHeader(http.StatusTemporaryRedirect) // 307
				break
			}
		case http.MethodPost:
			{
				if r.URL.Path != "/" {
					http.Error(w, "Bad request. POST allow only `/` ", http.StatusBadRequest)
					return
				}

				bodyRaw, err := io.ReadAll(r.Body)
				if err != nil || len(bodyRaw) == 0 {
					http.Error(w, "Body are not contain URL", http.StatusBadRequest)
					return
				}

				bodyString := string(bodyRaw)
				requestValue := repo.AddURL(bodyString)
				shortURLValue := "http://localhost:8080/" + requestValue

				//fmt.Println("POST", bodyString, shortURLValue)

				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusCreated) // 201
				var byteArray = []byte(shortURLValue)
				_, err = w.Write(byteArray)
				if err != nil {
					http.Error(w, "Something went wrong", http.StatusBadRequest)
					return
				}
				break
			}
		default:
			http.Error(w, "Only GET and POST methods are supported!", http.StatusBadRequest)
			return
		}
	}
}
