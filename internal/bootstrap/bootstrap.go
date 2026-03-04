package bootstrap

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"bufio"
	"encoding/csv"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bss/internal/places"

	_ "github.com/mattn/go-sqlite3"
)

// EnsureData гарантирует, что data/places.db существует.
// Если файла нет, он автоматически скачивает необходимые GeoNames‑файлы,
// генерирует CSV с городами и импортирует его в SQLite.
func EnsureData(logger *log.Logger) error {
	const dbPath = "data/places.db"
	if _, err := os.Stat(dbPath); err == nil {
		return nil
	}

	if logger != nil {
		logger.Printf("[bootstrap] places db %s not found, generating from GeoNames…", dbPath)
	}

	geoDir := filepath.Join("data", "geonames")
	outDir := filepath.Join("data", "out")
	sourceCSV := filepath.Join(outDir, "cities_ua.csv")

	if err := os.MkdirAll(geoDir, 0o755); err != nil {
		return fmt.Errorf("bootstrap: mkdir geonames: %w", err)
	}

	// 1) Скачать GeoNames дампы, если их нет.
	if err := ensureGeoFiles(geoDir, logger); err != nil {
		return err
	}

	// 2) Сгенерировать CSV с городами Украины.
	if err := generateCitiesCSV(geoDir, outDir, logger); err != nil {
		return err
	}

	// 3) Импортировать CSV в SQLite.
	if err := importPlacesCSV(sourceCSV, dbPath, logger); err != nil {
		return err
	}

	if logger != nil {
		logger.Printf("[bootstrap] places db generated at %s", dbPath)
	}
	return nil
}

func ensureGeoFiles(dir string, logger *log.Logger) error {
	uaZipURL := "https://download.geonames.org/export/dump/UA.zip"
	altZipURL := "https://download.geonames.org/export/dump/alternateNamesV2.zip"
	adminURL := "https://download.geonames.org/export/dump/admin1CodesASCII.txt"

	uaTxt := filepath.Join(dir, "UA.txt")
	adminTxt := filepath.Join(dir, "admin1CodesASCII.txt")
	altTxt := filepath.Join(dir, "alternateNamesV2.txt")

	if _, err := os.Stat(uaTxt); err != nil {
		if logger != nil {
			logger.Printf("[bootstrap] downloading UA.zip…")
		}
		if err := downloadAndUnzipSingle(uaZipURL, dir, "UA.txt"); err != nil {
			return fmt.Errorf("download UA.zip: %w", err)
		}
	}
	if _, err := os.Stat(adminTxt); err != nil {
		if logger != nil {
			logger.Printf("[bootstrap] downloading admin1CodesASCII.txt…")
		}
		if err := downloadFile(adminURL, adminTxt); err != nil {
			return fmt.Errorf("download admin1CodesASCII.txt: %w", err)
		}
	}
	if _, err := os.Stat(altTxt); err != nil {
		if logger != nil {
			logger.Printf("[bootstrap] downloading alternateNamesV2.zip… (may take a while)")
		}
		if err := downloadAndUnzipSingle(altZipURL, dir, "alternateNamesV2.txt"); err != nil {
			return fmt.Errorf("download alternateNamesV2.zip: %w", err)
		}
	}
	return nil
}

func downloadFile(url, dst string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func downloadAndUnzipSingle(url, dstDir, wantName string) error {
	tmpZip := filepath.Join(dstDir, "tmp-download.zip")
	if err := downloadFile(url, tmpZip); err != nil {
		return err
	}
	defer os.Remove(tmpZip)

	r, err := zip.OpenReader(tmpZip)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name != wantName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		dstPath := filepath.Join(dstDir, wantName)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}
		out, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
	return fmt.Errorf("file %s not found in zip", wantName)
}

// --- Генерация CSV с городами Украины (адаптация логики из build_ua_cities_csv) ---

type city struct {
	geonameID string
	nameUK    string
	lat       string
	lon       string
	admin1    string
	oblast    string
}

type ruAltName struct {
	name      string
	preferred bool
}

func generateCitiesCSV(geoDir, outDir string, logger *log.Logger) error {
	uaPath := filepath.Join(geoDir, "UA.txt")
	admin1Path := filepath.Join(geoDir, "admin1CodesASCII.txt")
	altNamesPath := filepath.Join(geoDir, "alternateNamesV2.txt")

	admin1Names, err := loadAdmin1(admin1Path)
	if err != nil {
		return fmt.Errorf("load admin1CodesASCII: %w", err)
	}

	cities, idSet, err := loadCities(uaPath, admin1Names)
	if err != nil {
		return fmt.Errorf("load UA.txt cities: %w", err)
	}

	ruNames, err := loadRuAltNames(altNamesPath, idSet)
	if err != nil {
		return fmt.Errorf("load alternateNamesV2: %w", err)
	}

	if logger != nil {
		logger.Printf("[bootstrap] generating CSV for %d cities…", len(cities))
	}
	outPath := filepath.Join(outDir, "cities_ua.csv")
	if err := writeCSV(outPath, cities, ruNames); err != nil {
		return fmt.Errorf("write csv: %w", err)
	}
	return nil
}

func loadAdmin1(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	res := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		code := strings.TrimSpace(parts[0]) // e.g. "UA.12"
		name := strings.TrimSpace(parts[1]) // oblast name
		if code == "" || name == "" {
			continue
		}
		if strings.HasPrefix(code, "UA.") {
			res[code] = name
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func loadCities(path string, admin1Names map[string]string) ([]*city, map[string]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	allowedFeatureCodes := map[string]struct{}{
		"PPL":  {},
		"PPLA": {},
		"PPLA2": {},
		"PPLA3": {},
		"PPLA4": {},
		"PPLC": {},
	}

	var (
		cities []*city
		idSet  = make(map[string]struct{})
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 11 {
			continue
		}

		geonameID := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		lat := strings.TrimSpace(parts[4])
		lon := strings.TrimSpace(parts[5])
		featureClass := strings.TrimSpace(parts[6])
		featureCode := strings.TrimSpace(parts[7])
		admin1Code := strings.TrimSpace(parts[10]) // e.g. "12"

		if geonameID == "" || name == "" {
			continue
		}
		if featureClass != "P" {
			continue
		}
		if _, ok := allowedFeatureCodes[featureCode]; !ok {
			continue
		}

		fullAdmin1 := ""
		if admin1Code != "" {
			fullAdmin1 = "UA." + admin1Code
		}
		oblastName := ""
		if fullAdmin1 != "" {
			if n, ok := admin1Names[fullAdmin1]; ok {
				oblastName = n
			}
		}

		if _, err := strconv.ParseFloat(lat, 64); err != nil {
			continue
		}
		if _, err := strconv.ParseFloat(lon, 64); err != nil {
			continue
		}

		c := &city{
			geonameID: geonameID,
			nameUK:    name,
			lat:       lat,
			lon:       lon,
			admin1:    admin1Code,
			oblast:    oblastName,
		}
		cities = append(cities, c)
		idSet[geonameID] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	return cities, idSet, nil
}

func loadRuAltNames(path string, ids map[string]struct{}) (map[string]ruAltName, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	res := make(map[string]ruAltName)

	scanner := bufio.NewScanner(f)
	const maxLine = 16 * 1024 * 1024
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, maxLine)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}

		geonameID := strings.TrimSpace(parts[1])
		if _, ok := ids[geonameID]; !ok {
			continue
		}

		iso := strings.TrimSpace(parts[2])
		if iso != "ru" {
			continue
		}

		altName := strings.TrimSpace(parts[3])
		if altName == "" {
			continue
		}

		preferred := false
		if len(parts) >= 5 && strings.TrimSpace(parts[4]) == "1" {
			preferred = true
		}

		current, ok := res[geonameID]
		if !ok || (!current.preferred && preferred) {
			res[geonameID] = ruAltName{name: altName, preferred: preferred}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func writeCSV(path string, cities []*city, ruNames map[string]ruAltName) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()

	if _, err := io.WriteString(w, "name_uk;name_ru;oblast;raion;type;lat;lon\n"); err != nil {
		return err
	}

	for _, c := range cities {
		ru := c.nameUK
		if alt, ok := ruNames[c.geonameID]; ok && alt.name != "" {
			ru = alt.name
		}
		line := fmt.Sprintf("%s;%s;%s;;місто;%s;%s\n",
			escapeSemi(c.nameUK),
			escapeSemi(ru),
			escapeSemi(c.oblast),
			c.lat,
			c.lon,
		)
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}

func escapeSemi(s string) string {
	return strings.ReplaceAll(s, "\r", " ")
}

// --- Импорт CSV в SQLite (адаптация логики из places_importer) ---

func importPlacesCSV(inputPath, outputPath string, logger *log.Logger) error {
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

		nameUK := getCSV(row, idx, "name_uk")
		if nameUK == "" {
			continue
		}
		nameRU := getCSV(row, idx, "name_ru")
		oblast := getCSV(row, idx, "oblast")
		raion := getCSV(row, idx, "raion")
		typ := getCSV(row, idx, "type")
		if typ == "" {
			typ = "місто"
		}

		latStr := getCSV(row, idx, "lat")
		lonStr := getCSV(row, idx, "lon")
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
		if alt := getCSV(row, idx, "alt_search"); alt != "" {
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
		if logger != nil && count%5000 == 0 {
			logger.Printf("[bootstrap] inserted %d rows into %s…", count, outputPath)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if logger != nil {
		logger.Printf("[bootstrap] done, inserted %d places into %s", count, outputPath)
	}
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

func getCSV(row []string, idx map[string]int, key string) string {
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

