package storage

import (
	"encoding/json"
	"os"
)

type producer struct {
	file    *os.File
	encoder *json.Encoder
}

func NewProducer(filename string) (*producer, error) {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &producer{
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

func ProducerWrite(filename string, item *ItemType) error {
	if filename == "" {
		return nil
	}
	producer, err := NewProducer(filename)
	if err != nil {
		return err
	}
	defer producer.Close()
	err = producer.WriteItem(item)
	if err != nil {
		return err
	}
	return nil
}

func (p *producer) WriteItem(item *ItemType) error {
	return p.encoder.Encode(&item)
}

func (p *producer) Close() error {
	return p.file.Close()
}
