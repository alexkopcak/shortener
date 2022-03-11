package storage

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDictionary_AddURL(t *testing.T) {
	type fields struct {
		MinShortURLLength int
		Items             map[string]string
	}
	type args struct {
		longURLValue []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "add value",
			fields: fields{
				MinShortURLLength: 5,
				Items:             map[string]string{},
			},
			args: args{
				longURLValue: []string{"http://abc.test/abc"},
			},
		},
		{
			name: "add the same value twise #1",
			fields: fields{
				MinShortURLLength: 5,
				Items: map[string]string{
					"0a1b2": "http://abc.test/abc",
				},
			},
			args: args{
				longURLValue: []string{"http://abc.test/abc"},
			},
		},
		{
			name: "add the same value twise #2",
			fields: fields{
				MinShortURLLength: 5,
				Items: map[string]string{
					"0a1b2": "http://abc.test/abc",
				},
			},
			args: args{
				longURLValue: []string{
					"http://abc.test/abc",
					"http://abc.test/abc",
				},
			},
		},
		{
			name: "add empty value",
			fields: fields{
				MinShortURLLength: 5,
				Items:             map[string]string{},
			},
			args: args{
				longURLValue: []string{""},
			},
		},
		{
			name: "add space value",
			fields: fields{
				MinShortURLLength: 5,
				Items:             map[string]string{},
			},
			args: args{
				longURLValue: []string{" "},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewDictionary("")
			require.NoError(t, err)

			d.Items = tt.fields.Items

			for _, item := range tt.args.longURLValue {
				got, _ := d.AddURL(item, 0)
				assert.Equal(t, strings.TrimSpace(item), d.Items[got])
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
				MinShortURLLength: tt.fields.MinShortURLLength,
				Items:             tt.fields.Items,
			}
			if got, _ := d.GetURL(tt.args.shortURLValue); got != tt.want {
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
		UserItems         map[uint64][]string
		fileStoragePath   string
	}
	type args struct {
		prefix string
		userID uint64
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
				UserItems:         map[uint64][]string{},
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
				UserItems: map[uint64][]string{
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
				MinShortURLLength: tt.fields.MinShortURLLength,
				Items:             tt.fields.Items,
				UserItems:         tt.fields.UserItems,
				fileStoragePath:   tt.fields.fileStoragePath,
			}
			fmt.Printf("%v\n", *d)
			if got := d.GetUserURL(tt.args.prefix, tt.args.userID); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dictionary.GetUserURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
