package opdbv2

import (
	"encoding/json"
	"strings"
	"unicode"
)

type Export struct {
	Entries []OPDB `json:"entries"`
}

type OPDB struct {
	OPDBID          string  `json:"opdbId" db:"opdb_id"`
	OPDBGroup       string  `json:"opdbGroup" db:"opdb_group"`
	OPDBMachine     *string `json:"opdbMachine" db:"opdb_machine"`
	Name            string  `json:"name" db:"name"`
	ShortName       *string `json:"shortName" db:"short_name"`
	CommonName      *string `json:"commonName" db:"common_name"`
	NameSort        string  `json:"nameSort" db:"name_sort"`
	Year            int     `json:"year" db:"year"`
	ManufactureDate Date    `json:"manufactureDate" db:"manufacture_date"`
	Description     *string `json:"description" db:"description"`
	Type            *string `json:"type" db:"type"`
	Display         *string `json:"display" db:"display"`
	PlayerCount     *int    `json:"playerCount" db:"player_count"`
	PhysicalMachine bool    `json:"physicalMachine" db:"physical_machine"`
	ManufacturerID  *int    `json:"manufacturerId" db:"manufacturer_id"`
	IpdbID          *int    `json:"ipdbId" db:"ipdb_id"`
	// URL ?
	PinballPrimerURL *string `json:"pinballPrimerUrl" db:"pinball_primer_url"`
	// URL ?
	PinballRulesURL *string `json:"pinballRulesUrl" db:"pinball_rules_url"`
	// URL ?
	PinballCardsURL *string `json:"pinballCardsUrl" db:"pinball_cards_url"`
	// URL ?
	BobsGuideURL *string `json:"bobsGuideUrl" db:"bobs_guide_url"`
	// URL ?
	CompetitionSetupURL *string `json:"competitionSetupUrl" db:"competition_setup_url"`
	// URL ?
	CompetitionNotesURL *string       `json:"competitionNotesUrl" db:"competition_notes_url"`
	HasCompetitionNotes bool          `json:"hasCompetitionNotes" db:"has_competition_notes"`
	HasCompetitionSetup bool          `json:"hasCompetitionSetup" db:"has_competition_setup"`
	CreatedAt           Timestamp     `json:"createdAt" db:"created_at"`
	UpdatedAt           Timestamp     `json:"updatedAt" db:"updated_at"`
	EntryType           string        `json:"entryType" db:"entry_type"`
	Manufacturer        *Manufacturer `json:"manufacturer" db:"-"`
	People              []Person      `json:"people" db:"-"`
	Images              []Image       `json:"images" db:"-"`
	Features            []Feature     `json:"features" db:"-"`
	Keywords            []string      `json:"keywords" db:"keywords"`
}

type Manufacturer struct {
	ManufacturerID int    `json:"manufacturerId" db:"manufacturer_id"`
	Name           string `json:"name" db:"name"`
	FullName       string `json:"fullName" db:"full_name"`
}

// UnmarshalJSON decodes a Manufacturer and cleans up its names (see cleanText).
// in V2 there is an entry Komplett Flipper\r\n(A. H. Geiger Co.)
// so we need a custom json Unmarshal
func (m *Manufacturer) UnmarshalJSON(data []byte) error {

	type Alias Manufacturer
	var raw Alias

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	raw.Name = cleanText(raw.Name)
	raw.FullName = cleanText(raw.FullName)
	*m = Manufacturer(raw)
	return nil
}

// cleanText drops non-printable characters other than whitespace and collapses
// every run of whitespace (including \r and \n) into a single space.
func cleanText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			return r
		}
		return -1
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

type Person struct {
	OpdbPersonID int    `json:"opdbPersonId" db:"opdb_person_id"`
	Name         string `json:"name" db:"name"`
	Role         string `json:"role" db:"role_name"`
	Index        int    `json:"index" db:"person_index"`
}

type Feature struct {
	FeatureID int    `json:"featureId" db:"feature_id"`
	Name      string `json:"name" db:"name"`
	Group     string `json:"group" db:"group_name"`
}

type Image struct {
	Group   string  `json:"group" db:"group_id"`
	Title   *string `json:"title" db:"title"`
	Primary bool    `json:"primary" db:"is_primary"`
	Type    string  `json:"type" db:"type"`
	URLs    URLs    `json:"urls" db:"-"`
	Sizes   Sizes   `json:"sizes" db:"-"`
}

type URLs struct {
	// URL ?
	Medium string `json:"medium,omitempty"`
	// URL ?
	Large string `json:"large,omitempty"`
	// URL ?
	Small string `json:"small,omitempty"`
}

type Sizes struct {
	Medium Size `json:"medium,omitempty"`
	Large  Size `json:"large,omitempty"`
	Small  Size `json:"small,omitempty"`
}

type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// IsEmpty reports whether the size has no width and height set.
func (s Size) IsEmpty() bool {
	return s == Size{}
}
