package main

import (
	"reflect"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestTypeDescription(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		want      string
	}{
		{"byte", "y", "byte"},
		{"boolean", "b", "boolean"},
		{"int16", "n", "int16"},
		{"uint16", "q", "uint16"},
		{"int32", "i", "int32"},
		{"uint32", "u", "uint32"},
		{"int64", "x", "int64"},
		{"uint64", "t", "uint64"},
		{"double", "d", "double"},
		{"unix fd", "h", "unix fd"},
		{"string", "s", "string"},
		{"object path", "o", "object path"},
		{"signature", "g", "signature"},
		{"variant", "v", "variant"},
		{"array", "as", "array<string>"},
		{"dictionary", "a{sv}", "dictionary<string, variant>"},
		{"struct", "(suv)", "struct<string, uint32, variant>"},
		{"nested array", "aas", "array<array<string>>"},
		{"array of structs", "a(su)", "array<struct<string, uint32>>"},
		{"nested dictionary", "a{sa{sv}}", "dictionary<string, dictionary<string, variant>>"},
		{"multiple complete types", "su", "string, uint32"},
		{"unknown", "z", "unknown(z)"},
		{"empty", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typeDescription(tt.signature)
			if got != tt.want {
				t.Fatalf("typeDescription(%q) = %q, want %q", tt.signature, got, tt.want)
			}
		})
	}
}

func TestParseDBusInputJSONish(t *testing.T) {
	t.Run("string array json", func(t *testing.T) {
		got, err := parseDBusInput("as", `["alpha","beta"]`)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"alpha", "beta"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("string array comma shorthand", func(t *testing.T) {
		got, err := parseDBusInput("as", `alpha, beta`)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"alpha", "beta"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("empty variant map", func(t *testing.T) {
		got, err := parseDBusInput("a{sv}", ``)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]dbus.Variant{}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("variant map json", func(t *testing.T) {
		got, err := parseDBusInput("a{sv}", `{"name":"demo","count":3,"enabled":true}`)
		if err != nil {
			t.Fatal(err)
		}
		m, ok := got.(map[string]dbus.Variant)
		if !ok {
			t.Fatalf("got %T, want map[string]dbus.Variant", got)
		}
		if m["name"].Value() != "demo" {
			t.Fatalf("name = %#v", m["name"].Value())
		}
		if m["count"].Value() != int32(3) {
			t.Fatalf("count = %#v", m["count"].Value())
		}
		if m["enabled"].Value() != true {
			t.Fatalf("enabled = %#v", m["enabled"].Value())
		}
	})
}
