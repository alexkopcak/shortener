package storage

import (
	"encoding/json"
	"os"
)

type consumer struct {
	file    *os.File
	decoder *json.Decoder
}

func NewConsumer(filename string) (*consumer, error) {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0400)
	if err != nil {
		return nil, err
	}

	return &consumer{
		file:    file,
		decoder: json.NewDecoder(file),
	}, nil
}

func (c *consumer) ReadItem() (*ItemType, error) {
	item := &ItemType{}
	if err := c.decoder.Decode(&item); err != nil {
		return nil, err
	}
	return item, nil
}

func (c *consumer) Close() error {
	return c.file.Close()
}
