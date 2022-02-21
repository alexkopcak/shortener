package storage

import (
	"io"
	"log"
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

type ItemType struct {
	ShortURLValue string `json:"shortURLValue"`
	LongURLValue  string `json:"longURLValue"`
}

type Dictionary struct {
	MinShortURLLength       int
	ShortURLLengthIncrement int
	AttemptsGenerateCount   int
	Items                   map[string]string
	fileStoragePath         string
}

func NewDictionary(filepath string) *Dictionary {
	items := make(map[string]string)

	_, err := os.Stat(filepath)
	if err == nil {
		consumerItem, err := NewConsumer(filepath)
		if err != nil {
			log.Fatal(err.Error())
		}
		defer consumerItem.Close()
		for {
			item, err := consumerItem.ReadItem()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err.Error())
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
	}
}

func (d *Dictionary) AddURL(longURLValue string) string {
	if longURLValue == "" || strings.TrimSpace(longURLValue) == "" {
		return ""
	}
	for shortURLLengthIncrement := 0; shortURLLengthIncrement < d.ShortURLLengthIncrement; shortURLLengthIncrement++ {
		for attempt := 0; attempt < d.AttemptsGenerateCount; attempt++ {
			shortURLvalue := shortURLGenerator(d.MinShortURLLength + shortURLLengthIncrement)
			_, exsist := d.Items[shortURLvalue]
			if !exsist {
				d.Items[shortURLvalue] = longURLValue

				if d.fileStoragePath != "" {
					producer, err := NewProducer(d.fileStoragePath)
					if err != nil {
						log.Fatal(err)
					}
					defer producer.Close()
					err = producer.WriteItem(&ItemType{
						ShortURLValue: shortURLvalue,
						LongURLValue:  longURLValue,
					})
					if err != nil {
						log.Fatal(err)
					}
				}
				return shortURLvalue
			}
		}
	}
	return ""
}

func (d *Dictionary) GetURL(shortURLValue string) string {
	if shortURLValue == "" ||
		strings.TrimSpace(shortURLValue) == "" ||
		d.Items == nil ||
		len(d.Items) == 0 {
		return ""
	}
	return d.Items[shortURLValue]
}
