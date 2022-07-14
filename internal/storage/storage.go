// packet storage provides interface and it's implementation.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"io"
	"math/rand"
	"time"

	"os"
	"strings"

	"github.com/alexkopcak/shortener/internal/config"
	"github.com/jackc/pgtype"
	_ "github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/stdlib"
)

const (
	minShortURLLengthConst = 5 // short URL length used at func shortURLGenerator
)

// Custom error implementation.
var (
	ErrDuplicateRecord = errors.New("record are duplicate") // record already exists
	ErrNotExistRecord  = errors.New("record not exist")     // record not exists
)

// type DeletedShortURLValues represents a structure from an array of ShortURLValues to be removed and User ID.
type DeletedShortURLValues struct {
	ShortURLValues []string
	UserIDValue    int32
}

// func ShortURLGenerator generates a random string value consisting of n characters.
func ShortURLGenerator() string {
	rand.Seed(time.Now().UnixNano())
	var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, minShortURLLengthConst)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// type Storage represents storage interface.
type Storage interface {
	AddURL(ctx context.Context, longURLValue string, shortURLValue string, userID int32) (string, error)
	GetURL(ctx context.Context, shortURLValue string) (string, error)
	GetUserURL(ctx context.Context, prefix string, userID int32) ([]UserExportType, error)
	GetInternalStats(ctx context.Context) (InternalStats, error)
	PostAPIBatch(ctx context.Context, shortURLArray *BatchRequestArray, prefix string, userID int32) (*BatchResponseArray, error)
	Ping(ctx context.Context) error
	DeleteUserURL(ctx context.Context, deletedURL *DeletedShortURLValues) error
	Close() error
}

// func InitializeStorage implements the choice of storage depending on the configuration, returns the storage interface.
func InitializeStorage(cfg config.Config, wg *sync.WaitGroup, dChannel chan *DeletedShortURLValues) (Storage, error) {
	if strings.TrimSpace(cfg.DBConnectionString) == "" {
		return NewDictionary(cfg, wg, dChannel)
	}
	return NewPostgresStorage(cfg, wg, dChannel)
}

// type PostgresStorage - postgres storage implenetation.
type PostgresStorage struct {
	db            *sql.DB
	WaitGroup     *sync.WaitGroup
	DeleteChannel chan *DeletedShortURLValues
}

// func NewPostgresStorage creates a new postgres storage object.
func NewPostgresStorage(cfg config.Config, wg *sync.WaitGroup, dChannel chan *DeletedShortURLValues) (Storage, error) {
	ps, err := sql.Open("pgx", cfg.DBConnectionString)
	if err != nil {
		return NewDictionary(cfg, wg, dChannel)
	}

	var cnt int
	err = ps.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM pg_database WHERE datname = 'shortener_db';").Scan(&cnt)

	if cnt != 1 || err != nil {
		_, err = ps.ExecContext(context.Background(), "CREATE DATABASE shortener_db OWNER postgres;")
		if err != nil {
			return NewDictionary(cfg, wg, dChannel)
		}
	}

	_, err = ps.ExecContext(context.Background(), "SELECT * FROM shortener LIMIT 1;")
	if err != nil {
		_, err = ps.ExecContext(context.Background(), "CREATE TABLE shortener (user_id INTEGER, short_url VARCHAR(5), original_url VARCHAR(255), deleted_at TIMESTAMP, UNIQUE(user_id, original_url));")
		if err != nil {
			return NewDictionary(cfg, wg, dChannel)
		}
	}

	pstorage := &PostgresStorage{
		db:            ps,
		WaitGroup:     wg,
		DeleteChannel: dChannel,
	}

	pstorage.startDeleteWorker()

	return pstorage, nil
}

// func AddURL adds original URL value to DB postgres, the function returns a short URL value.
func (ps *PostgresStorage) AddURL(ctx context.Context, longURLValue string, shortURLValue string, userID int32) (string, error) {
	if strings.TrimSpace(longURLValue) == "" {
		return "", errors.New("empty long URL value")
	}

	cTag, err := ps.db.ExecContext(ctx,
		"INSERT INTO shortener "+
			"(user_id, short_url, original_url) "+
			"VALUES ($1, $2, $3) "+
			"ON CONFLICT (user_id, original_url) DO NOTHING;",
		userID,
		shortURLValue,
		longURLValue)
	if err != nil {
		return "", err
	}

	cnt, err := cTag.RowsAffected()
	if err != nil {
		return "", err
	}
	if cnt == 0 {
		var shortURL string
		err = ps.db.QueryRowContext(ctx,
			"SELECT short_url "+
				"FROM shortener "+
				"WHERE user_id = $1 AND original_url = $2 ;",
			userID, longURLValue).Scan(&shortURL)

		if err != nil {
			return "", err
		}
		return shortURL, ErrDuplicateRecord
	}
	return shortURLValue, nil
}

// func GetURL get original URL value by a short value from the postgres DB.
func (ps *PostgresStorage) GetURL(ctx context.Context, shortURLValue string) (string, error) {
	var longURL string
	var deletedAt *time.Time

	err := ps.db.QueryRowContext(ctx,
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

// func GetUserURL get short URL value and original URL value pairs array created by user (userID)
func (ps *PostgresStorage) GetUserURL(ctx context.Context, prefix string, userID int32) ([]UserExportType, error) {
	result := []UserExportType{}
	rows, err := ps.db.QueryContext(ctx, "SELECT short_url, original_url FROM shortener WHERE user_id = $1 ;", userID)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	if err = rows.Err(); err != nil {
		return result, err
	}

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

// func PostAPIBatch group addition of short URL values to the database postgres via api.
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

	tx, err := ps.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()

	for _, v := range *items {
		batchResponseItem := BatchResponse{}
		batchResponseItem.CorrelationID = v.CorrelationID

		batchResponseItem.ShortURL = v.ShortURL
		if strings.TrimSpace(prefix) != "" {
			batchResponseItem.ShortURL = prefix + "/" + v.ShortURL
		}

		_, err = tx.ExecContext(ctx,
			"INSERT INTO shortener (user_id, short_url, original_url) VALUES ($1, $2, $3);",
			userID,
			v.ShortURL,
			v.OriginalURL,
		)
		if err != nil {
			return &BatchResponseArray{}, err
		}
		*result = append(*result, batchResponseItem)
	}
	err = tx.Commit()
	if err != nil {
		return &BatchResponseArray{}, err
	}
	return result, nil
}

// func Ping simple test database postgres connection.
func (ps *PostgresStorage) Ping(ctx context.Context) error {
	err := ps.db.PingContext(ctx)
	if err != nil {
		return err
	}
	return err
}

// func StartDeleteWorker launches three delete workers.
func (ps *PostgresStorage) startDeleteWorker() {
	workerCount := 3

	for i := 0; i < workerCount; i++ {
		ps.WaitGroup.Add(1)
		go ps.deleteWorker()
	}
}

// func DeleteWorker is a listened the DeleteChannel worker.
// when a value is received through the channel, worker starts DeleteUserURL func.
func (ps *PostgresStorage) deleteWorker() {
	defer ps.WaitGroup.Done()

	for job := range ps.DeleteChannel {
		ps.DeleteUserURL(context.Background(), job)
	}
}

// func DeleteUserURL sends an sql query to the postgres repository.
// for entries corresponding to deleted URLs in the
// postgres database, the deleted_at value is set to the current time.
func (ps *PostgresStorage) DeleteUserURL(ctx context.Context, deletedURLs *DeletedShortURLValues) error {
	idsArray := &pgtype.TextArray{}
	err := idsArray.Set(deletedURLs.ShortURLValues)

	if err != nil {
		return err
	}

	_, err = ps.db.ExecContext(ctx, "UPDATE shortener SET deleted_at = now() WHERE user_id = $1 and short_url = ANY($2);", deletedURLs.UserIDValue, idsArray)
	return err
}

// func Close close postgres connection.
func (ps *PostgresStorage) Close() error {
	return ps.db.Close()
}

// func GetInternalStats counts the number of URLs and the number of users in the service.
func (ps *PostgresStorage) GetInternalStats(ctx context.Context) (InternalStats, error) {
	internalStats := InternalStats{}
	err := ps.db.QueryRowContext(ctx,
		"SELECT user_nested.user_cnt, url_nested.url_cnt "+
			"FROM "+
			"(SELECT COUNT(DISTINCT user_id) AS user_cnt FROM shortener WHERE deleted_at IS NULL) AS user_nested, "+
			"(SELECT COUNT(*) AS url_cnt FROM shortener WHERE deleted_at IS NULL) AS url_nested;").
		Scan(&internalStats.Users, &internalStats.URLs)

	return internalStats, err
}

// type Dictionary - memory storage implementation.
type Dictionary struct {
	WaitGroup     *sync.WaitGroup
	DeleteChannel chan *DeletedShortURLValues

	Items           map[string]string
	UserItems       map[int32][]string
	fileStoragePath string
}

// func NewDictionary create a new memory storage object.
func NewDictionary(cfg config.Config, wg *sync.WaitGroup, dChan chan *DeletedShortURLValues) (Storage, error) {
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

	dic := &Dictionary{
		Items:           items,
		UserItems:       userItems,
		fileStoragePath: cfg.FileStoragePath,
		WaitGroup:       wg,
		DeleteChannel:   dChan,
	}

	dic.startDeleteWorker()
	return dic, nil
}

// func AddURL add original URL value to memory storage.
func (d *Dictionary) AddURL(ctx context.Context, longURLValue string, shortURLValue string, userID int32) (string, error) {
	if strings.TrimSpace(longURLValue) == "" {
		return "", errors.New("empty long URL value")
	}

	d.Items[shortURLValue] = longURLValue
	d.UserItems[userID] = append(d.UserItems[userID], shortURLValue)

	if err := ProducerWrite(d.fileStoragePath, &ItemType{
		ShortURLValue: shortURLValue,
		LongURLValue:  longURLValue,
	}); err != nil {
		return "", err
	}

	return shortURLValue, nil
}

// func GetURL get original URL value by a short value from memory storage.
func (d *Dictionary) GetURL(ctx context.Context, shortURLValue string) (string, error) {
	if strings.TrimSpace(shortURLValue) == "" {
		return "", errors.New("empty short URL value")
	}
	return d.Items[shortURLValue], nil
}

// func GetUserURL get short URL value and original URL value pairs array created by user.
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

// func PostAPIBatch is a group addition of short URL values to the memory storage via api.
//
// items - array of BatchRequest
// prefix - shortener service name
// userID - user ID
func (d *Dictionary) PostAPIBatch(ctx context.Context, items *BatchRequestArray, prefix string, userID int32) (*BatchResponseArray, error) {
	result := &BatchResponseArray{}
	for _, v := range *items {
		batchResponseItem := BatchResponse{}
		batchResponseItem.CorrelationID = v.CorrelationID

		batchResponseItem.ShortURL = v.ShortURL
		if strings.TrimSpace(prefix) != "" {
			batchResponseItem.ShortURL = prefix + "/" + v.ShortURL
		}

		d.Items[v.ShortURL] = v.OriginalURL
		d.UserItems[userID] = append(d.UserItems[userID], v.ShortURL)

		if err := ProducerWrite(d.fileStoragePath, &ItemType{
			ShortURLValue: v.ShortURL,
			LongURLValue:  v.OriginalURL,
		}); err != nil {
			return nil, err
		}
		*result = append(*result, batchResponseItem)
	}
	return result, nil
}

// func Ping - interface plug.
func (d *Dictionary) Ping(ctx context.Context) error {
	return nil
}

// func StartDeleteWorker launches three delete workers.
func (d *Dictionary) startDeleteWorker() {
	workerCount := 3

	for i := 0; i < workerCount; i++ {
		d.WaitGroup.Add(1)
		go d.deleteWorker()
	}
}

// func DeleteWorker is a listened the DeleteChannel worker.
// when a value is received through the channel, worker starts DeleteUserURL func.
func (d *Dictionary) deleteWorker() {
	defer d.WaitGroup.Done()

	for job := range d.DeleteChannel {
		d.DeleteUserURL(context.Background(), job)
	}
}

// func DeleteUserURL delete user URLs from memory storage
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

// func Close inteface plug.
func (d *Dictionary) Close() error {
	return nil
}

// func GetInternalStats counts the number of URLs and the number of users in the service.
func (d *Dictionary) GetInternalStats(ctx context.Context) (InternalStats, error) {
	return InternalStats{
		URLs:  len(d.Items),
		Users: len(d.UserItems),
	}, nil
}

// type URLItem is a linked list storage item.
type URLItem struct {
	Next             *URLItem
	ShortURLValue    string
	OriginalURLValue string
}

// type LinkedListURLItem is a linked list storage implementation.
type LinkedListURLItem struct {
	Head *URLItem
	Tail *URLItem
}

// type UsersLinkedListMemoryStorage is a multiuser linked list storage implementation.
type UsersLinkedListMemoryStorage struct {
	LinkedListStorage map[int32]*LinkedListURLItem
}

// func NewLinkedListStorage create a new linked list storage implementation.
func NewLinkedListStorage() Storage {
	lls := make(map[int32]*LinkedListURLItem)
	return UsersLinkedListMemoryStorage{
		LinkedListStorage: lls,
	}
}

// func AddURL add original URL value to linked list storage.
func (l UsersLinkedListMemoryStorage) AddURL(ctx context.Context, longURLValue string, shortURLValue string, userID int32) (string, error) {
	if strings.TrimSpace(longURLValue) == "" {
		return "", errors.New("empty long URL value")
	}

	u := &URLItem{
		ShortURLValue:    shortURLValue,
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

	return shortURLValue, nil
}

// func GetURL get original URL value by a short value from linked list storage.
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

// func GetUserURL get short URL value and original URL value pairs array created by user.
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

// func PostAPIBatch - group addition of short URL values to the linked list storage via api.
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
		batchResponseItem := BatchResponse{
			CorrelationID: v.CorrelationID,
			ShortURL:      v.ShortURL,
		}
		if strings.TrimSpace(prefix) != "" {
			batchResponseItem.ShortURL = prefix + "/" + v.ShortURL
		}

		item := &URLItem{
			ShortURLValue:    v.ShortURL,
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

// func Ping interface plug
func (l UsersLinkedListMemoryStorage) Ping(ctx context.Context) error {
	return nil
}

// func DeleteUserURL delete user URLs from linked list storage
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

// func Close interface plug.
func (l UsersLinkedListMemoryStorage) Close() error {
	return nil
}

// func GetInternalStats counts the number of URLs and the number of users in the service.
func (l UsersLinkedListMemoryStorage) GetInternalStats(ctx context.Context) (InternalStats, error) {
	counter := 0
	for _, v := range l.LinkedListStorage {
		currentItem := v.Head
		for currentItem != nil {
			counter++
			currentItem = currentItem.Next
		}
	}

	internalStats := InternalStats{
		URLs:  counter,
		Users: len(l.LinkedListStorage),
	}

	return internalStats, nil
}
