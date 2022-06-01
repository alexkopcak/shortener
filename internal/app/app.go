package app

import (
	"flag"
	"net/http"
	"sync"

	"github.com/alexkopcak/shortener/internal/config"
	"github.com/alexkopcak/shortener/internal/handlers"
	"github.com/alexkopcak/shortener/internal/storage"
	"github.com/caarlos0/env"
)

func Run() error {
	// Env configuration
	var cfg config.Config
	if err := env.Parse(&cfg); err != nil {
		return err
	}
	// flags configuration
	addrPointer := flag.String("a", cfg.ServerAddr, "Server address, example ip:port")
	baseAddrPointer := flag.String("b", cfg.BaseURL, "Base URL address, example http://127.0.0.1:8080")
	fileStoragePathPointer := flag.String("f", cfg.FileStoragePath, "File storage path")
	dbConnectionString := flag.String("d", cfg.DBConnectionString, "DB Connection string")
	flag.Parse()

	cfg.BaseURL = *baseAddrPointer
	cfg.ServerAddr = *addrPointer
	cfg.FileStoragePath = *fileStoragePathPointer
	cfg.DBConnectionString = *dbConnectionString

	wg := &sync.WaitGroup{}
	dChannel := make(chan *storage.DeletedShortURLValues)

	// Repository
	repository, err := storage.InitializeStorage(cfg, wg, dChannel)
	if err != nil {
		return err
	}

	//HTTP Server
	server := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: handlers.URLHandler(repository, cfg, dChannel),
	}

	// start server
	err = server.ListenAndServe()

	close(dChannel)
	wg.Wait()

	return err
}
