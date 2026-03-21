package i18n

import "strconv"

// DuplicatesHint returns the “N more homonyms” line for place pages.
func DuplicatesHint(lang string, count int) string {
	if count <= 0 {
		return ""
	}
	switch Normalize(lang) {
	case "uk":
		if count == 1 {
			return "Є ще 1 варіант назви"
		}
		if count >= 2 && count <= 4 {
			return "Є ще " + strconv.Itoa(count) + " варіанти"
		}
		return "Є ще " + strconv.Itoa(count) + " варіантів"
	case "en":
		if count == 1 {
			return "1 more matching name"
		}
		return strconv.Itoa(count) + " more matching names"
	default: // ru
		if count == 1 {
			return "Есть ещё 1 вариант названия"
		}
		if count >= 2 && count <= 4 {
			return "Есть ещё " + strconv.Itoa(count) + " варианта"
		}
		return "Есть ещё " + strconv.Itoa(count) + " вариантов"
	}
}
