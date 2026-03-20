package weather

import (
	"os"
	"strings"
	"sync"
)

// Переменные окружения для выбора провайдера и ключа.
const (
	EnvWeatherAPIProvider = "WEATHER_API_PROVIDER"
	EnvWeatherAPIKey      = "WEATHERAPI_KEY"
	EnvWeatherAPIBaseURL  = "WEATHERAPI_BASE_URL"
)

const (
	providerWeatherAPI   = "weatherapi"
	defaultWeatherAPIBase = "https://api.weatherapi.com/v1"
)

var logProviderOnce sync.Once

// normalizedProvider возвращает имя провайдера из окружения; пустое значение = weatherapi.
func normalizedProvider() string {
	p := strings.ToLower(strings.TrimSpace(os.Getenv(EnvWeatherAPIProvider)))
	if p == "" {
		return providerWeatherAPI
	}
	return p
}

func weatherAPIKey() string {
	return strings.TrimSpace(os.Getenv(EnvWeatherAPIKey))
}

func logProviderSelection(logger Logger) {
	if logger == nil {
		return
	}
	logProviderOnce.Do(func() {
		logger.Printf("weather provider selected: %s (override with %s)", normalizedProvider(), EnvWeatherAPIProvider)
	})
}

func weatherAPIBaseURL() string {
	base := strings.TrimSpace(os.Getenv(EnvWeatherAPIBaseURL))
	base = strings.TrimSuffix(base, "/")
	if base == "" {
		return defaultWeatherAPIBase
	}
	return base
}
