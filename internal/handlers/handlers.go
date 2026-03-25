package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"bss/internal/i18n"
	"bss/internal/places"
	"bss/internal/weather"
)

type Server struct {
	tmpl     *template.Template
	weather  *weather.Client
	places   *places.Store
	placesMu sync.RWMutex
	logger   *log.Logger
}

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
	WeatherCode     int
	IsNight         bool
	WeatherMood     string
	Lang            string
	Path            string
	Text            TextSet
	CityID          string
	CurrentCityName string
	// HeroCityTitle is the index hero H1 only (Kyiv uses custom copy per language).
	HeroCityTitle      string
	CurrentTemp        float64
	CurrentDescription string
	CurrentWind        float64
	CurrentHumidity    float64
	CurrentPressure    float64
	WeatherUnavailable bool
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

type CityPageData struct {
	IsIndex            bool
	WeatherCode        int
	IsNight            bool
	WeatherMood        string
	Lang               string
	Path               string
	Text               TextSet
	CityID             string
	CityName           string
	CityLocation       string
	CurrentTemp        float64
	CurrentDescription string
	CurrentWind        float64
	CurrentHumidity    float64
	CurrentPressure    float64
	WeatherUnavailable bool
	Forecast           []DailyView
	TodayLabel         string
	TodayMin           float64
	TodayMax           float64
	TomorrowLabel      string
	TomorrowMin        float64
	TomorrowMax        float64
	TrendText          string
	Cities             []CitySummary
	Hourly             []HourlyView
	WeatherJSON        template.JS
	DuplicatesCount    int
	DuplicatesLabel    string
	DuplicatesURL      string
	ClientI18n         template.JS
}

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	lang := detectLang(r)
	text := i18n.For(lang)

	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	cities := weather.AllCities()
	summaries := make([]CitySummary, 0, len(cities))

	var heroData *weather.WeatherData
	// Always show Kyiv on the main page by default.
	// (Some city lists are ordered such that the first item is not Kyiv.)
	const heroCityID = "kyiv"

	for _, city := range cities {
		data, err := s.weather.GetWeather(ctx, city.ID)
		if err != nil {
			s.logger.Printf("get weather for %s: %v", city.ID, err)
			continue
		}

		// Pick Kyiv as a hero weather; other cities are only for the cards list.
		if heroData == nil && city.ID == heroCityID {
			heroData = data
		}

		desc := strings.TrimSpace(data.Current.Description)
		if desc == "" {
			desc = i18n.WeatherDescription(data.Current.WeatherCode, lang)
		}
		if data.IsFallback {
			desc = text.WeatherUnavailableMsg
		}
		summaries = append(summaries, CitySummary{
			ID:                 data.CityID,
			Name:               weather.LocalizedCityName(data.CityID, lang),
			Temperature:        data.Current.Temperature,
			Description:        desc,
			Icon:               data.Current.Icon,
			WindSpeed:          data.Current.WindSpeed,
			Humidity:           data.Current.Humidity,
			WeatherUnavailable: data.IsFallback,
		})
	}

	if heroData == nil {
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
			heroData = &weather.WeatherData{IsFallback: true}
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
	if heroData.CityID == "kyiv" {
		switch strings.ToLower(lang) {
		case "ru":
			heroCityTitle = "Киев — город Димаса"
		case "uk":
			heroCityTitle = "Київ — місто Дімаса"
		default:
			heroCityTitle = "Kyiv — Dimas City"
		}
	}

	page := IndexPageData{
		IsIndex:            true,
		WeatherCode:        heroData.Current.WeatherCode,
		IsNight:            heroData.Current.IsNight,
		WeatherMood:        weatherMoodClass(heroData.Current.WeatherCode, heroData.Current.IsNight),
		Lang:               lang,
		Path:               r.URL.Path,
		Text:               text,
		CityID:             "",
		CurrentCityName:    weather.LocalizedCityName(heroData.CityID, lang),
		HeroCityTitle:      heroCityTitle,
		CurrentTemp:        heroData.Current.Temperature,
		CurrentDescription: currentDesc,
		CurrentWind:        heroData.Current.WindSpeed,
		CurrentHumidity:    heroData.Current.Humidity,
		CurrentPressure:    heroData.Current.Pressure,
		WeatherUnavailable: heroData.IsFallback,
		TodayLabel:         todayLabel,
		TodayMin:           todayMin,
		TodayMax:           todayMax,
		TomorrowLabel:      tomorrowLabel,
		TomorrowMin:        tomorrowMin,
		TomorrowMax:        tomorrowMax,
		DayAfterLabel:      dayAfterLabel,
		DayAfterMin:        dayAfterMin,
		DayAfterMax:        dayAfterMax,
		Cities:             summaries,
		WeatherJSON:        buildWeatherJSON(heroData.Sunrise, heroData.Sunset, heroData.Timezone, heroData.UTCOffsetSeconds),
		MetaDescription:    metaDesc,
		ClientI18n:         template.JS(i18n.MarshalClientJSON(lang)),
	}

	if err := s.tmpl.ExecuteTemplate(w, "index-page", page); err != nil {
		s.logger.Printf("render index: %v", err)
	}
}

func (s *Server) City(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/city/") {
		http.NotFound(w, r)
		return
	}

	cityID := strings.TrimPrefix(r.URL.Path, "/city/")
	if cityID == "" {
		http.NotFound(w, r)
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
		s.logger.Printf("get city weather %s: %v", cityID, err)
		http.Error(w, "не удалось получить данные погоды", http.StatusBadGateway)
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

	// hourly view (first 12 hours)
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
		IsIndex:            false,
		WeatherCode:        data.Current.WeatherCode,
		IsNight:            data.Current.IsNight,
		WeatherMood:        weatherMoodClass(data.Current.WeatherCode, data.Current.IsNight),
		Lang:               lang,
		Path:               r.URL.Path,
		Text:               text,
		CityID:             data.CityID,
		CityName:           weather.LocalizedCityName(data.CityID, lang),
		CurrentTemp:        data.Current.Temperature,
		CurrentDescription: currentDesc,
		CurrentWind:        data.Current.WindSpeed,
		CurrentHumidity:    data.Current.Humidity,
		CurrentPressure:    data.Current.Pressure,
		WeatherUnavailable: data.IsFallback,
		Forecast:           forecast,
		TodayLabel:         todayLabel,
		TodayMin:           todayMin,
		TodayMax:           todayMax,
		TomorrowLabel:      tomorrowLabel,
		TomorrowMin:        tomorrowMin,
		TomorrowMax:        tomorrowMax,
		TrendText:          trend,
		Cities:             cityCards,
		Hourly:             hourly,
		WeatherJSON:        buildWeatherJSON(data.Sunrise, data.Sunset, data.Timezone, data.UTCOffsetSeconds),
		ClientI18n:         template.JS(i18n.MarshalClientJSON(lang)),
	}

	if err := s.tmpl.ExecuteTemplate(w, "city-page", page); err != nil {
		s.logger.Printf("render city: %v", err)
	}
}

// --- API: weather by city id (existing) ---

type apiWeatherResponse struct {
	CityID   string      `json:"cityId"`
	CityName string      `json:"cityName"`
	Lang     string      `json:"lang"`
	Current  apiCurrent  `json:"current"`
	Hourly   []apiHourly `json:"hourly,omitempty"`
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
	Time        string  `json:"time"`
	Temperature float64 `json:"temperature"`
	Description string  `json:"description"`
	Icon        string  `json:"icon"`
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
		Sunrise:  sunriseStr,
		Sunset:   sunsetStr,
		Daily:    apiDailyFromForecast(data.Forecast),
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
			Icon:        data.Current.Icon,
			IsFallback:  data.IsFallback,
			Wind:        data.Current.WindSpeed,
			Humidity:    data.Current.Humidity,
			UVIndex:     data.Current.UVIndex,
			Visibility:  data.Current.VisibilityKm,
			Pressure:    data.Current.Pressure,
			WeatherCode: data.Current.WeatherCode,
			IsNight:     data.Current.IsNight,
		},
	}

	for _, h := range data.Hourly {
		resp.Hourly = append(resp.Hourly, apiHourly{
			Time:        h.Time.Format("15:04"),
			Temperature: h.Temperature,
			Description: i18n.WeatherDescription(h.WeatherCode, lang),
			Icon:        h.Icon,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
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
		Sunrise:  sunriseStr,
		Sunset:   sunsetStr,
		Daily:    apiDailyFromForecast(data.Forecast),
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
			Icon:        data.Current.Icon,
			IsFallback:  data.IsFallback,
			Wind:        data.Current.WindSpeed,
			Humidity:    data.Current.Humidity,
			UVIndex:     data.Current.UVIndex,
			Visibility:  data.Current.VisibilityKm,
			Pressure:    data.Current.Pressure,
			WeatherCode: data.Current.WeatherCode,
			IsNight:     data.Current.IsNight,
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Printf("encode api place weather: %v", err)
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
		http.NotFound(w, r)
		return
	}
	placesStore := s.getPlacesStore()
	if placesStore == nil {
		http.Error(w, "search is not configured", http.StatusServiceUnavailable)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/place/")
	if idStr == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
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
		http.NotFound(w, r)
		return
	}

	cacheKey := "place:" + strconv.FormatInt(place.ID, 10)
	displayName := places.LocalizedDisplayName(*place, lang)
	locationSubtitle := formatPlaceLocation(place, lang)
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
		http.Error(w, "не удалось получить данные погоды", http.StatusBadGateway)
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

	dupCount, dupLabel, dupURL := s.computePlaceDuplicates(ctx, place, lang, displayName)

	currentDesc := strings.TrimSpace(data.Current.Description)
	if currentDesc == "" {
		currentDesc = i18n.WeatherDescription(data.Current.WeatherCode, lang)
	}
	if data.IsFallback {
		currentDesc = text.WeatherUnavailableMsg
	}
	page := CityPageData{
		IsIndex:            false,
		WeatherCode:        data.Current.WeatherCode,
		IsNight:            data.Current.IsNight,
		WeatherMood:        weatherMoodClass(data.Current.WeatherCode, data.Current.IsNight),
		Lang:               lang,
		Path:               r.URL.Path,
		Text:               text,
		CityID:             "", // нет автообновления через /api/weather
		CityName:           displayName,
		CityLocation:       locationSubtitle,
		CurrentTemp:        data.Current.Temperature,
		CurrentDescription: currentDesc,
		CurrentWind:        data.Current.WindSpeed,
		CurrentHumidity:    data.Current.Humidity,
		CurrentPressure:    data.Current.Pressure,
		WeatherUnavailable: data.IsFallback,
		Forecast:           forecast,
		TodayLabel:         todayLabel,
		TodayMin:           todayMin,
		TodayMax:           todayMax,
		TomorrowLabel:      tomorrowLabel,
		TomorrowMin:        tomorrowMin,
		TomorrowMax:        tomorrowMax,
		TrendText:          trend,
		Cities:             cityCards,
		Hourly:             hourly,
		WeatherJSON:        buildWeatherJSON(data.Sunrise, data.Sunset, data.Timezone, data.UTCOffsetSeconds),
		DuplicatesCount:    dupCount,
		DuplicatesLabel:    dupLabel,
		DuplicatesURL:      dupURL,
		ClientI18n:         template.JS(i18n.MarshalClientJSON(lang)),
	}

	if err := s.tmpl.ExecuteTemplate(w, "city-page", page); err != nil {
		s.logger.Printf("render place: %v", err)
	}
}

func buildWeatherJSON(sunrise, sunset time.Time, tz string, offsetSeconds int) template.JS {
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
	oblastUK, oblastRU, oblastEN := deriveOblastNames(p.Oblast)
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
	q := displayName
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

func detectLang(r *http.Request) string {
	return i18n.Normalize(r.URL.Query().Get("lang"))
}
