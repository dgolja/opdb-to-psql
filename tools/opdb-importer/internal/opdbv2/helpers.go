package opdbv2

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

// Date custom Date type to satisfy both pgx and encoding/json, so that we can convert
// from SQL to json and from json to SQL.
type Date struct {
	time.Time
}

// UnmarshalJSON parses a plain "YYYY-MM-DD" JSON string.
func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		return nil
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return fmt.Errorf("parsing date %q: %w", s, err)
	}
	d.Time = t
	return nil
}

// MarshalJSON writes back out as "YYYY-MM-DD", or null when zero.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Format(dateLayout))
}

// Scan implements database/sql.Scanner, which pgx v5 uses as a fallback
// for custom Go types when scanning query results.
func (d *Date) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	t, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("Date.Scan: unsupported type %T", value)
	}
	d.Time = t
	return nil
}

// Value implements database/sql/driver.Valuer, for the reverse direction
// (inserting a Date value back into Postgres).
func (d Date) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.Time, nil
}

const timestampLayout = "2006-01-02T15:04:05.000000Z"

// Timestamp custom Timestamp type to satisfy both pgx and encoding/json, so that we can convert
// from SQL to json and from json to SQL.
type Timestamp struct {
	time.Time
}

// UnmarshalJSON parses a "2006-01-02T15:04:05.000000Z" JSON string.
func (d *Timestamp) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		return nil
	}
	t, err := time.Parse(timestampLayout, s)
	if err != nil {
		return fmt.Errorf("parsing timestamp %q: %w", s, err)
	}
	d.Time = t
	return nil
}

// MarshalJSON writes back out as a UTC "2006-01-02T15:04:05.000000Z" string, or null when zero.
func (d Timestamp) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Time.UTC().Format(timestampLayout))
}

// Scan implements database/sql.Scanner, which pgx v5 uses as a fallback
// for custom Go types when scanning query results.
func (d *Timestamp) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	t, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("Timestamp.Scan: unsupported type %T", value)
	}
	d.Time = t
	return nil
}

// Value implements database/sql/driver.Valuer, for the reverse direction
// (inserting a Timestamp value back into Postgres).
func (d Timestamp) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.Time, nil
}
