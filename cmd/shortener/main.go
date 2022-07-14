package main

import (
	"fmt"
	"log"

	"github.com/alexkopcak/shortener/internal/app"
	"github.com/alexkopcak/shortener/internal/config"
)

// link flags
var (
	buildVersion string = "N/A" // -X 'main.buildVersion=v1.0.1'
	buildDate    string = "N/A" // -X 'main.buildDate=$(date +'%Y/%m/%d %H:%M:%S')'
	buildCommit  string = "N/A" // -X 'main.buildCommit=$(git show -s --format=%s)'
)

func main() {
	// new configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	// link flags
	fmt.Println("Build version:", buildVersion)
	fmt.Println("Build date:", buildDate)
	fmt.Println("Build commit:", buildCommit)

	if err = app.NewApp(cfg).Run(); err != nil {
		log.Fatal(err)
	}
}
