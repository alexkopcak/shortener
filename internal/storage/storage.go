package storage

import (
	"errors"
	"io"

	"math/rand"
	"os"
	"strings"
	"time"
)

const (
	shortURLLengthIncrementConst = 5
	minShortURLLengthConst       = 5
	attemptsGenerateCountConst   = 5
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

type (
	ItemType struct {
		ShortURLValue string `json:"shortURLValue"`
		LongURLValue  string `json:"longURLValue"`
	}

	UserExportType struct {
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
	}

	Dictionary struct {
		MinShortURLLength       int
		ShortURLLengthIncrement int
		AttemptsGenerateCount   int
		Items                   map[string]string
		UserItems               map[uint64][]string
		fileStoragePath         string
	}
)

func NewDictionary(filepath string) (*Dictionary, error) {
	items := make(map[string]string)

	_, err := os.Stat(filepath)
	if err == nil {
		consumerItem, err := NewConsumer(filepath)
		if err != nil {
			return nil, err
		}
		defer consumerItem.Close()
		for {
			item, err := consumerItem.ReadItem()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			items[item.ShortURLValue] = item.LongURLValue
		}
	}

	return &Dictionary{
		MinShortURLLength:       minShortURLLengthConst,
		ShortURLLengthIncrement: shortURLLengthIncrementConst,
		AttemptsGenerateCount:   attemptsGenerateCountConst,
		Items:                   items,
		fileStoragePath:         filepath,
	}, nil
}

func (d *Dictionary) AddURL(longURLValue string, userID uint64) (string, error) {
	if longURLValue == "" || strings.TrimSpace(longURLValue) == "" {
		return "", errors.New("empty long URL value")
	}
	for shortURLLengthIncrement := 0; shortURLLengthIncrement < d.ShortURLLengthIncrement; shortURLLengthIncrement++ {
		for attempt := 0; attempt < d.AttemptsGenerateCount; attempt++ {
			shortURLvalue := shortURLGenerator(d.MinShortURLLength + shortURLLengthIncrement)
			_, exsist := d.Items[shortURLvalue]
			if !exsist {
				d.Items[shortURLvalue] = longURLValue
				d.UserItems[userID] = append(d.UserItems[userID], shortURLvalue)

				if d.fileStoragePath != "" {
					producer, err := NewProducer(d.fileStoragePath)
					if err != nil {
						return "", err
					}
					defer producer.Close()
					err = producer.WriteItem(&ItemType{
						ShortURLValue: shortURLvalue,
						LongURLValue:  longURLValue,
					})
					if err != nil {
						return "", err
					}
				}
				return shortURLvalue, nil
			}
		}
	}
	return "", errors.New("can't add long URL to storage")
}

func (d *Dictionary) GetURL(shortURLValue string) (string, error) {
	if shortURLValue == "" ||
		strings.TrimSpace(shortURLValue) == "" ||
		d.Items == nil ||
		len(d.Items) == 0 {
		return "", errors.New("empty short URL value")
	}
	return d.Items[shortURLValue], nil
}

func (d *Dictionary) GetUserURL(prefix string, userID uint64) []UserExportType {
	result := []UserExportType{}
	for _, v := range d.UserItems[userID] {
		longURL, err := d.GetURL(v)
		item := UserExportType{}

		if prefix == "" ||
			strings.TrimSpace(prefix) == "" {
			item.ShortURL = v
		} else {
			item.ShortURL = prefix + "/" + v
		}

		if err != nil {
			item.OriginalURL = longURL
		} else {
			continue
		}
		result = append(result, item)
	}
	return result
}
