package app

import (
	"log"
	"net/http"

	"github.com/alexkopcak/shortener/internal/handlers"
	"github.com/alexkopcak/shortener/internal/storage"
	"github.com/caarlos0/env"
)

type config struct {
	ServerAddr      string `env:"SERVER_ADDRESS" envDefault:"localhost:8080"`
	BaseURL         string `env:"BASE_URL" envDefault:"http://localhost:8080"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
}

func Run() {
	// Env configuration
	var cfg config
	err := env.Parse(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Repository
	dictionary := storage.NewDictionary(cfg.FileStoragePath)

	//HTTP Server
	server := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: handlers.URLHandler(dictionary, cfg.BaseURL),
	}

	// start server
	log.Fatal(server.ListenAndServe())
}
