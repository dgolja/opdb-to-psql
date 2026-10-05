package opdbv2

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDateJSONRoundTrip(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"1978-03-15"`), &d); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(1978, 3, 15, 0, 0, 0, 0, time.UTC); !d.Equal(want) {
		t.Fatalf("got %v, want %v", d.Time, want)
	}
	out, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `"1978-03-15"` {
		t.Fatalf("got %s", out)
	}
}

func TestDateEmptyAndInvalid(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`""`), &d); err != nil || !d.IsZero() {
		t.Fatalf("empty string should leave zero value, got %v, %v", d.Time, err)
	}
	if err := json.Unmarshal([]byte(`"15/03/1978"`), &d); err == nil {
		t.Fatal("expected error for bad layout")
	}
	out, _ := json.Marshal(Date{})
	if string(out) != "null" {
		t.Fatalf("zero date should marshal to null, got %s", out)
	}
}

func TestTimestampJSONRoundTrip(t *testing.T) {
	const in = `"2024-05-06T07:08:09.123456Z"`
	var ts Timestamp
	if err := json.Unmarshal([]byte(in), &ts); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(ts)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != in {
		t.Fatalf("got %s, want %s", out, in)
	}
}

func TestTimestampInvalid(t *testing.T) {
	var ts Timestamp
	if err := json.Unmarshal([]byte(`"2024-05-06"`), &ts); err == nil {
		t.Fatal("expected error for date-only value")
	}
}

func TestScanAndValue(t *testing.T) {
	now := time.Now()

	var d Date
	if err := d.Scan(now); err != nil || !d.Equal(now) {
		t.Fatalf("Date.Scan: %v, %v", d.Time, err)
	}
	if err := d.Scan("nope"); err == nil {
		t.Fatal("Date.Scan should reject non-time values")
	}
	if v, _ := (Date{}).Value(); v != nil {
		t.Fatalf("zero Date should be NULL, got %v", v)
	}

	var ts Timestamp
	if err := ts.Scan(now); err != nil || !ts.Equal(now) {
		t.Fatalf("Timestamp.Scan: %v, %v", ts.Time, err)
	}
	if err := ts.Scan(42); err == nil {
		t.Fatal("Timestamp.Scan should reject non-time values")
	}
	if v, _ := ts.Value(); v == nil {
		t.Fatal("non-zero Timestamp should not be NULL")
	}
}
