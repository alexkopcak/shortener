package storage

import (
	"reflect"
	"testing"
)

func TestNewConsumer(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    *consumer
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewConsumer(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConsumer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConsumer() = %v, want %v", got, tt.want)
			}
		})
	}
}
