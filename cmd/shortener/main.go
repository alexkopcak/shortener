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

func (dic *dictionary) addUrl(urlValue string) string {
	sUrlValue, ok := dic.items[urlValue]
	if ok == false {
		sUrlValue = dic.nextValue
		dic.nextValue++
		dic.items[urlValue] = sUrlValue
	}
	return strconv.Itoa(sUrlValue)
}

func (dic *dictionary) getUrl(shortUrlValue string) string {
	shortUrlValueString, err := strconv.Atoi(shortUrlValue)

	if err != nil {
		return ""
	}

	for key, value := range dic.items {
		if value == shortUrlValueString {
			return key
		}
	}
	return ""
}

func processing(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		{
			reqUrlValue := r.URL.Path[len("/"):]

			sUrlValue := dic.getUrl(reqUrlValue)
			if sUrlValue == "" {
				http.Error(w, "There are no any short Urls", http.StatusBadRequest)
				return
			}

			fmt.Println("GET", reqUrlValue, sUrlValue)

			w.Header().Set("Location", sUrlValue)
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
			urlValue := dic.addUrl(bodyString)

			fmt.Println("POST", bodyString, urlValue)

			w.WriteHeader(http.StatusCreated) // 201
			w.Header().Set("Content-Type", "text/plain")
			var byteArray = []byte(urlValue)
			_, err = w.Write(byteArray)
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
