package main

import (
	"reflect"
	"testing"
)

func TestEncodeDecodeSignedMessage(t *testing.T) {
	tests := []struct {
		name    string
		message any
		wantErr bool
	}{
		{
			name:    "Hello, string",
			message: "Hello, world!",
			wantErr: false,
		},
		{
			name:    "Hello, not large enough",
			message: "Привет мир! Cе",
			wantErr: false,
		},
		{
			name:    "Hello, barely large",
			message: "Привет мир! Cег",
			wantErr: true,
		},
		{
			name:    "Hello, LARGE",
			message: "Привет мир! Сегодня я зачем-то переизобрёл JWT. Не знаю зачем...",
			wantErr: true,
		},
		{
			name:    "Hello, map",
			message: map[string]string{"Hello": "world!"},
			wantErr: false,
		},
		{
			name:    "Hello, struct",
			message: struct{ Hello, World string }{"Hello", "world!"},
			wantErr: false,
		},
		{
			name:    "Hello, empty string",
			message: "",
			wantErr: false,
		},
		{
			name:    "Hello, nil string pointer",
			message: (*string)(nil),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeSignedMessage(tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeSignedMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			var decoded = reflect.New(reflect.TypeOf(tt.message)).Interface()
			if err := DecodeSignedMessage(encoded, decoded); (err != nil) != tt.wantErr {
				t.Errorf("DecodeSignedMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			deref := reflect.ValueOf(decoded).Elem().Interface()

			if !reflect.DeepEqual(tt.message, deref) {
				t.Errorf("decoded message is not the same: got %T %#v; want %T %#v", deref, deref, tt.message, tt.message)
				return
			}
		})
	}
}
