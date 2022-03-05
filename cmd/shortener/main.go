package main

import (
	"log"

	"github.com/alexkopcak/shortener/internal/app"
)

func main() {
	log.Fatal(app.Run())
}
