package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alexkopcak/shortener/internal/app"
	"github.com/alexkopcak/shortener/internal/config"
	"github.com/caarlos0/env"
)

// link flags
var (
	buildVersion string = "N/A" // -X 'main.buildVersion=v1.0.1'
	buildDate    string = "N/A" // -X 'main.buildDate=$(date +'%Y/%m/%d %H:%M:%S')'
	buildCommit  string = "N/A" // -X 'main.buildCommit=$(git show -s --format=%s)'
)

const (
	envConfigFile = "ENV_CONFIG_FILE"
)

func main() {
	// check config file os.Env
	cfgFileName := os.Getenv(envConfigFile)

	// env configuration
	cfg, err := config.NewConfig(cfgFileName)
	if err != nil {
		log.Fatal(err)
	}

	if err = env.Parse(&cfg); err != nil {
		log.Fatal(err)
	}
	// flags configuration
	flag.StringVar(&cfg.ServerAddr, "a", cfg.ServerAddr, "Server address, example ip:port")
	flag.StringVar(&cfg.BaseURL, "b", cfg.BaseURL, "Base URL address, example http://127.0.0.1:8080")
	flag.StringVar(&cfg.FileStoragePath, "f", cfg.FileStoragePath, "File storage path")
	flag.StringVar(&cfg.DBConnectionString, "d", cfg.DBConnectionString, "DB connection string")
	flag.BoolVar(&cfg.EnableHTTPS, "s", cfg.EnableHTTPS, "Enable HTTPS")
	flag.StringVar(&cfg.ConfigPath, "c", cfg.ConfigPath, "Config file path")
	flag.StringVar(&cfg.ConfigPath, "config", cfg.ConfigPath, "Config file path")

	flag.Parse()

	// check Base URL scheme
	if err = cfg.CheckURLvalueScheme(); err != nil {
		log.Fatal(err)
	}

	// if config file exsist, but not loaded
	if strings.TrimSpace(cfg.ConfigPath) != "" &&
		strings.TrimSpace(cfgFileName) == "" {

		name, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}

		var procAttr os.ProcAttr
		procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
		procAttr.Env = []string{fmt.Sprintf("%s=%s", envConfigFile, cfg.ConfigPath)}

		proc, err := os.StartProcess(name, os.Args, &procAttr)
		if err != nil {
			log.Fatal(err)
		}

		_, err = proc.Wait()
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	fmt.Println("Build version:", buildVersion)
	fmt.Println("Build date:", buildDate)
	fmt.Println("Build commit:", buildCommit)

	log.Fatal(app.Run(cfg))
}
