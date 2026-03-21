package places

import "testing"

func TestLocalizedNameTriple_fromDBFields(t *testing.T) {
	p := Place{
		Name:   "Харків",
		NameUK: "Харків",
		NameRU: "Харьков",
	}
	uk, ru, en := LocalizedNameTriple(p)
	if uk != "Харків" {
		t.Fatalf("uk = %q", uk)
	}
	if ru != "Харьков" {
		t.Fatalf("ru = %q", ru)
	}
	if en == "" {
		t.Fatal("expected EN transliteration")
	}
}

func TestLocalizedDisplayName_order(t *testing.T) {
	p := Place{NameUK: "Львів", NameRU: "Львов", Name: "Львів"}
	if got := LocalizedDisplayName(p, "uk"); got != "Львів" {
		t.Fatalf("uk UI: %q", got)
	}
	if got := LocalizedDisplayName(p, "ru"); got != "Львов" {
		t.Fatalf("ru UI: %q", got)
	}
}

func TestLocalizedNameUK_fallsBackToRU(t *testing.T) {
	p := Place{NameUK: "", Name: "", NameRU: "Одесса"}
	if got := LocalizedNameUK(p); got != "Одесса" {
		t.Fatalf("got %q", got)
	}
}

// Latin name_uk (bad GeoNames row) must not win over Cyrillic name_ru for Ukrainian UI.
func TestLocalizedNameUK_latinUK_skipsToCyrillicRU(t *testing.T) {
	p := Place{NameUK: "Vilnohirsk", NameRU: "Вільногірськ", Name: ""}
	if got := LocalizedNameUK(p); got != "Вільногірськ" {
		t.Fatalf("LocalizedNameUK = %q", got)
	}
	if got := LocalizedDisplayName(p, "uk"); got != "Вільногірськ" {
		t.Fatalf("uk UI: %q", got)
	}
}

func TestLocalizedDisplayName_volnogorskUKHotfix(t *testing.T) {
	p := Place{NameUK: "", NameRU: "Вольногорск", Name: ""}
	if got := LocalizedDisplayName(p, "uk"); got != "Вільногірськ" {
		t.Fatalf("uk UI: %q", got)
	}
	if got := LocalizedDisplayName(p, "ru"); got != "Вольногорск" {
		t.Fatalf("ru UI unchanged: %q", got)
	}
	p2 := Place{NameUK: "Volnogorsk", NameRU: "", Name: ""}
	if got := LocalizedDisplayName(p2, "uk"); got != "Вільногірськ" {
		t.Fatalf("uk UI slug-like name: %q", got)
	}
}

func TestLocalizedDisplayName_ukDoesNotUseEnglish(t *testing.T) {
	// No Cyrillic anywhere: only Latin uk + derived EN — uk chain must not pick EN.
	p := Place{NameUK: "Kyiv", NameRU: "", Name: ""}
	if got := LocalizedDisplayName(p, "uk"); got != "Kyiv" {
		t.Fatalf("uk UI: %q (expected raw uk/original, not translit EN)", got)
	}
}
