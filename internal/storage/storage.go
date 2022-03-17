package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"os"
	"strings"

	"github.com/alexkopcak/shortener/internal/config"
	"github.com/jackc/pgx/v4"
)

const (
	minShortURLLengthConst = 5
)

type Storage interface {
	AddURL(ctx context.Context, longURLValue string, userID int) (string, error)
	GetURL(ctx context.Context, shortURLValue string) (string, error)
	GetUserURL(ctx context.Context, prefix string, userID int) []UserExportType
	Ping(ctx context.Context) error
}

func InitializeStorage(cfg config.Config) (Storage, error) {
	fmt.Println("initialize storage")
	if strings.TrimSpace(cfg.DBConnectionString) == "" {
		return NewDictionary(cfg)
	} else {
		fmt.Println("use db")
		return NewPostgresStorage(cfg)
	}
}

type PostgresStorage struct {
	db *pgx.Conn
}

func NewPostgresStorage(cfg config.Config) (Storage, error) {
	ps, err := pgx.Connect(context.Background(), cfg.DBConnectionString)
	fmt.Println("connect to db")
	if err != nil {
		fmt.Println("error", err.Error())
		return NewDictionary(cfg)
	}
	// defer ps.Close(context.Background())

	var cnt int
	err = ps.QueryRow(context.Background(), "SELECT COUNT(*) FROM pg_database WHERE datname = 'shortener_db';").Scan(&cnt)

	if cnt != 1 || err != nil {
		fmt.Println("create db")
		_, err = ps.Exec(context.Background(), "CREATE DATABASE shortener_db OWNER postgres;")
		if err != nil {
			fmt.Println("can't create db")
			fmt.Printf("%v\n", err)
			return NewDictionary(cfg)
		}
	}

	_, err = ps.Exec(context.Background(), "SELECT * FROM shortener LIMIT 1;")
	fmt.Println("table exsist?")
	if err != nil {
		fmt.Printf("%v\n", err)
		fmt.Println("create table")
		_, err = ps.Exec(context.Background(), "CREATE TABLE shortener (user_id INTEGER, short_url VARCHAR(5), original_url VARCHAR(255));")
		if err != nil {
			fmt.Println("create table error", err.Error())
			fmt.Printf("%v", err)
			return NewDictionary(cfg)
		}
	}

	return &PostgresStorage{
		db: ps,
	}, nil
}

func (ps *PostgresStorage) AddURL(ctx context.Context, longURLValue string, userID int) (string, error) {
	if strings.TrimSpace(longURLValue) == "" {
		return "", errors.New("empty long URL value")
	}

	shortURLvalue := shortURLGenerator(minShortURLLengthConst)
	_, err := ps.db.Exec(ctx, "INSERT INTO shortener (user_id, short_url, original_url) VALUES ($1, $2, $3)", userID, shortURLvalue, longURLValue)
	if err != nil {
		fmt.Printf("%v\n", err)
		return "", err
	}
	return shortURLvalue, nil
}

func (ps *PostgresStorage) GetURL(ctx context.Context, shortURLValue string) (string, error) {
	var longURL string
	err := ps.db.QueryRow(ctx, "SELECT original_url FROM shortener WHERE short_url = $1", shortURLValue).Scan(&longURL)
	if err != nil {
		return "", err
	}
	return longURL, nil
}

func (ps *PostgresStorage) GetUserURL(ctx context.Context, prefix string, userID int) []UserExportType {
	result := []UserExportType{}
	rows, err := ps.db.Query(ctx, "SELECT short_url, original_url FROM shortener WHERE user_id = $1", userID)
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

func (ps *PostgresStorage) Ping(ctx context.Context) error {
	err := ps.db.Ping(ctx)
	if err != nil {
		fmt.Printf("%v\n", err)
		return err
	}
	return err
}

type Dictionary struct {
	Items           map[string]string
	UserItems       map[int][]string
	fileStoragePath string
}

func NewDictionary(cfg config.Config) (Storage, error) {
	items := make(map[string]string)
	userItems := make(map[int][]string)

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

func (d *Dictionary) AddURL(ctx context.Context, longURLValue string, userID int) (string, error) {
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

func (d *Dictionary) GetURL(ctx context.Context, shortURLValue string) (string, error) {
	if strings.TrimSpace(shortURLValue) == "" {
		return "", errors.New("empty short URL value")
	}
	return d.Items[shortURLValue], nil
}

func (d *Dictionary) GetUserURL(ctx context.Context, prefix string, userID int) []UserExportType {
	result := []UserExportType{}
	for _, v := range d.UserItems[userID] {
		longURL, err := d.GetURL(ctx, v)

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

func (d *Dictionary) Ping(ctx context.Context) error {
	return nil
}
