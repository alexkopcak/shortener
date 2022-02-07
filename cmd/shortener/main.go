package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

var dic dictionary

type dictionary struct {
	items     map[string]int
	nextValue int
}

func (dic *dictionary) addURL(value string) string {
	sURLValue, ok := dic.items[value]
	if ok {
		sURLValue = dic.nextValue
		dic.nextValue++
		dic.items[value] = sURLValue
	}
	return strconv.Itoa(sURLValue)
}

func (dic *dictionary) getURL(shortValue string) string {
	shortURLValueString, err := strconv.Atoi(shortValue)

	if err != nil {
		return ""
	}

	for key, value := range dic.items {
		if value == shortURLValueString {
			return key
		}
	}
	return ""
}

func processing(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		{
			requestValue := r.URL.Path[len("/"):]

			shortURLValue := dic.getURL(requestValue)
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
			requestValue := dic.addURL(bodyString)

			fmt.Println("POST", bodyString, requestValue)

			w.WriteHeader(http.StatusCreated) // 201
			w.Header().Set("Content-Type", "text/plain")
			var byteArray = []byte(requestValue)
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

func main() {
	http.HandleFunc("/", processing)
	server := &http.Server{
		Addr: "localhost:8080",
	}
	dic.items = make(map[string]int)
	dic.nextValue = 0

	writer := os.Stdout
	_, err := fmt.Fprintln(writer, server.ListenAndServe())
	if err != nil {
		fmt.Println(err.Error())
	}
}
