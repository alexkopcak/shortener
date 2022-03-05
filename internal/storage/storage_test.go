package storage

import (
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
				got, _ := d.AddURL(item)
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
