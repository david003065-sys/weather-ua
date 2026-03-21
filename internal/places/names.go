// Package places provides SQLite-backed settlement search.
//
// LocalizedDisplayName (UI + API pick by lang) uses strict per-language fallbacks — English is never
// used when UI is uk or ru:
//
//	lang uk: name_uk slot → name_ru slot → original (Name, then raw columns)
//	lang ru: name_ru slot → name_uk slot → original
//	lang en: name_en slot (derived) → name_uk → name_ru → original
//
// Slots are built from SQLite name_uk / name_ru / name (see LocalizedNameUK/RU/EN).
// Static /city/* labels remain in package weather (LocalizedCityName).
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

// LocalizedNameUK builds the Ukrainian-label slot (for JSON name_uk and lang=uk).
// Latin-only name_uk is skipped when Cyrillic exists in name/name_ru so UI is not shown as English.
func LocalizedNameUK(p Place) string {
	rawUK := strings.TrimSpace(p.NameUK)
	rawName := strings.TrimSpace(p.Name)
	rawRU := strings.TrimSpace(p.NameRU)

	pickCyrillic := func() string {
		if rawName != "" && hasCyrillic(rawName) {
			return rawName
		}
		if rawRU != "" && hasCyrillic(rawRU) {
			return rawRU
		}
		return ""
	}

	if rawUK != "" {
		if !isLatinLettersOnly(rawUK) {
			return rawUK
		}
		if cy := pickCyrillic(); cy != "" {
			return cy
		}
		return rawUK
	}
	if rawName != "" {
		if !isLatinLettersOnly(rawName) {
			return rawName
		}
		if cy := pickCyrillic(); cy != "" {
			return cy
		}
		return rawName
	}
	if rawRU != "" {
		return rawRU
	}
	return ""
}

// LocalizedNameRU builds the Russian-label slot (for JSON name_ru and lang=ru).
// Latin-only name_ru is skipped when Cyrillic exists in name_uk/name.
func LocalizedNameRU(p Place) string {
	rawRU := strings.TrimSpace(p.NameRU)
	rawUK := strings.TrimSpace(p.NameUK)
	rawName := strings.TrimSpace(p.Name)

	pickCyrillic := func() string {
		if rawUK != "" && hasCyrillic(rawUK) {
			return rawUK
		}
		if rawName != "" && hasCyrillic(rawName) {
			return rawName
		}
		return ""
	}

	if rawRU != "" {
		if !isLatinLettersOnly(rawRU) {
			return rawRU
		}
		if cy := pickCyrillic(); cy != "" {
			return cy
		}
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

// originalPlaceName is last-resort label: Name mirror, then raw name_uk, name_ru (never derived EN).
func originalPlaceName(p Place) string {
	if s := strings.TrimSpace(p.Name); s != "" {
		return s
	}
	return firstNonEmpty(strings.TrimSpace(p.NameUK), strings.TrimSpace(p.NameRU))
}

// LocalizedDisplayName returns the line to show for the active UI language.
// English is never chosen when lang is uk or ru.
func LocalizedDisplayName(p Place, lang string) string {
	uk := LocalizedNameUK(p)
	ru := LocalizedNameRU(p)
	en := LocalizedNameEN(p)
	orig := originalPlaceName(p)

	switch NormalizePlaceLang(lang) {
	case "uk":
		return firstNonEmpty(uk, ru, orig)
	case "en":
		return firstNonEmpty(en, uk, ru, orig)
	default: // ru
		return firstNonEmpty(ru, uk, orig)
	}
}
