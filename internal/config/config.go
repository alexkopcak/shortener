package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/caarlos0/env"
)

type Config struct {
	ServerAddr         string `json:"server_address" env:"SERVER_ADDRESS"`
	BaseURL            string `json:"base_url" env:"BASE_URL"`
	FileStoragePath    string `json:"file_storage_path" env:"FILE_STORAGE_PATH"`
	SecretKey          string `json:"-" env:"SHORTENER_SECRET_KEY"`
	CookieAuthName     string `json:"-" env:"COOKIE_ATUH_NAME"`
	DBConnectionString string `json:"database_dsn" env:"DATABASE_DSN"`
	ConfigPath         string `json:"-" env:"CONFIG"`
	TrustedSubnet      string `json:"trusted_subnet" env:"TRUSTED_SUBNET"`
	EnableHTTPS        bool   `json:"enable_https" env:"ENABLE_HTTPS"`
}

const (
	envConfigFile = "ENV_CONFIG_FILE"
)

func (c *Config) SetDefaultValues() {
	c.ServerAddr = "localhost:8080"
	c.BaseURL = "http://localhost:8080"
	c.SecretKey = "We learn Go language"
	c.CookieAuthName = "shortenerId"
	c.DBConnectionString = "postgres://postgres:mypassword@localhost:5432/shortener_db"
	c.EnableHTTPS = false
	c.TrustedSubnet = ""
}

func NewConfig() (Config, error) {
	cfg := Config{}
	cfg.SetDefaultValues()

	// get configuration from config file
	cfg, err := GetConfigurationFromFile(cfg)
	if err != nil {
		return cfg, err
	}

	// get configuration from env variables
	if err = env.Parse(&cfg); err != nil {
		return cfg, err
	}

	// flags configuration
	cfg.GetFlagConfiguration()

	// check Base URL scheme
	if err = cfg.CheckURLvalueScheme(); err != nil {
		return cfg, err
	}

	if err = cfg.ConfigFileExsistButNotLoaded(); err != nil {
		return cfg, err
	}

	return cfg, err
}

func GetConfigurationFromFile(cfg Config) (Config, error) {
	// check config file os.Env
	cfgFileName := os.Getenv(envConfigFile)

	configFileStat, err := os.Stat(cfgFileName)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}

	if !configFileStat.Mode().IsRegular() {
		return cfg, nil
	}

	file, err := os.Open(cfgFileName)
	if err != nil {
		return cfg, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cfg)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) CheckURLvalueScheme() error {
	// Parse Base URL address
	urlValue, err := url.Parse(c.BaseURL)
	if err != nil {
		return err
	}

	if c.EnableHTTPS {
		urlValue.Scheme = "https"
	} else {
		urlValue.Scheme = "http"
	}
	c.BaseURL = urlValue.String()

	return nil
}

func (c *Config) GetFlagConfiguration() {
	// flags configuration
	flag.StringVar(&c.ServerAddr, "a", c.ServerAddr, "Server address, example ip:port")
	flag.StringVar(&c.BaseURL, "b", c.BaseURL, "Base URL address, example http://127.0.0.1:8080")
	flag.StringVar(&c.FileStoragePath, "f", c.FileStoragePath, "File storage path")
	flag.StringVar(&c.DBConnectionString, "d", c.DBConnectionString, "DB connection string")
	flag.BoolVar(&c.EnableHTTPS, "s", c.EnableHTTPS, "Enable HTTPS")
	flag.StringVar(&c.ConfigPath, "c", c.ConfigPath, "Config file path")
	flag.StringVar(&c.ConfigPath, "config", c.ConfigPath, "Config file path")
	flag.StringVar(&c.TrustedSubnet, "t", c.TrustedSubnet, "Trusted subnet CIDR notation")

	flag.Parse()
}

func (c *Config) ConfigFileExsistButNotLoaded() error {
	// if config file exsist, but not loaded
	if strings.TrimSpace(c.ConfigPath) != "" && strings.TrimSpace(os.Getenv(envConfigFile)) == "" {
		var name string
		name, err := os.Executable()
		if err != nil {
			return err
		}

		var procAttr os.ProcAttr
		procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
		procAttr.Env = []string{fmt.Sprintf("%s=%s", envConfigFile, c.ConfigPath)}

		var proc *os.Process
		proc, err = os.StartProcess(name, os.Args, &procAttr)
		if err != nil {
			return err
		}

		_, err = proc.Wait()
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}
