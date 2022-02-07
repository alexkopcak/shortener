package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

var dic dictionary

type dictionary struct {
	items map[string]string
}

func (dic *dictionary) generateShortUrl(sUrl string) string {
	hashCounter := md5.New()
	return hex.EncodeToString(hashCounter.Sum([]byte(sUrl)))
}

func (dic *dictionary) addUrl(urlValue string) string {
	sUrlValue, ok := dic.items[urlValue]
	if ok == false {
		sUrlValue = dic.generateShortUrl(urlValue)
		dic.items[urlValue] = sUrlValue
	}
	return sUrlValue
}

func (dic *dictionary) getUrl(shortUrlValue string) (string, error) {
	if shortUrlValue == "" {
		return "", errors.New("URL are empty")
	}
	for urlValue, shortValue := range dic.items {
		if shortValue == shortUrlValue {
			return urlValue, nil
		}
	}

	return "", errors.New("URL not found")
}

func processing(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		{
			if len(r.URL.Path) < 2 {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			reqUrlValue := r.URL.Path[1:]

			sUrlValue, err := dic.getUrl(reqUrlValue)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
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
	dic.items = make(map[string]string)

	writer := os.Stdout
	_, err := fmt.Fprintln(writer, server.ListenAndServe())
	if err != nil {
		fmt.Println(err.Error())
	}
}
