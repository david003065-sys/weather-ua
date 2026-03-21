package i18n

import "strings"

// Normalize returns a supported UI language tag: uk | en | ru (default ru).
func Normalize(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "en", "uk", "ru":
		return strings.ToLower(lang)
	default:
		return "ru"
	}
}
