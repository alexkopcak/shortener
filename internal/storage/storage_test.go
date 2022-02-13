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
		{
			name: "simple storage test #1",
			fields: fields{
				Items:     map[string]int{},
				NextValue: 1,
			},
			args: args{"http://abc.test/abc"},
			want: "1",
		},
		{
			name: "add emtpy url string",
			fields: fields{
				Items:     map[string]int{},
				NextValue: 5,
			},
			args: args{""},
			want: "5",
		},
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
		{
			name: "simple storage test #1",
			fields: fields{
				Items: map[string]int{
					"http://test1.tst": 1,
					"http://test2.tst": 2,
					"http://tst34.tst": 8,
					"http://tst22.tst": 3,
				},
				NextValue: 11,
			},
			args: args{"8"},
			want: "http://tst34.tst",
		},
		{
			name: "empty output test",
			fields: fields{
				Items: map[string]int{
					"http://test1.tst": 1,
					"http://test2.tst": 2,
					"http://tst34.tst": 8,
					"http://tst22.tst": 3,
				},
				NextValue: 11,
			},
			args: args{""},
			want: "",
		},
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
