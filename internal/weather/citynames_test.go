package weather

import "testing"

func TestLocalizedCityName(t *testing.T) {
	tests := []struct {
		id   string
		lang string
		want string
	}{
		{"volnogorsk", "uk", "Вільногірськ"},
		{"volnogorsk", "ru", "Вольногорск"},
		{"volnogorsk", "en", "Vilnohorsk"},
		{"VOLNOGORSK", "uk", "Вільногірськ"},
		{"kyiv", "uk", "Київ"},
		{"kyiv", "ru", "Киев"},
		{"kyiv", "en", "Kyiv"},
		{"unknown-slug", "uk", "unknown-slug"},
	}
	for _, tt := range tests {
		got := LocalizedCityName(tt.id, tt.lang)
		if got != tt.want {
			t.Errorf("LocalizedCityName(%q, %q) = %q; want %q", tt.id, tt.lang, got, tt.want)
		}
	}
}
