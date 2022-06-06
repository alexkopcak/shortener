// packet storage provides interface and it's implementation.
package storage

import (
	"context"
	"errors"
	"sync"

	"io"
	"math/rand"
	"time"

	"os"
	"strings"

	"github.com/alexkopcak/shortener/internal/config"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
)

const (
	minShortURLLengthConst = 5 // short URL length used at func shortURLGenerator
)

// Custom error implementation.
var (
	ErrDuplicateRecord = errors.New("record are duplicate") // record already exists
	ErrNotExistRecord  = errors.New("record not exist")     // record not exists
)

// type represents a structure from an array of ShortURLValues to be removed and User ID.
type DeletedShortURLValues struct {
	ShortURLValues []string
	UserIDValue    int32
}

// generates a random string value consisting of n characters.
func shortURLGenerator(n int) string {
	rand.Seed(time.Now().UnixNano())
	var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// type Storage represents storage interface.
type Storage interface {
	AddURL(ctx context.Context, longURLValue string, userID int32) (string, error)
	GetURL(ctx context.Context, shortURLValue string) (string, error)
	GetUserURL(ctx context.Context, prefix string, userID int32) ([]UserExportType, error)
	PostAPIBatch(ctx context.Context, shortURLArray *BatchRequestArray, prefix string, userID int32) (*BatchResponseArray, error)
	Ping(ctx context.Context) error
	DeleteUserURL(ctx context.Context, deletedURL *DeletedShortURLValues) error
}

// implements the choice of storage depending on the configuration, returns the storage interface.
func InitializeStorage(cfg config.Config, wg *sync.WaitGroup, dChannel chan *DeletedShortURLValues) (Storage, error) {
	if strings.TrimSpace(cfg.DBConnectionString) == "" {
		return NewDictionary(cfg)
	}
	return NewPostgresStorage(cfg, wg, dChannel)
}

// postgres storage implenetation.
type PostgresStorage struct {
	db            *pgx.Conn
	WaitGroup     *sync.WaitGroup
	DeleteChannel chan *DeletedShortURLValues
}

// creates a new postgres storage object.
func NewPostgresStorage(cfg config.Config, wg *sync.WaitGroup, dChannel chan *DeletedShortURLValues) (Storage, error) {
	ps, err := pgx.Connect(context.Background(), cfg.DBConnectionString)
	if err != nil {
		return NewDictionary(cfg)
	}

	var cnt int
	err = ps.QueryRow(context.Background(), "SELECT COUNT(*) FROM pg_database WHERE datname = 'shortener_db';").Scan(&cnt)

	if cnt != 1 || err != nil {
		_, err = ps.Exec(context.Background(), "CREATE DATABASE shortener_db OWNER postgres;")
		if err != nil {
			return NewDictionary(cfg)
		}
	}

	_, err = ps.Exec(context.Background(), "SELECT * FROM shortener LIMIT 1;")
	if err != nil {
		_, err = ps.Exec(context.Background(), "CREATE TABLE shortener (user_id INTEGER, short_url VARCHAR(5), original_url VARCHAR(255), deleted_at TIMESTAMP, UNIQUE(user_id, original_url));")
		if err != nil {
			return NewDictionary(cfg)
		}
	}

	pstorage := &PostgresStorage{
		db:            ps,
		WaitGroup:     wg,
		DeleteChannel: dChannel,
	}

	pstorage.StartDeleteWorker()

	return pstorage, nil
}

// adds original URL value to DB postgres, the function returns a short URL value.
func (ps *PostgresStorage) AddURL(ctx context.Context, longURLValue string, userID int32) (string, error) {
	if strings.TrimSpace(longURLValue) == "" {
		return "", errors.New("empty long URL value")
	}

	shortURLvalue := shortURLGenerator(minShortURLLengthConst)
	cTag, err := ps.db.Exec(ctx,
		"INSERT INTO shortener "+
			"(user_id, short_url, original_url) "+
			"VALUES ($1, $2, $3) "+
			"ON CONFLICT (user_id, original_url) DO NOTHING;",
		userID,
		shortURLvalue,
		longURLValue)
	if err != nil {
		return "", err
	}

	if cTag.RowsAffected() == 0 {
		var shortURL string
		err = ps.db.QueryRow(ctx,
			"SELECT short_url "+
				"FROM shortener "+
				"WHERE user_id = $1 AND original_url = $2 ;",
			userID, longURLValue).Scan(&shortURL)

		if err != nil {
			return "", err
		}
		shortURLvalue = shortURL
		return shortURLvalue, ErrDuplicateRecord
	}
	return shortURLvalue, nil
}

// get original URL value by a short value from the postgres DB.
func (ps *PostgresStorage) GetURL(ctx context.Context, shortURLValue string) (string, error) {
	var longURL string
	var deletedAt *time.Time

	err := ps.db.QueryRow(ctx,
		"SELECT original_url, deleted_at "+
			"FROM shortener "+
			"WHERE short_url = $1 ;",
		shortURLValue).Scan(&longURL, &deletedAt)
	if err != nil {
		return "", err
	}

	if deletedAt != nil {
		return longURL, ErrNotExistRecord
	} else {
		return longURL, nil
	}
}

// get short URL value and original URL value pairs array created by user (userID)
func (ps *PostgresStorage) GetUserURL(ctx context.Context, prefix string, userID int32) ([]UserExportType, error) {
	result := []UserExportType{}
	rows, err := ps.db.Query(ctx,
		"SELECT short_url, original_url "+
			"FROM shortener "+
			"WHERE user_id = $1 ;",
		userID)
	if err != nil {
		return result, err
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
	return result, nil
}

// group addition of short URL values to the database postgres via api.
//
// items - array of BatchRequest
// prefix - shortener service name
// userID - user ID
func (ps *PostgresStorage) PostAPIBatch(
	ctx context.Context,
	items *BatchRequestArray,
	prefix string,
	userID int32) (*BatchResponseArray, error) {
	result := &BatchResponseArray{}
	if ps.db == nil {
		return result, errors.New("db is nil")
	}

	tx, err := ps.db.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)

	for _, v := range *items {
		batchResponseItem := BatchResponse{}
		batchResponseItem.CorrelationID = v.CorrelationID
		shortURLValue := shortURLGenerator(minShortURLLengthConst)

		batchResponseItem.ShortURL = shortURLValue
		if strings.TrimSpace(prefix) != "" {
			batchResponseItem.ShortURL = prefix + "/" + shortURLValue
		}

		_, err := tx.Exec(ctx,
			"INSERT INTO shortener (user_id, short_url, original_url) VALUES ($1, $2, $3);",
			userID,
			shortURLValue,
			v.OriginalURL,
		)
		if err != nil {
			return &BatchResponseArray{}, err
		}
		*result = append(*result, batchResponseItem)
	}
	err = tx.Commit(ctx)
	if err != nil {
		return &BatchResponseArray{}, err
	}
	return result, nil
}

// simple test database postgres connection.
func (ps *PostgresStorage) Ping(ctx context.Context) error {
	err := ps.db.Ping(ctx)
	if err != nil {
		return err
	}
	return err
}

// the function launches three delete workers.
func (ps *PostgresStorage) StartDeleteWorker() {
	workerCount := 3

	for i := 0; i < workerCount; i++ {
		ps.WaitGroup.Add(1)
		go ps.DeleteWorker()
	}
}

// worker is listening to the DeleteChannel.
// when a value is received through the channel, worker starts DeleteUserURL func.
func (ps *PostgresStorage) DeleteWorker() {
	defer ps.WaitGroup.Done()

	for job := range ps.DeleteChannel {
		ps.DeleteUserURL(context.Background(), job)
	}
}

// sends an sql query to the postgres repository.
// for entries corresponding to deleted URLs in the
// postgres database, the deleted_at value is set to the current time.
func (ps *PostgresStorage) DeleteUserURL(ctx context.Context, deletedURLs *DeletedShortURLValues) error {
	idsArray := &pgtype.TextArray{}
	err := idsArray.Set(deletedURLs.ShortURLValues)

	if err != nil {
		return err
	}

	_, err = ps.db.Exec(ctx, "UPDATE shortener SET deleted_at = now() WHERE user_id = $1 and short_url = ANY($2);", deletedURLs.UserIDValue, idsArray)
	return err
}

// memory storage implementation.
type Dictionary struct {
	Items           map[string]string
	UserItems       map[int32][]string
	fileStoragePath string
}

// create a new memory storage object.
func NewDictionary(cfg config.Config) (Storage, error) {
	items := make(map[string]string)
	userItems := make(map[int32][]string)

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

// add original URL value to memory storage.
func (d *Dictionary) AddURL(ctx context.Context, longURLValue string, userID int32) (string, error) {
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

// get original URL value by a short value from memory storage.
func (d *Dictionary) GetURL(ctx context.Context, shortURLValue string) (string, error) {
	if strings.TrimSpace(shortURLValue) == "" {
		return "", errors.New("empty short URL value")
	}
	return d.Items[shortURLValue], nil
}

// get short URL value and original URL value pairs array created by user.
func (d *Dictionary) GetUserURL(ctx context.Context, prefix string, userID int32) ([]UserExportType, error) {
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
	return result, nil
}

// group addition of short URL values to the memory storage via api.
//
// items - array of BatchRequest
// prefix - shortener service name
// userID - user ID
func (d *Dictionary) PostAPIBatch(ctx context.Context, items *BatchRequestArray, prefix string, userID int32) (*BatchResponseArray, error) {
	result := &BatchResponseArray{}
	for _, v := range *items {
		batchResponseItem := BatchResponse{}
		batchResponseItem.CorrelationID = v.CorrelationID
		shortURLValue := shortURLGenerator(minShortURLLengthConst)

		batchResponseItem.ShortURL = shortURLValue
		if strings.TrimSpace(prefix) != "" {
			batchResponseItem.ShortURL = prefix + "/" + shortURLValue
		}

		d.Items[shortURLValue] = v.OriginalURL
		d.UserItems[userID] = append(d.UserItems[userID], shortURLValue)

		if err := ProducerWrite(d.fileStoragePath, &ItemType{
			ShortURLValue: shortURLValue,
			LongURLValue:  v.OriginalURL,
		}); err != nil {
			return nil, err
		}
		*result = append(*result, batchResponseItem)
	}
	return result, nil
}

// interface plug
func (d *Dictionary) Ping(ctx context.Context) error {
	return nil
}

// delete user URLs from memory storage
func (d *Dictionary) DeleteUserURL(ctx context.Context, deletedURLs *DeletedShortURLValues) error {
	if deletedURLs == nil {
		return nil
	}

	index := func(item string, ar []string) int {
		for id, val := range ar {
			if val == item {
				return id
			}
		}
		return -1
	}

	for _, item := range deletedURLs.ShortURLValues {
		delete(d.Items, item)

		a := d.UserItems[deletedURLs.UserIDValue]
		i := index(item, a)
		if i == -1 {
			continue
		}

		a[i] = a[len(a)-1]
		a[len(a)-1] = ""
		d.UserItems[deletedURLs.UserIDValue] = a[:len(a)-1]
	}
	return nil
}

// linked list storage item.
type URLItem struct {
	ShortURLValue    string
	OriginalURLValue string
	Next             *URLItem
}

// linked list storage implementation.
type LinkedListURLItem struct {
	Head *URLItem
	Tail *URLItem
}

// multiuser linked list storage implementation.
type UsersLinkedListMemoryStorage struct {
	LinkedListStorage map[int32]*LinkedListURLItem
}

// create a new linked list storage implementation.
func NewLinkedListStorage() Storage {
	lls := make(map[int32]*LinkedListURLItem)
	return UsersLinkedListMemoryStorage{
		LinkedListStorage: lls,
	}
}

// add original URL value to linked list storage.
func (l UsersLinkedListMemoryStorage) AddURL(ctx context.Context, longURLValue string, userID int32) (string, error) {
	if strings.TrimSpace(longURLValue) == "" {
		return "", errors.New("empty long URL value")
	}

	shortURL := shortURLGenerator(minShortURLLengthConst)
	u := &URLItem{
		ShortURLValue:    shortURL,
		OriginalURLValue: longURLValue,
		Next:             nil,
	}

	item := l.LinkedListStorage[userID]
	if item == nil {
		item = &LinkedListURLItem{}
	}

	if item.Head == nil {
		item.Head = u
	} else {
		currentNode := item.Tail
		currentNode.Next = u
	}
	item.Tail = u

	l.LinkedListStorage[userID] = item

	return shortURL, nil
}

// get original URL value by a short value from linked list storage.
func (l UsersLinkedListMemoryStorage) GetURL(ctx context.Context, shortURLValue string) (string, error) {
	if strings.TrimSpace(shortURLValue) == "" {
		return "", errors.New("empty short URL value")
	}

	for _, v := range l.LinkedListStorage {
		currentNode := v.Head
		if currentNode.ShortURLValue == shortURLValue {
			return currentNode.OriginalURLValue, nil
		}
		if currentNode == nil {
			continue
		}
		for currentNode.Next != nil {
			if currentNode.ShortURLValue == shortURLValue {
				return currentNode.OriginalURLValue, nil
			}
			currentNode = currentNode.Next
		}
	}
	return "", nil
}

// get short URL value and original URL value pairs array created by user.
func (l UsersLinkedListMemoryStorage) GetUserURL(ctx context.Context, prefix string, userID int32) ([]UserExportType, error) {
	items := l.LinkedListStorage[userID]

	result := []UserExportType{}
	if items == nil || items.Head == nil {
		return result, nil
	}

	currntItem := items.Head
	for currntItem != nil {
		item := &UserExportType{
			ShortURL:    currntItem.ShortURLValue,
			OriginalURL: currntItem.OriginalURLValue,
		}

		if strings.TrimSpace(prefix) != "" {
			item.ShortURL = prefix + "/" + item.ShortURL
		}
		result = append(result, *item)

		currntItem = currntItem.Next
	}

	return result, nil
}

// group addition of short URL values to the linked list storage via api.
//
// items - array of BatchRequest
// prefix - shortener service name
// userID - user ID
func (l UsersLinkedListMemoryStorage) PostAPIBatch(ctx context.Context, items *BatchRequestArray, prefix string, userID int32) (*BatchResponseArray, error) {
	list := l.LinkedListStorage[userID]
	if list == nil {
		list = &LinkedListURLItem{}
	}

	result := &BatchResponseArray{}
	for _, v := range *items {
		shortURL := shortURLGenerator(minShortURLLengthConst)
		batchResponseItem := BatchResponse{
			CorrelationID: v.CorrelationID,
			ShortURL:      shortURL,
		}
		if strings.TrimSpace(prefix) != "" {
			batchResponseItem.ShortURL = prefix + "/" + shortURL
		}

		item := &URLItem{
			ShortURLValue:    shortURL,
			OriginalURLValue: v.OriginalURL,
			Next:             nil,
		}

		if list.Head == nil {
			list.Head = item
		} else {
			currentNode := list.Tail
			currentNode.Next = item
		}
		list.Tail = item
		*result = append(*result, batchResponseItem)
	}

	l.LinkedListStorage[userID] = list
	return result, nil
}

// interface plug
func (l UsersLinkedListMemoryStorage) Ping(ctx context.Context) error {
	return nil
}

// delete user URLs from linked list storage
func (l UsersLinkedListMemoryStorage) DeleteUserURL(ctx context.Context, deletedURLs *DeletedShortURLValues) error {
	userID := deletedURLs.UserIDValue
	list := l.LinkedListStorage[userID]

	if list == nil || list.Head == nil {
		return nil
	}

	for _, deletedShortURL := range deletedURLs.ShortURLValues {
		currentItem := list.Head
		if currentItem.ShortURLValue == deletedShortURL {
			if currentItem.Next == nil {
				list.Head = nil
				list.Tail = nil
				break
			}
			list.Head = currentItem.Next
			continue
		}
		for currentItem.Next != nil {
			if currentItem.Next.ShortURLValue == deletedShortURL {
				if currentItem.Next.Next != nil {
					tempItem := currentItem.Next.Next
					currentItem.Next = tempItem
				} else {
					currentItem.Next = nil
					list.Tail = currentItem
				}
				break
			}

			currentItem = currentItem.Next
		}
	}

	l.LinkedListStorage[userID] = list

	return nil
}
