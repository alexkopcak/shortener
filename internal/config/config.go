package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	ServerAddr         string `json:"server_address" env:"SERVER_ADDRESS"`
	BaseURL            string `json:"base_url" env:"BASE_URL"`
	FileStoragePath    string `json:"file_storage_path" env:"FILE_STORAGE_PATH"`
	SecretKey          string `json:"-" env:"SHORTENER_SECRET_KEY"`
	CookieAuthName     string `json:"-" env:"COOKIE_ATUH_NAME"`
	DBConnectionString string `json:"database_dsn" env:"DATABASE_DSN"`
	ConfigPath         string `json:"-" env:"CONFIG"`
	EnableHTTPS        bool   `json:"enable_https" env:"ENABLE_HTTPS"`
}

func (c *Config) SetDefaultValues() {
	c.ServerAddr = "localhost:8080"
	c.BaseURL = "http://localhost:8080"
	c.SecretKey = "We learn Go language"
	c.CookieAuthName = "shortenerId"
	c.DBConnectionString = "postgres://postgres:mypassword@localhost:5432/shortener_db"
	c.EnableHTTPS = false
}

func NewConfig(configFileName string) Config {
	cfg := Config{}
	cfg.SetDefaultValues()

	configFileStat, err := os.Stat(configFileName)
	if err != nil {
		return cfg
	}

	if !configFileStat.Mode().IsRegular() {
		return cfg
	}

	file, err := os.Open(configFileName)
	if err != nil {
		return cfg
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cfg)
	if err != nil {
		return cfg
	}
	return cfg
}
