package storage

import (
	"testing"
)

func TestDictionary_AddURL(t *testing.T) {
	type fields struct {
		Items     map[string]int
		NextValue int
	}
	type args struct {
		value string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dic := &Dictionary{
				Items:     tt.fields.Items,
				NextValue: tt.fields.NextValue,
			}
			if got := dic.AddURL(tt.args.value); got != tt.want {
				t.Errorf("Dictionary.AddURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDictionary_GetURL(t *testing.T) {
	type fields struct {
		Items     map[string]int
		NextValue int
	}
	type args struct {
		shortValue string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dic := &Dictionary{
				Items:     tt.fields.Items,
				NextValue: tt.fields.NextValue,
			}
			if got := dic.GetURL(tt.args.shortValue); got != tt.want {
				t.Errorf("Dictionary.GetURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
