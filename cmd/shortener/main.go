package main

import (
	"fmt"
	"log"

	"github.com/alexkopcak/shortener/internal/app"
)

// link flags
var (
	buildVersion string = "N/A" // -X 'main.buildVersion=v1.0.1'
	buildDate    string = "N/A" // -X 'main.buildDate=$(date +'%Y/%m/%d %H:%M:%S')'
	buildCommit  string = "N/A" // -X 'main.buildCommit=$(git show -s --format=%s)'
)

func main() {
	fmt.Println("Build version:", buildVersion)
	fmt.Println("Build date:", buildDate)
	fmt.Println("Build commit:", buildCommit)
	log.Fatal(app.Run())
}
