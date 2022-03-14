package storage

import (
	"context"
	"errors"
	"io"
	"time"

	"os"
	"strings"

	"github.com/alexkopcak/shortener/internal/config"
	"github.com/jackc/pgx/v4"
)

const (
	minShortURLLengthConst = 5
)

type Storage interface {
	AddURL(longURLValue string, userID uint64) (string, error)
	GetURL(shortURLValue string) (string, error)
	GetUserURL(prefix string, userID uint64) []UserExportType
	Ping() error
}

func InitializeStorage(cfg config.Config) (Storage, error) {
	if strings.TrimSpace(cfg.DBConnectionString) == "" {
		return NewPostgresStorage(cfg)
	} else {
		return NewDictionary(cfg)
	}
}

type PostgresStorage struct {
	db *pgx.Conn
}

func NewPostgresStorage(cfg config.Config) (Storage, error) {
	ps, err := pgx.Connect(context.Background(), cfg.DBConnectionString)
	if err != nil {
		return NewDictionary(cfg)
	}
	defer ps.Close(context.Background())

	_, err = ps.Exec(context.Background(), "CREATE TABLE IF NOT EXSISTS shortener (user_id BIGINT short_url VARCHAR(5) original_url VARCHAR(255));")
	if err != nil {
		return NewDictionary(cfg)
	}

	return &PostgresStorage{
		db: ps,
	}, nil
}

func (ps *PostgresStorage) AddURL(longURLValue string, userID uint64) (string, error) {
	if strings.TrimSpace(longURLValue) == "" {
		return "", errors.New("empty long URL value")
	}

	shortURLvalue := shortURLGenerator(minShortURLLengthConst)
	_, err := ps.db.Exec(context.Background(), "INSERT INTO shortener (user_id, short_url, original_url) VALUES ($1, $2, $3)", userID, shortURLvalue, longURLValue)
	if err != nil {
		return "", err
	}
	return shortURLvalue, nil
}

func (ps *PostgresStorage) GetURL(shortURLValue string) (string, error) {
	var longURL string
	err := ps.db.QueryRow(context.Background(), "SELECT original_url FROM shortener WHERE short_url = $1", shortURLValue).Scan(&longURL)
	if err != nil {
		return "", err
	}
	return longURL, nil
}

func (ps *PostgresStorage) GetUserURL(prefix string, userID uint64) []UserExportType {
	result := []UserExportType{}
	rows, err := ps.db.Query(context.Background(), "SELECT short_url, original_url FROM shortener WHERE user_id = $1", userID)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		item := UserExportType{}
		var value string
		err := rows.Scan(&value, &item.OriginalURL)
		if err != nil {
			continue
		}

		if strings.TrimSpace(prefix) == "" {
			item.ShortURL = value
		} else {
			item.ShortURL = prefix + "/" + value
		}
		result = append(result, item)
	}
	return result
}

func (ps *PostgresStorage) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := ps.db.Ping(ctx)
	if err != nil {
		return err
	}
	return err
}

type Dictionary struct {
	//MinShortURLLength int
	Items           map[string]string
	UserItems       map[uint64][]string
	fileStoragePath string
}

func NewDictionary(cfg config.Config) (Storage, error) {
	items := make(map[string]string)
	userItems := make(map[uint64][]string)

	_, err := os.Stat(cfg.FileStoragePath)
	if err == nil {
		consumerItem, err := NewConsumer(cfg.FileStoragePath)
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
		Items:           items,
		UserItems:       userItems,
		fileStoragePath: cfg.FileStoragePath,
	}, nil
}

func (d *Dictionary) AddURL(longURLValue string, userID uint64) (string, error) {
	if strings.TrimSpace(longURLValue) == "" {
		return "", errors.New("empty long URL value")
	}

	shortURLvalue := shortURLGenerator(minShortURLLengthConst)
	d.Items[shortURLvalue] = longURLValue
	d.UserItems[userID] = append(d.UserItems[userID], shortURLvalue)

	if err := ProducerWrite(d.fileStoragePath, &ItemType{
		ShortURLValue: shortURLvalue,
		LongURLValue:  longURLValue,
	}); err != nil {
		return "", err
	}

	return shortURLvalue, nil
}

func (d *Dictionary) GetURL(shortURLValue string) (string, error) {
	if strings.TrimSpace(shortURLValue) == "" {
		return "", errors.New("empty short URL value")
	}
	return d.Items[shortURLValue], nil
}

func (d *Dictionary) GetUserURL(prefix string, userID uint64) []UserExportType {
	result := []UserExportType{}
	for _, v := range d.UserItems[userID] {
		longURL, err := d.GetURL(v)

		item := UserExportType{}
		if err != nil {
			continue
		} else {
			item.OriginalURL = longURL
		}

		if prefix == "" ||
			strings.TrimSpace(prefix) == "" {
			item.ShortURL = v
		} else {
			item.ShortURL = prefix + "/" + v
		}
		result = append(result, item)
	}
	return result
}

func (d *Dictionary) Ping() error {
	return nil
}
