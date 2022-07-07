package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alexkopcak/shortener/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDictionaryAddURL(t *testing.T) {
	tests := []struct {
		name         string
		longURLValue string
		err          bool
	}{
		{
			name:         "add value",
			longURLValue: "http://abc.test/abc",
			err:          false,
		},
		{
			name:         "add empty value",
			longURLValue: "",
			err:          true,
		},
		{
			name:         "add space value",
			longURLValue: " ",
			err:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewDictionary(config.Config{})
			require.NoError(t, err)
			ctx := context.Background()
			got, err := d.AddURL(ctx, tt.longURLValue, ShortURLGenerator(), 0)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				longValue, err := d.GetURL(ctx, got)
				require.NoError(t, err)
				assert.Equal(t, longValue, tt.longURLValue)
			}
		})
	}
}

func TestDictionaryGetURL(t *testing.T) {
	type fields struct {
		Items             map[string]string
		MinShortURLLength int
	}
	type args struct {
		shortURLValue string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name:   "get value",
			fields: fields{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dictionary{
				Items: tt.fields.Items,
			}
			ctx := context.Background()
			if got, _ := d.GetURL(ctx, tt.args.shortURLValue); got != tt.want {
				t.Errorf("Dictionary.GetURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShortURLGenerator(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{
			name:  "generated value are not empty and equal",
			count: 1000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := []string{}
			for i := 1; i <= tt.count; i++ {
				got := ShortURLGenerator()
				assert.Equal(t, minShortURLLengthConst, len(got))
				require.NotContains(t, val, got)
				val = append(val, got)
			}
		})
	}
}

func TestDictionaryGetUserURL(t *testing.T) {
	type fields struct {
		Items             map[string]string
		UserItems         map[int32][]string
		fileStoragePath   string
		MinShortURLLength int
	}
	type args struct {
		prefix string
		userID int32
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []UserExportType
	}{
		{
			name: "get value from empty dictionary",
			fields: fields{
				MinShortURLLength: 5,
				Items:             map[string]string{},
				UserItems:         map[int32][]string{},
				fileStoragePath:   "",
			},
			args: args{
				prefix: "http://localhost:8080",
				userID: 0,
			},
			want: []UserExportType{},
		},
		{
			name: "get value with prefix from empty dictionary",
			fields: fields{
				MinShortURLLength: 5,
				Items:             map[string]string{},
				UserItems:         map[int32][]string{},
				fileStoragePath:   "",
			},
			args: args{
				prefix: "  http://localhost:8080  ",
				userID: 0,
			},
			want: []UserExportType{},
		},
		{
			name: "get value from dictionary",
			fields: fields{
				MinShortURLLength: 5,
				Items: map[string]string{
					"shortURL1": "http://longURL1",
					"shortURL2": "http://longURL2",
					"shortURL3": "http://longURL3",
				},
				UserItems: map[int32][]string{
					3: {"shortURL1"},
				},
				fileStoragePath: "",
			},
			args: args{
				prefix: "http://localhost:8080",
				userID: 3,
			},
			want: []UserExportType{
				{
					ShortURL:    "http://localhost:8080/shortURL1",
					OriginalURL: "http://longURL1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dictionary{
				Items:           tt.fields.Items,
				UserItems:       tt.fields.UserItems,
				fileStoragePath: tt.fields.fileStoragePath,
			}
			ctx := context.Background()
			got, err := d.GetUserURL(ctx, tt.args.prefix, tt.args.userID)
			require.NoError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dictionary.GetUserURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDictionaryPostAPIBatch(t *testing.T) {
	type fields struct {
		Items           map[string]string
		UserItems       map[int32][]string
		fileStoragePath string
	}
	type args struct {
		ctx    context.Context
		items  *BatchRequestArray
		prefix string
		userID int32
	}
	tests := []struct {
		want    *BatchResponseArray
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "simple add json value",
			fields: fields{
				Items:           map[string]string{},
				UserItems:       map[int32][]string{},
				fileStoragePath: "",
			},
			args: args{
				ctx: context.Background(),
				items: &BatchRequestArray{
					BatchRequest{
						CorrelationID: "correlation ID 1",
						OriginalURL:   "http://test.tst1",
						ShortURL:      ShortURLGenerator(),
					},
				},
				prefix: "http://localhost:8080",
				userID: 18,
			},
			want: &BatchResponseArray{
				BatchResponse{
					CorrelationID: "correlation ID 1",
					ShortURL:      "http://localhost:8080",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dictionary{
				Items:           tt.fields.Items,
				UserItems:       tt.fields.UserItems,
				fileStoragePath: tt.fields.fileStoragePath,
			}
			got, err := d.PostAPIBatch(tt.args.ctx, tt.args.items, tt.args.prefix, tt.args.userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dictionary.PostAPIBatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			shortURL := strings.ReplaceAll((*got)[0].ShortURL, fmt.Sprint(tt.args.prefix, "/"), "")
			originalURL, err := d.GetURL(tt.args.ctx, shortURL)

			assert.Equal(t, (*tt.args.items)[0].CorrelationID, (*got)[0].CorrelationID)
			assert.Equal(t, (*tt.args.items)[0].OriginalURL, originalURL)
			require.NoError(t, err)
		})
	}
}

func TestProducerConsumer(t *testing.T) {
	type args struct {
		item     *ItemType
		filename string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			args: args{
				filename: "localStorage.test",
				item: &ItemType{
					ShortURLValue: "this is the short URL",
					LongURLValue:  "this is the long URL",
				},
			},
			name:    "test producer and consumer",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Remove(tt.args.filename)

			if err := ProducerWrite(tt.args.filename, tt.args.item); (err != nil) != tt.wantErr {
				t.Errorf("ProducerWrite() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			consumer, err := NewConsumer(tt.args.filename)
			require.NoError(t, err)

			defer consumer.Close()

			item, err := consumer.ReadItem()
			require.NoError(t, err)

			assert.Equal(t, tt.args.item.LongURLValue, item.LongURLValue)
			assert.Equal(t, tt.args.item.ShortURLValue, item.ShortURLValue)
		})
	}
}

func TestProducerErrors(t *testing.T) {
	type args struct {
		item     *ItemType
		filename string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			args: args{
				filename: "///",
				item:     &ItemType{},
			},
			name:    "bad file name",
			wantErr: true,
		},
		{
			args: args{
				filename: "test",
				item:     &ItemType{},
			},
			name:    "item is nil",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Remove(tt.args.filename)
			if err := ProducerWrite(tt.args.filename, tt.args.item); (err != nil) != tt.wantErr {
				t.Errorf("ProducerWrite() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestConsumerErrors(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			filename: "\\///",
			name:     "bad file name",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewConsumer(tt.filename); (err != nil) != tt.wantErr {
				t.Errorf("ProducerWrite() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func BenchmarkDictionaryStorage(b *testing.B) {
	runsCount := 1000
	shortURLs := make([]string, runsCount)

	var userCount int32 = 3
	var err error
	var userID int32 = 0

	dic, err := NewDictionary(config.Config{
		DBConnectionString: "",
		FileStoragePath:    "",
	})
	if err != nil {
		panic(err)
	}
	b.ResetTimer()

	addedURL := "LongURLValue"

	b.Run("addURL", func(b *testing.B) {
		for ; userID < userCount; userID++ {
			for i := 0; i < runsCount; i++ {
				shortURLs[i], err = dic.AddURL(context.Background(), fmt.Sprintf("%s_%v", addedURL, i), ShortURLGenerator(), userID)
			}
		}
	})
	if err != nil {
		panic(err)
	}

	getedURL := shortURLs[rand.Intn(runsCount)]
	b.Run("getURL", func(b *testing.B) {
		for i := 0; i < runsCount; i++ {
			dic.GetURL(context.Background(), getedURL)
		}
	})

	b.Run("getUserURL", func(b *testing.B) {
		for i := 0; i < runsCount; i++ {
			dic.GetUserURL(context.Background(), "", userID)
		}
	})

	deletedURLs := make([]string, 1)
	b.Run("DeleteUserURL", func(b *testing.B) {
		for i := 0; i < runsCount; i++ {
			deletedURLs[0] = shortURLs[rand.Intn(runsCount)]
			dic.DeleteUserURL(context.Background(), &DeletedShortURLValues{ShortURLValues: deletedURLs, UserIDValue: userID})
		}
	})
}

func BenchmarkLinkedListStorage(b *testing.B) {
	runsCount := 1000
	shortURLs := make([]string, runsCount)

	var userCount int32 = 3
	var err error
	var userID int32 = 0

	dic := NewLinkedListStorage().(UsersLinkedListMemoryStorage)
	b.ResetTimer()

	addedURL := "LongURLValue"

	b.Run("addURL", func(b *testing.B) {
		for ; userID < userCount; userID++ {
			for i := 0; i < runsCount; i++ {
				shortURLs[i], err = dic.AddURL(context.Background(), fmt.Sprintf("%s_%v", addedURL, i), ShortURLGenerator(), userID)
			}
		}
	})
	if err != nil {
		panic(err)
	}

	getedURL := shortURLs[rand.Intn(runsCount)]
	b.Run("getURL", func(b *testing.B) {
		for i := 0; i < runsCount; i++ {
			dic.GetURL(context.Background(), getedURL)
		}
	})

	b.Run("getUserURL", func(b *testing.B) {
		for i := 0; i < runsCount; i++ {
			dic.GetUserURL(context.Background(), "", userID)
		}
	})

	deletedURLs := make([]string, 1)
	b.Run("DeleteUserURL", func(b *testing.B) {
		for i := 0; i < runsCount; i++ {
			deletedURLs[0] = shortURLs[rand.Intn(runsCount)]
			dic.DeleteUserURL(context.Background(), &DeletedShortURLValues{ShortURLValues: deletedURLs, UserIDValue: userID})
		}
	})
}

func TestDictionaryDeleteUserURL(t *testing.T) {
	type fields struct {
		Items           map[string]string
		UserItems       map[int32][]string
		fileStoragePath string
	}
	type args struct {
		ctx         context.Context
		deletedURLs *DeletedShortURLValues
	}
	tests := []struct {
		fields  fields
		args    args
		name    string
		wantErr bool
	}{
		{
			name: "Delete User URL from directory",
			fields: fields{
				Items:           map[string]string{},
				UserItems:       map[int32][]string{},
				fileStoragePath: "",
			},
			args: args{
				ctx: context.Background(),
				deletedURLs: &DeletedShortURLValues{
					ShortURLValues: make([]string, 0),
					UserIDValue:    0,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				FileStoragePath: "localStorage.test",
			}

			defer os.Remove(cfg.FileStoragePath)

			err := ProducerWrite(cfg.FileStoragePath, &ItemType{
				ShortURLValue: "short URL value",
				LongURLValue:  "long URL value",
			})

			require.NoError(t, err)

			d := &Dictionary{
				Items:           tt.fields.Items,
				UserItems:       tt.fields.UserItems,
				fileStoragePath: tt.fields.fileStoragePath,
			}
			require.NoError(t, err)

			item := ItemType{
				ShortURLValue: "",
				LongURLValue:  "this is long URL value",
			}

			item.ShortURLValue, err = d.AddURL(tt.args.ctx, item.LongURLValue, ShortURLGenerator(), tt.args.deletedURLs.UserIDValue)
			require.NoError(t, err)

			longURL, err := d.GetURL(tt.args.ctx, item.ShortURLValue)
			require.NoError(t, err)
			assert.Equal(t, item.LongURLValue, longURL)

			assert.Len(t, d.Items, 1)
			assert.Len(t, d.UserItems, 1)

			tt.args.deletedURLs.ShortURLValues = append(tt.args.deletedURLs.ShortURLValues, item.ShortURLValue)

			if err = d.DeleteUserURL(tt.args.ctx, tt.args.deletedURLs); (err != nil) != tt.wantErr {
				t.Errorf("Dictionary.DeleteUserURL() error = %v, wantErr %v", err, tt.wantErr)
			}

			require.NoError(t, err)
			assert.Len(t, d.Items, 0)
			assert.Len(t, d.UserItems[tt.args.deletedURLs.UserIDValue], 0)

		})
	}
}

func TestDictionaryNewDictionaryWithStorage(t *testing.T) {
	tests := []struct {
		name        string
		fileStorage string
		wantErr     bool
	}{
		{
			name:        "New Dictionary With Storage",
			fileStorage: "localStorage.test",
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				FileStoragePath: tt.fileStorage,
			}

			defer os.Remove(cfg.FileStoragePath)

			err := ProducerWrite(cfg.FileStoragePath, &ItemType{
				ShortURLValue: "short URL value",
				LongURLValue:  "long URL value",
			})

			require.NoError(t, err)

			d, err := NewDictionary(cfg)
			require.NoError(t, err)
			assert.Len(t, d.(*Dictionary).Items, 1)

		})
	}
}

func TestDictionaryDeleteUserURLSecond(t *testing.T) {
	type fields struct {
		Items           map[string]string
		UserItems       map[int32][]string
		fileStoragePath string
	}
	type args struct {
		ctx         context.Context
		deletedURLs *DeletedShortURLValues
	}
	tests := []struct {
		fields  fields
		args    args
		name    string
		wantErr bool
	}{
		{
			name: "Delete User URL from directory empty URLs",
			fields: fields{
				Items:           map[string]string{},
				UserItems:       map[int32][]string{},
				fileStoragePath: "",
			},
			args: args{
				ctx:         context.Background(),
				deletedURLs: nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dictionary{
				Items:           tt.fields.Items,
				UserItems:       tt.fields.UserItems,
				fileStoragePath: tt.fields.fileStoragePath,
			}
			if err := d.DeleteUserURL(tt.args.ctx, tt.args.deletedURLs); (err != nil) != tt.wantErr {
				t.Errorf("Dictionary.DeleteUserURL() error = %v, wantErr %v", err, tt.wantErr)
			}

		})
	}
}

func TestDictionaryPing(t *testing.T) {
	type fields struct {
		Items           map[string]string
		UserItems       map[int32][]string
		fileStoragePath string
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		fields  fields
		args    args
		name    string
		wantErr bool
	}{
		{
			name:    "simple ping test",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dictionary{
				Items:           tt.fields.Items,
				UserItems:       tt.fields.UserItems,
				fileStoragePath: tt.fields.fileStoragePath,
			}
			if err := d.Ping(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("Dictionary.Ping() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLinkedListAddURL(t *testing.T) {
	tests := []struct {
		name         string
		longURLValue string
		err          bool
	}{
		{
			name:         "add value",
			longURLValue: "http://abc.test/abc",
			err:          false,
		},
		{
			name:         "add empty value",
			longURLValue: "",
			err:          true,
		},
		{
			name:         "add space value",
			longURLValue: " ",
			err:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewLinkedListStorage()
			ctx := context.Background()
			got, err := d.AddURL(ctx, tt.longURLValue, ShortURLGenerator(), 0)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				longValue, err := d.GetURL(ctx, got)
				require.NoError(t, err)
				assert.Equal(t, tt.longURLValue, longValue)
			}
		})
	}
}

func TestLinkedListGetURL(t *testing.T) {
	type importItemType struct {
		OriginalURLValue string
		RemoveAfter      bool
		UserID           int32
	}
	type args struct {
		shortURLValue string
		importedItem  []importItemType
	}
	tests := []struct {
		name string
		want string
		args args
	}{
		{
			name: "get value",
			args: args{
				shortURLValue: "short URL Value1",
				importedItem:  []importItemType{},
			},
		},
		{
			name: "get empty value",
			args: args{
				shortURLValue: "",
				importedItem:  []importItemType{},
			},
		},
		{
			name: "get value, add some records before",
			args: args{
				shortURLValue: "",
				importedItem: []importItemType{
					{
						OriginalURLValue: "original URL value 1",
						RemoveAfter:      false,
						UserID:           0,
					},
					{
						OriginalURLValue: "original URL value 2",
						RemoveAfter:      false,
						UserID:           0,
					},
				},
			},
		},
		{
			name: "get empty value",
			args: args{
				shortURLValue: "",
				importedItem: []importItemType{
					{
						OriginalURLValue: "original URL value 1",
						RemoveAfter:      true,
						UserID:           0,
					},
					{
						OriginalURLValue: "original URL value 2",
						RemoveAfter:      false,
						UserID:           1,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewLinkedListStorage()

			for _, v := range tt.args.importedItem {
				val, err := d.AddURL(context.Background(), v.OriginalURLValue, ShortURLGenerator(), v.UserID)
				assert.NoError(t, err)
				d.DeleteUserURL(context.Background(), &DeletedShortURLValues{
					ShortURLValues: []string{val},
					UserIDValue:    v.UserID,
				})
			}

			if got, _ := d.GetURL(context.Background(), tt.args.shortURLValue); got != tt.want {
				t.Errorf("Dictionary.GetURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinkedListGetUserURL(t *testing.T) {
	type args struct {
		prefix       string
		originalURL  string
		userID       int32
		emptyStorage bool
	}
	tests := []struct {
		name string
		args args
		want []UserExportType
	}{
		{
			name: "get value from empty dictionary",
			args: args{
				prefix:       "http://localhost:8080",
				userID:       0,
				originalURL:  "long URL value 1",
				emptyStorage: true,
			},
		},
		{
			name: "get value from dictionary",
			args: args{
				prefix:       "http://localhost:8080",
				userID:       3,
				originalURL:  "long URL value 2",
				emptyStorage: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			d := NewLinkedListStorage()

			val1 := ""
			var err error
			if !tt.args.emptyStorage {
				val1, err = d.AddURL(ctx, tt.args.originalURL, ShortURLGenerator(), tt.args.userID)
				require.NoError(t, err)
			}

			got, err := d.GetUserURL(ctx, tt.args.prefix, tt.args.userID)
			require.NoError(t, err)
			if tt.args.emptyStorage {
				want := []UserExportType{}
				reflect.DeepEqual(got, want)
			} else {
				want := []UserExportType{}
				want = append(want, UserExportType{
					ShortURL:    val1,
					OriginalURL: tt.args.originalURL,
				})
				reflect.DeepEqual(got, want)
			}
		})
	}
}

func TestLinedListPostAPIBatch(t *testing.T) {
	type args struct {
		ctx    context.Context
		items  *BatchRequestArray
		prefix string
		userID int32
	}
	tests := []struct {
		want    *BatchResponseArray
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "simple add json value",
			args: args{
				ctx: context.Background(),
				items: &BatchRequestArray{
					BatchRequest{
						CorrelationID: "correlation ID 1",
						OriginalURL:   "http://test.tst1",
						ShortURL:      ShortURLGenerator(),
					},
				},
				prefix: "http://localhost:8080",
				userID: 18,
			},
			want: &BatchResponseArray{
				BatchResponse{
					CorrelationID: "correlation ID 1",
					ShortURL:      "http://localhost:8080",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewLinkedListStorage()
			got, err := d.PostAPIBatch(tt.args.ctx, tt.args.items, tt.args.prefix, tt.args.userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dictionary.PostAPIBatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			shortURL := strings.ReplaceAll((*got)[0].ShortURL, fmt.Sprint(tt.args.prefix, "/"), "")
			originalURL, err := d.GetURL(tt.args.ctx, shortURL)

			assert.Equal(t, (*tt.args.items)[0].CorrelationID, (*got)[0].CorrelationID)
			assert.Equal(t, (*tt.args.items)[0].OriginalURL, originalURL)
			require.NoError(t, err)
		})
	}
}

func TestUsersLinkedListMemoryStorageDeleteUserURL(t *testing.T) {
	type importURLvalue struct {
		OriginalURLValue string
		DeleteAfter      bool
		UserID           int32
	}

	type fields struct {
		LinkedListStorage map[int32]*LinkedListURLItem
	}
	type args struct {
		deletedURLs       *DeletedShortURLValues
		importedURLValues []importURLvalue
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "delete from empty storage",
			fields: fields{
				LinkedListStorage: map[int32]*LinkedListURLItem{},
			},
			args: args{
				deletedURLs: &DeletedShortURLValues{
					ShortURLValues: []string{
						"short URL 1",
					},
					UserIDValue: 0,
				},
			},
			wantErr: false,
		},
		{
			name: "delete all from storage",
			fields: fields{
				LinkedListStorage: map[int32]*LinkedListURLItem{},
			},
			args: args{
				deletedURLs: &DeletedShortURLValues{},
				importedURLValues: []importURLvalue{
					{
						OriginalURLValue: "original URL value 1",
						DeleteAfter:      true,
						UserID:           1,
					},
					{
						OriginalURLValue: "original URL value 2",
						DeleteAfter:      true,
						UserID:           1,
					},
					{
						OriginalURLValue: "original URL value 3",
						DeleteAfter:      true,
						UserID:           1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete all from storage except first",
			fields: fields{
				LinkedListStorage: map[int32]*LinkedListURLItem{},
			},
			args: args{
				deletedURLs: &DeletedShortURLValues{},
				importedURLValues: []importURLvalue{
					{
						OriginalURLValue: "original URL value 1",
						DeleteAfter:      false,
						UserID:           1,
					},
					{
						OriginalURLValue: "original URL value 2",
						DeleteAfter:      true,
						UserID:           1,
					},
					{
						OriginalURLValue: "original URL value 3",
						DeleteAfter:      true,
						UserID:           1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete all from storage except last",
			fields: fields{
				LinkedListStorage: map[int32]*LinkedListURLItem{},
			},
			args: args{
				deletedURLs: &DeletedShortURLValues{},
				importedURLValues: []importURLvalue{
					{
						OriginalURLValue: "original URL value 1",
						DeleteAfter:      false,
						UserID:           1,
					},
					{
						OriginalURLValue: "original URL value 2",
						DeleteAfter:      false,
						UserID:           1,
					},
					{
						OriginalURLValue: "original URL value 3",
						DeleteAfter:      true,
						UserID:           1,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := UsersLinkedListMemoryStorage{
				LinkedListStorage: tt.fields.LinkedListStorage,
			}
			for _, v := range tt.args.importedURLValues {
				val, err := l.AddURL(context.Background(), v.OriginalURLValue, ShortURLGenerator(), v.UserID)
				require.NoError(t, err)
				if v.DeleteAfter {
					tt.args.deletedURLs.ShortURLValues = append(tt.args.deletedURLs.ShortURLValues, val)
					tt.args.deletedURLs.UserIDValue = v.UserID
				}
			}

			if err := l.DeleteUserURL(context.Background(), tt.args.deletedURLs); (err != nil) != tt.wantErr {
				t.Errorf("UsersLinkedListMemoryStorage.DeleteUserURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func NewMock() (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		log.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	return db, mock
}

type tableModel struct {
	Stamp       *time.Time
	ShortURL    string
	OriginalURL string
	ID          int32
}

func TestPostgresGetURL(t *testing.T) {

	var i = &tableModel{
		ShortURL:    "shortURL",
		OriginalURL: "http://test.tst",
	}

	db, mock := NewMock()
	repo := &PostgresStorage{db, &sync.WaitGroup{}, nil}

	defer func() {
		repo.Close()
	}()

	query := "SELECT original_url, deleted_at FROM shortener WHERE short_url \\= \\$1 ;"

	rows := sqlmock.NewRows([]string{"original_url", "deleted_at"}).AddRow(i.OriginalURL, i.Stamp)

	mock.ExpectQuery(query).WithArgs(i.ShortURL).WillReturnRows(rows)

	originalURL, err := repo.GetURL(context.Background(), i.ShortURL)
	assert.NotNil(t, originalURL)
	require.NoError(t, err)
}

func TestPostgresGetURLDeletedRecord(t *testing.T) {
	var i = &tableModel{
		ShortURL:    "shortURL",
		OriginalURL: "http://test.tst",
	}

	stamp := time.Now()
	i.Stamp = &stamp

	db, mock := NewMock()
	repo := &PostgresStorage{db, &sync.WaitGroup{}, nil}

	defer func() {
		repo.Close()
	}()

	query := "SELECT original_url, deleted_at FROM shortener WHERE short_url \\= \\$1 ;"

	rows := sqlmock.NewRows([]string{"original_url", "deleted_at"}).AddRow(i.OriginalURL, i.Stamp)

	mock.ExpectQuery(query).WithArgs(i.ShortURL).WillReturnRows(rows)

	originalURL, err := repo.GetURL(context.Background(), i.ShortURL)
	assert.NotNil(t, originalURL)
	require.Error(t, err)
}

func TestPostgresGetUserURL(t *testing.T) {
	var i = &tableModel{
		ID:          1,
		ShortURL:    "shortURL",
		OriginalURL: "http://test.tst",
	}

	db, mock := NewMock()
	repo := &PostgresStorage{db, &sync.WaitGroup{}, nil}

	defer func() {
		repo.Close()
	}()

	query := "SELECT short_url, original_url " +
		"FROM shortener " +
		"WHERE user_id \\= \\$1 ;"

	rows := sqlmock.NewRows([]string{"short_url", "original_url"}).AddRow(i.ShortURL, i.OriginalURL)

	mock.ExpectQuery(query).WithArgs(i.ID).WillReturnRows(rows)

	result, err := repo.GetUserURL(context.Background(), "prefix", i.ID)
	assert.NotNil(t, result)
	require.NoError(t, err)
}

func TestPostgresAddURL(t *testing.T) {
	var i = &tableModel{
		ID:          1,
		ShortURL:    "shortURL",
		OriginalURL: "http://original.url",
	}

	db, mock := NewMock()
	repo := &PostgresStorage{db, &sync.WaitGroup{}, nil}

	defer func() {
		repo.Close()
	}()

	query := "INSERT INTO shortener \\(user_id, short_url, original_url\\) VALUES \\(\\$1, \\$2, \\$3\\)" +
		" ON CONFLICT \\(user_id, original_url\\) DO NOTHING; "

	mock.ExpectExec(query).WithArgs(i.ID, i.ShortURL, i.OriginalURL).WillReturnResult(sqlmock.NewResult(0, 1))

	query2 := "SELECT short_url " +
		"FROM shortener " +
		"WHERE user_id \\= \\$1 AND original_url \\= \\$2 ;"

	rows := sqlmock.NewRows([]string{"short_url"}).AddRow(i.ShortURL)
	mock.ExpectQuery(query2).WithArgs(i.ID, i.OriginalURL).WillReturnRows(rows)

	val, err := repo.AddURL(context.Background(), i.OriginalURL, i.ShortURL, i.ID)
	require.NoError(t, err)
	require.NotEmpty(t, val)
}

func TestPostgresAddURLError(t *testing.T) {
	var i = &tableModel{
		ID:          1,
		ShortURL:    "shortURL",
		OriginalURL: "http://original.url",
	}

	db, mock := NewMock()
	repo := &PostgresStorage{db, &sync.WaitGroup{}, nil}

	defer func() {
		repo.Close()
	}()

	query := "INSERT INTO shortener \\(user_id, short_url, original_url\\) VALUES \\(\\$1, \\$2, \\$3\\)" +
		" ON CONFLICT \\(user_id, original_url\\) DO NOTHING; "

	mock.ExpectExec(query).WithArgs(i.ID, i.ShortURL, i.OriginalURL).WillReturnResult(sqlmock.NewResult(0, 0))

	query2 := "SELECT short_url " +
		"FROM shortener " +
		"WHERE user_id \\= \\$1 AND original_url \\= \\$2 ;"

	rows := sqlmock.NewRows([]string{"short_url"}).AddRow(i.ShortURL)
	mock.ExpectQuery(query2).WithArgs(i.ID, i.OriginalURL).WillReturnRows(rows)

	val, err := repo.AddURL(context.Background(), i.OriginalURL, i.ShortURL, i.ID)
	require.ErrorIs(t, err, ErrDuplicateRecord)
	require.NotEmpty(t, val)
}

func TestPostgresAddURLErrorEmptyOriginalURL(t *testing.T) {
	var i = &tableModel{
		ID:          1,
		ShortURL:    "shortURL",
		OriginalURL: "",
	}

	db, mock := NewMock()
	repo := &PostgresStorage{db, &sync.WaitGroup{}, nil}

	defer func() {
		repo.Close()
	}()

	query := "INSERT INTO shortener \\(user_id, short_url, original_url\\) VALUES \\(\\$1, \\$2, \\$3\\)" +
		" ON CONFLICT \\(user_id, original_url\\) DO NOTHING; "

	mock.ExpectExec(query).WithArgs(i.ID, i.ShortURL, i.OriginalURL).WillReturnResult(sqlmock.NewResult(0, 0))

	query2 := "SELECT short_url " +
		"FROM shortener " +
		"WHERE user_id \\= \\$1 AND original_url \\= \\$2 ;"

	rows := sqlmock.NewRows([]string{"short_url"}).AddRow(i.ShortURL)
	mock.ExpectQuery(query2).WithArgs(i.ID, i.OriginalURL).WillReturnRows(rows)

	val, err := repo.AddURL(context.Background(), i.OriginalURL, i.ShortURL, i.ID)
	require.Error(t, err)
	require.Empty(t, val)
}

func TestPostgresPostAPIBatch(t *testing.T) {
	var i = &tableModel{
		ID:          1,
		ShortURL:    "shortURL",
		OriginalURL: "http://original.url",
	}

	db, mock := NewMock()
	repo := &PostgresStorage{db, &sync.WaitGroup{}, nil}

	defer func() {
		repo.Close()
	}()

	query := "INSERT INTO shortener \\(user_id, short_url, original_url\\) VALUES \\(\\$1, \\$2, \\$3\\);"

	mock.ExpectBegin()
	mock.ExpectExec(query).WithArgs(i.ID, i.ShortURL, i.OriginalURL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	batch := BatchRequest{
		CorrelationID: "correlation ID",
		OriginalURL:   i.OriginalURL,
		ShortURL:      i.ShortURL,
	}

	batchArray := &BatchRequestArray{batch}

	val, err := repo.PostAPIBatch(context.Background(), batchArray, "prefix", i.ID)
	require.NoError(t, err)
	require.NotEmpty(t, val)
}

func TestPostgresPing(t *testing.T) {
	db, mock := NewMock()
	repo := &PostgresStorage{db, &sync.WaitGroup{}, nil}
	defer func() {
		repo.Close()
	}()

	mock.ExpectPing()

	err := repo.Ping(context.Background())
	require.NoError(t, err)
}

func TestPostgresDeleteUserURL(t *testing.T) {
	var i = &tableModel{
		ID:       1,
		ShortURL: "shortURL",
	}

	db, mock := NewMock()
	repo := &PostgresStorage{db, &sync.WaitGroup{}, nil}
	defer func() {
		repo.Close()
	}()

	query := "UPDATE shortener SET deleted_at \\= now\\(\\) WHERE user_id \\= \\$1 and short_url \\= ANY\\(\\$2\\);"

	mock.ExpectExec(query).WithArgs(i.ID, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteUserURL(context.Background(), &DeletedShortURLValues{
		[]string{i.ShortURL},
		i.ID,
	})

	require.NoError(t, err)
}

func TestPostgresGetInternalStats(t *testing.T) {
	db, mock := NewMock()
	repo := &PostgresStorage{db, &sync.WaitGroup{}, nil}
	defer func() {
		repo.Close()
	}()

	user_cnt := 2
	url_cnt := 3

	query := "SELECT user_nested.user_cnt, url_nested.url_cnt " +
		"FROM " +
		"\\(SELECT COUNT\\(DISTINCT user_id\\) AS user_cnt FROM shortener WHERE deleted_at IS NULL\\) AS user_nested, " +
		"\\(SELECT COUNT\\(\\*\\) AS url_cnt FROM shortener WHERE deleted_at IS NULL\\) AS url_nested;"

	rows := sqlmock.NewRows([]string{"user_nested.user_cnt", "url_nested.url_cnt"}).AddRow(user_cnt, url_cnt)
	mock.ExpectQuery(query).WithArgs().WillReturnRows(rows)

	val, err := repo.GetInternalStats(context.Background())

	require.NoError(t, err)
	require.Equal(t, user_cnt, val.Users)
	require.Equal(t, url_cnt, val.URLs)

}

func TestDictionaryClose(t *testing.T) {
	d := &Dictionary{
		Items:     map[string]string{},
		UserItems: map[int32][]string{},
	}
	err := d.Close()
	require.NoError(t, err)
}

func TestDictionaryGetInternalStats(t *testing.T) {
	d := &Dictionary{
		Items:     map[string]string{},
		UserItems: map[int32][]string{},
	}
	val, err := d.GetInternalStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, val.URLs)
	require.Equal(t, 0, val.Users)
}

func TestUserLinkedListPing(t *testing.T) {
	ll := &UsersLinkedListMemoryStorage{
		LinkedListStorage: map[int32]*LinkedListURLItem{},
	}
	err := ll.Ping(context.Background())
	require.NoError(t, err)
}

func TestUserLinkedListClose(t *testing.T) {
	ll := &UsersLinkedListMemoryStorage{
		LinkedListStorage: map[int32]*LinkedListURLItem{},
	}
	err := ll.Close()
	require.NoError(t, err)
}

func TestUserLinkedListGetInternalStats(t *testing.T) {
	ui := &URLItem{
		ShortURLValue:    "short URL",
		OriginalURLValue: "original URL",
		Next:             nil,
	}
	ll := &UsersLinkedListMemoryStorage{
		LinkedListStorage: map[int32]*LinkedListURLItem{
			0: {
				Tail: ui,
				Head: ui,
			},
		},
	}
	val, err := ll.GetInternalStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, val.Users)
	require.Equal(t, 1, val.URLs)
}
