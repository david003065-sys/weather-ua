package weather

import "strings"

var cities = []City{
	{
		ID:        "dnipro",
		Name:      "Днепр",
		Latitude:  48.467,
		Longitude: 35.040,
	},
	{
		ID:        "kyiv",
		Name:      "Киев — город Димы",
		Latitude:  50.4501,
		Longitude: 30.5234,
	},
	{
		ID:        "pavlograd",
		Name:      "Павлоград",
		Latitude:  48.533,
		Longitude: 35.866,
	},
	{
		ID:        "volnogorsk",
		Name:      "Вольногорск",
		Latitude:  48.486,
		Longitude: 34.016,
	},
}

func AllCities() []City {
	out := make([]City, len(cities))
	copy(out, cities)
	return out
}

func cityByID(id string) (City, bool) {
	for _, c := range cities {
		if c.ID == id {
			return c, true
		}
	}
	return City{}, false
}

func IsKnownCity(id string) bool {
	_, ok := cityByID(id)
	return ok
}

func LocalizedCityName(id, lang string) string {
	switch id {
	case "dnipro":
		switch lang {
		case "uk":
			return "Дніпро"
		case "en":
			return "Dnipro"
		default:
			return "Днепр"
		}
	case "kyiv":
		switch lang {
		case "uk":
			return "Київ"
		case "en":
			return "Kyiv"
		default:
			return "Киев — город Димы"
		}
	case "pavlograd":
		switch lang {
		case "uk":
			return "Павлоград"
		case "en":
			return "Pavlohrad"
		default:
			return "Павлоград"
		}
	case "volnogorsk":
		switch lang {
		case "uk":
			return "Вільногірськ"
		case "en":
			return "Vilnohorsk"
		default:
			return "Вольногорск"
		}
	default:
		return id
	}
}

func NearestCity(lat, lon float64) (City, bool) {
	if len(cities) == 0 {
		return City{}, false
	}

	best := cities[0]
	bestDist := distanceSq(lat, lon, best.Latitude, best.Longitude)
	for _, c := range cities[1:] {
		d := distanceSq(lat, lon, c.Latitude, c.Longitude)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best, true
}

// MatchKnownCityByCoords tries to match coordinates to one of known cities.
// Returns city only when coordinates are close enough to avoid false matches.
func MatchKnownCityByCoords(lat, lon float64) (City, bool) {
	if len(cities) == 0 {
		return City{}, false
	}
	best := cities[0]
	bestDist := distanceSq(lat, lon, best.Latitude, best.Longitude)
	for _, c := range cities[1:] {
		d := distanceSq(lat, lon, c.Latitude, c.Longitude)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	// ~5-6km tolerance in degrees for matching known city coordinates.
	const maxDistanceSq = 0.0025 // 0.05^2
	if bestDist > maxDistanceSq {
		return City{}, false
	}
	return best, true
}

// MatchKnownCityByName checks exact known city IDs/names in common languages.
func MatchKnownCityByName(name string) (City, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return City{}, false
	}
	switch n {
	case "dnipro", "дніпро", "днепр":
		return cityByID("dnipro")
	case "kyiv", "київ", "киев":
		return cityByID("kyiv")
	case "pavlograd", "павлоград":
		return cityByID("pavlograd")
	case "volnogorsk", "vilnohorsk", "вольногорск", "вільногірськ":
		return cityByID("volnogorsk")
	default:
		return City{}, false
	}
}

func distanceSq(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := lat1 - lat2
	dLon := lon1 - lon2
	return dLat*dLat + dLon*dLon
}

