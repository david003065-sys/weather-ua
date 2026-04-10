package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"unicode/utf8"

	"bss/internal/places"

	_ "modernc.org/sqlite"
)

type rowData struct {
	ID         int64
	NameUK     string
	NameRU     sql.NullString
	Oblast     string
	Raion      sql.NullString
	Type       string
	Population sql.NullInt64
	Lat        float64
	Lon        float64
	SearchName string
}

func main() {
	dbPath := flag.String("db", "data/places.db", "path to places sqlite database")
	limit := flag.Int("limit", 200, "max broken rows to print")
	flag.Parse()

	logger := log.New(os.Stdout, "[validate_places] ", log.LstdFlags)
	if err := validatePlacesDB(*dbPath, *limit, logger); err != nil {
		logger.Fatal(err)
	}
}

func validatePlacesDB(path string, printLimit int, logger *log.Logger) error {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL", path))
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
SELECT id, name_uk, name_ru, oblast, raion, type, population, lat, lon, search_name
FROM places
ORDER BY id
`)
	if err != nil {
		return fmt.Errorf("query places: %w", err)
	}
	defer rows.Close()

	total := 0
	broken := 0
	printed := 0
	for rows.Next() {
		var r rowData
		if err := rows.Scan(
			&r.ID, &r.NameUK, &r.NameRU, &r.Oblast, &r.Raion, &r.Type, &r.Population, &r.Lat, &r.Lon, &r.SearchName,
		); err != nil {
			return fmt.Errorf("scan row: %w", err)
		}
		total++
		issues := validateRow(r)
		if len(issues) == 0 {
			continue
		}
		broken++
		if printed < printLimit {
			logger.Printf("BROKEN id=%d issues=%s name_uk=%q name_ru=%q oblast=%q type=%q lat=%v lon=%v search_name=%q",
				r.ID,
				strings.Join(issues, "; "),
				r.NameUK,
				nullToString(r.NameRU),
				r.Oblast,
				r.Type,
				r.Lat,
				r.Lon,
				r.SearchName,
			)
			printed++
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration: %w", err)
	}

	logger.Printf("checked=%d broken=%d printed=%d", total, broken, printed)
	if broken > printed {
		logger.Printf("output truncated by -limit=%d", printLimit)
	}
	return nil
}

func validateRow(r rowData) []string {
	issues := make([]string, 0, 8)

	// Required fields for search/indexing path.
	if strings.TrimSpace(r.NameUK) == "" {
		issues = append(issues, "empty name_uk (required)")
	}
	if strings.TrimSpace(r.Oblast) == "" {
		issues = append(issues, "empty oblast (required)")
	}
	if strings.TrimSpace(r.Type) == "" {
		issues = append(issues, "empty type (required)")
	}
	if strings.TrimSpace(r.SearchName) == "" {
		issues = append(issues, "empty search_name (required)")
	}

	// UTF-8 sanity for fields used by handlers/search formatting.
	checkUTF8 := func(label, s string) {
		if s != "" && !utf8.ValidString(s) {
			issues = append(issues, "invalid utf8 in "+label)
		}
	}
	checkUTF8("name_uk", r.NameUK)
	checkUTF8("name_ru", nullToString(r.NameRU))
	checkUTF8("oblast", r.Oblast)
	checkUTF8("raion", nullToString(r.Raion))
	checkUTF8("type", r.Type)
	checkUTF8("search_name", r.SearchName)

	// Coordinate sanity.
	if math.IsNaN(r.Lat) || math.IsInf(r.Lat, 0) || r.Lat < -90 || r.Lat > 90 {
		issues = append(issues, "invalid lat")
	}
	if math.IsNaN(r.Lon) || math.IsInf(r.Lon, 0) || r.Lon < -180 || r.Lon > 180 {
		issues = append(issues, "invalid lon")
	}

	// Search normalization check: if both become empty, search may skip these names.
	if strings.TrimSpace(r.NameUK) != "" && places.Normalize(r.NameUK) == "" {
		issues = append(issues, "normalize(name_uk) empty")
	}
	if strings.TrimSpace(r.SearchName) != "" && places.Normalize(r.SearchName) == "" {
		issues = append(issues, "normalize(search_name) empty")
	}

	return issues
}

func nullToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
