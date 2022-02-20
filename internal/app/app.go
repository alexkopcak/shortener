package app

import (
	"log"
	"net/http"
	"os"

	hand "github.com/alexkopcak/shortener/internal/handlers"
	storage "github.com/alexkopcak/shortener/internal/storage"
)

const (
	serverEnvVariable  = "SERVER_ADDRESS"
	baseURLEnvVariable = "BASE_URL"
)

func Run() {
	// Repository
	var dictionary storage.Dictionary
	dictionary.Items = make(map[string]string)

	// Get Env variables
	serverAddr, exsist := os.LookupEnv(serverEnvVariable)
	if !exsist {
		log.Fatal("Unknown SERVER_ADDRESS variable!")
		return
	}
	baseURL, exsist := os.LookupEnv(baseURLEnvVariable)
	if !exsist {
		log.Fatal("Unknown BASE_URL variable!")
	}

	//HTTP Server
	server := &http.Server{
		Addr:    serverAddr,
		Handler: hand.URLHandler(&dictionary, baseURL),
	}

	// start server
	log.Fatal(server.ListenAndServe())
}
