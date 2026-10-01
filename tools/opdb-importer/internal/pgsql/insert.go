package pgsql

import (
	"context"
	"fmt"

	"github.com/dgolja/opdb-to-psql/tools/opdb-importer/internal/opdbv2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// tally counts, for one table, the rows inserted and the rows skipped because
// ON CONFLICT DO NOTHING found an existing row.
type tally struct {
	table    string
	inserted int64
	skipped  int64
}

// record counts the outcome of a single-row INSERT ... ON CONFLICT DO NOTHING.
func (t *tally) record(tag pgconn.CommandTag) {
	if n := tag.RowsAffected(); n > 0 {
		t.inserted += n
	} else {
		t.skipped++
	}
}

// logTallies reports every tally at Info level. Inside LoadToDB the counts only
// become final once the transaction commits ("import committed").
func (i OPDBv2) logTallies(ctx context.Context, tallies ...*tally) {
	for _, t := range tallies {
		i.log.InfoContext(ctx, "step done", "table", t.table, "inserted", t.inserted, "skipped", t.skipped)
	}
}

// LoadOptions controls LoadToDB.
type LoadOptions struct {
	// Truncate empties every importer table before loading, inside the same
	// transaction as the load.
	Truncate bool
}

// LoadToDB inserts all entries of i.Data into the database in a single
// transaction: either everything is loaded (and truncated first, if requested)
// or, on any error, nothing changes. Parent tables (manufacturers, people,
// features, images) are loaded before opdb entries so foreign keys resolve.
// Existing rows are left untouched (ON CONFLICT DO NOTHING).
func (i OPDBv2) LoadToDB(ctx context.Context, opts LoadOptions) error {
	err := pgx.BeginFunc(ctx, i.conn, func(tx pgx.Tx) error {
		if opts.Truncate {
			i.log.InfoContext(ctx, "truncating tables")
			if err := truncateTables(ctx, tx); err != nil {
				return fmt.Errorf("truncating tables: %w", err)
			}
		}

		steps := []struct {
			name string
			run  func(context.Context, pgx.Tx) ([]*tally, error)
		}{
			{"manufacturers", i.insertManufacturers},
			{"people", i.insertPeople},
			{"features", i.insertFeatures},
			{"images", i.insertImages},
			{"opdb entries", i.insertOPDB},
		}
		for _, step := range steps {
			i.log.InfoContext(ctx, "inserting", "table", step.name)
			t, err := step.run(ctx, tx)
			if err != nil {
				return err
			}
			// Logged as each step finishes. They are not final until the commit below.
			i.logTallies(ctx, t...)
		}
		return nil
	})
	if err != nil {
		i.log.WarnContext(ctx, "import failed, transaction rolled back: nothing was saved, including any counts logged above")
		return err
	}

	i.log.InfoContext(ctx, "import committed")
	return nil
}

// uniqueManufacturers returns the distinct manufacturers keyed by ID.
func uniqueManufacturers(entries []opdbv2.OPDB) map[int]opdbv2.Manufacturer {
	manufacturers := make(map[int]opdbv2.Manufacturer)
	for _, entry := range entries {
		if entry.Manufacturer != nil {
			if _, exists := manufacturers[entry.Manufacturer.ManufacturerID]; !exists {
				manufacturers[entry.Manufacturer.ManufacturerID] = *entry.Manufacturer
			}
		}
	}
	return manufacturers
}

// insertManufacturers inserts the distinct manufacturers referenced by the entries.
func (i OPDBv2) insertManufacturers(ctx context.Context, tx pgx.Tx) ([]*tally, error) {
	manufacturers := uniqueManufacturers(i.Data.Entries)
	t := &tally{table: tableManufacturers}
	for _, m := range manufacturers {
		tag, err := tx.Exec(ctx,
			`INSERT INTO manufacturers (manufacturer_id, name, full_name)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (manufacturer_id) DO NOTHING`,
			m.ManufacturerID, m.Name, m.FullName,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting manufacturer %d: %w", m.ManufacturerID, err)
		}
		t.record(tag)
	}
	return []*tally{t}, nil
}

// uniquePeople returns the distinct people keyed by person ID.
func uniquePeople(entries []opdbv2.OPDB) map[int]opdbv2.Person {
	people := make(map[int]opdbv2.Person)
	for _, entry := range entries {
		for _, person := range entry.People {
			if _, exists := people[person.OpdbPersonID]; !exists {
				people[person.OpdbPersonID] = person
			}
		}
	}
	return people
}

// insertPeople inserts the distinct people referenced by the entries.
func (i OPDBv2) insertPeople(ctx context.Context, tx pgx.Tx) ([]*tally, error) {
	people := uniquePeople(i.Data.Entries)
	t := &tally{table: tablePeople}
	for _, p := range people {
		tag, err := tx.Exec(ctx,
			`INSERT INTO people (opdb_person_id, name)
			 VALUES ($1, $2)
			 ON CONFLICT (opdb_person_id, name) DO NOTHING`,
			p.OpdbPersonID, p.Name,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting person %d: %w", p.OpdbPersonID, err)
		}
		t.record(tag)
	}
	return []*tally{t}, nil
}

// uniqueFeatures returns the distinct features keyed by feature ID.
func uniqueFeatures(entries []opdbv2.OPDB) map[int]opdbv2.Feature {
	features := make(map[int]opdbv2.Feature)
	for _, entry := range entries {
		for _, feature := range entry.Features {
			if _, exists := features[feature.FeatureID]; !exists {
				features[feature.FeatureID] = feature
			}
		}
	}
	return features
}

// insertFeatures inserts the distinct features referenced by the entries.
func (i OPDBv2) insertFeatures(ctx context.Context, tx pgx.Tx) ([]*tally, error) {
	features := uniqueFeatures(i.Data.Entries)
	t := &tally{table: tableFeatures}
	for _, f := range features {
		tag, err := tx.Exec(ctx,
			`INSERT INTO features (feature_id, name, group_name)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (feature_id) DO NOTHING`,
			f.FeatureID, f.Name, f.Group,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting feature %d: %w", f.FeatureID, err)
		}
		t.record(tag)
	}
	return []*tally{t}, nil
}

// uniqueImages returns the distinct images keyed by group ID.
func uniqueImages(entries []opdbv2.OPDB) map[string]opdbv2.Image {
	images := make(map[string]opdbv2.Image)
	for _, entry := range entries {
		for _, image := range entry.Images {
			if _, exists := images[image.Group]; !exists {
				images[image.Group] = image
			}
		}
	}
	return images
}

// insertImages inserts the distinct images referenced by the entries.
func (i OPDBv2) insertImages(ctx context.Context, tx pgx.Tx) ([]*tally, error) {
	images := uniqueImages(i.Data.Entries)
	imgTally := &tally{table: tableImages}
	variantTally := &tally{table: tableImageVariants}
	for _, image := range images {
		tag, err := tx.Exec(ctx,
			`INSERT INTO images (group_id, title, is_primary, type)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (group_id) DO NOTHING`,
			image.Group, image.Title, image.Primary, image.Type,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting image %q: %w", image.Group, err)
		}
		imgTally.record(tag)
		variants := []struct {
			name string
			url  string
			size opdbv2.Size
		}{
			{"medium", image.URLs.Medium, image.Sizes.Medium},
			{"large", image.URLs.Large, image.Sizes.Large},
			{"small", image.URLs.Small, image.Sizes.Small},
		}

		for _, v := range variants {
			if v.url == "" {
				continue
			}

			tag, err := tx.Exec(ctx,
				`INSERT INTO image_variants (group_id, variant, url, width, height)
				 VALUES ($1, $2, $3, $4, $5)
				 ON CONFLICT (group_id, variant) DO NOTHING`,
				image.Group, v.name, v.url, v.size.Width, v.size.Height,
			)
			if err != nil {
				return nil, fmt.Errorf("inserting image variant %q for %s: %w", v.name, image.Group, err)
			}
			variantTally.record(tag)
		}
	}
	return []*tally{imgTally, variantTally}, nil
}

// insertOPDB inserts the opdb entries and their links to people, features and images.
// Must run after the parent tables have been populated.
func (i OPDBv2) insertOPDB(ctx context.Context, tx pgx.Tx) ([]*tally, error) {
	opdbTally := &tally{table: tableOPDB}
	peopleTally := &tally{table: tableOPDBPeople}
	featuresTally := &tally{table: tableOPDBFeatures}
	imagesTally := &tally{table: tableOPDBImages}
	for _, entry := range i.Data.Entries {
		tag, err := tx.Exec(ctx,
			`INSERT INTO opdb
			  (opdb_id, opdb_group, opdb_machine,
			  name, short_name, common_name,
			  name_sort, year, manufacture_date,
			  description, type, display,
			  player_count, physical_machine, manufacturer_id,
			  ipdb_id, pinball_primer_url, pinball_rules_url,
			  pinball_cards_url, bobs_guide_url, competition_setup_url,
			  competition_notes_url, has_competition_notes, has_competition_setup,
			  created_at, updated_at, entry_type,
			  keywords)
			 VALUES (
			 	$1, $2, $3, $4, $5,
			 	$6, $7, $8, $9, $10,
			 	$11, $12, $13, $14, $15,
			 	$16, $17, $18, $19, $20,
			 	$21, $22, $23, $24, $25,
			 	$26, $27, $28
			 )
			 ON CONFLICT (opdb_id) DO NOTHING`,
			entry.OPDBID, entry.OPDBGroup, entry.OPDBMachine,
			entry.Name, entry.ShortName, entry.CommonName,
			entry.NameSort, entry.Year, entry.ManufactureDate,
			entry.Description, entry.Type, entry.Display,
			entry.PlayerCount, entry.PhysicalMachine, entry.ManufacturerID,
			entry.IpdbID, entry.PinballPrimerURL, entry.PinballRulesURL,
			entry.PinballCardsURL, entry.BobsGuideURL, entry.CompetitionSetupURL,
			entry.CompetitionNotesURL, entry.HasCompetitionNotes, entry.HasCompetitionSetup,
			entry.CreatedAt, entry.UpdatedAt, entry.EntryType,
			entry.Keywords,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting OPDB %s: %w", entry.OPDBID, err)
		}
		opdbTally.record(tag)
		// Let's add the people relations
		for _, person := range entry.People {
			tag, err := tx.Exec(ctx,
				`INSERT INTO opdb_people (opdb_id, opdb_person_id, role_name, person_index)
				 VALUES ($1, $2, $3, $4)
				 ON CONFLICT (opdb_id, opdb_person_id,role_name) DO NOTHING`,
				entry.OPDBID, person.OpdbPersonID, person.Role, person.Index,
			)
			if err != nil {
				return nil, fmt.Errorf("inserting person %d for OPDB %s: %w", person.OpdbPersonID, entry.OPDBID, err)
			}
			peopleTally.record(tag)
		}
		// Let's add the features
		for _, feature := range entry.Features {
			tag, err := tx.Exec(ctx,
				`INSERT INTO opdb_features (opdb_id, feature_id)
				 VALUES ($1, $2)
				 ON CONFLICT (opdb_id, feature_id) DO NOTHING`,
				entry.OPDBID, feature.FeatureID,
			)
			if err != nil {
				return nil, fmt.Errorf("inserting feature %d for OPDB %s: %w", feature.FeatureID, entry.OPDBID, err)
			}
			featuresTally.record(tag)
		}
		// Let's add images
		for _, image := range entry.Images {
			tag, err := tx.Exec(ctx,
				`INSERT INTO opdb_images (opdb_id, group_id)
				 VALUES ($1, $2)
				 ON CONFLICT (opdb_id, group_id) DO NOTHING`,
				entry.OPDBID, image.Group,
			)
			if err != nil {
				return nil, fmt.Errorf("inserting image %q for OPDB %s: %w", image.Group, entry.OPDBID, err)
			}
			imagesTally.record(tag)
		}
	}
	return []*tally{opdbTally, peopleTally, featuresTally, imagesTally}, nil
}
