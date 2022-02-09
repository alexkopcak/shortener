package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var dic dictionary

type dictionary struct {
	items     map[string]int
	nextValue int
}

func (dic *dictionary) addURL(value string) string {
	sURLValue, ok := dic.items[value]
	if !ok {
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

			shortURLValue := dic.getURL(requestValue)
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
			requestValue := dic.addURL(bodyString)
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

func main() {
	writer := os.Stdout
	dic.items = make(map[string]int)
	dic.nextValue = 0

	server := &http.Server{
		Addr: "localhost:8080",
	}
	http.HandleFunc("/", processing)

	_, err := fmt.Fprintln(writer, server.ListenAndServe())
	if err != nil {
		fmt.Println(err.Error())
	}
}
