package places

import "strings"

// SettlementTypeSchemaVersion is stored in SQLite PRAGMA user_version after bootstrap import.
// Bump when normalization rules change so existing places.db is regenerated on next server start.
const SettlementTypeSchemaVersion = 2

// NormalizeSettlementType maps a GeoNames feature code (class P) and population to the
// Ukrainian canonical values stored in places.type and shown via handlers.deriveTypeNames:
//   місто | селище | село
//
// GeoNames uses PPL for any populated place that is not an administrative seat; many Ukrainian
// cities (e.g. Вольногорськ / Volnogorsk) are PPL with large population, not PPLA*. Treating all
// PPL as "селище" produced wrong labels (Russian «посёлок»).
//
// Reference: https://www.geonames.org/export/codes.html — PPLC = capital, PPLA* = admin seats, PPL = generic.
func NormalizeSettlementType(featureCode string, population int64) string {
	code := strings.TrimSpace(strings.ToUpper(featureCode))
	switch code {
	case "PPLC":
		// Country / capital-level seat — always a city in UI terms.
		return "місто"
	case "PPLA", "PPLA2", "PPLA3", "PPLA4":
		// Administrative centre of oblast / raion / etc.
		return "місто"
	case "PPL":
		return normalizePPLPopulation(population)
	default:
		return ""
	}
}

// Thresholds for generic PPL (GeoNames does not mark every Ukrainian "місто" as PPLA*).
// Capital / oblast seats are handled via PPLC / PPLA* above — these apply only to PPL.
// Tuned so real cities like Вольногорськ (~22k, PPL) are місто, while смт-scale places stay селище.
const (
	minPopulationCity       int64 = 5_000
	minPopulationSettlement int64 = 500
)

func normalizePPLPopulation(population int64) string {
	if population < 0 {
		population = 0
	}
	switch {
	case population >= minPopulationCity:
		return "місто"
	case population >= minPopulationSettlement:
		return "селище"
	default:
		// Very small places and missing (0) population default to village —
		// most zero-population PPL rows in dumps are hamlets / село.
		return "село"
	}
}
