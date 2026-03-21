package i18n

// weatherPhrase is RU / EN / UK (Ukrainian) + emoji — WMO-style codes from WeatherAPI mapping.
type weatherPhrase struct {
	RU, EN, UK string
	Icon       string
}

func phraseForCode(code int) weatherPhrase {
	switch code {
	case 0:
		return weatherPhrase{"Ясно", "Clear sky", "Ясно", "☀️"}
	case 1, 2:
		return weatherPhrase{"Преимущественно ясно", "Mostly clear", "Переважно ясно", "🌤"}
	case 3:
		return weatherPhrase{"Облачно", "Cloudy", "Хмарно", "☁️"}
	case 45, 48:
		return weatherPhrase{"Туман", "Fog", "Туман", "🌫"}
	case 51, 53, 55:
		return weatherPhrase{"Морось", "Light drizzle", "Морось", "🌦"}
	case 61, 63, 65:
		return weatherPhrase{"Дождь", "Rain", "Дощ", "🌧"}
	case 66, 67:
		return weatherPhrase{"Ледяной дождь", "Freezing rain", "Крижаний дощ", "🌧"}
	case 71, 73, 75, 77:
		return weatherPhrase{"Снег", "Snow", "Сніг", "❄️"}
	case 80, 81, 82:
		return weatherPhrase{"Ливни", "Rain showers", "Зливи", "🌧"}
	case 95:
		return weatherPhrase{"Гроза", "Thunderstorm", "Гроза", "⛈"}
	case 96, 97, 99:
		return weatherPhrase{"Гроза с градом", "Thunderstorm with hail", "Гроза з градом", "⛈"}
	default:
		return weatherPhrase{"Неизвестно", "Unknown", "Невідомо", "❔"}
	}
}

// WeatherIcon returns the emoji for a WMO-style weather code (for cards / hourly rows).
func WeatherIcon(code int) string {
	return phraseForCode(code).Icon
}

// WeatherDescription returns a short condition text for the given WMO-style code.
func WeatherDescription(code int, lang string) string {
	p := phraseForCode(code)
	switch Normalize(lang) {
	case "en":
		return p.EN
	case "uk":
		return p.UK
	default:
		return p.RU
	}
}
