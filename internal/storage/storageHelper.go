package storage

import (
	"math/rand"
	"time"
)

type (
	ItemType struct {
		ShortURLValue string `json:"shortURLValue"`
		LongURLValue  string `json:"longURLValue"`
	}

	UserExportType struct {
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
	}
)

func shortURLGenerator(n int) string {
	rand.Seed(time.Now().UnixNano())
	var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
