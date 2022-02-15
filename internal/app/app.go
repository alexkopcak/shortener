package app

import (
	"log"
	"net/http"

	hand "github.com/alexkopcak/shortener/internal/handlers"
	storage "github.com/alexkopcak/shortener/internal/storage"
)

func Run() {
	// Repository
	var dictionary storage.Dictionary
	dictionary.Items = make(map[string]string)

	//HTTP Server
	server := &http.Server{
		Addr:    "localhost:8080",
		Handler: hand.URLHandler(&dictionary),
	}

	// start server
	log.Fatal(server.ListenAndServe())
}
