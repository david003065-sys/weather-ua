// Package places provides SQLite-backed settlement search.
//
// Place display names (for /place, /api/places, etc.) come from DB columns first:
//
//	Ukrainian (UI uk): name_uk → name (mirror) → name_ru → (then cross-lang in LocalizedDisplayName)
//	Russian (UI ru):   name_ru → name_uk if Cyrillic → name → name_uk
//	English (UI en):   Latin name_uk if any → TranslitLatin(name_ru) → TranslitLatin(name_uk) → name_ru → name_uk
//
// There is no name_en column yet; English is derived. LocalizedDisplayName then applies per-language preference:
//
//	uk: uk, ru, en, Name
//	en: en, uk, ru, Name
//	ru: ru, uk, en, Name
//
// Static /city/* labels remain in package weather (LocalizedCityName), not mixed into DB resolution.
package places

import (
	"strings"
	"unicode"
)

// NormalizePlaceLang returns uk | en | ru (default ru) for place display logic.
func NormalizePlaceLang(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "uk", "en", "ru":
		return strings.ToLower(lang)
	default:
		return "ru"
	}
}

func hasCyrillic(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Cyrillic) {
			return true
		}
	}
	return false
}

func isLatinLettersOnly(s string) bool {
	hasLetter := false
	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		hasLetter = true
		if !unicode.Is(unicode.Latin, r) {
			return false
		}
	}
	return hasLetter
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// LocalizedNameUK prefers SQLite name_uk, then legacy Name mirror, then name_ru.
func LocalizedNameUK(p Place) string {
	rawUK := strings.TrimSpace(p.NameUK)
	rawName := strings.TrimSpace(p.Name)
	rawRU := strings.TrimSpace(p.NameRU)

	if rawUK != "" {
		return rawUK
	}
	if rawName != "" {
		return rawName
	}
	if rawRU != "" {
		return rawRU
	}
	return ""
}

// LocalizedNameRU prefers SQLite name_ru, then Ukrainian Cyrillic, then other fields.
func LocalizedNameRU(p Place) string {
	rawRU := strings.TrimSpace(p.NameRU)
	rawUK := strings.TrimSpace(p.NameUK)
	rawName := strings.TrimSpace(p.Name)

	if rawRU != "" {
		return rawRU
	}
	if rawUK != "" && hasCyrillic(rawUK) {
		return rawUK
	}
	if rawName != "" {
		return rawName
	}
	if rawUK != "" {
		return rawUK
	}
	return ""
}

// LocalizedNameEN builds English labels: Latin name_uk if present, else transliteration.
// There is no name_en column yet; when added, read it here first.
func LocalizedNameEN(p Place) string {
	rawUK := strings.TrimSpace(p.NameUK)
	rawRU := strings.TrimSpace(p.NameRU)

	// Latin exonym stored in name_uk (e.g. some DB rows)
	if isLatinLettersOnly(rawUK) {
		return rawUK
	}
	if rawRU != "" && hasCyrillic(rawRU) {
		return TranslitLatin(rawRU)
	}
	if rawUK != "" && hasCyrillic(rawUK) {
		return TranslitLatin(rawUK)
	}
	if rawRU != "" {
		return rawRU
	}
	if rawUK != "" {
		return rawUK
	}
	return ""
}

// LocalizedNameTriple returns display strings for API fields name_uk / name_ru / name_en.
func LocalizedNameTriple(p Place) (uk, ru, en string) {
	return LocalizedNameUK(p), LocalizedNameRU(p), LocalizedNameEN(p)
}

// LocalizedDisplayName returns the line to show for the active UI language.
// Order per language is documented in package comments / project README.
func LocalizedDisplayName(p Place, lang string) string {
	uk, ru, en := LocalizedNameTriple(p)
	fallback := strings.TrimSpace(p.Name)
	if fallback == "" {
		fallback = firstNonEmpty(uk, ru, en)
	}

	switch NormalizePlaceLang(lang) {
	case "uk":
		return firstNonEmpty(uk, ru, en, fallback)
	case "en":
		return firstNonEmpty(en, uk, ru, fallback)
	default: // ru
		return firstNonEmpty(ru, uk, en, fallback)
	}
}
