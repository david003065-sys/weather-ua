package weather

// MainConditionPhysics returns a short English label for Canvas / data-attributes (WMO-style codes 0–99).
func MainConditionPhysics(wmo int) string {
	switch wmo {
	case 0, 1, 2:
		return "Clear"
	case 3:
		return "Clouds"
	case 45, 48:
		return "Fog"
	case 51, 53, 55:
		return "Drizzle"
	case 61, 63, 65, 66, 67, 80, 81, 82:
		return "Rain"
	case 71, 73, 75, 77, 85, 86:
		return "Snow"
	case 95, 96, 97, 99:
		return "Thunderstorm"
	default:
		return "Unknown"
	}
}
