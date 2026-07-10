package bencode

import (
	"reflect"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	/*
		note:
			add test cases for leading zeros, etc.
	*/
	tests := []struct {
		name    string
		input   string
		want    any
		wantErr bool
	}{
		{
			name:  "positive integer",
			input: "i42e",
			want:  42,
		},
		{
			name:  "negative integer",
			input: "i-42e",
			want:  -42,
		},
		{
			name:  "zero integer",
			input: "i0e",
			want:  0,
		},
		{
			name:  "string",
			input: "5:hello",
			want:  "hello",
		},
		{
			name:  "empty string",
			input: "0:",
			want:  "",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "unterminated integer",
			input:   "i42",
			wantErr: true,
		},
		{
			name:    "empty integer",
			input:   "ie",
			wantErr: true,
		},
		{
			name:    "invalid string length",
			input:   "5:hell",
			wantErr: true,
		},
		{
			name:    "junk after value",
			input:   "3:catjunk",
			wantErr: true,
		},
		{
			name:  "empty list",
			input: "le",
			want:  []any{},
		},
		{
			name:  "mixed list",
			input: "li42e3:cate",
			want:  []any{42, "cat"},
		},
		{
			name:  "empty dictionary",
			input: "de",
			want:  map[string]any{},
		},
		{
			name:  "simple dictionary",
			input: "d3:agei25e4:name5:Alicee",
			want: map[string]any{
				"age":  25,
				"name": "Alice",
			},
		},
		{
			name:  "nested list",
			input: "ll3:abce3:xyze",
			want: []any{
				[]any{"abc"},
				"xyz",
			},
		},
		{
			name:  "nested dictionary",
			input: "d4:listli1ei2eee",
			want: map[string]any{
				"list": []any{1, 2},
			},
		},
		{
			name:  "dictionary containing dictionary",
			input: "d4:metad5:admini1eee",
			want: map[string]any{
				"meta": map[string]any{
					"admin": 1,
				},
			},
		},
		{
			name:    "unterminated list",
			input:   "l3:cat",
			wantErr: true,
		},
		{
			name:    "dictionary key without value",
			input:   "d3:fooe",
			wantErr: true,
		},
		{
			name:    "unterminated dictionary",
			input:   "d3:agei25",
			wantErr: true,
		},
		{
			name:  "list containing dictionary",
			input: "ld3:foo3:baree",
			want: []any{
				map[string]any{
					"foo": "bar",
				},
			},
		},
		{
			name:  "dictionary containing list and dictionary",
			input: "d4:listli1ei2ee4:metad5:admini1eee",
			want: map[string]any{
				"list": []any{1, 2},
				"meta": map[string]any{
					"admin": 1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Unmarshal([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
