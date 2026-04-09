package places

import (
	"strings"
	"testing"
)

func TestLoadAlternateNamesV2ForIDsFromReader(t *testing.T) {
	ids := map[string]struct{}{
		"100": {},
		"200": {},
	}
	// alternateNameId, geonameId, isolanguage, alternate name, isPreferredName
	raw := "" +
		"1\t100\ten\tAlpha\t1\n" +
		"2\t100\tuk\tАльфа\t0\n" +
		"3\t100\tuk\tАльфа офіційна\t1\n" +
		"4\t100\tru\tАльфа RU\t1\n" +
		"5\t200\tru\tБета\t0\n" +
		"6\t200\tru\tБета предпочт\t1\n" +
		"7\t999\ten\tSkip\t1\n"

	uk, ru, en, err := loadAlternateNamesV2FromReader(strings.NewReader(raw), ids)
	if err != nil {
		t.Fatal(err)
	}
	if got := uk["100"]; got != "Альфа офіційна" {
		t.Fatalf("uk[100] = %q want preferred Ukrainian", got)
	}
	if got := ru["100"]; got != "Альфа RU" {
		t.Fatalf("ru[100] = %q", got)
	}
	if got := en["100"]; got != "Alpha" {
		t.Fatalf("en[100] = %q", got)
	}
	if got := ru["200"]; got != "Бета предпочт" {
		t.Fatalf("ru[200] = %q want preferred Russian", got)
	}
	if _, ok := uk["200"]; ok {
		t.Fatalf("uk[200] should be absent")
	}
}
