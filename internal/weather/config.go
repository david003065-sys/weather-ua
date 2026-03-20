package weather

import (
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
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
var logMissingKeyOnce sync.Once
var loadDotEnvOnce sync.Once

func ensureDotEnvLoaded() {
	loadDotEnvOnce.Do(func() {
		// Silent load: .env is optional (Render uses real env vars).
		_ = godotenv.Load(".env")
	})
}

func getenvWithDotEnv(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v != "" {
		return v
	}
	ensureDotEnvLoaded()
	return strings.TrimSpace(os.Getenv(key))
}

// normalizedProvider возвращает имя провайдера из окружения; пустое значение = weatherapi.
func normalizedProvider() string {
	p := strings.ToLower(getenvWithDotEnv(EnvWeatherAPIProvider))
	if p == "" {
		return providerWeatherAPI
	}
	return p
}

func weatherAPIKey() string {
	return getenvWithDotEnv(EnvWeatherAPIKey)
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
	base := getenvWithDotEnv(EnvWeatherAPIBaseURL)
	base = strings.TrimSuffix(base, "/")
	if base == "" {
		return defaultWeatherAPIBase
	}
	return base
}

func logMissingAPIKeyOnce(logger Logger) {
	if logger == nil {
		return
	}
	logMissingKeyOnce.Do(func() {
		logger.Printf("weather provider key missing: set %s (or put it in .env for local dev)", EnvWeatherAPIKey)
	})
}
