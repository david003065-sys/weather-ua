package places

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// BuildDatabase читает CSV (разделитель `;`) или JSON-массив объектов и собирает новый SQLite-файл
// со схемой places, FTS5, триггерами и заполненным индексом (через триггеры при INSERT).
// Расширение входа: .json → JSON, иначе CSV.
func BuildDatabase(inputPath, outputPath string, logger *log.Logger) error {
	ext := strings.ToLower(filepath.Ext(inputPath))
	switch ext {
	case ".json":
		return buildFromJSON(inputPath, outputPath, logger)
	default:
		return buildFromCSV(inputPath, outputPath, logger)
	}
}

func logf(logger *log.Logger, format string, args ...interface{}) {
	if logger != nil {
		logger.Printf(format, args...)
	}
}

const placesSchemaSQL = `
CREATE TABLE places (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name_uk TEXT NOT NULL,
	name_ru TEXT,
	oblast TEXT NOT NULL,
	raion TEXT,
	type TEXT NOT NULL,
	population INTEGER NOT NULL DEFAULT 0,
	lat REAL NOT NULL,
	lon REAL NOT NULL,
	search_name TEXT NOT NULL
);
CREATE INDEX idx_places_search_name ON places(search_name);
`

const ftsSchemaSQL = `
CREATE VIRTUAL TABLE places_fts USING fts5(
	name_uk,
	name_ru,
	oblast,
	raion,
	search_name,
	content='places',
	content_rowid='id',
	tokenize='unicode61 remove_diacritics 2'
);
CREATE TRIGGER places_fts_ai AFTER INSERT ON places BEGIN
	INSERT INTO places_fts(rowid, name_uk, name_ru, oblast, raion, search_name)
	VALUES (new.id, new.name_uk, new.name_ru, new.oblast, new.raion, new.search_name);
END;
CREATE TRIGGER places_fts_ad AFTER DELETE ON places BEGIN
	DELETE FROM places_fts WHERE rowid = old.id;
END;
CREATE TRIGGER places_fts_au AFTER UPDATE ON places BEGIN
	UPDATE places_fts SET
		name_uk = new.name_uk,
		name_ru = new.name_ru,
		oblast = new.oblast,
		raion = new.raion,
		search_name = new.search_name
	WHERE rowid = new.id;
END;
`

func openBuildDB(outputPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	if err := os.RemoveAll(outputPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove old db: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL", outputPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func buildFromCSV(inputPath, outputPath string, logger *log.Logger) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	db, err := openBuildDB(outputPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(placesSchemaSQL); err != nil {
		return fmt.Errorf("create places schema: %w", err)
	}
	if _, err := db.Exec(ftsSchemaSQL); err != nil {
		return fmt.Errorf("create fts5: %w", err)
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
		INSERT INTO places (name_uk, name_ru, oblast, raion, type, population, lat, lon, search_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
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
			_ = tx.Rollback()
			return fmt.Errorf("read row: %w", err)
		}

		nameUK := csvGet(row, idx, "name_uk")
		if nameUK == "" {
			continue
		}
		nameRU := csvGet(row, idx, "name_ru")
		oblast := csvGet(row, idx, "oblast")
		raion := csvGet(row, idx, "raion")
		typ := csvGet(row, idx, "type")
		if typ == "" {
			typ = "місто"
		}

		latStr := csvGet(row, idx, "lat")
		lonStr := csvGet(row, idx, "lon")
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
		var population int64
		if popStr := csvGet(row, idx, "population"); popStr != "" {
			if popVal, err := strconv.ParseInt(strings.TrimSpace(popStr), 10, 64); err == nil && popVal > 0 {
				population = popVal
			}
		}

		searchName := buildSearchName(nameUK, nameRU, csvGet(row, idx, "alt_search"))

		if _, err := stmt.Exec(nameUK, nullString(nameRU), oblast, nullString(raion), typ, population, lat, lon, searchName); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert row: %w", err)
		}
		count++
		if logger != nil && count%5000 == 0 {
			logger.Printf("[build_db] inserted %d rows…", count)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, SettlementTypeSchemaVersion)); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}

	if err := verifyBuildCounts(db); err != nil {
		return err
	}

	logf(logger, "[build_db] done, %d places → %s", count, outputPath)
	return nil
}

type jsonPlaceRow struct {
	NameUK     string  `json:"name_uk"`
	NameRU     string  `json:"name_ru"`
	Oblast     string  `json:"oblast"`
	Raion      string  `json:"raion"`
	Type       string  `json:"type"`
	Population int64   `json:"population"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	AltSearch  string  `json:"alt_search"`
}

func buildFromJSON(inputPath, outputPath string, logger *log.Logger) error {
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read json: %w", err)
	}

	var rows []jsonPlaceRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return fmt.Errorf("parse json array: %w", err)
	}

	db, err := openBuildDB(outputPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(placesSchemaSQL); err != nil {
		return fmt.Errorf("create places schema: %w", err)
	}
	if _, err := db.Exec(ftsSchemaSQL); err != nil {
		return fmt.Errorf("create fts5: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO places (name_uk, name_ru, oblast, raion, type, population, lat, lon, search_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for _, r := range rows {
		nameUK := strings.TrimSpace(r.NameUK)
		if nameUK == "" {
			continue
		}
		oblast := strings.TrimSpace(r.Oblast)
		if oblast == "" {
			continue
		}
		typ := strings.TrimSpace(r.Type)
		if typ == "" {
			typ = "місто"
		}
		if r.Lat == 0 && r.Lon == 0 {
			continue
		}

		searchName := buildSearchName(nameUK, r.NameRU, r.AltSearch)
		if _, err := stmt.Exec(
			nameUK, nullString(r.NameRU), oblast, nullString(r.Raion), typ, r.Population, r.Lat, r.Lon, searchName,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert row: %w", err)
		}
		count++
		if logger != nil && count%5000 == 0 {
			logger.Printf("[build_db] inserted %d rows…", count)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, SettlementTypeSchemaVersion)); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}

	if err := verifyBuildCounts(db); err != nil {
		return err
	}

	logf(logger, "[build_db] done, %d places → %s", count, outputPath)
	return nil
}

func buildSearchName(nameUK, nameRU, altSearch string) string {
	normUK := Normalize(nameUK)
	normRU := ""
	if nameRU != "" {
		normRU = Normalize(nameRU)
	}
	searchName := normUK
	if normRU != "" && normRU != normUK {
		searchName = normUK + "|" + normRU
	}
	if alt := strings.TrimSpace(altSearch); alt != "" {
		if altNorm := Normalize(alt); altNorm != "" && !strings.Contains(searchName, altNorm) {
			if searchName == "" {
				searchName = altNorm
			} else {
				searchName = searchName + "|" + altNorm
			}
		}
	}
	return searchName
}

func verifyBuildCounts(db *sql.DB) error {
	var nPlace, nFTS int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM places`).Scan(&nPlace); err != nil {
		return fmt.Errorf("count places: %w", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM places_fts`).Scan(&nFTS); err != nil {
		return fmt.Errorf("count places_fts: %w", err)
	}
	if nPlace != nFTS {
		return fmt.Errorf("FTS row count mismatch: places=%d places_fts=%d", nPlace, nFTS)
	}
	return nil
}

func csvGet(row []string, idx map[string]int, key string) string {
	i, ok := idx[key]
	if !ok || i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func nullString(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
