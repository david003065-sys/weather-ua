package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Logger для сообщений кэша (если nil — логирование отключено).
type Logger interface {
	Printf(format string, args ...interface{})
}

const (
	// cacheTTLDefault — TTL «свежего» кэша (успешный ответ API).
	cacheTTLDefault = 15 * time.Minute
	// apiCooldownAfter429 — глобальная пауза запросов к API после 429 (5–10 мин).
	apiCooldownAfter429 = 7 * time.Minute
	cooldownLogEvery    = 1 * time.Minute
)

var (
	apiBlockedUntil time.Time
	apiBlockMu      sync.RWMutex
	lastCooldownLog time.Time
	cooldownLogMu   sync.Mutex
)

var (
	errRateLimited   = errors.New("open-meteo rate limited")
	errCooldownActive = errors.New("api cooldown active")
)

type Client struct {
	httpClient *http.Client
	cacheTTL   time.Duration
	logger     Logger

	mu    sync.RWMutex
	cache map[string]CachedWeather

	sf singleflight.Group
}

func NewClient(cacheTTL, timeout time.Duration) *Client {
	if cacheTTL <= 0 {
		cacheTTL = cacheTTLDefault
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		cacheTTL: cacheTTL,
		cache:    make(map[string]CachedWeather),
	}
}

// SetLogger задаёт логгер для сообщений кэша.
func (c *Client) SetLogger(l Logger) {
	c.logger = l
}

func normalizeCacheKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

// cacheKeyFromLatLon возвращает стабильный ключ по координатам (округление до 3 знаков).
func cacheKeyFromLatLon(lat, lon float64) string {
	const prec = 3
	latR := float64(int(lat*1e3+0.5)) / 1e3
	lonR := float64(int(lon*1e3+0.5)) / 1e3
	return "loc:" + strconv.FormatFloat(latR, 'f', prec, 64) + "_" + strconv.FormatFloat(lonR, 'f', prec, 64)
}

func (c *Client) GetWeather(ctx context.Context, cityID string) (*WeatherData, error) {
	city, ok := cityByID(cityID)
	if !ok {
		return nil, fmt.Errorf("unknown city: %s", cityID)
	}
	key := normalizeCacheKey(cityID)
	return c.getWeatherForCity(ctx, key, city)
}

// GetWeatherForLocation возвращает погоду по произвольным координатам.
// cacheKey — стабильный ключ (например "place:123"); если пустой — используется lat_lon до 3 знаков.
func (c *Client) GetWeatherForLocation(ctx context.Context, cacheKey, name string, lat, lon float64) (*WeatherData, error) {
	key := normalizeCacheKey(cacheKey)
	if key == "" {
		key = cacheKeyFromLatLon(lat, lon)
	}
	city := City{
		ID:        key,
		Name:      name,
		Latitude:  lat,
		Longitude: lon,
	}
	return c.getWeatherForCity(ctx, key, city)
}

func (c *Client) getWeatherForCity(ctx context.Context, cacheKey string, city City) (*WeatherData, error) {
	cacheKey = normalizeCacheKey(cacheKey)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	now := time.Now()
	c.mu.RLock()
	entry := c.cache[cacheKey]
	c.mu.RUnlock()

	hasValid := entry.Data != nil && entry.IsValid
	fresh := hasValid && now.Sub(entry.Timestamp) < c.cacheTTL

	if fresh {
		if c.logger != nil {
			c.logger.Printf("cache hit (fresh) key=%s", cacheKey)
		}
		return dupWeather(entry.Data, false, false), nil
	}

	if isAPICooldownActive() {
		if c.logger != nil && shouldLogCooldown() {
			c.logger.Printf("cooldown active")
		}
		if hasValid {
			if c.logger != nil {
				c.logger.Printf("cache hit (stale) key=%s", cacheKey)
			}
			return dupWeather(entry.Data, true, false), nil
		}
		if c.logger != nil {
			c.logger.Printf("api error: no cache while cooldown key=%s", cacheKey)
		}
		return emptyNoData(city), nil
	}

	v, err, _ := c.sf.Do(cacheKey, func() (interface{}, error) {
		d, e := c.singleflightFetch(ctx, cacheKey, city)
		return d, e
	})
	if err != nil {
		return nil, err
	}
	return v.(*WeatherData), nil
}

func (c *Client) singleflightFetch(ctx context.Context, cacheKey string, city City) (*WeatherData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	now := time.Now()
	c.mu.RLock()
	entry := c.cache[cacheKey]
	c.mu.RUnlock()

	hasValid := entry.Data != nil && entry.IsValid
	fresh := hasValid && now.Sub(entry.Timestamp) < c.cacheTTL
	if fresh {
		if c.logger != nil {
			c.logger.Printf("cache hit (fresh) key=%s (coalesced)", cacheKey)
		}
		return dupWeather(entry.Data, false, false), nil
	}

	if isAPICooldownActive() {
		if c.logger != nil && shouldLogCooldown() {
			c.logger.Printf("cooldown active")
		}
		if hasValid {
			if c.logger != nil {
				c.logger.Printf("cache hit (stale) key=%s", cacheKey)
			}
			return dupWeather(entry.Data, true, false), nil
		}
		return emptyNoData(city), nil
	}

	if c.logger != nil {
		c.logger.Printf("api request key=%s city=%s lat=%.4f lon=%.4f", cacheKey, city.Name, city.Latitude, city.Longitude)
	}

	data, err := c.fetchFromAPI(ctx, city)
	if err != nil {
		if c.logger != nil {
			c.logger.Printf("api error key=%s: %v", cacheKey, err)
		}
		c.mu.RLock()
		entry = c.cache[cacheKey]
		c.mu.RUnlock()
		hasValid = entry.Data != nil && entry.IsValid
		if hasValid {
			if c.logger != nil {
				c.logger.Printf("cache hit (stale) key=%s after api error", cacheKey)
			}
			return dupWeather(entry.Data, true, false), nil
		}
		return emptyNoData(city), nil
	}

	stored := dupStoreWeather(data)
	now = time.Now()
	c.mu.Lock()
	c.cache[cacheKey] = CachedWeather{
		Data:      stored,
		Timestamp: now,
		IsValid:   true,
		LastError: "",
	}
	c.mu.Unlock()

	if c.logger != nil {
		c.logger.Printf("cache store success key=%s", cacheKey)
	}
	return dupWeather(stored, false, false), nil
}

func emptyNoData(city City) *WeatherData {
	return &WeatherData{
		CityID:     city.ID,
		CityName:   city.Name,
		IsStale:    false,
		IsFallback: true,
	}
}

func dupWeather(src *WeatherData, stale, fallback bool) *WeatherData {
	out := *src
	out.IsStale = stale
	out.IsFallback = fallback
	return &out
}

func dupStoreWeather(src *WeatherData) *WeatherData {
	if src == nil {
		return nil
	}
	out := *src
	out.IsStale = false
	out.IsFallback = false
	return &out
}

func isAPICooldownActive() bool {
	apiBlockMu.RLock()
	defer apiBlockMu.RUnlock()
	return time.Now().Before(apiBlockedUntil)
}

func blockAPIAfter429() {
	apiBlockMu.Lock()
	apiBlockedUntil = time.Now().Add(apiCooldownAfter429)
	apiBlockMu.Unlock()
}

func shouldLogCooldown() bool {
	cooldownLogMu.Lock()
	defer cooldownLogMu.Unlock()
	now := time.Now()
	if lastCooldownLog.IsZero() || now.Sub(lastCooldownLog) >= cooldownLogEvery {
		lastCooldownLog = now
		return true
	}
	return false
}

type WarmTarget struct {
	Key  string
	Name string
	Lat  float64
	Lon  float64
}

func defaultWarmTargets() []WarmTarget {
	return []WarmTarget{
		{Key: "kyiv", Name: "Kyiv", Lat: 50.4501, Lon: 30.5234},
		{Key: "dnipro", Name: "Dnipro", Lat: 48.467, Lon: 35.040},
		{Key: "warm:kharkiv", Name: "Kharkiv", Lat: 49.9935, Lon: 36.2304},
		{Key: "warm:lviv", Name: "Lviv", Lat: 49.8397, Lon: 24.0297},
		{Key: "warm:odesa", Name: "Odesa", Lat: 46.4825, Lon: 30.7233},
	}
}

// WarmCache performs background warm-up for popular cities.
func (c *Client) WarmCache(ctx context.Context) {
	if c.logger != nil {
		c.logger.Printf("cache warm started")
	}
	targets := defaultWarmTargets()
	success := 0
	for _, t := range targets {
		callCtx, cancel := context.WithTimeout(ctx, 7*time.Second)
		data, err := c.GetWeatherForLocation(callCtx, t.Key, t.Name, t.Lat, t.Lon)
		cancel()
		if err != nil || data == nil || data.IsFallback {
			if c.logger != nil {
				c.logger.Printf("cache warm failed")
			}
			continue
		}
		success++
	}
	if c.logger != nil {
		if success > 0 {
			c.logger.Printf("cache warm success")
		} else {
			c.logger.Printf("cache warm failed")
		}
	}
}

func (c *Client) fetchFromAPI(ctx context.Context, city City) (*WeatherData, error) {
	if isAPICooldownActive() {
		return nil, errCooldownActive
	}

	if c.logger != nil {
		c.logger.Printf("open-meteo request started")
	}
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	baseURL := "https://api.open-meteo.com/v1/forecast"
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(city.Latitude, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(city.Longitude, 'f', 4, 64))
	q.Set("current_weather", "true")
	q.Set("hourly", "temperature_2m,weathercode,relativehumidity_2m,surface_pressure")
	q.Set("daily", "temperature_2m_max,temperature_2m_min,weathercode,sunrise,sunset")
	q.Set("timezone", "auto")
	q.Set("forecast_days", "3")
	fullURL := baseURL + "?" + q.Encode()

	if c.logger != nil {
		c.logger.Printf("open-meteo request url: %s", fullURL)
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Printf("open-meteo network error: %v", err)
		}
		return nil, err
	}
	defer resp.Body.Close()
	if c.logger != nil {
		c.logger.Printf("open-meteo response status code: %d", resp.StatusCode)
	}

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		if c.logger != nil {
			c.logger.Printf("open-meteo read body failed: %v", readErr)
		}
		return nil, readErr
	}
	if c.logger != nil && os.Getenv("WEATHER_DEBUG") == "1" {
		c.logger.Printf("open-meteo response body: %s", snippet(bodyBytes, 1000))
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			if c.logger != nil {
				c.logger.Printf("429 detected: global cooldown %v", apiCooldownAfter429)
			}
			blockAPIAfter429()
			return nil, errRateLimited
		}
		if c.logger != nil {
			c.logger.Printf("open-meteo non-200 status: %s body=%s", resp.Status, snippet(bodyBytes, 500))
		}
		return nil, fmt.Errorf("open-meteo status: %s", resp.Status)
	}

	var apiRes openMeteoResponse
	if err := json.Unmarshal(bodyBytes, &apiRes); err != nil {
		if c.logger != nil {
			c.logger.Printf("open-meteo decode failed: %v body=%s", err, snippet(bodyBytes, 500))
		}
		return nil, err
	}
	if c.logger != nil {
		c.logger.Printf("open-meteo decode success")
	}

	if apiRes.CurrentWeather == nil || apiRes.Daily == nil {
		if c.logger != nil {
			c.logger.Printf("open-meteo parsed empty payload")
		}
		return nil, errors.New("missing current_weather in response")
	}
	if c.logger != nil {
		hCount := 0
		dCount := 0
		if apiRes.Hourly != nil {
			hCount = len(apiRes.Hourly.Temperature2M)
		}
		if apiRes.Daily != nil {
			dCount = len(apiRes.Daily.TempMax)
		}
		c.logger.Printf("open-meteo parsed current weather: temp=%.1f wind=%.1f", apiRes.CurrentWeather.Temperature, apiRes.CurrentWeather.Windspeed)
		c.logger.Printf("open-meteo parsed hourly count: %d", hCount)
		c.logger.Printf("open-meteo parsed daily count: %d", dCount)
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
		IsStale:          false,
		IsFallback:       false,
	}
	if c.logger != nil {
		c.logger.Printf("open-meteo success")
	}

	return data, nil
}

func snippet(b []byte, max int) string {
	if len(b) <= max {
		return strings.ReplaceAll(string(b), "\n", " ")
	}
	return strings.ReplaceAll(string(b[:max]), "\n", " ") + "...(truncated)"
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
		Time    []string  `json:"time"`
		TempMax []float64 `json:"temperature_2m_max"`
		TempMin []float64 `json:"temperature_2m_min"`
		Weather []int     `json:"weathercode"`
		Sunrise []string  `json:"sunrise"`
		Sunset  []string  `json:"sunset"`
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
