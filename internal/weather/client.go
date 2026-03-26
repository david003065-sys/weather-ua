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

	"bss/internal/i18n"

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
	errProviderRateLimited = errors.New("provider rate limited")
	errCooldownActive      = errors.New("api cooldown active")
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

	logProviderSelection(c.logger)

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
		return enrichFromCacheEntry(entry, false, false), nil
	}

	// Provider precheck: no key -> no provider flow, use cache/no-data only.
	if !providerReady(c.logger) {
		if hasValid {
			return c.returnStaleFromCache(entry, cacheKey, false), nil
		}
		return emptyNoData(city), nil
	}

	if isAPICooldownActive() {
		if c.logger != nil && shouldLogCooldown() {
			c.logger.Printf("cooldown active")
		}
		if hasValid {
			return c.returnStaleFromCache(entry, cacheKey, false), nil
		}
		return c.returnNoData(city, cacheKey, "cooldown active, no valid cache"), nil
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
		return enrichFromCacheEntry(entry, false, false), nil
	}

	// Provider precheck: no key -> no provider flow, use cache/no-data only.
	if !providerReady(c.logger) {
		if hasValid {
			return c.returnStaleFromCache(entry, cacheKey, false), nil
		}
		return emptyNoData(city), nil
	}

	if isAPICooldownActive() {
		if c.logger != nil && shouldLogCooldown() {
			c.logger.Printf("cooldown active")
		}
		if hasValid {
			return c.returnStaleFromCache(entry, cacheKey, false), nil
		}
		return c.returnNoData(city, cacheKey, "cooldown active, no valid cache"), nil
	}

	if c.logger != nil {
		c.logger.Printf("weather request cityID=%s cacheKey=%s", city.ID, cacheKey)
		c.logger.Printf("provider request started provider=%s key=%s city=%s lat=%.4f lon=%.4f",
			normalizedProvider(), cacheKey, city.Name, city.Latitude, city.Longitude)
	}

	data, err := c.fetchFromAPI(ctx, city)
	if err != nil {
		if c.logger != nil {
			c.logger.Printf("provider error key=%s: %v", cacheKey, err)
		}
		c.mu.RLock()
		entry = c.cache[cacheKey]
		c.mu.RUnlock()
		hasValid = entry.Data != nil && entry.IsValid
		if hasValid {
			return c.returnStaleFromCache(entry, cacheKey, true), nil
		}
		return c.returnNoData(city, cacheKey, "provider error, no valid cache"), nil
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
	out.LastUpdated = time.Now().UTC()
	return &out
}

func enrichFromCacheEntry(entry CachedWeather, stale, fallback bool) *WeatherData {
	out := dupWeather(entry.Data, stale, fallback)
	if out.LastUpdated.IsZero() {
		out.LastUpdated = entry.Timestamp.UTC()
	}
	return out
}

func (c *Client) returnStaleFromCache(entry CachedWeather, cacheKey string, afterProviderError bool) *WeatherData {
	out := enrichFromCacheEntry(entry, true, false)
	if c.logger != nil {
		if afterProviderError {
			c.logger.Printf("cache hit (stale) key=%s after provider error", cacheKey)
		} else {
			c.logger.Printf("cache hit (stale) key=%s", cacheKey)
		}
		c.logger.Printf("stale cache returned key=%s", cacheKey)
	}
	return out
}

func (c *Client) returnNoData(city City, cacheKey, reason string) *WeatherData {
	if c.logger != nil {
		c.logger.Printf("no cache available key=%s (%s)", cacheKey, reason)
	}
	return emptyNoData(city)
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

func shouldTriggerProviderCooldown(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusForbidden:
		return true
	default:
		return false
	}
}

func shouldTriggerCooldownFromAPIErrorCode(code int) bool {
	// 2007 quota exceeded, 2008 disabled key — backoff to protect the app
	switch code {
	case 2007, 2008:
		return true
	default:
		return false
	}
}

func (c *Client) fetchFromAPI(ctx context.Context, city City) (*WeatherData, error) {
	if isAPICooldownActive() {
		return nil, errCooldownActive
	}

	if normalizedProvider() != providerWeatherAPI {
		if c.logger != nil {
			c.logger.Printf("provider error: unsupported provider %q (only %q supported)", normalizedProvider(), providerWeatherAPI)
		}
		return nil, fmt.Errorf("unsupported weather provider %q", normalizedProvider())
	}

	apiKey := weatherAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("%s is not set", EnvWeatherAPIKey)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	q := url.Values{}
	q.Set("key", apiKey)
	q.Set("q", fmt.Sprintf("%.4f,%.4f", city.Latitude, city.Longitude))
	q.Set("days", "3")
	// Почасовые ряды берём из forecast.forecastday[].hour (temp + condition) — аналог запроса hourly к Open-Meteo.
	fullURL := weatherAPIBaseURL() + "/forecast.json?" + q.Encode()

	if c.logger != nil {
		c.logger.Printf("provider http request url=%s/forecast.json q=%.4f,%.4f days=3", weatherAPIBaseURL(), city.Latitude, city.Longitude)
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Printf("provider error: network: %v", err)
		}
		return nil, err
	}
	defer resp.Body.Close()

	if c.logger != nil {
		c.logger.Printf("provider response status=%d", resp.StatusCode)
	}

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		if c.logger != nil {
			c.logger.Printf("provider error: read body: %v", readErr)
		}
		return nil, readErr
	}
	if c.logger != nil && os.Getenv("WEATHER_DEBUG") == "1" {
		c.logger.Printf("provider response body (debug): %s", snippet(bodyBytes, 1000))
	}

	if resp.StatusCode != http.StatusOK {
		if shouldTriggerProviderCooldown(resp.StatusCode) {
			if c.logger != nil {
				if resp.StatusCode == http.StatusTooManyRequests {
					c.logger.Printf("provider rate limit detected: http_status=%d cooldown=%v", resp.StatusCode, apiCooldownAfter429)
				} else {
					c.logger.Printf("provider quota or access denied: http_status=%d cooldown=%v", resp.StatusCode, apiCooldownAfter429)
				}
			}
			blockAPIAfter429()
			return nil, errProviderRateLimited
		}
		if c.logger != nil {
			c.logger.Printf("provider error: http %s body=%s", resp.Status, snippet(bodyBytes, 500))
		}
		return nil, fmt.Errorf("provider http status: %s", resp.Status)
	}

	var apiRes weatherAPIForecastResponse
	if err := json.Unmarshal(bodyBytes, &apiRes); err != nil {
		if c.logger != nil {
			c.logger.Printf("provider error: json decode: %v body=%s", err, snippet(bodyBytes, 500))
		}
		return nil, err
	}

	if apiRes.Error != nil && apiRes.Error.Message != "" {
		if c.logger != nil {
			c.logger.Printf("provider error: api code=%d message=%s", apiRes.Error.Code, apiRes.Error.Message)
		}
		if shouldTriggerCooldownFromAPIErrorCode(apiRes.Error.Code) {
			if c.logger != nil {
				c.logger.Printf("provider rate limit detected: api_error_code=%d cooldown=%v", apiRes.Error.Code, apiCooldownAfter429)
			}
			blockAPIAfter429()
			return nil, errProviderRateLimited
		}
		return nil, fmt.Errorf("provider api error %d: %s", apiRes.Error.Code, apiRes.Error.Message)
	}

	if apiRes.Location == nil || apiRes.Current == nil || apiRes.Forecast == nil || len(apiRes.Forecast.Forecastday) == 0 {
		if c.logger != nil {
			c.logger.Printf("provider error: empty payload")
		}
		return nil, errors.New("provider: missing location, current or forecast")
	}

	loc, err := time.LoadLocation(apiRes.Location.TzID)
	if err != nil {
		loc = time.UTC
	}

	data, convErr := buildWeatherDataFromWeatherAPI(city, apiRes, loc)
	if convErr != nil {
		if c.logger != nil {
			c.logger.Printf("provider error: map response: %v", convErr)
		}
		return nil, convErr
	}

	if c.logger != nil {
		nh := len(data.Hourly)
		nd := len(data.Forecast)
		c.logger.Printf("provider success: temp_c=%.1f wind_kph=%.1f hourly=%d daily=%d",
			data.Current.Temperature, data.Current.WindSpeed, nh, nd)
	}

	return data, nil
}

func snippet(b []byte, max int) string {
	if len(b) <= max {
		return strings.ReplaceAll(string(b), "\n", " ")
	}
	return strings.ReplaceAll(string(b[:max]), "\n", " ") + "...(truncated)"
}

// weatherAPIForecastResponse — подмножество ответа /forecast.json (WeatherAPI.com).
type weatherAPIForecastResponse struct {
	Location *struct {
		TzID           string `json:"tz_id"`
		LocaltimeEpoch int64  `json:"localtime_epoch"`
		Localtime      string `json:"localtime"`
	} `json:"location"`
	Current *struct {
		TempC      float64 `json:"temp_c"`
		FeelslikeC float64 `json:"feelslike_c"`
		IsDay      int     `json:"is_day"`
		WindKph    float64 `json:"wind_kph"`
		// UV index (WeatherAPI: current.uv)
		UV float64 `json:"uv"`
		// Visibility: km preferred; miles used if km is zero (аналог open-meteo visibility).
		VisKm      float64 `json:"vis_km"`
		VisMiles   float64 `json:"vis_miles"`
		PressureMb float64 `json:"pressure_mb"`
		Humidity   int     `json:"humidity"`
		Condition  struct {
			Code int    `json:"code"`
			Text string `json:"text"`
			Icon string `json:"icon"`
		} `json:"condition"`
	} `json:"current"`
	Forecast *struct {
		Forecastday []weatherAPIForecastDay `json:"forecastday"`
	} `json:"forecast"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type weatherAPIForecastDay struct {
	Date string `json:"date"`
	Day  struct {
		MaxtempC  float64 `json:"maxtemp_c"`
		MintempC  float64 `json:"mintemp_c"`
		DailyChanceOfRain int `json:"daily_chance_of_rain"`
		DailyChanceOfSnow int `json:"daily_chance_of_snow"`
		Condition struct {
			Code int `json:"code"`
		} `json:"condition"`
	} `json:"day"`
	Astro struct {
		Sunrise string `json:"sunrise"`
		Sunset  string `json:"sunset"`
	} `json:"astro"`
	Hour []weatherAPIHour `json:"hour"`
}

type weatherAPIHour struct {
	TimeEpoch  int64   `json:"time_epoch"`
	Time       string  `json:"time"`
	TempC      float64 `json:"temp_c"`
	FeelslikeC float64 `json:"feelslike_c"`
	Condition  struct {
		Code int `json:"code"`
	} `json:"condition"`
}

// mapWeatherAPICodeToWMO переводит код условий WeatherAPI в WMO-подобный код для describeCode / UI.
func mapWeatherAPICodeToWMO(code int) int {
	switch code {
	case 1000:
		return 0
	case 1003:
		return 2
	case 1006, 1009:
		return 3
	case 1030, 1135, 1147:
		return 45
	case 1063, 1150, 1153, 1180, 1183, 1240:
		return 61
	case 1186, 1189, 1192, 1195, 1198, 1201, 1243, 1246, 1252, 1255, 1258, 1261, 1264:
		return 65
	case 1066, 1114, 1117, 1204, 1207, 1210, 1213, 1216, 1219, 1222, 1225, 1237:
		return 71
	case 1087, 1273, 1276, 1279, 1282:
		return 95
	default:
		if code >= 1063 && code <= 1201 {
			return 65
		}
		if code >= 1210 && code <= 1237 {
			return 71
		}
		if code >= 1273 && code <= 1282 {
			return 95
		}
		return 3
	}
}

func parseWeatherAPILocaltime(s string, loc *time.Location) time.Time {
	s = strings.TrimSpace(s)
	if s == "" || loc == nil {
		return time.Time{}
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		return time.Time{}
	}
	return t
}

// parseAstroTime парсит время восхода/заката вида "07:47 AM" для даты "2006-01-02".
func parseAstroTime(dateISO, clock string, loc *time.Location) time.Time {
	dateISO = strings.TrimSpace(dateISO)
	clock = strings.TrimSpace(clock)
	if dateISO == "" || clock == "" || loc == nil {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02 3:04 PM",
		"2006-01-02 03:04 PM",
		"2006-01-02 15:04",
	}
	for _, ly := range layouts {
		if t, err := time.ParseInLocation(ly, dateISO+" "+clock, loc); err == nil {
			return t
		}
	}
	return time.Time{}
}

func buildHourlySeries(days []weatherAPIForecastDay, loc *time.Location, nowLocal time.Time) []Hourly {
	type slot struct {
		t    time.Time
		tmp  float64
		feel float64
		wa   int
	}
	var slots []slot
	for _, fd := range days {
		for _, h := range fd.Hour {
			t := time.Unix(h.TimeEpoch, 0).In(loc)
			slots = append(slots, slot{t: t, tmp: h.TempC, feel: h.FeelslikeC, wa: h.Condition.Code})
		}
	}
	if len(slots) == 0 {
		return nil
	}
	startIdx := 0
	if !nowLocal.IsZero() {
		for i := range slots {
			if !slots[i].t.Before(nowLocal) {
				startIdx = i
				break
			}
			startIdx = i
		}
	}
	n := 12
	if startIdx+n > len(slots) {
		n = len(slots) - startIdx
	}
	// Если «сейчас» оказалось после последнего слота API (редкий сдвиг TZ/времени) — показываем с начала суток прогноза.
	if n <= 0 {
		startIdx = 0
		n = 12
		if n > len(slots) {
			n = len(slots)
		}
	}
	if n <= 0 {
		return nil
	}
	out := make([]Hourly, 0, n)
	for i := 0; i < n; i++ {
		s := slots[startIdx+i]
		wmo := mapWeatherAPICodeToWMO(s.wa)
		out = append(out, Hourly{
			Time:        s.t,
			Temperature: s.tmp,
			FeelsLike:   s.feel,
			WeatherCode: wmo,
			Description: "",
			Icon:        i18n.WeatherIcon(wmo),
		})
	}
	return out
}

func buildDailySeries(days []weatherAPIForecastDay, loc *time.Location) []Daily {
	n := len(days)
	if n > 3 {
		n = 3
	}
	out := make([]Daily, 0, n)
	for i := 0; i < n; i++ {
		fd := days[i]
		date, err := time.ParseInLocation("2006-01-02", fd.Date, loc)
		if err != nil {
			continue
		}
		wmo := mapWeatherAPICodeToWMO(fd.Day.Condition.Code)
		sr := parseAstroTime(fd.Date, fd.Astro.Sunrise, loc)
		ss := parseAstroTime(fd.Date, fd.Astro.Sunset, loc)
		var srStr, ssStr string
		if !sr.IsZero() {
			srStr = sr.Format("15:04")
		}
		if !ss.IsZero() {
			ssStr = ss.Format("15:04")
		}
		out = append(out, Daily{
			Date:         date,
			MinTemp:      fd.Day.MintempC,
			MaxTemp:      fd.Day.MaxtempC,
			WeatherCode:  wmo,
			Description:  "",
			Icon:         i18n.WeatherIcon(wmo),
			SunriseLocal: srStr,
			SunsetLocal:  ssStr,
		})
	}
	return out
}

func buildWeatherDataFromWeatherAPI(city City, res weatherAPIForecastResponse, loc *time.Location) (*WeatherData, error) {
	cur := res.Current
	locInfo := res.Location
	days := res.Forecast.Forecastday

	wmoNow := mapWeatherAPICodeToWMO(cur.Condition.Code)

	nowLocal := parseWeatherAPILocaltime(locInfo.Localtime, loc)
	if nowLocal.IsZero() && locInfo.LocaltimeEpoch > 0 {
		nowLocal = time.Unix(locInfo.LocaltimeEpoch, 0).In(loc)
	}

	var sunrise, sunset time.Time
	if len(days) > 0 {
		sunrise = parseAstroTime(days[0].Date, days[0].Astro.Sunrise, loc)
		sunset = parseAstroTime(days[0].Date, days[0].Astro.Sunset, loc)
	}

	isNight := cur.IsDay == 0
	if !sunrise.IsZero() && !sunset.IsZero() && !nowLocal.IsZero() {
		isNight = nowLocal.Before(sunrise) || nowLocal.After(sunset)
	}

	// Давление: мм рт. ст. (hPa / 1.333).
	pressureMM := cur.PressureMb / 1.333

	visKm := cur.VisKm
	if visKm <= 0 && cur.VisMiles > 0 {
		visKm = cur.VisMiles * 1.60934
	}
	precipChance := -1
	if len(days) > 0 {
		precipChance = days[0].Day.DailyChanceOfRain
		if days[0].Day.DailyChanceOfSnow > precipChance {
			precipChance = days[0].Day.DailyChanceOfSnow
		}
		if precipChance < 0 || precipChance > 100 {
			precipChance = -1
		}
	}

	current := Current{
		Temperature:  cur.TempC,
		FeelsLike:    cur.FeelslikeC,
		WeatherCode:  wmoNow,
		Description:  "",
		Icon:         i18n.WeatherIcon(wmoNow),
		WindSpeed:    cur.WindKph,
		Humidity:     float64(cur.Humidity),
		UVIndex:      float64(cur.UV),
		VisibilityKm: visKm,
		Pressure:     pressureMM,
		PrecipitationChance: precipChance,
		IsNight:      isNight,
	}

	forecast := buildDailySeries(days, loc)
	hourly := buildHourlySeries(days, loc, nowLocal)

	off := 0
	if !nowLocal.IsZero() {
		_, off = nowLocal.Zone()
	} else if locInfo.LocaltimeEpoch > 0 {
		t := time.Unix(locInfo.LocaltimeEpoch, 0).In(loc)
		_, off = t.Zone()
	}

	return &WeatherData{
		CityID:           city.ID,
		CityName:         city.Name,
		Current:          current,
		Forecast:         forecast,
		Hourly:           hourly,
		Sunrise:          sunrise,
		Sunset:           sunset,
		Timezone:         locInfo.TzID,
		UTCOffsetSeconds: off,
		IsStale:          false,
		IsFallback:       false,
	}, nil
}

// LocalizedDescription forwards to the shared i18n layer (handlers should prefer i18n.WeatherDescription).
func LocalizedDescription(code int, lang string) string {
	return i18n.WeatherDescription(code, lang)
}
