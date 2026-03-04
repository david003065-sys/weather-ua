package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GeoNames formats used:
// - UA.txt: full dump for Ukraine (tab-separated)
// - admin1CodesASCII.txt: "UA.XX<TAB>OblastName<...>"
// - alternateNamesV2.txt: big alt-names dump (tab-separated)

type city struct {
	geonameID string
	nameUK    string
	lat       string
	lon       string
	admin1    string // admin1 code like "01"
	oblast    string
}

type ruAltName struct {
	name      string
	preferred bool
}

func main() {
	var baseDir string
	var outDir string

	flag.StringVar(&baseDir, "geonames-dir", "data/geonames", "directory with GeoNames dumps (UA.txt, admin1CodesASCII.txt, alternateNamesV2.txt)")
	flag.StringVar(&outDir, "out-dir", "data/out", "output directory for cities_ua.csv")
	flag.Parse()

	if err := run(baseDir, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "build_ua_cities_csv: %v\n", err)
		os.Exit(1)
	}
}

func run(geoDir, outDir string) error {
	uaPath := filepath.Join(geoDir, "UA.txt")
	admin1Path := filepath.Join(geoDir, "admin1CodesASCII.txt")
	altNamesPath := filepath.Join(geoDir, "alternateNamesV2.txt")

	// Check required files early for clearer errors.
	if _, err := os.Stat(uaPath); err != nil {
		return fmt.Errorf("UA.txt not found at %q (download UA.zip from GeoNames and extract UA.txt there): %w", uaPath, err)
	}
	if _, err := os.Stat(admin1Path); err != nil {
		return fmt.Errorf("admin1CodesASCII.txt not found at %q (download from GeoNames dump): %w", admin1Path, err)
	}
	if _, err := os.Stat(altNamesPath); err != nil {
		return fmt.Errorf("alternateNamesV2.txt not found at %q (download alternateNamesV2.zip and extract alternateNamesV2.txt): %w", altNamesPath, err)
	}

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

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir out dir: %w", err)
	}

	outPath := filepath.Join(outDir, "cities_ua.csv")
	if err := writeCSV(outPath, cities, ruNames); err != nil {
		return fmt.Errorf("write csv: %w", err)
	}

	fmt.Printf("Wrote %d cities to %s\n", len(cities), outPath)
	return nil
}

// loadAdmin1 reads admin1CodesASCII.txt and builds "UA.xx" -> oblastName map.
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

// loadCities parses UA.txt, filters only P* cities and returns slice + id set.
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
	// UA.txt lines are not extremely huge; default buffer is fine.
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

		// Simple validation of lat/lon (must be float).
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

// loadRuAltNames scans alternateNamesV2.txt once and collects best RU name for given geoname IDs.
func loadRuAltNames(path string, ids map[string]struct{}) (map[string]ruAltName, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	res := make(map[string]ruAltName)

	scanner := bufio.NewScanner(f)
	// alternateNamesV2.txt can have very long lines; increase buffer.
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
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf("line too long in alternateNamesV2.txt (increase scanner buffer): %w", err)
		}
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

	// Header
	if _, err := io.WriteString(w, "name_uk;name_ru;oblast;raion;type;lat;lon\n"); err != nil {
		return err
	}

	for _, c := range cities {
		ru := c.nameUK
		if alt, ok := ruNames[c.geonameID]; ok && alt.name != "" {
			ru = alt.name
		}
		// raion всегда пустой (двойной ;;)
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

// escapeSemi: здесь мы не добавляем кавычки, просто оставляем как есть.
// Если вдруг в имени окажется ';', CSV станет сложнее парсить, но это редкий случай.
func escapeSemi(s string) string {
	return strings.ReplaceAll(s, "\r", " ")
}

