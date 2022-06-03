package storage

import (
	"context"
	"fmt"
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
		MinShortURLLength int
		Items             map[string]string
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
		MinShortURLLength int
		Items             map[string]string
		UserItems         map[int32][]string
		fileStoragePath   string
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
		name    string
		fields  fields
		args    args
		want    *BatchResponseArray
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
		filename string
		item     *ItemType
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

func BenchmarkAddURL(b *testing.B) {
	runsCount := 10000
	var userCount int32 = 3

	dic, err := NewDictionary(config.Config{
		DBConnectionString: "",
		FileStoragePath:    "",
	})
	if err != nil {
		panic(err)
	}
	b.ResetTimer()

	var userID int32 = 0
	addedURL := "LongURLValue"
	b.Run("addURL", func(b *testing.B) {
		for ; userID < userCount; userID++ {
			for i := 0; i < runsCount; i++ {
				dic.AddURL(context.Background(), addedURL, userID)
			}
		}
	})

	getedURL := "ShortURLValue"
	b.Run("getURL", func(b *testing.B) {
		for i := 0; i < runsCount; i++ {
			dic.GetURL(context.Background(), getedURL)
		}
	})

	userID = 0
	b.Run("getUserURL", func(b *testing.B) {
		for i := 0; i < runsCount; i++ {
			dic.GetUserURL(context.Background(), "", userID)
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
		name    string
		fields  fields
		args    args
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

			if err := d.DeleteUserURL(tt.args.ctx, tt.args.deletedURLs); (err != nil) != tt.wantErr {
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
		name    string
		wantErr bool
	}{
		{
			name: "New Dictionary With Storage",
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
		name    string
		fields  fields
		args    args
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
		name    string
		fields  fields
		args    args
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
