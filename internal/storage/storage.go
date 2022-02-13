package storage

import (
	"strconv"
)

type Dictionary struct {
	Items     map[string]int
	NextValue int
}

func (d *Dictionary) AddURL(value string) string {
	if d.Items == nil {
		d.Items = make(map[string]int)
	}
	sURLValue, ok := d.Items[value]
	if !ok {
		sURLValue = d.NextValue
		d.NextValue++
		d.Items[value] = sURLValue
	}

	return strconv.Itoa(sURLValue)
}

func (d *Dictionary) GetURL(shortValue string) string {
	shortURLValueString, err := strconv.Atoi(shortValue)

	if err != nil {
		return ""
	}

	for key, value := range d.Items {
		if value == shortURLValueString {
			return key
		}
	}
	return ""
}
