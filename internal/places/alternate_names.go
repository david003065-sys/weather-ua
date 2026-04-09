package places

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// altPick holds one alternate name row and whether GeoNames marks it preferred for that language.
type altPick struct {
	name      string
	preferred bool
}

// mergeAltPick keeps the preferred name when multiple alternates exist for the same id+language.
func mergeAltPick(cur altPick, ok bool, name string, preferred bool) altPick {
	name = strings.TrimSpace(name)
	if name == "" {
		if ok {
			return cur
		}
		return altPick{}
	}
	if !ok {
		return altPick{name: name, preferred: preferred}
	}
	if !cur.preferred && preferred {
		return altPick{name: name, preferred: true}
	}
	return cur
}

// LoadAlternateNamesV2ForIDs reads GeoNames alternateNamesV2.txt (tab-separated) and returns
// the chosen alternate name per geonameId for uk, ru, and en when present in the dump.
// Only rows whose geonameId is in ids are considered (full-file scan; ids should be the UA subset).
//
// Column layout matches GeoNames: alternateNameId, geonameId, isolanguage, alternate name,
// isPreferredName, ...
func LoadAlternateNamesV2ForIDs(path string, ids map[string]struct{}) (uk, ru, en map[string]string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()
	return loadAlternateNamesV2FromReader(f, ids)
}

func loadAlternateNamesV2FromReader(r io.Reader, ids map[string]struct{}) (uk, ru, en map[string]string, err error) {
	ukM := make(map[string]altPick)
	ruM := make(map[string]altPick)
	enM := make(map[string]altPick)

	scanner := bufio.NewScanner(r)
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
		if iso == "" {
			continue
		}

		altName := strings.TrimSpace(parts[3])
		if altName == "" {
			continue
		}

		preferred := len(parts) >= 5 && strings.TrimSpace(parts[4]) == "1"

		switch iso {
		case "uk":
			cur, ok := ukM[geonameID]
			ukM[geonameID] = mergeAltPick(cur, ok, altName, preferred)
		case "ru":
			cur, ok := ruM[geonameID]
			ruM[geonameID] = mergeAltPick(cur, ok, altName, preferred)
		case "en":
			cur, ok := enM[geonameID]
			enM[geonameID] = mergeAltPick(cur, ok, altName, preferred)
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, nil, nil, fmt.Errorf("alternateNamesV2: line exceeds scanner buffer (max %d bytes): %w", maxLine, err)
		}
		return nil, nil, nil, err
	}

	toStr := func(m map[string]altPick) map[string]string {
		out := make(map[string]string, len(m))
		for id, p := range m {
			if p.name != "" {
				out[id] = p.name
			}
		}
		return out
	}

	return toStr(ukM), toStr(ruM), toStr(enM), nil
}
