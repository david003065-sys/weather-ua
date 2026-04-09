package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bss/internal/places"
)

// GeoNames formats used:
// - UA.txt: full dump for Ukraine (tab-separated)
// - admin1CodesASCII.txt: "UA.XX<TAB>OblastName<...>"
// - alternateNamesV2.txt: big alt-names dump (tab-separated)

type city struct {
	geonameID   string
	nameUK      string
	lat         string
	lon         string
	admin1      string // admin1 code like "01"
	oblast      string
	featureCode string // GeoNames feature code (PPL/PPLA*/PPLC)
	population  int64
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

	ukNames, ruNames, enNames, err := places.LoadAlternateNamesV2ForIDs(altNamesPath, idSet)
	if err != nil {
		return fmt.Errorf("load alternateNamesV2: %w", err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir out dir: %w", err)
	}

	outPath := filepath.Join(outDir, "cities_ua.csv")
	if err := writeCSV(outPath, cities, ukNames, ruNames, enNames); err != nil {
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
		if len(parts) < 15 {
			continue
		}

		geonameID := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		lat := strings.TrimSpace(parts[4])
		lon := strings.TrimSpace(parts[5])
		featureClass := strings.TrimSpace(parts[6])
		featureCode := strings.TrimSpace(parts[7])
		admin1Code := strings.TrimSpace(parts[10]) // e.g. "12"
		population := parsePopulation(parts[14])

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
			geonameID:   geonameID,
			nameUK:      name,
			lat:         lat,
			lon:         lon,
			admin1:      admin1Code,
			oblast:      oblastName,
			featureCode: featureCode,
			population:  population,
		}
		cities = append(cities, c)
		idSet[geonameID] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	return cities, idSet, nil
}

func parsePopulation(field string) int64 {
	field = strings.TrimSpace(field)
	if field == "" {
		return 0
	}
	n, err := strconv.ParseInt(field, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func writeCSV(path string, cities []*city, ukNames, ruNames, enNames map[string]string) error {
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

	// Header (alt_search holds GeoNames en alternate when present — helps FTS / search.)
	if _, err := io.WriteString(w, "name_uk;name_ru;oblast;raion;type;lat;lon;alt_search\n"); err != nil {
		return err
	}

	for _, c := range cities {
		uk := strings.TrimSpace(c.nameUK)
		if v, ok := ukNames[c.geonameID]; ok && strings.TrimSpace(v) != "" {
			uk = strings.TrimSpace(v)
		}
		ru := strings.TrimSpace(c.nameUK)
		if v, ok := ruNames[c.geonameID]; ok && strings.TrimSpace(v) != "" {
			ru = strings.TrimSpace(v)
		}
		altSearch := ""
		if v, ok := enNames[c.geonameID]; ok {
			altSearch = strings.TrimSpace(v)
		}
		typ := places.NormalizeSettlementType(c.featureCode, c.population)
		line := fmt.Sprintf("%s;%s;%s;;%s;%s;%s;%s\n",
			escapeSemi(uk),
			escapeSemi(ru),
			escapeSemi(c.oblast),
			typ,
			c.lat,
			c.lon,
			escapeSemi(altSearch),
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

