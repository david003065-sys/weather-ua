package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Logger для сообщений кэша (если nil — логирование отключено).
type Logger interface {
	Printf(format string, args ...interface{})
}

type Client struct {
	httpClient *http.Client
	cacheTTL   time.Duration
	logger     Logger

	mu    sync.RWMutex
	cache map[string]cachedWeather
}

type cachedWeather struct {
	data      *WeatherData
	expiresAt time.Time
}

var errRateLimited = errors.New("weather api 429 too many requests")

func NewClient(cacheTTL, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		cacheTTL: cacheTTL,
		cache:    make(map[string]cachedWeather),
	}
}

// SetLogger задаёт логгер для сообщений кэша (weather cache hit/miss/update).
func (c *Client) SetLogger(l Logger) {
	c.logger = l
}

func (c *Client) GetWeather(ctx context.Context, cityID string) (*WeatherData, error) {
	city, ok := cityByID(cityID)
	if !ok {
		return nil, fmt.Errorf("unknown city: %s", cityID)
	}
	return c.getWeatherForCity(ctx, cityID, city)
}

// GetWeatherForLocation возвращает погоду по произвольным координатам.
// cacheKey должен быть стабильным (например, "place:123").
func (c *Client) GetWeatherForLocation(ctx context.Context, cacheKey, name string, lat, lon float64) (*WeatherData, error) {
	city := City{
		ID:        cacheKey,
		Name:      name,
		Latitude:  lat,
		Longitude: lon,
	}
	return c.getWeatherForCity(ctx, cacheKey, city)
}

func (c *Client) getWeatherForCity(ctx context.Context, cacheKey string, city City) (*WeatherData, error) {
	now := time.Now()

	c.mu.RLock()
	cached := c.cache[cacheKey]
	c.mu.RUnlock()

	if cached.data != nil && now.Before(cached.expiresAt) {
		if c.logger != nil {
			c.logger.Printf("weather cache hit")
		}
		return cached.data, nil
	}

	if c.logger != nil {
		c.logger.Printf("weather cache miss")
	}

	data, err := c.fetchFromAPI(ctx, city)
	if err != nil {
		if cached.data != nil {
			if c.logger != nil && errors.Is(err, errRateLimited) {
				c.logger.Printf("weather api 429, using cache")
			}
			return cached.data, nil
		}
		return nil, err
	}

	c.mu.Lock()
	c.cache[cacheKey] = cachedWeather{
		data:      data,
		expiresAt: now.Add(c.cacheTTL),
	}
	c.mu.Unlock()

	if c.logger != nil {
		c.logger.Printf("weather cache update")
	}
	return data, nil
}

func (c *Client) fetchFromAPI(ctx context.Context, city City) (*WeatherData, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&timezone=auto&current_weather=true&hourly=temperature_2m,weathercode,relativehumidity_2m,surface_pressure&daily=temperature_2m_max,temperature_2m_min,weathercode,sunrise,sunset&forecast_days=3",
		city.Latitude, city.Longitude,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, errRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather api status: %s", resp.Status)
	}

	var apiRes openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiRes); err != nil {
		return nil, err
	}

	if apiRes.CurrentWeather == nil || apiRes.Daily == nil {
		return nil, errors.New("missing current_weather in response")
	}

	humidity := c.extractHumidity(apiRes)
	pressure := c.extractPressure(apiRes)
	cond := describeCode(apiRes.CurrentWeather.WeatherCode)

	now := parseLocalTime(apiRes.CurrentWeather.Time)
	sunrise, sunset := extractSunTimes(apiRes)

	isNight := false
	if !sunrise.IsZero() && !sunset.IsZero() && !now.IsZero() {
		isNight = now.Before(sunrise) || now.After(sunset)
	} else if apiRes.CurrentWeather.IsDay == 0 {
		isNight = true
	}

	current := Current{
		Temperature: apiRes.CurrentWeather.Temperature,
		WeatherCode: apiRes.CurrentWeather.WeatherCode,
		Description: "",
		Icon:        cond.Icon,
		WindSpeed:   apiRes.CurrentWeather.Windspeed,
		Humidity:    humidity,
		Pressure:    pressure,
		IsNight:     isNight,
	}

	forecast := c.buildForecast(apiRes)
	hourly := c.buildHourly(apiRes)

	data := &WeatherData{
		CityID:           city.ID,
		CityName:         city.Name,
		Current:          current,
		Forecast:         forecast,
		Hourly:           hourly,
		Sunrise:          sunrise,
		Sunset:           sunset,
		Timezone:         apiRes.Timezone,
		UTCOffsetSeconds: apiRes.UTCOffsetSeconds,
	}

	return data, nil
}

type openMeteoResponse struct {
	Timezone         string `json:"timezone"`
	UTCOffsetSeconds int    `json:"utc_offset_seconds"`
	CurrentWeather   *struct {
		Temperature float64 `json:"temperature"`
		Windspeed   float64 `json:"windspeed"`
		WeatherCode int     `json:"weathercode"`
		Time        string  `json:"time"`
		IsDay       int     `json:"is_day"`
	} `json:"current_weather"`
	Hourly *struct {
		Time             []string  `json:"time"`
		Temperature2M    []float64 `json:"temperature_2m"`
		WeatherCode      []int     `json:"weathercode"`
		RelativeHumidity []float64 `json:"relativehumidity_2m"`
		SurfacePressure  []float64 `json:"surface_pressure"`
	} `json:"hourly"`
	Daily *struct {
		Time     []string  `json:"time"`
		TempMax  []float64 `json:"temperature_2m_max"`
		TempMin  []float64 `json:"temperature_2m_min"`
		Weather  []int     `json:"weathercode"`
		Sunrise  []string  `json:"sunrise"`
		Sunset   []string  `json:"sunset"`
	} `json:"daily"`
}

func (c *Client) extractHumidity(res openMeteoResponse) float64 {
	if res.Hourly == nil || len(res.Hourly.Time) == 0 || len(res.Hourly.RelativeHumidity) == 0 {
		return 0
	}

	target := ""
	if res.CurrentWeather != nil {
		target = res.CurrentWeather.Time
	}

	if target != "" {
		for i, t := range res.Hourly.Time {
			if t == target && i < len(res.Hourly.RelativeHumidity) {
				return res.Hourly.RelativeHumidity[i]
			}
		}
	}

	return res.Hourly.RelativeHumidity[0]
}

func (c *Client) extractPressure(res openMeteoResponse) float64 {
	if res.Hourly == nil || len(res.Hourly.Time) == 0 || len(res.Hourly.SurfacePressure) == 0 {
		return 0
	}

	target := ""
	if res.CurrentWeather != nil {
		target = res.CurrentWeather.Time
	}

	if target != "" {
		for i, t := range res.Hourly.Time {
			if t == target && i < len(res.Hourly.SurfacePressure) {
				// гПа ~ миллибар ~= мм рт. ст. * 1.333
				return res.Hourly.SurfacePressure[i] / 1.333
			}
		}
	}

	return res.Hourly.SurfacePressure[0] / 1.333
}

func (c *Client) buildForecast(res openMeteoResponse) []Daily {
	if res.Daily == nil || len(res.Daily.Time) == 0 {
		return nil
	}

	n := len(res.Daily.Time)
	if n > 3 {
		n = 3
	}

	out := make([]Daily, 0, n)
	for i := 0; i < n; i++ {
		dateStr := res.Daily.Time[i]
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		code := 0
		if i < len(res.Daily.Weather) {
			code = res.Daily.Weather[i]
		}
		cond := describeCode(code)

		minTemp := 0.0
		maxTemp := 0.0
		if i < len(res.Daily.TempMin) {
			minTemp = res.Daily.TempMin[i]
		}
		if i < len(res.Daily.TempMax) {
			maxTemp = res.Daily.TempMax[i]
		}

		out = append(out, Daily{
			Date:        date,
			MinTemp:     minTemp,
			MaxTemp:     maxTemp,
			WeatherCode: code,
			Description: "",
			Icon:        cond.Icon,
		})
	}

	return out
}

func (c *Client) buildHourly(res openMeteoResponse) []Hourly {
	if res.Hourly == nil || len(res.Hourly.Time) == 0 || len(res.Hourly.Temperature2M) == 0 {
		return nil
	}

	// try to align with current time
	startIdx := 0
	currentTime := ""
	if res.CurrentWeather != nil {
		currentTime = res.CurrentWeather.Time
	}
	if currentTime != "" {
		for i, t := range res.Hourly.Time {
			if t == currentTime {
				startIdx = i
				break
			}
		}
	}

	n := 12
	if startIdx+n > len(res.Hourly.Time) {
		n = len(res.Hourly.Time) - startIdx
	}
	if n <= 0 {
		return nil
	}

	out := make([]Hourly, 0, n)
	for i := 0; i < n; i++ {
		idx := startIdx + i
		if idx >= len(res.Hourly.Time) {
			break
		}
		t := parseLocalTime(res.Hourly.Time[idx])
		temp := res.Hourly.Temperature2M[idx]

		code := 0
		if idx < len(res.Hourly.WeatherCode) {
			code = res.Hourly.WeatherCode[idx]
		}
		cond := describeCode(code)

		out = append(out, Hourly{
			Time:        t,
			Temperature: temp,
			WeatherCode: code,
			Description: "",
			Icon:        cond.Icon,
		})
	}

	return out
}

func parseLocalTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	// Open‑Meteo uses "2006-01-02T15:04" without offset
	t, err := time.Parse("2006-01-02T15:04", value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func extractSunTimes(res openMeteoResponse) (time.Time, time.Time) {
	if res.Daily == nil || len(res.Daily.Sunrise) == 0 || len(res.Daily.Sunset) == 0 {
		return time.Time{}, time.Time{}
	}
	sunrise := parseLocalTime(res.Daily.Sunrise[0])
	sunset := parseLocalTime(res.Daily.Sunset[0])
	return sunrise, sunset
}

type condition struct {
	RU   string
	EN   string
	UK   string
	Icon string
}

func describeCode(code int) condition {
	switch code {
	case 0:
		return condition{"Ясно", "Clear sky", "Ясно", "☀️"}
	case 1, 2:
		return condition{"Преимущественно ясно", "Mostly clear", "Переважно ясно", "🌤"}
	case 3:
		return condition{"Облачно", "Cloudy", "Хмарно", "☁️"}
	case 45, 48:
		return condition{"Туман", "Fog", "Туман", "🌫"}
	case 51, 53, 55:
		return condition{"Мелкий дождь", "Light drizzle", "Невеликий дощ", "🌦"}
	case 61, 63, 65:
		return condition{"Дождь", "Rain", "Дощ", "🌧"}
	case 66, 67:
		return condition{"Ледяной дождь", "Freezing rain", "Крижаний дощ", "🌧"}
	case 71, 73, 75, 77:
		return condition{"Снег", "Snow", "Сніг", "❄️"}
	case 80, 81, 82:
		return condition{"Ливни", "Rain showers", "Зливи", "🌧"}
	case 95:
		return condition{"Гроза", "Thunderstorm", "Гроза", "⛈"}
	case 96, 97, 99:
		return condition{"Гроза с осадками", "Thunderstorm with hail", "Гроза з опадами", "⛈"}
	default:
		return condition{"Неизвестно", "Unknown", "Невідомо", "❔"}
	}
}

func LocalizedDescription(code int, lang string) string {
	c := describeCode(code)
	switch lang {
	case "en":
		return c.EN
	case "uk":
		return c.UK
	default:
		return c.RU
	}
}

