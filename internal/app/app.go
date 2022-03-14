package app

import (
	"database/sql"
	"flag"
	"net/http"

	"github.com/alexkopcak/shortener/internal/handlers"
	"github.com/alexkopcak/shortener/internal/storage"
	"github.com/caarlos0/env"
)

func Run() error {
	// Env configuration
	var cfg handlers.Config
	err := env.Parse(&cfg)
	if err != nil {
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
	// Repository
	dictionary, err := storage.NewDictionary(cfg.FileStoragePath)
	if err != nil {
		return err
	}

	// database
	//var db *sql.DB
	if cfg.DBConnectionString != "" {
		cfg.DB, err = sql.Open("postgres", cfg.DBConnectionString)
		if err != nil {
			cfg.DB = nil
		} else {
			defer cfg.DB.Close()
		}
	}

	//HTTP Server
	server := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: handlers.URLHandler(dictionary, cfg),
	}

	// start server
	return server.ListenAndServe()
}
