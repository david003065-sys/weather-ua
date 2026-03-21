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
