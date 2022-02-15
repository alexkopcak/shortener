package storage

import (
	"math/rand"
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

type Dictionary struct {
	MinShortURLLength       int
	ShortURLLengthIncrement int
	AttemptsGenerateCount   int
	Items                   map[string]string
}

func (d *Dictionary) init() {
	if d.MinShortURLLength <= 0 {
		d.MinShortURLLength = minShortURLLengthConst
	}
	if d.ShortURLLengthIncrement <= 0 {
		d.ShortURLLengthIncrement = shortURLLengthIncrementConst
	}
	if d.AttemptsGenerateCount <= 0 {
		d.AttemptsGenerateCount = attemptsGenerateCountConst
	}
}

func (d *Dictionary) AddURL(longURLValue string) string {
	if longURLValue == "" || strings.TrimSpace(longURLValue) == "" {
		return ""
	}
	d.init()
	if d.Items == nil {
		d.Items = make(map[string]string)
	}
	for shortURLLengthIncrement := 0; shortURLLengthIncrement < d.ShortURLLengthIncrement; shortURLLengthIncrement++ {
		for attempt := 0; attempt < d.AttemptsGenerateCount; attempt++ {
			shortURLvalue := shortURLGenerator(d.MinShortURLLength + shortURLLengthIncrement)
			_, exsist := d.Items[shortURLvalue]
			if !exsist {
				d.Items[shortURLvalue] = longURLValue
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
