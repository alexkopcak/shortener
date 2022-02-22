package app

import (
	"flag"
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
	// flags configuration
	addrPointer := flag.String("a", cfg.ServerAddr, "Server address, example ip:port")
	baseAddrPointer := flag.String("b", cfg.BaseURL, "Base URL address, example http://127.0.0.1:8080")
	fileStoragePathPointer := flag.String("f", cfg.FileStoragePath, "File storage path")
	flag.Parse()

	cfg.BaseURL = *baseAddrPointer
	cfg.ServerAddr = *addrPointer
	cfg.FileStoragePath = *fileStoragePathPointer

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
