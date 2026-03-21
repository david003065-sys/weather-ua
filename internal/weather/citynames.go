package weather

import "strings"

// cityNameLoc holds official-style display names per UI language for static /weather cities.
// Keys in localizedCityNames are City.ID slugs (lowercase Latin).
type cityNameLoc struct {
	RU string
	UK string
	EN string
}

var localizedCityNames = map[string]cityNameLoc{
	"dnipro": {
		RU: "Днепр",
		UK: "Дніпро",
		EN: "Dnipro",
	},
	"kyiv": {
		RU: "Киев",
		UK: "Київ",
		EN: "Kyiv",
	},
	"pavlograd": {
		RU: "Павлоград",
		UK: "Павлоград",
		EN: "Pavlohrad",
	},
	"volnogorsk": {
		RU: "Вольногорск",
		UK: "Вільногірськ",
		EN: "Vilnohorsk",
	},
}

func normalizeCityID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func normalizeLangForCity(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "uk", "en", "ru":
		return strings.ToLower(lang)
	default:
		return "ru"
	}
}

// LocalizedCityName returns the display name for a known weather city slug (e.g. "kyiv", "volnogorsk").
// If the slug is unknown, falls back to the canonical Name from the cities list, then to the raw id.
func LocalizedCityName(id, lang string) string {
	key := normalizeCityID(id)
	if loc, ok := localizedCityNames[key]; ok {
		switch normalizeLangForCity(lang) {
		case "uk":
			return loc.UK
		case "en":
			return loc.EN
		default:
			return loc.RU
		}
	}
	if c, ok := cityByID(key); ok {
		if n := strings.TrimSpace(c.Name); n != "" {
			return n
		}
	}
	if strings.TrimSpace(id) == "" {
		return id
	}
	return id
}
