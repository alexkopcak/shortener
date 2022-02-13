package app

import (
	"fmt"
	"net/http"
	"os"

	hand "github.com/alexkopcak/shortener/internal/handlers"
	storage "github.com/alexkopcak/shortener/internal/storage"
)

func Run() {
	// Repository
	var dic storage.Dictionary
	dic.Items = make(map[string]int)
	dic.NextValue = 0

	//HTTP Server
	writer := os.Stdout

	server := &http.Server{
		Addr: "localhost:8080",
	}
	h := new(hand.Handler)
	h.Store = &dic
	http.Handle("/", h)

	_, err := fmt.Fprintln(writer, server.ListenAndServe())
	if err != nil {
		fmt.Println(err.Error())
	}

}
