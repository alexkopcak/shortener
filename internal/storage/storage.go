package storage

import (
	"strconv"
)

type Repositories interface {
	AddURL(string) string
	GetURL(string) string
}

type Dictionary struct {
	Items     map[string]int
	NextValue int
}

// func New() *Dictionary {
// 	return &Dictionary{
// 		Items: make(map[string]int),
// 	}
// }

func (dic *Dictionary) AddURL(value string) string {
	if dic.Items == nil {
		dic.Items = make(map[string]int)
	}
	sURLValue, ok := dic.Items[value]
	if !ok {
		sURLValue = dic.NextValue
		dic.NextValue++
		dic.Items[value] = sURLValue
	}

	return strconv.Itoa(sURLValue)
}

func (dic *Dictionary) GetURL(shortValue string) string {
	shortURLValueString, err := strconv.Atoi(shortValue)

	if err != nil {
		return ""
	}

	for key, value := range dic.Items {
		if value == shortURLValueString {
			return key
		}
	}
	return ""
}
