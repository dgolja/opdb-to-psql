// code with all the load to from SQL to struct
package pgsql

import (
	"context"
	"fmt"

	"github.com/dgolja/opdb-to-psql/tools/opdb-importer/internal/opdbv2"
	"github.com/jackc/pgx/v5"
)

// LoadFromDB reads the whole database into i.Data: all entries with their
// manufacturers, features, people and images. It is the reverse of LoadToDB.
func (i *OPDBv2) LoadFromDB(ctx context.Context) error {
	entries, err := i.loadOPDB(ctx)
	if err != nil {
		return err
	}
	i.Data = &opdbv2.Export{Entries: entries}

	if err := i.loadManufacturers(ctx); err != nil {
		return err
	}

	if err := i.loadFeatures(ctx); err != nil {
		return err
	}

	if err := i.loadPeople(ctx); err != nil {
		return err
	}

	return i.loadImages(ctx)
}

// loadOPDB returns all opdb entries, without their manufacturer, features, people or images.
func (i *OPDBv2) loadOPDB(ctx context.Context) ([]opdbv2.OPDB, error) {

	rows, err := i.conn.Query(ctx, `
		SELECT
		  opdb_id, opdb_group, opdb_machine,
		  name, short_name, common_name,
		  name_sort, year, manufacture_date,
		  description, type, display,
		  player_count, physical_machine, manufacturer_id,
		  ipdb_id, pinball_primer_url, pinball_rules_url,
		  pinball_cards_url, bobs_guide_url, competition_setup_url,
		  competition_notes_url, has_competition_notes, has_competition_setup,
		  created_at, updated_at, entry_type,
		  keywords
		FROM opdb ORDER BY opdb_id;
	`)
	if err != nil {
		return nil, fmt.Errorf("opdb query failed: %w", err)
	}
	defer rows.Close()

	opdbs, err := pgx.CollectRows(rows, pgx.RowToStructByName[opdbv2.OPDB])
	if err != nil {
		return nil, fmt.Errorf("collecting opdb rows: %w", err)
	}

	return opdbs, nil
}

// loadManufacturers attaches the matching manufacturer to every entry that has a
// manufacturer_id, and returns an error if that manufacturer does not exist.
// Runs after loadOPDB.
func (i *OPDBv2) loadManufacturers(ctx context.Context) error {

	rows, err := i.conn.Query(ctx, `SELECT manufacturer_id, name, full_name FROM manufacturers;`)
	if err != nil {
		return fmt.Errorf("manufacturers query failed: %w", err)
	}

	defer rows.Close()

	manufacturers, err := pgx.CollectRows(rows, pgx.RowToStructByName[opdbv2.Manufacturer])
	if err != nil {
		return fmt.Errorf("collecting rows: %w", err)
	}

	manufacturersByID := make(map[int]*opdbv2.Manufacturer, len(manufacturers))
	for idx := range manufacturers {
		manufacturer := &manufacturers[idx]
		manufacturersByID[manufacturer.ManufacturerID] = manufacturer
	}

	for idx, entry := range i.Data.Entries {
		if entry.ManufacturerID == nil {
			continue
		}

		value, ok := manufacturersByID[*entry.ManufacturerID]
		if !ok {
			// Unlikely: the opdb.manufacturer_id foreign key guarantees the
			// manufacturer exists. Checked anyway as a safeguard.
			return fmt.Errorf("opdb entry %s references unknown manufacturer %d", entry.OPDBID, *entry.ManufacturerID)
		}
		i.Data.Entries[idx].Manufacturer = value
	}
	return nil
}

// loadFeatures attaches the list of features to every entry, using the
// opdb_features join table. Entries without features get an empty list.
func (i *OPDBv2) loadFeatures(ctx context.Context) error {

	// retrieve all features since they are not many
	rows, err := i.conn.Query(ctx, `SELECT feature_id, name, group_name FROM features ORDER BY feature_id;`)
	if err != nil {
		return fmt.Errorf("features query failed: %w", err)
	}
	defer rows.Close()

	features, err := pgx.CollectRows(rows, pgx.RowToStructByName[opdbv2.Feature])
	if err != nil {
		return fmt.Errorf("collecting rows: %w", err)
	}

	featuresByID := make(map[int]*opdbv2.Feature, len(features))
	for idx := range features {
		feature := &features[idx]
		featuresByID[feature.FeatureID] = feature
	}

	// retrieve all features for all entries in one go
	rows, err = i.conn.Query(ctx, `SELECT opdb_id, feature_id FROM opdb_features ORDER BY feature_id;`)
	if err != nil {
		return fmt.Errorf("opdb_features query failed: %w", err)
	}

	defer rows.Close()

	var opdbFeatures = make(map[string][]int)

	for rows.Next() {
		var opdbID string
		var featureId int
		if err := rows.Scan(&opdbID, &featureId); err != nil {
			return fmt.Errorf("opdb_features query scan failed: %w", err)
		}
		opdbFeatures[opdbID] = append(opdbFeatures[opdbID], featureId)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("opdb_features row iteration failed: %w", err)
	}

	// pair feature with an entry
	for idx, entry := range i.Data.Entries {
		i.Data.Entries[idx].Features = []opdbv2.Feature{}
		if value, ok := opdbFeatures[entry.OPDBID]; ok {
			for _, v := range value {
				feature, ok := featuresByID[v]
				if !ok {
					// Unlikely: the opdb_features foreign key guarantees the feature
					// exists. Checked anyway to avoid a nil dereference.
					return fmt.Errorf("opdb entry %s references unknown feature %d", entry.OPDBID, v)
				}
				i.Data.Entries[idx].Features = append(i.Data.Entries[idx].Features, *feature)
			}
		}
	}
	return nil
}

// loadPeople attaches the list of people (with their role and index) to every
// entry, using the opdb_people join table. Entries without people get an empty list.
func (i *OPDBv2) loadPeople(ctx context.Context) error {

	// retrieve all people since they are not many
	rows, err := i.conn.Query(ctx, `
		SELECT opdb_people.opdb_id, people.opdb_person_id, name, role_name, person_index
		  FROM people, opdb_people
		  WHERE people.opdb_person_id = opdb_people.opdb_person_id ORDER BY opdb_people.opdb_person_id;
	`)
	if err != nil {
		return fmt.Errorf("people query failed: %w", err)
	}

	defer rows.Close()
	var people = make(map[string][]opdbv2.Person)

	for rows.Next() {
		var opdbID, name, role string
		var OpdbPersonID, index int

		if err := rows.Scan(&opdbID, &OpdbPersonID, &name, &role, &index); err != nil {
			return fmt.Errorf("people, opdb_people query scan failed: %w", err)
		}
		people[opdbID] = append(people[opdbID], opdbv2.Person{OpdbPersonID: OpdbPersonID, Name: name, Role: role, Index: index})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("people, opdb_people row iteration failed: %w", err)
	}

	// pair people with an entry
	for idx, entry := range i.Data.Entries {
		i.Data.Entries[idx].People = []opdbv2.Person{}
		if value, ok := people[entry.OPDBID]; ok {
			i.Data.Entries[idx].People = value
		}
	}
	return nil
}

// loadImages attaches the list of images to every entry, using the opdb_images
// join table. Entries without images get an empty list.
func (i *OPDBv2) loadImages(ctx context.Context) error {
	images, err := i.loadAllImages(ctx)
	if err != nil {
		return err
	}

	// retrieve all images per OPDB entry
	rows, err := i.conn.Query(ctx, `SELECT opdb_id, group_id FROM opdb_images;`)
	if err != nil {
		return fmt.Errorf("opdb_images query failed: %w", err)
	}

	defer rows.Close()

	opdbImages := map[string][]string{}

	for rows.Next() {
		var opdbID, groupID string
		if err := rows.Scan(&opdbID, &groupID); err != nil {
			return fmt.Errorf("opdb_images query scan failed: %w", err)
		}
		opdbImages[opdbID] = append(opdbImages[opdbID], groupID)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("opdb_images row iteration failed: %w", err)
	}

	// add images to the main data source
	for idx, entry := range i.Data.Entries {
		i.Data.Entries[idx].Images = []opdbv2.Image{}
		if opdbIDImages, ok := opdbImages[entry.OPDBID]; ok {
			for _, groupID := range opdbIDImages {
				image, ok := images[groupID]
				if !ok {
					// Unlikely: the opdb_images foreign key guarantees the image
					// exists. Checked anyway as a safeguard.
					return fmt.Errorf("opdb entry %s references unknown image group %q", entry.OPDBID, groupID)
				}
				i.Data.Entries[idx].Images = append(i.Data.Entries[idx].Images, image)
			}
		}
	}
	return nil
}

// loadAllImages returns every image keyed by group ID, with its small, medium
// and large variants (URL and size) merged in from image_variants.
func (i *OPDBv2) loadAllImages(ctx context.Context) (map[string]opdbv2.Image, error) {
	// Query 1: base image rows
	imgRows, err := i.conn.Query(ctx, `SELECT group_id, title, is_primary, type FROM images`)
	if err != nil {
		return nil, fmt.Errorf("querying images: %w", err)
	}
	defer imgRows.Close()
	images, err := pgx.CollectRows(imgRows, pgx.RowToStructByName[opdbv2.Image])
	if err != nil {
		return nil, fmt.Errorf("collecting images: %w", err)
	}

	// Index by group_id for O(1) lookup while merging variants
	imagesByGroup := make(map[string]opdbv2.Image, len(images))
	for _, img := range images {
		imagesByGroup[img.Group] = img
	}

	type imageVariantRow struct {
		GroupID string `db:"group_id"`
		Variant string `db:"variant"`
		URL     string `db:"url"`
		Width   int    `db:"width"`
		Height  int    `db:"height"`
	}

	// Query 2: all variants, in one shot (not per-image)
	varRows, err := i.conn.Query(ctx, `SELECT group_id, variant, url, width, height FROM image_variants`)
	if err != nil {
		return nil, fmt.Errorf("querying image_variants: %w", err)
	}
	defer varRows.Close()
	variants, err := pgx.CollectRows(varRows, pgx.RowToStructByName[imageVariantRow])
	if err != nil {
		return nil, fmt.Errorf("collecting image_variants: %w", err)
	}

	// Merge variants into their parent image
	for _, v := range variants {
		img, ok := imagesByGroup[v.GroupID]
		if !ok {
			continue // orphaned variant, shouldn't happen given the FK constraint
		}

		size := opdbv2.Size{Width: v.Width, Height: v.Height}

		switch v.Variant {
		case "medium":
			img.URLs.Medium = v.URL
			img.Sizes.Medium = size
		case "large":
			img.URLs.Large = v.URL
			img.Sizes.Large = size
		case "small":
			img.URLs.Small = v.URL
			img.Sizes.Small = size
		}

		imagesByGroup[v.GroupID] = img
	}

	return imagesByGroup, nil
}
