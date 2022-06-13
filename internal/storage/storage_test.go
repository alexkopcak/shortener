package storage

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/alexkopcak/shortener/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDictionary_AddURL(t *testing.T) {
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
			got, err := d.AddURL(ctx, tt.longURLValue, 0)
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

func TestDictionary_GetURL(t *testing.T) {
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

func Test_shortURLGenerator(t *testing.T) {
	type args struct {
		n int
	}
	tests := []struct {
		name  string
		args  args
		count int
	}{
		{
			name: "generated value are not empty and equal",
			args: args{
				n: 5,
			},
			count: 1000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := []string{}
			for i := 1; i <= tt.count; i++ {
				got := shortURLGenerator(tt.args.n)
				assert.Equal(t, tt.args.n, len(got))
				require.NotContains(t, val, got)
				val = append(val, got)
			}
		})
	}
}

func TestDictionary_GetUserURL(t *testing.T) {
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

func TestDictionary_PostAPIBatch(t *testing.T) {
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
				shortURLs[i], err = dic.AddURL(context.Background(), fmt.Sprintf("%s_%v", addedURL, i), userID)
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
				shortURLs[i], err = dic.AddURL(context.Background(), fmt.Sprintf("%s_%v", addedURL, i), userID)
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

func TestDictionary_DeleteUserURL(t *testing.T) {
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

			item.ShortURLValue, err = d.AddURL(tt.args.ctx, item.LongURLValue, tt.args.deletedURLs.UserIDValue)
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

func TestDictionary_NewDictionaryWithStorage(t *testing.T) {
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

func TestDictionary_DeleteUserURLSecond(t *testing.T) {
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

func TestDictionary_Ping(t *testing.T) {
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

func TestLinkedList_AddURL(t *testing.T) {
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
			got, err := d.AddURL(ctx, tt.longURLValue, 0)
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

func TestLinkedList_GetURL(t *testing.T) {
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
				val, err := d.AddURL(context.Background(), v.OriginalURLValue, v.UserID)
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

func TestLinkedList_GetUserURL(t *testing.T) {
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
				val1, err = d.AddURL(ctx, tt.args.originalURL, tt.args.userID)
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

func TestLinedList_PostAPIBatch(t *testing.T) {
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

func TestUsersLinkedListMemoryStorage_DeleteUserURL(t *testing.T) {
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
				val, err := l.AddURL(context.Background(), v.OriginalURLValue, v.UserID)
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
