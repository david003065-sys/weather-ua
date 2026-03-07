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
	WeatherCode int
	Description string
	Icon        string
	WindSpeed   float64
	Humidity    float64
	Pressure    float64
	IsNight     bool
}

type Daily struct {
	Date        time.Time
	MinTemp     float64
	MaxTemp     float64
	WeatherCode int
	Description string
	Icon        string
}

type Hourly struct {
	Time        time.Time
	Temperature float64
	WeatherCode int
	Description string
	Icon        string
}

type WeatherData struct {
	CityID   string
	CityName string
	Current  Current
	Forecast []Daily
	Hourly   []Hourly
	Sunrise  time.Time
	Sunset   time.Time
	Timezone         string
	UTCOffsetSeconds int
	// IsFallback: true если данные недоступны (429 и кэша нет) — показать "Данные временно недоступны".
	IsFallback bool
}

// CachedWeather хранит ответ погоды и время кэширования.
type CachedWeather struct {
	Weather   *WeatherData
	Timestamp time.Time
}

// WeatherCache — кэш погоды по ключу (город или place:id).
// Ограничение: не более одного запроса к API на ключ в течение CacheTTL (10 мин).
type WeatherCache struct {
	Data map[string]CachedWeather
}

