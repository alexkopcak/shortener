package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	storage "github.com/alexkopcak/shortener/internal/storage"
)

type Handler struct {
	Store *storage.Dictionary
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//func Processing(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintf(os.Stdout, "%v", dic)
	//fmt.Println(r)
	switch r.Method {
	case http.MethodGet:
		{
			requestValue := r.URL.Path[len("/"):]
			fmt.Println(requestValue)
			if requestValue == "" || strings.Contains(requestValue, "/") {
				http.Error(w, "Empty URL", http.StatusBadRequest)
				return
			}

			shortURLValue := h.Store.GetURL(requestValue)
			fmt.Println(shortURLValue)
			if shortURLValue == "" {
				http.Error(w, "There are no any short Urls", http.StatusBadRequest)
				return
			}

			fmt.Println("GET", requestValue, shortURLValue)

			w.Header().Set("Location", shortURLValue)
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
			requestValue := h.Store.AddURL(bodyString)
			shortURL := "http://localhost:8080/" + requestValue

			fmt.Println("POST", bodyString, shortURL)

			w.WriteHeader(http.StatusCreated) // 201
			var byteArray = []byte(shortURL)
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
