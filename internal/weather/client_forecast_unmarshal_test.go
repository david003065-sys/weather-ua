package weather

import (
	"encoding/json"
	"testing"
)

func TestWeatherAPIHourUnmarshalNumericChanceFields(t *testing.T) {
	// WeatherAPI returns chance_of_rain / chance_of_snow as JSON numbers.
	const raw = `{"time_epoch":1704110400,"time":"2024-01-01 12:00","temp_c":5.2,"feelslike_c":4.1,"precip_mm":0.1,"snow_cm":0,"chance_of_rain":35,"chance_of_snow":10,"condition":{"code":1003}}`
	var h weatherAPIHour
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		t.Fatal(err)
	}
	if h.ChanceOfRain != 35 || h.ChanceOfSnow != 10 {
		t.Fatalf("ChanceOfRain=%d ChanceOfSnow=%d", h.ChanceOfRain, h.ChanceOfSnow)
	}
}
