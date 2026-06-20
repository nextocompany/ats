package auth

import (
	"encoding/json"
	"strings"
	"testing"
)

const testClientID = "57C7D338-47BE-4726-BEC5-560853620D1F"

func TestExtensionClaimPrefix(t *testing.T) {
	// dashes stripped, lower-cased.
	want := "extension_57c7d33847be4726bec5560853620d1f_"
	if got := extensionClaimPrefix(testClientID); got != want {
		t.Fatalf("extensionClaimPrefix=%q want %q", got, want)
	}
}

func TestResolveStoreID(t *testing.T) {
	ext := extensionClaimPrefix(testClientID) + "store_id"
	cases := []struct {
		name string
		raw  map[string]any
		want *int
	}{
		{"short name, number (json default float64)", map[string]any{"store_id": float64(7)}, intPtr(7)},
		{"short name, string", map[string]any{"store_id": "7"}, intPtr(7)},
		{"extension name, string (directory extension shape)", map[string]any{ext: "12"}, intPtr(12)},
		{"short name preferred over extension", map[string]any{"store_id": float64(3), ext: "99"}, intPtr(3)},
		{"absent", map[string]any{}, nil},
		{"blank string", map[string]any{"store_id": "  "}, nil},
		{"unparseable string", map[string]any{"store_id": "abc"}, nil},
		{"wrong type bool", map[string]any{"store_id": true}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveStoreID(tc.raw, testClientID)
			if (got == nil) != (tc.want == nil) {
				t.Fatalf("nil-ness mismatch: got %v want %v", got, tc.want)
			}
			if got != nil && *got != *tc.want {
				t.Fatalf("got %d want %d", *got, *tc.want)
			}
		})
	}
}

func TestResolveSubregion(t *testing.T) {
	ext := extensionClaimPrefix(testClientID) + "subregion"
	cases := []struct {
		name string
		raw  map[string]any
		want string
	}{
		{"short name", map[string]any{"subregion": "East"}, "East"},
		{"extension name", map[string]any{ext: "West"}, "West"},
		{"short preferred over extension", map[string]any{"subregion": "East", ext: "West"}, "East"},
		{"trimmed", map[string]any{"subregion": "  North  "}, "North"},
		{"absent", map[string]any{}, ""},
		{"blank", map[string]any{"subregion": "   "}, ""},
		{"wrong type", map[string]any{"subregion": float64(3)}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveSubregion(tc.raw, testClientID); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestCoerceIntJSONNumber(t *testing.T) {
	// A decoder using UseNumber yields json.Number; confirm that path works too.
	var raw map[string]any
	dec := json.NewDecoder(strings.NewReader(`{"store_id": 42}`))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := resolveStoreID(raw, testClientID)
	if got == nil || *got != 42 {
		t.Fatalf("got %v want 42", got)
	}
}
