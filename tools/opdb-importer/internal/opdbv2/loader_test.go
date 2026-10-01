package opdbv2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleExport = `{"entries":[{
  "opdbId":"G1-M2","opdbGroup":"G1","name":"Test Machine","nameSort":"test machine",
  "year":1990,"manufactureDate":"1990-01-02","physicalMachine":true,
  "createdAt":"2024-01-01T00:00:00.000000Z","updatedAt":"2024-01-02T00:00:00.000000Z",
  "entryType":"machine","manufacturerId":8,
  "manufacturer":{"manufacturerId":8,"name":"Bally","fullName":"Bally Manufacturing Co."},
  "features":[{"featureId":1,"name":"Ramp","group":"playfield"}],
  "people":[{"opdbPersonId":3,"name":"Jane","role":"Design","index":0}],
  "images":[{"group":"img1","primary":true,"type":"backglass","urls":{"large":"http://x/l.jpg"},"sizes":{"large":{"width":10,"height":20}}}],
  "keywords":["a","b"]
}]}`

func TestLoadFromReader(t *testing.T) {
	exp, err := LoadFromReader(strings.NewReader(sampleExport))
	if err != nil {
		t.Fatal(err)
	}
	if len(exp.Entries) != 1 {
		t.Fatalf("got %d entries", len(exp.Entries))
	}
	e := exp.Entries[0]
	if e.OPDBID != "G1-M2" || e.Year != 1990 || !e.PhysicalMachine {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if e.Manufacturer == nil || e.Manufacturer.Name != "Bally" {
		t.Fatalf("manufacturer not parsed: %+v", e.Manufacturer)
	}
	if len(e.Features) != 1 || len(e.People) != 1 || len(e.Images) != 1 || len(e.Keywords) != 2 {
		t.Fatalf("nested data not parsed: %+v", e)
	}
	img := e.Images[0]
	if img.URLs.Large == "" || img.Sizes.Large.IsEmpty() || !img.Sizes.Small.IsEmpty() {
		t.Fatalf("image variants not parsed: %+v", img)
	}
	if e.OPDBMachine != nil {
		t.Fatal("missing optional field should be nil")
	}
}

func TestLoadFromReaderInvalidJSON(t *testing.T) {
	if _, err := LoadFromReader(strings.NewReader(`{`)); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadFromFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(p, []byte(sampleExport), 0o644); err != nil {
		t.Fatal(err)
	}
	exp, err := LoadFromFile(p)
	if err != nil || len(exp.Entries) != 1 {
		t.Fatalf("got %v, %v", exp, err)
	}
	if _, err := LoadFromFile(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
