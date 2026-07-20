package bencode

import (
	"reflect"
	"strings"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    any
		wantErr bool
	}{
		{
			name:  "positive integer",
			input: "i42e",
			want:  int64(42),
		},
		{
			name:  "negative integer",
			input: "i-42e",
			want:  int64(-42),
		},
		{
			name:  "zero integer",
			input: "i0e",
			want:  int64(0),
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
			want:  []any{int64(42), "cat"},
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
				"age":  int64(25),
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
				"list": []any{int64(1), int64(2)},
			},
		},
		{
			name:  "dictionary containing dictionary",
			input: "d4:metad5:admini1eee",
			want: map[string]any{
				"meta": map[string]any{
					"admin": int64(1),
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
				"list": []any{int64(1), int64(2)},
				"meta": map[string]any{
					"admin": int64(1),
				},
			},
		},
		{
			name:    "leading plus sign on integer",
			input:   "i+42e",
			wantErr: true,
		},
		{
			name:    "negative zero",
			input:   "i-0e",
			wantErr: true,
		},
		{
			name:    "leading zero on positive integer",
			input:   "i03e",
			wantErr: true,
		},
		{
			name:    "leading zero on negative integer",
			input:   "i-03e",
			wantErr: true,
		},
		{
			name:    "bare minus sign",
			input:   "i-e",
			wantErr: true,
		},
		{
			name:  "zero is the only valid single-digit zero form",
			input: "i0e",
			want:  int64(0),
		},
		{
			name:    "leading plus sign on string length (as dict key)",
			input:   "d+3:foo3:bare",
			wantErr: true,
		},
		{
			name:    "leading zero on string length (as dict key)",
			input:   "d03:foo3:bare",
			wantErr: true,
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

func TestUnmarshalDepthGuard(t *testing.T) {
	atLimit := strings.Repeat("l", 1000) + strings.Repeat("e", 1000)
	if _, err := Unmarshal([]byte(atLimit)); err != nil {
		t.Fatalf("1000 levels of nesting should be allowed, got error: %v", err)
	}

	beyondLimit := strings.Repeat("l", 1001) + strings.Repeat("e", 1001)
	if _, err := Unmarshal([]byte(beyondLimit)); err == nil {
		t.Fatal("1001 levels of nesting should be rejected, got no error")
	}
}
