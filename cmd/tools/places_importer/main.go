package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bss/internal/places"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	var inputPath string
	var outputPath string

	flag.StringVar(&inputPath, "input", "data/source/places.csv", "path to source CSV with settlements")
	flag.StringVar(&outputPath, "output", "data/places.db", "path to output SQLite database")
	flag.Parse()

	if err := run(inputPath, outputPath); err != nil {
		log.Fatalf("places_importer: %v", err)
	}
}

func run(inputPath, outputPath string) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("mkdir data: %w", err)
	}

	if err := os.RemoveAll(outputPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove old db: %w", err)
	}

	db, err := sql.Open("sqlite3", outputPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := createSchema(db); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	reader := csv.NewReader(f)
	reader.Comma = ';'
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	required := []string{"name_uk", "oblast", "type", "lat", "lon"}
	for _, col := range required {
		if _, ok := idx[col]; !ok {
			return fmt.Errorf("missing required column %q in csv", col)
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO places (name_uk, name_ru, oblast, raion, type, lat, lon, search_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for {
		row, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read row: %w", err)
		}

		nameUK := get(row, idx, "name_uk")
		if nameUK == "" {
			continue
		}
		nameRU := get(row, idx, "name_ru")
		oblast := get(row, idx, "oblast")
		raion := get(row, idx, "raion")
		typ := get(row, idx, "type")
		if typ == "" {
			typ = "місто"
		}

		latStr := get(row, idx, "lat")
		lonStr := get(row, idx, "lon")
		if latStr == "" || lonStr == "" {
			continue
		}
		lat, err := strconv.ParseFloat(strings.ReplaceAll(latStr, ",", "."), 64)
		if err != nil {
			continue
		}
		lon, err := strconv.ParseFloat(strings.ReplaceAll(lonStr, ",", "."), 64)
		if err != nil {
			continue
		}

		normUK := places.Normalize(nameUK)
		normRU := ""
		if nameRU != "" {
			normRU = places.Normalize(nameRU)
		}
		searchName := normUK
		if normRU != "" && normRU != normUK {
			searchName = normUK + "|" + normRU
		}
		if alt := get(row, idx, "alt_search"); alt != "" {
			if altNorm := places.Normalize(alt); altNorm != "" && !strings.Contains(searchName, altNorm) {
				if searchName == "" {
					searchName = altNorm
				} else {
					searchName = searchName + "|" + altNorm
				}
			}
		}

		if _, err := stmt.Exec(nameUK, nullOr(nameRU), oblast, nullOr(raion), typ, lat, lon, searchName); err != nil {
			return fmt.Errorf("insert row: %w", err)
		}
		count++
		if count%1000 == 0 {
			log.Printf("inserted %d rows...", count)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	log.Printf("done, inserted %d places into %s", count, outputPath)
	return nil
}

func createSchema(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS places (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name_uk TEXT NOT NULL,
	name_ru TEXT,
	oblast TEXT NOT NULL,
	raion TEXT,
	type TEXT NOT NULL,
	lat REAL NOT NULL,
	lon REAL NOT NULL,
	search_name TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_places_search_name ON places(search_name);
`
	_, err := db.Exec(schema)
	return err
}

func get(row []string, idx map[string]int, key string) string {
	i, ok := idx[key]
	if !ok || i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func nullOr(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
