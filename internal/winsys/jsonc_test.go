package winsys

import (
	"reflect"
	"testing"
)

func TestParseJSONC(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want map[string]any
	}{
		{
			name: "line comments",
			in: `{
				// Windows Terminal writes comments like this
				"defaultProfile": "{abc}", // trailing note
				"profiles": {"list": []}
			}`,
			want: map[string]any{"defaultProfile": "{abc}"},
		},
		{
			name: "block comments",
			in: `{
				/* block
				   comment */
				"schemes": []
			}`,
			want: map[string]any{"schemes": []any{}},
		},
		{
			name: "trailing commas",
			in: `{
				"a": 1,
				"b": [1, 2,],
				"c": {"d": 2,},
			}`,
			want: map[string]any{"a": float64(1), "b": []any{float64(1), float64(2)}, "c": map[string]any{"d": float64(2)}},
		},
		{
			name: "comment markers inside strings survive",
			in: `{
				"url": "https://example.com//path",
				"glob": "/* not a comment */",
				"escaped": "quote \" // still string"
			}`,
			want: map[string]any{
				"url":     "https://example.com//path",
				"glob":    "/* not a comment */",
				"escaped": "quote \" // still string",
			},
		},
		{
			name: "bom",
			in:   "\ufeff{\"a\":1}",
			want: map[string]any{"a": float64(1)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseJSONC([]byte(tc.in))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			for key, wantValue := range tc.want {
				gotValue, ok := got[key]
				if !ok {
					t.Fatalf("missing key %q in %#v", key, got)
				}
				switch wantValue := wantValue.(type) {
				case string:
					if gotValue != wantValue {
						t.Errorf("%s = %v, want %q", key, gotValue, wantValue)
					}
				default:
					if !reflect.DeepEqual(gotValue, wantValue) {
						t.Errorf("%s = %#v, want %#v", key, gotValue, wantValue)
					}
				}
			}
		})
	}
}
