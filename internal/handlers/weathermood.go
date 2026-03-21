package handlers

// weatherMoodClass maps WMO weather codes + day/night to a single ambient mood class on .weather-app.
// Aligns with client mapCodeToMoodClass in static/script.js.
func weatherMoodClass(code int, night bool) string {
	if night {
		return "weather-night"
	}
	if code == 0 {
		return "weather-clear"
	}
	if code >= 1 && code <= 3 {
		return "weather-cloudy"
	}
	if (code >= 51 && code <= 67) || (code >= 80 && code <= 82) || code >= 95 {
		return "weather-rain"
	}
	if (code >= 71 && code <= 77) || code == 85 || code == 86 {
		return "weather-snow"
	}
	return "weather-cloudy"
}
