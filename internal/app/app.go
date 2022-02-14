package app

import (
	"fmt"
	"net/http"
	"os"

	hand "github.com/alexkopcak/shortener/internal/handlers"
	storage "github.com/alexkopcak/shortener/internal/storage"
)

func Run() {
	// Repository
	var dictionary storage.Dictionary
	dictionary.Items = make(map[string]int)
	dictionary.NextValue = 0

	//HTTP Server
	server := &http.Server{
		Addr:    "localhost:8080",
		Handler: hand.URLHandler(&dictionary),
	}

	// start server
	writer := os.Stdout
	_, err := fmt.Fprintln(writer, server.ListenAndServe())
	if err != nil {
		fmt.Println(err.Error())
	}

}
