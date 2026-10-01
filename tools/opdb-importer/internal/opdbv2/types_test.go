package opdbv2

import (
	"encoding/json"
	"testing"
)

func TestManufacturerUnmarshalCleansNames(t *testing.T) {
	in := `{"manufacturerId": 7, "name": " Komplett\u0000  ", "fullName": "Komplett Flipper\r\n(A. H. Geiger Co.)"}`
	var m Manufacturer
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatal(err)
	}
	if m.ManufacturerID != 7 || m.Name != "Komplett" {
		t.Fatalf("unexpected manufacturer: %+v", m)
	}
	if want := "Komplett Flipper (A. H. Geiger Co.)"; m.FullName != want {
		t.Fatalf("got %q, want %q", m.FullName, want)
	}
}

func TestManufacturerUnmarshalInvalid(t *testing.T) {
	var m Manufacturer
	if err := json.Unmarshal([]byte(`{"manufacturerId": "x"}`), &m); err == nil {
		t.Fatal("expected type error")
	}
}

func TestSizeIsEmpty(t *testing.T) {
	if !(Size{}).IsEmpty() {
		t.Fatal("zero size should be empty")
	}
	if (Size{Width: 1}).IsEmpty() {
		t.Fatal("size with width should not be empty")
	}
}
