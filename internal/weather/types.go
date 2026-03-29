package weather

import "time"

type City struct {
	ID        string
	Name      string
	Latitude  float64
	Longitude float64
}

type Current struct {
	Temperature float64
	// FeelsLike — ощущается как (°C), WeatherAPI: feelslike_c
	FeelsLike    float64
	WeatherCode  int
	Description  string
	Icon         string
	WindSpeed    float64
	Humidity     float64
	UVIndex      float64
	VisibilityKm float64
	Pressure     float64
	// PrecipitationChance — вероятность осадков на сегодня (%), -1 если недоступно.
	PrecipitationChance int
	IsNight      bool
}

type Daily struct {
	Date        time.Time
	MinTemp     float64
	MaxTemp     float64
	WeatherCode int
	Description string
	Icon        string
	// SunriseLocal, SunsetLocal — локальное время суток "15:04" из astro первого дня прогноза.
	SunriseLocal string
	SunsetLocal  string
}

type Hourly struct {
	Time        time.Time
	Temperature float64
	// FeelsLike — ощущается как (°C) для слота; WeatherAPI: hour.feelslike_c (аналог open-meteo apparent_temperature).
	FeelsLike   float64
	WeatherCode int
	Description string
	Icon        string
}

type WeatherData struct {
	CityID           string
	CityName         string
	Current          Current
	Forecast         []Daily
	Hourly           []Hourly
	Sunrise          time.Time
	Sunset           time.Time
	Timezone         string
	UTCOffsetSeconds int
	// LastUpdated — момент получения этого снимка от провайдера (UTC). Для кэша без поля используется Timestamp записи кэша.
	LastUpdated time.Time
	// IsStale: true только для устаревших, но реальных данных из кэша.
	IsStale bool
	// IsFallback: true только если реальных данных нет (кэш пуст и провайдер недоступен).
	IsFallback bool
}

// CachedWeather хранит ответ погоды и время кэширования.
type CachedWeather struct {
	Data      *WeatherData
	Timestamp time.Time
	IsValid   bool
	LastError string
}

// WeatherCache — кэш погоды по ключу (город или place:id).
// Ограничение: не более одного запроса к API на ключ в течение CacheTTL (10 мин).
type WeatherCache struct {
	Data map[string]CachedWeather
}
