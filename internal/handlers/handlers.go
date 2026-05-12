package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"bss/internal/analytics"
	"bss/internal/i18n"
	"bss/internal/places"
	"bss/internal/weather"
)

type Server struct {
	tmpl      *template.Template
	weather   *weather.Client
	places    *places.Store
	placesMu  sync.RWMutex
	logger    *log.Logger
	analytics *analytics.Analytics
}

var serverStartTime = time.Now()

func NewServer(tmpl *template.Template, weatherClient *weather.Client, placesStore *places.Store, logger *log.Logger) *Server {
	return &Server{
		tmpl:    tmpl,
		weather: weatherClient,
		places:  placesStore,
		logger:  logger,
	}
}

func (s *Server) SetPlacesStore(ps *places.Store) {
	s.placesMu.Lock()
	s.places = ps
	s.placesMu.Unlock()
}

func (s *Server) SetAnalytics(a *analytics.Analytics) {
	s.analytics = a
}

// pulseJSON is the stable JSON shape for GET /api/pulse (used by static/pulse.js).
type pulseJSON struct {
	Goroutines    int                        `json:"goroutines"`
	MemoryAllocMB float64                    `json:"memory_alloc_mb"`
	MemorySysMB   float64                    `json:"memory_sys_mb"`
	GCCycles      uint32                     `json:"gc_cycles"`
	Uptime        string                     `json:"uptime"`
	Analytics     analytics.AnalyticsSummary `json:"analytics,omitempty"`
}

func (s *Server) HandlePulse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Track this request (bots excluded from "today" and "unique IPs")
	if s.analytics != nil {
		s.analytics.TrackRequest(r.URL.Path, s.getClientIP(r), r.UserAgent())
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	resp := pulseJSON{
		Goroutines:    runtime.NumGoroutine(),
		MemoryAllocMB: float64(m.Alloc) / 1024.0 / 1024.0,
		MemorySysMB:   float64(m.Sys) / 1024.0 / 1024.0,
		GCCycles:      m.NumGC,
		Uptime:        time.Since(serverStartTime).String(),
	}

	// Include analytics summary
	if s.analytics != nil {
		resp.Analytics = s.analytics.Summary()
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	body, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(body)
}

// getClientIP extracts client IP from request.
func (s *Server) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies like Render)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take first IP if multiple
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return xff
	}
	// Check X-Real-Ip
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// trackExtendedInfo collects detailed analytics from HTTP headers.
func (s *Server) trackExtendedInfo(r *http.Request) {
	if s.analytics == nil {
		return
	}
	referrer := r.Header.Get("Referer")
	userAgent := r.Header.Get("User-Agent")
	acceptLang := r.Header.Get("Accept-Language")
	// Country detection via CF-IPCountry header (Cloudflare/Render)
	country := r.Header.Get("CF-IPCountry")
	s.analytics.TrackExtended(referrer, userAgent, acceptLang, country)
}

func (s *Server) getPlacesStore() *places.Store {
	s.placesMu.RLock()
	ps := s.places
	s.placesMu.RUnlock()
	return ps
}

type CitySummary struct {
	ID                 string
	Name               string
	Temperature        float64
	Description        string
	Icon               string
	WindSpeed          float64
	Humidity           float64
	WeatherUnavailable bool
}

// TextSet is the SSR string bundle (see package i18n).
type TextSet = i18n.UI

type IndexPageData struct {
	IsIndex         bool
	IsNotFound      bool
	WeatherCode     int
	IsNight         bool
	WeatherMood     string
	Lang            string
	Path            string
	Text            TextSet
	CityID          string
	CurrentCityName string
	// HeroCityTitle is the index hero H1 only (Kyiv uses custom copy per language).
	HeroCityTitle       string
	CurrentTemp         float64
	CurrentDescription  string
	CurrentFeelsLike    float64
	CurrentPrecipChance int
	CurrentWind         float64
	CurrentHumidity     float64
	CurrentPressure     float64
	WeatherUnavailable  bool
	WeatherStale        bool
	WeatherStaleText    string
	WeatherSourceText   string
	// WeatherSourceKind: live_api | server_cache | server_cache_stale | no_data (SSR; browser_cache только в JS).
	WeatherSourceKind  string
	RandomAdvice       string
	WeatherUpdatedText string
	TodayLabel         string
	TodayMin           float64
	TodayMax           float64
	TomorrowLabel      string
	TomorrowMin        float64
	TomorrowMax        float64
	DayAfterLabel      string
	DayAfterMin        float64
	DayAfterMax        float64
	Cities             []CitySummary
	WeatherJSON        template.JS
	MetaDescription    string
	ClientI18n         template.JS
}

type DailyView struct {
	Date        string
	Label       string
	MinTemp     float64
	MaxTemp     float64
	Description string
	Icon        string
}

type HourlyView struct {
	TimeLabel   string
	Temperature float64
	Description string
	Icon        string
}

// CityWeatherPhysics — поля для #js-weather-physics-data (Canvas, data-атрибуты).
type CityWeatherPhysics struct {
	MainCondition string
	WindSpeed     float64
}

type CityPageData struct {
	IsIndex     bool
	IsNotFound  bool
	WeatherCode int
	IsNight     bool
	WeatherMood string
	Lang        string
	Path        string
	Text        TextSet
	CityID      string
	// FavListID — id для localStorage weather_favorites и data-city-id у звезды (/city/kyiv или numeric place id).
	FavListID           string
	CityName            string
	CityLocation        string
	CurrentTemp         float64
	CurrentFeelsLike    float64
	CurrentDescription  string
	CurrentPrecipChance int
	CurrentWind         float64
	CurrentHumidity     float64
	CurrentPressure     float64
	WeatherUnavailable  bool
	WeatherStale        bool
	WeatherStaleText    string
	WeatherSourceText   string
	WeatherSourceKind   string
	WeatherUpdatedText  string
	RandomAdvice        string
	Forecast            []DailyView
	TodayLabel          string
	TodayMin            float64
	TodayMax            float64
	TomorrowLabel       string
	TomorrowMin         float64
	TomorrowMax         float64
	TrendText           string
	Cities              []CitySummary
	Hourly              []HourlyView
	WeatherJSON         template.JS
	DuplicatesCount     int
	DuplicatesLabel     string
	DuplicatesURL       string
	ClientI18n          template.JS
	RadarLat            float64
	RadarLon            float64
	// Current — снимок для Canvas-физики (шаблон: .Current.MainCondition, .Current.WindSpeed).
	Current CityWeatherPhysics
}

// NotFoundPageData drives templates/404.html (same layout and atmosphere as other pages).
type NotFoundPageData struct {
	IsNotFound  bool
	IsIndex     bool
	WeatherCode int
	IsNight     bool
	WeatherMood string
	Lang        string
	Path        string
	Text        TextSet
	CityID      string
	WeatherJSON template.JS
	ClientI18n  template.JS
}

func cityCoordsByID(cityID string) (float64, float64) {
	for _, c := range weather.AllCities() {
		if c.ID == cityID {
			return c.Latitude, c.Longitude
		}
	}
	return 0, 0
}

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Track request (bots excluded from "today" and "unique IPs")
	if s.analytics != nil {
		s.analytics.TrackRequest(r.URL.Path, s.getClientIP(r), r.UserAgent())
		s.trackExtendedInfo(r)
	}

	lang := detectLang(r)
	text := i18n.For(lang)

	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	// Герой — только Киев; список карточек на главной заполняется из localStorage + /api/favorites.
	const heroCityID = "kyiv"
	var heroData *weather.WeatherData
	data, err := s.weather.GetWeather(ctx, heroCityID)
	if err != nil {
		s.logger.Printf("get weather for hero %s: %v", heroCityID, err)
	} else {
		heroData = data
	}

	if heroData == nil {
		cities := weather.AllCities()
		// Погода недоступна (ошибка/таймаут Open-Meteo). Страницу рендерим всё равно,
		// показываем fallback-данные.
		if len(cities) > 0 {
			var c0 weather.City
			found := false
			for _, c := range cities {
				if c.ID == heroCityID {
					c0 = c
					found = true
					break
				}
			}
			if !found {
				c0 = cities[0]
			}
			heroData = &weather.WeatherData{
				CityID: c0.ID,
				// CityName не используется напрямую в шаблоне, но пусть будет.
				CityName:   c0.Name,
				IsFallback: true,
				DataSource: "no_data",
				Current: weather.Current{
					Temperature: 0,
					WeatherCode: 3, // neutral cloudy-like background
					Description: "",
					Icon:        "❔",
					WindSpeed:   0,
					Humidity:    0,
					Pressure:    0,
					IsNight:     false,
				},
			}
		} else {
			// На практике cities не пустой (weather.AllCities()),
			// но на всякий случай вернём empty index.
			heroData = &weather.WeatherData{IsFallback: true, DataSource: "no_data"}
		}
	}

	var todayLabel, tomorrowLabel, dayAfterLabel string
	var todayMin, todayMax, tomorrowMin, tomorrowMax, dayAfterMin, dayAfterMax float64
	if len(heroData.Forecast) > 0 {
		today := heroData.Forecast[0]
		todayLabel = today.Date.Format("02.01")
		todayMin = today.MinTemp
		todayMax = today.MaxTemp
	}
	if len(heroData.Forecast) > 1 {
		tomorrow := heroData.Forecast[1]
		tomorrowLabel = tomorrow.Date.Format("02.01")
		tomorrowMin = tomorrow.MinTemp
		tomorrowMax = tomorrow.MaxTemp
	}
	if len(heroData.Forecast) > 2 {
		d3 := heroData.Forecast[2]
		dayAfterLabel = d3.Date.Format("02.01")
		dayAfterMin = d3.MinTemp
		dayAfterMax = d3.MaxTemp
	}

	currentDesc := i18n.WeatherDescription(heroData.Current.WeatherCode, lang)
	if heroData.IsFallback {
		currentDesc = text.WeatherUnavailableMsg
	}
	var metaNames []string
	for _, c := range weather.AllCities() {
		metaNames = append(metaNames, weather.LocalizedCityName(c.ID, lang))
	}
	metaDesc := fmt.Sprintf(text.MetaDescriptionTemplate, strings.Join(metaNames, ", "))

	heroCityTitle := weather.LocalizedCityName(heroData.CityID, lang)

	page := IndexPageData{
		IsIndex:             true,
		WeatherCode:         heroData.Current.WeatherCode,
		IsNight:             heroData.Current.IsNight,
		WeatherMood:         weatherMoodClass(heroData.Current.WeatherCode, heroData.Current.IsNight),
		Lang:                lang,
		Path:                r.URL.Path,
		Text:                text,
		CityID:              "",
		CurrentCityName:     weather.LocalizedCityName(heroData.CityID, lang),
		HeroCityTitle:       heroCityTitle,
		CurrentTemp:         heroData.Current.Temperature,
		CurrentFeelsLike:    heroData.Current.FeelsLike,
		CurrentDescription:  currentDesc,
		CurrentPrecipChance: heroData.Current.PrecipitationChance,
		CurrentWind:         heroData.Current.WindSpeed,
		CurrentHumidity:     heroData.Current.Humidity,
		CurrentPressure:     heroData.Current.Pressure,
		WeatherUnavailable:  heroData.IsFallback,
		WeatherStale:        heroData.IsStale,
		WeatherStaleText:    staleWeatherNotice(lang, heroData),
		WeatherSourceText:   weatherSourceText(lang, heroData),
		WeatherSourceKind:   weatherSourceKind(heroData),
		WeatherUpdatedText:  weatherUpdatedText(lang, heroData),
		RandomAdvice:        text.GetRandomAdvice(),
		TodayLabel:          todayLabel,
		TodayMin:            todayMin,
		TodayMax:            todayMax,
		TomorrowLabel:       tomorrowLabel,
		TomorrowMin:         tomorrowMin,
		TomorrowMax:         tomorrowMax,
		DayAfterLabel:       dayAfterLabel,
		DayAfterMin:         dayAfterMin,
		DayAfterMax:         dayAfterMax,
		Cities:              nil,
		WeatherJSON:         buildWeatherJSON(heroData.Sunrise, heroData.Sunset, heroData.Timezone, heroData.UTCOffsetSeconds, weather.LocalizedCityName(heroData.CityID, lang)),
		MetaDescription:     metaDesc,
		ClientI18n:          template.JS(i18n.MarshalClientJSON(lang)),
	}

	if err := s.tmpl.ExecuteTemplate(w, "index-page", page); err != nil {
		s.logger.Printf("render index: %v", err)
	}
}

func cityWeatherPhysicsFrom(data *weather.WeatherData) CityWeatherPhysics {
	if data == nil || data.IsFallback {
		return CityWeatherPhysics{MainCondition: "Unknown", WindSpeed: 0}
	}
	return CityWeatherPhysics{
		MainCondition: weather.MainConditionPhysics(data.Current.WeatherCode),
		WindSpeed:     data.Current.WindSpeed,
	}
}

func staleWeatherNotice(lang string, data *weather.WeatherData) string {
	if data == nil || !data.IsStale {
		return ""
	}
	ageMin := 0
	if !data.LastUpdated.IsZero() {
		ageMin = int(time.Since(data.LastUpdated).Minutes())
		if ageMin < 0 {
			ageMin = 0
		}
	}
	switch i18n.Normalize(lang) {
	case "en":
		return fmt.Sprintf("Weather data is outdated (~%d min).", ageMin)
	case "uk":
		return fmt.Sprintf("Дані погоди застарілі (~%d хв).", ageMin)
	default:
		return fmt.Sprintf("Данные погоды устарели (~%d мин).", ageMin)
	}
}

func weatherSourceKind(data *weather.WeatherData) string {
	if data == nil {
		return "live_api"
	}
	s := strings.TrimSpace(data.DataSource)
	if s == "" {
		return "live_api"
	}
	return s
}

func weatherSourceText(lang string, data *weather.WeatherData) string {
	source := ""
	if data != nil {
		source = strings.TrimSpace(data.DataSource)
	}
	switch i18n.Normalize(lang) {
	case "en":
		switch source {
		case "server_cache":
			return "Data from cache"
		case "server_cache_stale":
			return "Data from cache (may be outdated)"
		case "no_data":
			return "No live data available"
		default:
			return "Live data"
		}
	case "uk":
		switch source {
		case "server_cache":
			return "Дані з кешу"
		case "server_cache_stale":
			return "Дані з кешу (можуть бути застарілі)"
		case "no_data":
			return "Немає актуальних даних"
		default:
			return "Актуальні дані"
		}
	default:
		switch source {
		case "server_cache":
			return "Данные из кэша"
		case "server_cache_stale":
			return "Данные из кэша (могут быть устаревшими)"
		case "no_data":
			return "Нет актуальных данных"
		default:
			return "Актуальные данные"
		}
	}
}

func weatherUpdatedText(lang string, data *weather.WeatherData) string {
	prefix := map[string]string{
		"en": "Updated:",
		"uk": "Оновлено:",
		"ru": "Обновлено:",
	}[i18n.Normalize(lang)]
	if prefix == "" {
		prefix = "Обновлено:"
	}
	if data == nil || data.LastUpdated.IsZero() {
		return prefix + " —"
	}
	local := data.LastUpdated
	if tz := strings.TrimSpace(data.Timezone); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			local = local.In(loc)
		}
	}
	return fmt.Sprintf("%s %s", prefix, local.Format("15:04"))
}

func (s *Server) City(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/city/") {
		s.renderNotFound(w, r)
		return
	}

	cityID := strings.TrimPrefix(r.URL.Path, "/city/")
	if cityID == "" {
		s.renderNotFound(w, r)
		return
	}

	if !weather.IsKnownCity(cityID) {
		s.renderNotFound(w, r)
		return
	}

	// Track city view (bots excluded from "today" and "unique IPs")
	if s.analytics != nil {
		s.analytics.TrackRequest(r.URL.Path, s.getClientIP(r), r.UserAgent())
		s.analytics.TrackCity(cityID)
		s.trackExtendedInfo(r)
	}

	lang := detectLang(r)
	text := i18n.For(lang)

	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	data, err := s.weather.GetWeather(ctx, cityID)
	if err != nil {
		s.logger.Printf("get city weather %s: %v", cityID, err)
		http.Error(w, "failed to get weather data", http.StatusBadGateway)
		return
	}

	forecast := make([]DailyView, len(data.Forecast))
	for i, d := range data.Forecast {
		label := d.Date.Format("02.01")
		forecast[i] = DailyView{
			Date:        d.Date.Format("2006-01-02"),
			Label:       label,
			MinTemp:     d.MinTemp,
			MaxTemp:     d.MaxTemp,
			Description: i18n.WeatherDescription(d.WeatherCode, lang),
			Icon:        d.Icon,
		}
	}

	var todayLabel, tomorrowLabel, trend string
	var todayMin, todayMax, tomorrowMin, tomorrowMax float64
	if len(data.Forecast) > 0 {
		td := data.Forecast[0]
		todayLabel = td.Date.Format("02.01")
		todayMin = td.MinTemp
		todayMax = td.MaxTemp
	}
	if len(data.Forecast) > 1 {
		tm := data.Forecast[1]
		tomorrowLabel = tm.Date.Format("02.01")
		tomorrowMin = tm.MinTemp
		tomorrowMax = tm.MaxTemp

		diff := tomorrowMax - todayMax
		trend = i18n.TrendSentence(lang, diff)
	}

	// мини‑карточки других городов (переиспользуем уже загруженный город)
	otherCities := weather.AllCities()
	cityCards := make([]CitySummary, 0, len(otherCities))
	for _, c := range otherCities {
		var d *weather.WeatherData
		if c.ID == cityID {
			d = data
		} else {
			var err error
			d, err = s.weather.GetWeather(ctx, c.ID)
			if err != nil {
				s.logger.Printf("get weather for %s: %v", c.ID, err)
				continue
			}
		}
		cardDesc := strings.TrimSpace(d.Current.Description)
		if cardDesc == "" {
			cardDesc = i18n.WeatherDescription(d.Current.WeatherCode, lang)
		}
		if d.IsFallback {
			cardDesc = text.WeatherUnavailableMsg
		}
		cityCards = append(cityCards, CitySummary{
			ID:                 d.CityID,
			Name:               weather.LocalizedCityName(d.CityID, lang),
			Temperature:        d.Current.Temperature,
			Description:        cardDesc,
			Icon:               d.Current.Icon,
			WindSpeed:          d.Current.WindSpeed,
			Humidity:           d.Current.Humidity,
			WeatherUnavailable: d.IsFallback,
		})
	}

	// hourly view (next 24 hours)
	hourly := make([]HourlyView, 0, len(data.Hourly))
	for _, h := range data.Hourly {
		hourly = append(hourly, HourlyView{
			TimeLabel:   h.Time.Format("15:04"),
			Temperature: h.Temperature,
			Description: i18n.WeatherDescription(h.WeatherCode, lang),
			Icon:        h.Icon,
		})
	}

	currentDesc := strings.TrimSpace(data.Current.Description)
	if currentDesc == "" {
		currentDesc = i18n.WeatherDescription(data.Current.WeatherCode, lang)
	}
	if data.IsFallback {
		currentDesc = text.WeatherUnavailableMsg
	}
	page := CityPageData{
		IsIndex:             false,
		WeatherCode:         data.Current.WeatherCode,
		IsNight:             data.Current.IsNight,
		WeatherMood:         weatherMoodClass(data.Current.WeatherCode, data.Current.IsNight),
		Lang:                lang,
		Path:                r.URL.Path,
		Text:                text,
		CityID:              data.CityID,
		FavListID:           data.CityID,
		CityName:            weather.LocalizedCityName(data.CityID, lang),
		CurrentTemp:         data.Current.Temperature,
		CurrentFeelsLike:    data.Current.FeelsLike,
		CurrentDescription:  currentDesc,
		CurrentPrecipChance: data.Current.PrecipitationChance,
		CurrentWind:         data.Current.WindSpeed,
		CurrentHumidity:     data.Current.Humidity,
		CurrentPressure:     data.Current.Pressure,
		WeatherUnavailable:  data.IsFallback,
		WeatherStale:        data.IsStale,
		WeatherStaleText:    staleWeatherNotice(lang, data),
		WeatherSourceText:   weatherSourceText(lang, data),
		WeatherSourceKind:   weatherSourceKind(data),
		WeatherUpdatedText:  weatherUpdatedText(lang, data),
		Forecast:            forecast,
		TodayLabel:          todayLabel,
		TodayMin:            todayMin,
		TodayMax:            todayMax,
		TomorrowLabel:       tomorrowLabel,
		TomorrowMin:         tomorrowMin,
		TomorrowMax:         tomorrowMax,
		TrendText:           trend,
		Cities:              cityCards,
		Hourly:              hourly,
		WeatherJSON:         buildWeatherJSON(data.Sunrise, data.Sunset, data.Timezone, data.UTCOffsetSeconds, weather.LocalizedCityName(data.CityID, lang)),
		ClientI18n:          template.JS(i18n.MarshalClientJSON(lang)),
		RadarLat:            0,
		RadarLon:            0,
		Current:             cityWeatherPhysicsFrom(data),
	}
	page.RadarLat, page.RadarLon = cityCoordsByID(data.CityID)

	if err := s.tmpl.ExecuteTemplate(w, "city-page", page); err != nil {
		s.logger.Printf("render city: %v", err)
	}
}

func (s *Server) renderNotFound(w http.ResponseWriter, r *http.Request) {
	if s.tmpl == nil {
		http.NotFound(w, r)
		return
	}
	lang := detectLang(r)
	text := i18n.For(lang)
	w.WriteHeader(http.StatusNotFound)
	page := NotFoundPageData{
		IsNotFound:  true,
		IsIndex:     false,
		WeatherCode: 0,
		IsNight:     false,
		WeatherMood: weatherMoodClass(0, false),
		Lang:        lang,
		Path:        r.URL.Path,
		Text:        text,
		CityID:      "",
		WeatherJSON: template.JS("{}"),
		ClientI18n:  template.JS(i18n.MarshalClientJSON(lang)),
	}
	if err := s.tmpl.ExecuteTemplate(w, "notfound-page", page); err != nil {
		s.logger.Printf("render 404: %v", err)
	}
}

// --- API: weather by city id (existing) ---

type apiWeatherResponse struct {
	CityID      string      `json:"cityId"`
	CityName    string      `json:"cityName"`
	Lang        string      `json:"lang"`
	Source      string      `json:"source,omitempty"`
	LastUpdated string      `json:"lastUpdated,omitempty"`
	Current     apiCurrent  `json:"current"`
	Hourly      []apiHourly `json:"hourly,omitempty"`
	// Sunrise/Sunset — сегодня, локальное время "15:04" (из astro первого дня WeatherAPI).
	Sunrise string          `json:"sunrise,omitempty"`
	Sunset  string          `json:"sunset,omitempty"`
	Daily   []apiDailyAstro `json:"daily,omitempty"`
}

type apiDailyAstro struct {
	Date    string `json:"date"`
	Sunrise string `json:"sunrise"`
	Sunset  string `json:"sunset"`
}

type apiCurrent struct {
	Temperature float64 `json:"temperature"`
	// ApparentTemperature — ощущается как °C (WeatherAPI feelslike_c).
	ApparentTemperature float64 `json:"apparent_temperature"`
	Description         string  `json:"description"`
	Icon                string  `json:"icon"`
	IsFallback          bool    `json:"isFallback"`
	Wind                float64 `json:"wind"`
	Humidity            float64 `json:"humidity"`
	UVIndex             float64 `json:"uv_index"`
	Visibility          float64 `json:"visibility"`
	Pressure            float64 `json:"pressure"`
	PrecipitationChance int     `json:"precipitation_chance"`
	WeatherCode         int     `json:"weatherCode"`
	IsNight             bool    `json:"isNight"`
}

func apiSunStrings(data *weather.WeatherData) (rise, set string) {
	if data == nil {
		return "", ""
	}
	if !data.Sunrise.IsZero() {
		rise = data.Sunrise.Format("15:04")
	}
	if !data.Sunset.IsZero() {
		set = data.Sunset.Format("15:04")
	}
	return rise, set
}

func apiDailyFromForecast(forecast []weather.Daily) []apiDailyAstro {
	if len(forecast) == 0 {
		return nil
	}
	out := make([]apiDailyAstro, 0, len(forecast))
	for _, d := range forecast {
		out = append(out, apiDailyAstro{
			Date:    d.Date.Format("2006-01-02"),
			Sunrise: d.SunriseLocal,
			Sunset:  d.SunsetLocal,
		})
	}
	return out
}

type apiHourly struct {
	Time                string  `json:"time"`
	Temperature         float64 `json:"temperature"`
	ApparentTemperature float64 `json:"apparent_temperature"`
	Description         string  `json:"description"`
	Icon                string  `json:"icon"`
	PrecipChance        int     `json:"precip_chance"`
}

// fillAPIHourlyPayload заполняет hourly для JSON (/api/weather и /api/place_weather).
// Почасовые ряды на бэкенде строятся из WeatherAPI forecast.forecastday[].hour
// (temp_c, feelslike_c, condition.code — аналог open-meteo hourly=temperature_2m,weather_code,apparent_temperature).
func fillAPIHourlyPayload(resp *apiWeatherResponse, data *weather.WeatherData, lang string) {
	for _, h := range data.Hourly {
		resp.Hourly = append(resp.Hourly, apiHourly{
			Time:                h.Time.Format("15:04"),
			Temperature:         h.Temperature,
			ApparentTemperature: h.FeelsLike,
			Description:         i18n.WeatherDescription(h.WeatherCode, lang),
			Icon:                h.Icon,
			PrecipChance:        h.PrecipChance,
		})
	}
}

func (s *Server) APIWeather(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/api/weather/") {
		http.NotFound(w, r)
		return
	}

	cityID := strings.TrimPrefix(r.URL.Path, "/api/weather/")
	if cityID == "" {
		http.Error(w, "missing city id", http.StatusBadRequest)
		return
	}

	if !weather.IsKnownCity(cityID) {
		http.NotFound(w, r)
		return
	}

	lang := detectLang(r)
	text := i18n.For(lang)

	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	data, err := s.weather.GetWeather(ctx, cityID)
	if err != nil {
		s.logger.Printf("api get city weather %s: %v", cityID, err)
		http.Error(w, "failed to fetch weather", http.StatusBadGateway)
		return
	}

	sunriseStr, sunsetStr := apiSunStrings(data)
	resp := apiWeatherResponse{
		CityID:   data.CityID,
		CityName: weather.LocalizedCityName(data.CityID, lang),
		Lang:     lang,
		Source:   strings.TrimSpace(data.DataSource),
		LastUpdated: func() string {
			if data.LastUpdated.IsZero() {
				return ""
			}
			return data.LastUpdated.UTC().Format(time.RFC3339)
		}(),
		Sunrise: sunriseStr,
		Sunset:  sunsetStr,
		Daily:   apiDailyFromForecast(data.Forecast),
		Current: apiCurrent{
			Temperature:         data.Current.Temperature,
			ApparentTemperature: data.Current.FeelsLike,
			Description: func() string {
				if data.IsFallback {
					return text.WeatherUnavailableMsg
				}
				if s := strings.TrimSpace(data.Current.Description); s != "" {
					return s
				}
				return i18n.WeatherDescription(data.Current.WeatherCode, lang)
			}(),
			Icon:                data.Current.Icon,
			IsFallback:          data.IsFallback,
			Wind:                data.Current.WindSpeed,
			Humidity:            data.Current.Humidity,
			UVIndex:             data.Current.UVIndex,
			Visibility:          data.Current.VisibilityKm,
			Pressure:            data.Current.Pressure,
			PrecipitationChance: data.Current.PrecipitationChance,
			WeatherCode:         data.Current.WeatherCode,
			IsNight:             data.Current.IsNight,
		},
	}

	fillAPIHourlyPayload(&resp, data, lang)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Printf("encode api weather: %v", err)
	}
}

// APIPlaceWeather возвращает текущую погоду по произвольному place.id из базы places.
func (s *Server) APIPlaceWeather(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	placesStore := s.getPlacesStore()
	if placesStore == nil {
		http.Error(w, "search is not configured", http.StatusServiceUnavailable)
		return
	}

	idStr := strings.TrimSpace(r.URL.Query().Get("id"))
	if idStr == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	lang := detectLang(r)
	text := i18n.For(lang)

	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	place, err := placesStore.GetByID(ctx, id)
	if err != nil {
		s.logger.Printf("api place weather get %d: %v", id, err)
		http.Error(w, "failed to load place", http.StatusInternalServerError)
		return
	}
	if place == nil {
		http.NotFound(w, r)
		return
	}

	displayName := places.LocalizedDisplayName(*place, lang)

	cacheKey := "place:" + strconv.FormatInt(place.ID, 10)
	var data *weather.WeatherData
	if known, ok := weather.MatchKnownCityByCoords(place.Lat, place.Lon); ok {
		cacheKey = known.ID
		data, err = s.weather.GetWeather(ctx, known.ID)
	} else if known, ok := weather.MatchKnownCityByName(displayName); ok {
		cacheKey = known.ID
		data, err = s.weather.GetWeather(ctx, known.ID)
	} else {
		data, err = s.weather.GetWeatherForLocation(ctx, cacheKey, displayName, place.Lat, place.Lon)
	}
	if err != nil {
		s.logger.Printf("api place weather %d: %v", id, err)
		http.Error(w, "failed to fetch weather", http.StatusBadGateway)
		return
	}

	sunriseStr, sunsetStr := apiSunStrings(data)
	resp := apiWeatherResponse{
		CityID:   cacheKey,
		CityName: displayName,
		Lang:     lang,
		Source:   strings.TrimSpace(data.DataSource),
		LastUpdated: func() string {
			if data.LastUpdated.IsZero() {
				return ""
			}
			return data.LastUpdated.UTC().Format(time.RFC3339)
		}(),
		Sunrise: sunriseStr,
		Sunset:  sunsetStr,
		Daily:   apiDailyFromForecast(data.Forecast),
		Current: apiCurrent{
			Temperature:         data.Current.Temperature,
			ApparentTemperature: data.Current.FeelsLike,
			Description: func() string {
				if data.IsFallback {
					return text.WeatherUnavailableMsg
				}
				if s := strings.TrimSpace(data.Current.Description); s != "" {
					return s
				}
				return i18n.WeatherDescription(data.Current.WeatherCode, lang)
			}(),
			Icon:                data.Current.Icon,
			IsFallback:          data.IsFallback,
			Wind:                data.Current.WindSpeed,
			Humidity:            data.Current.Humidity,
			UVIndex:             data.Current.UVIndex,
			Visibility:          data.Current.VisibilityKm,
			Pressure:            data.Current.Pressure,
			PrecipitationChance: data.Current.PrecipitationChance,
			WeatherCode:         data.Current.WeatherCode,
			IsNight:             data.Current.IsNight,
		},
	}

	fillAPIHourlyPayload(&resp, data, lang)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Printf("encode api place weather: %v", err)
	}
}

// apiFavoriteItem — карточка для /api/favorites (главная, localStorage weather_favorites).
type apiFavoriteItem struct {
	Kind               string  `json:"kind"`
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Icon               string  `json:"icon"`
	Temperature        float64 `json:"temperature"`
	Description        string  `json:"description"`
	WindSpeed          float64 `json:"windSpeed"`
	Humidity           float64 `json:"humidity"`
	WeatherUnavailable bool    `json:"weatherUnavailable"`
}

const maxFavoriteIDsParam = 24

func isAllDecimal(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (s *Server) favoriteFromCityID(ctx context.Context, cityID string, lang string, text i18n.UI) (*apiFavoriteItem, error) {
	if !weather.IsKnownCity(cityID) {
		return nil, fmt.Errorf("unknown city %q", cityID)
	}
	data, err := s.weather.GetWeather(ctx, cityID)
	if err != nil {
		return nil, err
	}
	desc := strings.TrimSpace(data.Current.Description)
	if desc == "" {
		desc = i18n.WeatherDescription(data.Current.WeatherCode, lang)
	}
	if data.IsFallback {
		desc = text.WeatherUnavailableMsg
	}
	return &apiFavoriteItem{
		Kind:               "city",
		ID:                 data.CityID,
		Name:               weather.LocalizedCityName(data.CityID, lang),
		Icon:               data.Current.Icon,
		Temperature:        data.Current.Temperature,
		Description:        desc,
		WindSpeed:          data.Current.WindSpeed,
		Humidity:           data.Current.Humidity,
		WeatherUnavailable: data.IsFallback,
	}, nil
}

func (s *Server) favoriteFromPlaceID(ctx context.Context, id int64, lang string, text i18n.UI) (*apiFavoriteItem, error) {
	placesStore := s.getPlacesStore()
	if placesStore == nil {
		return nil, fmt.Errorf("places store off")
	}
	place, err := placesStore.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if place == nil {
		return nil, fmt.Errorf("place %d not found", id)
	}
	displayName := places.LocalizedDisplayName(*place, lang)
	var data *weather.WeatherData
	if known, ok := weather.MatchKnownCityByCoords(place.Lat, place.Lon); ok {
		data, err = s.weather.GetWeather(ctx, known.ID)
	} else if known, ok := weather.MatchKnownCityByName(displayName); ok {
		data, err = s.weather.GetWeather(ctx, known.ID)
	} else {
		key := "place:" + strconv.FormatInt(place.ID, 10)
		data, err = s.weather.GetWeatherForLocation(ctx, key, displayName, place.Lat, place.Lon)
	}
	if err != nil {
		return nil, err
	}
	desc := strings.TrimSpace(data.Current.Description)
	if desc == "" {
		desc = i18n.WeatherDescription(data.Current.WeatherCode, lang)
	}
	if data.IsFallback {
		desc = text.WeatherUnavailableMsg
	}
	return &apiFavoriteItem{
		Kind:               "place",
		ID:                 strconv.FormatInt(place.ID, 10),
		Name:               displayName,
		Icon:               data.Current.Icon,
		Temperature:        data.Current.Temperature,
		Description:        desc,
		WindSpeed:          data.Current.WindSpeed,
		Humidity:           data.Current.Humidity,
		WeatherUnavailable: data.IsFallback,
	}, nil
}

func (s *Server) favoriteItemForToken(ctx context.Context, tok string, lang string, text i18n.UI) *apiFavoriteItem {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return nil
	}
	if isAllDecimal(tok) {
		id, err := strconv.ParseInt(tok, 10, 64)
		if err != nil || id <= 0 {
			return nil
		}
		it, err := s.favoriteFromPlaceID(ctx, id, lang, text)
		if err != nil {
			s.logger.Printf("favorites place %d: %v", id, err)
			return nil
		}
		return it
	}
	if !weather.IsKnownCity(tok) {
		return nil
	}
	it, err := s.favoriteFromCityID(ctx, tok, lang, text)
	if err != nil {
		s.logger.Printf("favorites city %s: %v", tok, err)
		return nil
	}
	return it
}

// APIFavorites возвращает JSON-массив погоды по query `ids` (через запятую): slug города или числовой place id.
func (s *Server) APIFavorites(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := strings.TrimSpace(r.URL.Query().Get("ids"))
	if raw == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode([]apiFavoriteItem{})
		return
	}

	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{})
	var tokens []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		tokens = append(tokens, p)
		if len(tokens) >= maxFavoriteIDsParam {
			break
		}
	}

	lang := detectLang(r)
	text := i18n.For(lang)
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	results := make([]apiFavoriteItem, len(tokens))
	filled := make([]bool, len(tokens))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, tok := range tokens {
		idx := i
		tokCopy := tok
		wg.Add(1)
		go func(cityID string) {
			defer wg.Done()
			if it := s.favoriteItemForToken(ctx, cityID, lang, text); it != nil {
				mu.Lock()
				results[idx] = *it
				filled[idx] = true
				mu.Unlock()
			}
		}(tokCopy)
	}
	wg.Wait()

	clean := make([]apiFavoriteItem, 0, len(results))
	for i := range results {
		if filled[i] {
			clean = append(clean, results[i])
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(clean); err != nil {
		s.logger.Printf("encode api favorites: %v", err)
	}
}

func (s *Server) GeoRedirect(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	if latStr == "" || lonStr == "" {
		http.Error(w, "missing lat/lon", http.StatusBadRequest)
		return
	}

	lat, err1 := strconv.ParseFloat(latStr, 64)
	lon, err2 := strconv.ParseFloat(lonStr, 64)
	if err1 != nil || err2 != nil {
		http.Error(w, "invalid lat/lon", http.StatusBadRequest)
		return
	}

	// сначала пробуем найти ближайший населённый пункт в базе places
	placesStore := s.getPlacesStore()
	if placesStore != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 600*time.Millisecond)
		defer cancel()
		place, err := placesStore.Nearest(ctx, lat, lon)
		if err != nil {
			s.logger.Printf("nearest place failed: %v", err)
		} else if place != nil {
			lang := detectLang(r)
			target := "/place/" + strconv.FormatInt(place.ID, 10) + "?lang=" + lang
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
	}

	// фолбэк на статический список городов погоды
	city, ok := weather.NearestCity(lat, lon)
	if !ok {
		http.Error(w, "no cities configured", http.StatusServiceUnavailable)
		return
	}

	lang := detectLang(r)
	target := "/city/" + city.ID + "?lang=" + lang
	http.Redirect(w, r, target, http.StatusFound)
}

// Health — простой endpoint для проверки живости сервиса.
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// --- /api/places: search suggestions ---

type apiPlace struct {
	ID int64 `json:"id"`

	// City names (best-effort per language)
	NameUK string `json:"name_uk"`
	NameRU string `json:"name_ru"`
	NameEN string `json:"name_en"`

	// Oblast / region (for now, mostly in Latin; kept for all langs)
	OblastUK string `json:"oblast_uk"`
	OblastRU string `json:"oblast_ru"`
	OblastEN string `json:"oblast_en"`

	// Settlement type (місто / город / city)
	TypeUK string `json:"type_uk"`
	TypeRU string `json:"type_ru"`
	TypeEN string `json:"type_en"`

	Raion *string `json:"raion,omitempty"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
}

func (s *Server) PlacesSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	// простая защита от слишком длинных запросов
	if len([]rune(q)) > 64 {
		q = string([]rune(q)[:64])
	}
	norm := places.Normalize(q)
	if norm == "" || len([]rune(norm)) < 2 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte("[]"))
		return
	}

	// Track search query
	if s.analytics != nil {
		s.analytics.TrackSearch(norm)
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 50 {
			limit = v
		}
	}

	s.logger.Printf("places search q=%q norm=%q limit=%d", q, norm, limit)

	placesStore := s.getPlacesStore()
	if placesStore == nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte("[]"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 800*time.Millisecond)
	defer cancel()

	list, err := placesStore.Search(ctx, q, limit)
	if err != nil {
		s.logger.Printf("places search %q: %v", q, err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}
	if len(list) == 0 {
		s.logger.Printf("places search q=%q norm=%q: 0 results", q, norm)
	}

	resp := make([]apiPlace, 0, len(list))
	for _, p := range list {
		nameUK, nameRU, nameEN := places.LocalizedNameTriple(p)
		oblastUK, oblastRU, oblastEN := deriveOblastNames(p.Oblast)
		typeUK, typeRU, typeEN := deriveTypeNames(p.Type)

		resp = append(resp, apiPlace{
			ID: p.ID,

			NameUK: nameUK,
			NameRU: nameRU,
			NameEN: nameEN,

			OblastUK: oblastUK,
			OblastRU: oblastRU,
			OblastEN: oblastEN,

			TypeUK: typeUK,
			TypeRU: typeRU,
			TypeEN: typeEN,

			Raion: p.Raion,
			Lat:   p.Lat,
			Lon:   p.Lon,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Printf("encode places: %v", err)
	}
}

// --- /api/find-city: nearest place → route key ---
type apiFindCityResponse struct {
	CityName    string `json:"cityName"`              // either weather city ID (kyiv/dnipro) or "place:<places.id>"
	DisplayName string `json:"displayName,omitempty"` // best-effort localized title
	PlaceID     *int64 `json:"placeId,omitempty"`
}

func (s *Server) APIFindCity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	placesStore := s.getPlacesStore()
	if placesStore == nil {
		http.Error(w, "search is not configured", http.StatusServiceUnavailable)
		return
	}

	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	if latStr == "" || lonStr == "" {
		http.Error(w, "missing lat/lon", http.StatusBadRequest)
		return
	}

	lat, err1 := strconv.ParseFloat(latStr, 64)
	lon, err2 := strconv.ParseFloat(lonStr, 64)
	if err1 != nil || err2 != nil {
		http.Error(w, "invalid lat/lon", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 600*time.Millisecond)
	defer cancel()

	place, err := placesStore.Nearest(ctx, lat, lon)
	if err != nil {
		s.logger.Printf("find city nearest failed: %v", err)
		http.Error(w, "find city failed", http.StatusInternalServerError)
		return
	}
	if place == nil {
		http.Error(w, "no place found", http.StatusNotFound)
		return
	}

	lang := detectLang(r)
	displayName := places.LocalizedDisplayName(*place, lang)

	// If it's close enough to one of the “known weather cities” — return its ID.
	// Otherwise, fall back to a place route key.
	if known, ok := weather.MatchKnownCityByCoords(place.Lat, place.Lon); ok {
		resp := apiFindCityResponse{
			CityName:    known.ID,
			DisplayName: displayName,
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	pid := place.ID
	resp := apiFindCityResponse{
		CityName:    "place:" + strconv.FormatInt(pid, 10),
		DisplayName: displayName,
		PlaceID:     &pid,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

// --- /place/{id}: weather for arbitrary settlement ---

func (s *Server) Place(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/place/") {
		s.renderNotFound(w, r)
		return
	}
	placesStore := s.getPlacesStore()
	if placesStore == nil {
		http.Error(w, "search is not configured", http.StatusServiceUnavailable)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/place/")
	if idStr == "" {
		s.renderNotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.renderNotFound(w, r)
		return
	}

	// Track place view (bots excluded from "today" and "unique IPs")
	if s.analytics != nil {
		s.analytics.TrackRequest(r.URL.Path, s.getClientIP(r), r.UserAgent())
		s.trackExtendedInfo(r)
	}

	lang := detectLang(r)
	text := i18n.For(lang)

	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	place, err := placesStore.GetByID(ctx, id)
	if err != nil {
		s.logger.Printf("get place %d: %v", id, err)
		http.Error(w, "failed to load place", http.StatusInternalServerError)
		return
	}
	if place == nil {
		s.renderNotFound(w, r)
		return
	}

	cacheKey := "place:" + strconv.FormatInt(place.ID, 10)
	displayName := clampRunes(strings.TrimSpace(places.LocalizedDisplayName(*place, lang)), 128)
	if displayName == "" {
		displayName = strconv.FormatInt(place.ID, 10)
	}
	locationSubtitle := clampRunes(formatPlaceLocation(place, lang), 256)
	s.logger.Printf("place %d (%s) location subtitle: %s", place.ID, displayName, locationSubtitle)
	var data *weather.WeatherData
	if known, ok := weather.MatchKnownCityByCoords(place.Lat, place.Lon); ok {
		cacheKey = known.ID
		data, err = s.weather.GetWeather(ctx, known.ID)
	} else if known, ok := weather.MatchKnownCityByName(displayName); ok {
		cacheKey = known.ID
		data, err = s.weather.GetWeather(ctx, known.ID)
	} else {
		data, err = s.weather.GetWeatherForLocation(ctx, cacheKey, displayName, place.Lat, place.Lon)
	}
	if err != nil {
		s.logger.Printf("get weather for place %d: %v", id, err)
		http.Error(w, "failed to get weather data", http.StatusBadGateway)
		return
	}

	forecast := make([]DailyView, len(data.Forecast))
	for i, d := range data.Forecast {
		label := d.Date.Format("02.01")
		forecast[i] = DailyView{
			Date:        d.Date.Format("2006-01-02"),
			Label:       label,
			MinTemp:     d.MinTemp,
			MaxTemp:     d.MaxTemp,
			Description: i18n.WeatherDescription(d.WeatherCode, lang),
			Icon:        d.Icon,
		}
	}

	var todayLabel, tomorrowLabel, trend string
	var todayMin, todayMax, tomorrowMin, tomorrowMax float64
	if len(data.Forecast) > 0 {
		td := data.Forecast[0]
		todayLabel = td.Date.Format("02.01")
		todayMin = td.MinTemp
		todayMax = td.MaxTemp
	}
	if len(data.Forecast) > 1 {
		tm := data.Forecast[1]
		tomorrowLabel = tm.Date.Format("02.01")
		tomorrowMin = tm.MinTemp
		tomorrowMax = tm.MaxTemp

		diff := tomorrowMax - todayMax
		trend = i18n.TrendSentence(lang, diff)
	}

	// reuse static cities for quick switch
	baseCities := weather.AllCities()
	cityCards := make([]CitySummary, 0, len(baseCities))
	for _, c := range baseCities {
		d, err := s.weather.GetWeather(ctx, c.ID)
		if err != nil {
			s.logger.Printf("get weather for %s: %v", c.ID, err)
			continue
		}
		cardDesc := strings.TrimSpace(d.Current.Description)
		if cardDesc == "" {
			cardDesc = i18n.WeatherDescription(d.Current.WeatherCode, lang)
		}
		if d.IsFallback {
			cardDesc = text.WeatherUnavailableMsg
		}
		cityCards = append(cityCards, CitySummary{
			ID:                 d.CityID,
			Name:               weather.LocalizedCityName(d.CityID, lang),
			Temperature:        d.Current.Temperature,
			Description:        cardDesc,
			Icon:               d.Current.Icon,
			WindSpeed:          d.Current.WindSpeed,
			Humidity:           d.Current.Humidity,
			WeatherUnavailable: d.IsFallback,
		})
	}

	hourly := make([]HourlyView, 0, len(data.Hourly))
	for _, h := range data.Hourly {
		hourly = append(hourly, HourlyView{
			TimeLabel:   h.Time.Format("15:04"),
			Temperature: h.Temperature,
			Description: i18n.WeatherDescription(h.WeatherCode, lang),
			Icon:        h.Icon,
		})
	}

	dupCtx, dupCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	dupCount, dupLabel, dupURL := s.computePlaceDuplicates(dupCtx, place, lang, displayName)
	dupCancel()

	currentDesc := strings.TrimSpace(data.Current.Description)
	if currentDesc == "" {
		currentDesc = i18n.WeatherDescription(data.Current.WeatherCode, lang)
	}
	if data.IsFallback {
		currentDesc = text.WeatherUnavailableMsg
	}
	page := CityPageData{
		IsIndex:             false,
		WeatherCode:         data.Current.WeatherCode,
		IsNight:             data.Current.IsNight,
		WeatherMood:         weatherMoodClass(data.Current.WeatherCode, data.Current.IsNight),
		Lang:                lang,
		Path:                r.URL.Path,
		Text:                text,
		CityID:              "", // пусто: иначе client дергает /api/weather/{id} вместо /api/place_weather
		FavListID:           strconv.FormatInt(place.ID, 10),
		CityName:            displayName,
		CityLocation:        locationSubtitle,
		CurrentTemp:         data.Current.Temperature,
		CurrentFeelsLike:    data.Current.FeelsLike,
		CurrentDescription:  currentDesc,
		CurrentPrecipChance: data.Current.PrecipitationChance,
		CurrentWind:         data.Current.WindSpeed,
		CurrentHumidity:     data.Current.Humidity,
		CurrentPressure:     data.Current.Pressure,
		WeatherUnavailable:  data.IsFallback,
		WeatherStale:        data.IsStale,
		WeatherStaleText:    staleWeatherNotice(lang, data),
		WeatherSourceText:   weatherSourceText(lang, data),
		WeatherSourceKind:   weatherSourceKind(data),
		WeatherUpdatedText:  weatherUpdatedText(lang, data),
		RandomAdvice:        text.GetRandomAdvice(),
		Forecast:            forecast,
		TodayLabel:          todayLabel,
		TodayMin:            todayMin,
		TodayMax:            todayMax,
		TomorrowLabel:       tomorrowLabel,
		TomorrowMin:         tomorrowMin,
		TomorrowMax:         tomorrowMax,
		TrendText:           trend,
		Cities:              cityCards,
		Hourly:              hourly,
		WeatherJSON:         buildWeatherJSON(data.Sunrise, data.Sunset, data.Timezone, data.UTCOffsetSeconds, displayName),
		DuplicatesCount:     dupCount,
		DuplicatesLabel:     dupLabel,
		DuplicatesURL:       dupURL,
		ClientI18n:          template.JS(i18n.MarshalClientJSON(lang)),
		RadarLat:            place.Lat,
		RadarLon:            place.Lon,
		Current:             cityWeatherPhysicsFrom(data),
	}

	if err := s.tmpl.ExecuteTemplate(w, "city-page", page); err != nil {
		s.logger.Printf("render place: %v", err)
	}
}

func buildWeatherJSON(sunrise, sunset time.Time, tz string, offsetSeconds int, cityName string) template.JS {
	if sunrise.IsZero() || sunset.IsZero() {
		return template.JS("{}")
	}

	sunriseMinutes := sunrise.Hour()*60 + sunrise.Minute()
	sunsetMinutes := sunset.Hour()*60 + sunset.Minute()

	payload := struct {
		Sun struct {
			SunriseISO     string `json:"sunriseISO"`
			SunsetISO      string `json:"sunsetISO"`
			SunriseMinutes int    `json:"sunriseMinutes"`
			SunsetMinutes  int    `json:"sunsetMinutes"`
		} `json:"sun"`
		Meta struct {
			TZ            string `json:"tz"`
			OffsetSeconds int    `json:"offsetSeconds"`
		} `json:"meta"`
		CityName string `json:"cityName,omitempty"`
	}{
		Sun: struct {
			SunriseISO     string `json:"sunriseISO"`
			SunsetISO      string `json:"sunsetISO"`
			SunriseMinutes int    `json:"sunriseMinutes"`
			SunsetMinutes  int    `json:"sunsetMinutes"`
		}{
			SunriseISO:     sunrise.UTC().Format(time.RFC3339),
			SunsetISO:      sunset.UTC().Format(time.RFC3339),
			SunriseMinutes: sunriseMinutes,
			SunsetMinutes:  sunsetMinutes,
		},
		Meta: struct {
			TZ            string `json:"tz"`
			OffsetSeconds int    `json:"offsetSeconds"`
		}{TZ: tz, OffsetSeconds: offsetSeconds},
		CityName: strings.TrimSpace(cityName),
	}
	b, _ := json.Marshal(payload)
	return template.JS(b)
}

// --- helpers for localization of places ---

func hasCyrillic(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Cyrillic) {
			return true
		}
	}
	return false
}

func deriveOblastNames(oblastRaw string) (string, string, string) {
	o := strings.TrimSpace(oblastRaw)
	if o == "" {
		return "", "", ""
	}

	// Нормализуем исходное значение (поддерживаются и латиница, и кириллица).
	key := places.Normalize(o)

	type oblastNames struct {
		uk string
		ru string
		en string
	}

	// Словарь основных областей Украины.
	var oblastMap = map[string]oblastNames{
		"vinnytsia":              {"Вінницька область", "Винницкая область", "Vinnytsia Oblast"},
		"vinnytskaoblast":        {"Вінницька область", "Винницкая область", "Vinnytsia Oblast"},
		"volyn":                  {"Волинська область", "Волынская область", "Volyn Oblast"},
		"volynskaoblast":         {"Волинська область", "Волынская область", "Volyn Oblast"},
		"dnipropetrovsk":         {"Дніпропетровська область", "Днепропетровская область", "Dnipropetrovsk Oblast"},
		"dnipropetrovska":        {"Дніпропетровська область", "Днепропетровская область", "Dnipropetrovsk Oblast"},
		"dnipropetrovskaoblast":  {"Дніпропетровська область", "Днепропетровская область", "Dnipropetrovsk Oblast"},
		"donetsk":                {"Донецька область", "Донецкая область", "Donetsk Oblast"},
		"donetskaoblast":         {"Донецька область", "Донецкая область", "Donetsk Oblast"},
		"zhytomyr":               {"Житомирська область", "Житомирская область", "Zhytomyr Oblast"},
		"zhytomyrskaoblast":      {"Житомирська область", "Житомирская область", "Zhytomyr Oblast"},
		"zakarpattia":            {"Закарпатська область", "Закарпатская область", "Zakarpattia Oblast"},
		"zakarpatskaoblast":      {"Закарпатська область", "Закарпатская область", "Zakarpattia Oblast"},
		"zaporizhzhia":           {"Запорізька область", "Запорожская область", "Zaporizhzhia Oblast"},
		"zaporizkaoblast":        {"Запорізька область", "Запорожская область", "Zaporizhzhia Oblast"},
		"ivano-frankivsk":        {"Івано‑Франківська область", "Ивано‑Франковская область", "Ivano‑Frankivsk Oblast"},
		"ivanofrankivsk":         {"Івано‑Франківська область", "Ивано‑Франковская область", "Ivano‑Frankivsk Oblast"},
		"ivano frankivskaoblast": {"Івано‑Франківська область", "Ивано‑Франковская область", "Ivano‑Frankivsk Oblast"},
		"ivano-frankivskaoblast": {"Івано‑Франківська область", "Ивано‑Франковская область", "Ivano‑Frankivsk Oblast"},
		"kyiv":                   {"Київська область", "Киевская область", "Kyiv Oblast"},
		"kyivskaoblast":          {"Київська область", "Киевская область", "Kyiv Oblast"},
		"kirovohrad":             {"Кіровоградська область", "Кировоградская область", "Kirovohrad Oblast"},
		"kirovohradskaobla":      {"Кіровоградська область", "Кировоградская область", "Kirovohrad Oblast"},
		"kirovohradskaoblast":    {"Кіровоградська область", "Кировоградская область", "Kirovohrad Oblast"},
		"luhansk":                {"Луганська область", "Луганская область", "Luhansk Oblast"},
		"luhanskaoblast":         {"Луганська область", "Луганская область", "Luhansk Oblast"},
		"lviv":                   {"Львівська область", "Львовская область", "Lviv Oblast"},
		"lvivskaoblast":          {"Львівська область", "Львовская область", "Lviv Oblast"},
		"mykolaiv":               {"Миколаївська область", "Николаевская область", "Mykolaiv Oblast"},
		"mykolaivskaoblast":      {"Миколаївська область", "Николаевская область", "Mykolaiv Oblast"},
		"odesa":                  {"Одеська область", "Одесская область", "Odesa Oblast"},
		"odeskaoblast":           {"Одеська область", "Одесская область", "Odesa Oblast"},
		"poltava":                {"Полтавська область", "Полтавская область", "Poltava Oblast"},
		"poltavskaoblast":        {"Полтавська область", "Полтавская область", "Poltava Oblast"},
		"rivne":                  {"Рівненська область", "Ровенская область", "Rivne Oblast"},
		"rivnenskaoblast":        {"Рівненська область", "Ровенская область", "Rivne Oblast"},
		"sumy":                   {"Сумська область", "Сумская область", "Sumy Oblast"},
		"sumskaoblast":           {"Сумська область", "Сумская область", "Sumy Oblast"},
		"ternopil":               {"Тернопільська область", "Тернопольская область", "Ternopil Oblast"},
		"ternopilskaoblast":      {"Тернопільська область", "Тернопольская область", "Ternopil Oblast"},
		"kharkiv":                {"Харківська область", "Харьковская область", "Kharkiv Oblast"},
		"kharkivskaoblast":       {"Харківська область", "Харьковская область", "Kharkiv Oblast"},
		"kherson":                {"Херсонська область", "Херсонская область", "Kherson Oblast"},
		"khersonskaoblast":       {"Херсонська область", "Херсонская область", "Kherson Oblast"},
		"khmelnytskyi":           {"Хмельницька область", "Хмельницкая область", "Khmelnytskyi Oblast"},
		"khmelnytskaoblast":      {"Хмельницька область", "Хмельницкая область", "Khmelnytskyi Oblast"},
		"cherkasy":               {"Черкаська область", "Черкасская область", "Cherkasy Oblast"},
		"cherkaskaoblast":        {"Черкаська область", "Черкасская область", "Cherkasy Oblast"},
		"chernivtsi":             {"Чернівецька область", "Черновицкая область", "Chernivtsi Oblast"},
		"chernivetskaoblast":     {"Чернівецька область", "Черновицкая область", "Chernivtsi Oblast"},
		"chernihiv":              {"Чернігівська область", "Черниговская область", "Chernihiv Oblast"},
		"chernihivskaoblast":     {"Чернігівська область", "Черниговская область", "Chernihiv Oblast"},
		"crimea":                 {"Автономна Республіка Крим", "Автономная Республика Крым", "Autonomous Republic of Crimea"},
		"respublikakrym":         {"Автономна Республіка Крим", "Автономная Республика Крым", "Autonomous Republic of Crimea"},
		"sevastopol":             {"Севастополь", "Севастополь", "Sevastopol"},
	}

	if v, ok := oblastMap[key]; ok {
		return v.uk, v.ru, v.en
	}
	// Пробуем найти по префиксу/суффиксу: "kherson" vs "khersonskaoblast" и т.п.
	for k, v := range oblastMap {
		if strings.HasPrefix(k, key) || strings.HasPrefix(key, k) {
			return v.uk, v.ru, v.en
		}
	}

	// Если в словаре нет: если есть кириллица — считаем это укр‑названием.
	if hasCyrillic(o) {
		uk := o
		ru := o
		en := places.TranslitLatin(o)
		return uk, ru, en
	}

	// Фоллбек: латиница без словаря — используем как EN и просто дублируем.
	return o, o, o
}

func deriveTypeNames(base string) (string, string, string) {
	b := strings.ToLower(strings.TrimSpace(base))
	if b == "" {
		return "", "", ""
	}
	switch b {
	// Ukrainian/Russian canonical
	case "місто", "город", "city":
		return "місто", "город", "city"
	// селище міського типу / urban-type settlement (manual CSV may use "смт")
	case "селище", "смт", "посёлок", "поселок", "town", "settlement", "містечко":
		return "селище", "посёлок", "settlement"
	case "село", "village":
		return "село", "село", "village"
	default:
		// Если тип неизвестен/не задан — не подставляем "город",
		// чтобы UI показал "населённый пункт".
		return "", "", ""
	}
}

func countryName(lang string) string {
	switch lang {
	case "uk":
		return "Україна"
	case "en":
		return "Ukraine"
	default:
		return "Украина"
	}
}

func kyivDistrictSubtitle(p *places.Place, lang string) (string, bool) {
	if p == nil {
		return "", false
	}

	var raionRaw string
	if p.Raion != nil {
		raionRaw = strings.TrimSpace(*p.Raion)
	}
	nameNorm := places.Normalize(strings.TrimSpace(p.Name))
	if nameNorm == "" {
		nameNorm = places.Normalize(strings.TrimSpace(p.NameRU))
	}
	if nameNorm == "" {
		nameNorm = places.Normalize(strings.TrimSpace(p.NameUK))
	}
	oblastNorm := places.Normalize(strings.TrimSpace(p.Oblast))
	raionNorm := places.Normalize(raionRaw)

	isBortnychi := nameNorm == "bortnychi" || nameNorm == "бортничі" || nameNorm == "бортничи"
	isKyivContext := strings.Contains(oblastNorm, "kyiv") ||
		strings.Contains(oblastNorm, "київ") ||
		strings.Contains(oblastNorm, "киев") ||
		strings.Contains(raionNorm, "kyiv") ||
		strings.Contains(raionNorm, "київ") ||
		strings.Contains(raionNorm, "киев")
	if !isKyivContext && !isBortnychi {
		return "", false
	}

	districtBase := strings.TrimSpace(raionRaw)
	if districtBase == "" && isBortnychi {
		// Explicit override requested for Bortnychi.
		switch strings.ToLower(lang) {
		case "uk":
			districtBase = "Дарницький"
		case "en":
			districtBase = "Darnytskyi"
		default:
			districtBase = "Дарницкий"
		}
	}
	if districtBase == "" {
		return "", false
	}

	districtNorm := places.Normalize(districtBase)
	switch strings.ToLower(lang) {
	case "uk":
		switch {
		case strings.Contains(districtNorm, "дарниц"):
			districtBase = "Дарницький"
		case strings.Contains(districtNorm, "деснян"):
			districtBase = "Деснянський"
		case strings.Contains(districtNorm, "дніпров"):
			districtBase = "Дніпровський"
		case strings.Contains(districtNorm, "голос"):
			districtBase = "Голосіївський"
		case strings.Contains(districtNorm, "оболон"):
			districtBase = "Оболонський"
		case strings.Contains(districtNorm, "печер"):
			districtBase = "Печерський"
		case strings.Contains(districtNorm, "поділ"):
			districtBase = "Подільський"
		case strings.Contains(districtNorm, "святош"):
			districtBase = "Святошинський"
		case strings.Contains(districtNorm, "солом"):
			districtBase = "Солом'янський"
		case strings.Contains(districtNorm, "шевчен"):
			districtBase = "Шевченківський"
		}
		return districtBase + " район, Київ", true
	case "en":
		switch {
		case strings.Contains(districtNorm, "дарниц") || strings.Contains(districtNorm, "darn"):
			districtBase = "Darnytskyi"
		case strings.Contains(districtNorm, "деснян") || strings.Contains(districtNorm, "desn"):
			districtBase = "Desnianskyi"
		case strings.Contains(districtNorm, "дніпров") || strings.Contains(districtNorm, "dnipr"):
			districtBase = "Dniprovskyi"
		case strings.Contains(districtNorm, "голос") || strings.Contains(districtNorm, "holos"):
			districtBase = "Holosiivskyi"
		case strings.Contains(districtNorm, "оболон") || strings.Contains(districtNorm, "obol"):
			districtBase = "Obolonskyi"
		case strings.Contains(districtNorm, "печер") || strings.Contains(districtNorm, "pecher"):
			districtBase = "Pecherskyi"
		case strings.Contains(districtNorm, "поділ") || strings.Contains(districtNorm, "podil"):
			districtBase = "Podilskyi"
		case strings.Contains(districtNorm, "святош") || strings.Contains(districtNorm, "sviat"):
			districtBase = "Sviatoshynskyi"
		case strings.Contains(districtNorm, "солом") || strings.Contains(districtNorm, "solom"):
			districtBase = "Solomianskyi"
		case strings.Contains(districtNorm, "шевчен") || strings.Contains(districtNorm, "shev"):
			districtBase = "Shevchenkivskyi"
		}
		return districtBase + " District, Kyiv", true
	default:
		switch {
		case strings.Contains(districtNorm, "дарниц"):
			districtBase = "Дарницкий"
		case strings.Contains(districtNorm, "деснян"):
			districtBase = "Деснянский"
		case strings.Contains(districtNorm, "дніпров") || strings.Contains(districtNorm, "днепров"):
			districtBase = "Днепровский"
		case strings.Contains(districtNorm, "голос"):
			districtBase = "Голосеевский"
		case strings.Contains(districtNorm, "оболон"):
			districtBase = "Оболонский"
		case strings.Contains(districtNorm, "печер"):
			districtBase = "Печерский"
		case strings.Contains(districtNorm, "поділ") || strings.Contains(districtNorm, "подол"):
			districtBase = "Подольский"
		case strings.Contains(districtNorm, "святош"):
			districtBase = "Святошинский"
		case strings.Contains(districtNorm, "солом"):
			districtBase = "Соломенский"
		case strings.Contains(districtNorm, "шевчен"):
			districtBase = "Шевченковский"
		}
		return districtBase + " район, Киев", true
	}
}

func formatPlaceLocation(p *places.Place, lang string) string {
	if subtitle, ok := kyivDistrictSubtitle(p, lang); ok {
		return subtitle
	}

	var raion string
	if p.Raion != nil {
		raion = strings.TrimSpace(*p.Raion)
	}
	typeUK, typeRU, typeEN := deriveTypeNames(p.Type)
	var typ string
	switch lang {
	case "uk":
		typ = typeUK
	case "en":
		typ = typeEN
	default:
		typ = typeRU
	}
	if typ == "" {
		// Если тип отсутствует или неизвестен — не подставляем "город", показываем общий вариант.
		switch lang {
		case "uk":
			typ = "населений пункт"
		case "en":
			typ = "settlement"
		default:
			typ = "населённый пункт"
		}
	}
	oblastUK, oblastRU, oblastEN := deriveOblastNames(clampRunes(p.Oblast, 128))
	var oblast string
	switch lang {
	case "uk":
		oblast = oblastUK
	case "en":
		oblast = oblastEN
	default:
		oblast = oblastRU
	}
	oblast = strings.TrimSpace(oblast)

	parts := make([]string, 0, 4)
	if typ != "" {
		parts = append(parts, typ)
	}
	if raion != "" {
		parts = append(parts, raion)
	}
	if oblast != "" {
		parts = append(parts, oblast)
	}
	parts = append(parts, countryName(lang))
	return strings.Join(parts, ", ")
}

func (s *Server) computePlaceDuplicates(ctx context.Context, p *places.Place, lang, displayName string) (int, string, string) {
	placesStore := s.getPlacesStore()
	if placesStore == nil {
		return 0, "", ""
	}

	// Ищем все населённые пункты с таким же названием (в выбранном языке).
	q := clampRunes(strings.TrimSpace(displayName), 64)
	if q == "" {
		return 0, "", ""
	}
	list, err := placesStore.Search(ctx, q, 10)
	if err != nil || len(list) == 0 {
		return 0, "", ""
	}

	normalized := func(name string) string {
		return strings.ToLower(strings.TrimSpace(name))
	}
	target := normalized(displayName)

	count := 0
	for _, other := range list {
		if other.ID == p.ID {
			continue
		}
		candidate := places.LocalizedDisplayName(other, lang)
		if candidate == "" {
			candidate = other.Name
		}
		if normalized(candidate) == target {
			count++
		}
	}
	if count == 0 {
		return 0, "", ""
	}

	label := i18n.DuplicatesHint(lang, count)

	// Ссылка на главную с предзаполненным поиском
	dupURL := "/?lang=" + lang + "&query=" + url.QueryEscape(displayName)
	return count, label, dupURL
}

// Sitemap generates dynamic sitemap.xml for SEO (all cities and places).
func (s *Server) Sitemap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://weather-ua.onrender.com"
	}
	// Ensure no trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	var buf strings.Builder
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")

	// Homepage
	buf.WriteString("<url>")
	buf.WriteString("<loc>" + xmlEscape(baseURL) + "/</loc>")
	buf.WriteString("<changefreq>hourly</changefreq>")
	buf.WriteString("<priority>1.0</priority>")
	buf.WriteString("</url>")

	// Static cities
	for _, c := range weather.AllCities() {
		buf.WriteString("<url>")
		buf.WriteString("<loc>" + xmlEscape(baseURL+"/city/"+c.ID) + "</loc>")
		buf.WriteString("<changefreq>hourly</changefreq>")
		buf.WriteString("<priority>0.9</priority>")
		buf.WriteString("</url>")
	}

	// Places from SQLite (with population-based priority)
	ps := s.getPlacesStore()
	if ps != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		// Get all places with population info
		placesList, err := ps.Search(ctx, "", 50000) // Empty query = all places
		if err == nil {
			for _, p := range placesList {
				priority := placePriority(p.Population)
				buf.WriteString("<url>")
				buf.WriteString("<loc>" + xmlEscape(baseURL+"/place/"+strconv.FormatInt(p.ID, 10)) + "</loc>")
				buf.WriteString("<changefreq>hourly</changefreq>")
				buf.WriteString("<priority>" + priority + "</priority>")
				buf.WriteString("</url>")
			}
		}
	}

	buf.WriteString("</urlset>")
	_, _ = w.Write([]byte(buf.String()))
}

// placePriority returns priority based on population.
func placePriority(population int64) string {
	switch {
	case population > 10000:
		return "0.8"
	case population >= 1000:
		return "0.7"
	default:
		return "0.5"
	}
}

// xmlEscape basic XML escaping for URLs.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func detectLang(r *http.Request) string {
	return i18n.Normalize(r.URL.Query().Get("lang"))
}

func clampRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max]))
}
