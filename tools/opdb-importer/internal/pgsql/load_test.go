package pgsql

import (
	"strings"
	"testing"

	"github.com/dgolja/opdb-to-psql/tools/opdb-importer/internal/opdbv2"
	"github.com/pashagolub/pgxmock/v5"
)

func TestLoadFeaturesUnknownFeature(t *testing.T) {
	mock := newMock(t)
	mock.ExpectQuery("SELECT feature_id").WillReturnRows(
		pgxmock.NewRows([]string{"feature_id", "name", "group_name"}).AddRow(1, "Ramp", "playfield"))
	mock.ExpectQuery("SELECT opdb_id, feature_id").WillReturnRows(
		pgxmock.NewRows([]string{"opdb_id", "feature_id"}).AddRow("A", 99))

	db := NewWithDB(mock, &opdbv2.Export{Entries: []opdbv2.OPDB{{OPDBID: "A"}}})
	err := db.loadFeatures(t.Context())
	if err == nil || !strings.Contains(err.Error(), "unknown feature 99") {
		t.Fatalf("expected unknown feature error, got %v", err)
	}
	done(t, mock)
}

func TestLoadImagesUnknownGroup(t *testing.T) {
	mock := newMock(t)
	mock.ExpectQuery("SELECT group_id, title").WillReturnRows(
		pgxmock.NewRows([]string{"group_id", "title", "is_primary", "type"}).AddRow("img1", (*string)(nil), true, "backglass"))
	mock.ExpectQuery("SELECT group_id, variant").WillReturnRows(
		pgxmock.NewRows([]string{"group_id", "variant", "url", "width", "height"}))
	mock.ExpectQuery("SELECT opdb_id, group_id").WillReturnRows(
		pgxmock.NewRows([]string{"opdb_id", "group_id"}).AddRow("A", "missing"))

	db := NewWithDB(mock, &opdbv2.Export{Entries: []opdbv2.OPDB{{OPDBID: "A"}}})
	err := db.loadImages(t.Context())
	if err == nil || !strings.Contains(err.Error(), `unknown image group "missing"`) {
		t.Fatalf("expected unknown image error, got %v", err)
	}
	done(t, mock)
}

func TestLoadManufacturersUnknownManufacturer(t *testing.T) {
	mock := newMock(t)
	mock.ExpectQuery("SELECT manufacturer_id").WillReturnRows(
		pgxmock.NewRows([]string{"manufacturer_id", "name", "full_name"}).AddRow(8, "Bally", "Bally Manufacturing Co."))

	id := 99
	db := NewWithDB(mock, &opdbv2.Export{Entries: []opdbv2.OPDB{{OPDBID: "A", ManufacturerID: &id}}})
	err := db.loadManufacturers(t.Context())
	if err == nil || !strings.Contains(err.Error(), "unknown manufacturer 99") {
		t.Fatalf("expected unknown manufacturer error, got %v", err)
	}
	done(t, mock)
}
