package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"bss/internal/places"
	"bss/internal/weather"
)

type Server struct {
	tmpl    *template.Template
	weather *weather.Client
	places  *places.Store
	logger  *log.Logger
}

func NewServer(tmpl *template.Template, weatherClient *weather.Client, placesStore *places.Store, logger *log.Logger) *Server {
	return &Server{
		tmpl:    tmpl,
		weather: weatherClient,
		places:  placesStore,
		logger:  logger,
	}
}

type CitySummary struct {
	ID          string
	Name        string
	Temperature float64
	Description string
	Icon        string
	WindSpeed   float64
	Humidity    float64
}

type TextSet struct {
	BrandTop            string
	BrandBottom         string
	NavNow              string
	NavCities           string
	BadgeNow            string
	CitiesLabel         string
	CitiesTitle         string
	CityBadge           string
	MetricWind          string
	MetricHumidity      string
	MetricPressure      string
	Today               string
	Tomorrow            string
	RangeFrom           string
	RangeTo             string
	CurrentMetricsAria  string
	CurrentWeatherAria  string
	ShortForecastAria   string
	ForecastLabel       string
	ForecastTitle       string
	ForecastAria        string
	TrendLabel          string
	ChartTitle          string
	ChartAria           string
	OtherCitiesAria     string
	QuickSwitchTitle    string
	SearchPlaceholder       string
	Footer                  string
	MoreLink                string
	WeatherUnavailableMsg   string
}

type IndexPageData struct {
	IsIndex            bool
	WeatherCode        int
	IsNight            bool
	Lang               string
	Path               string
	Text               TextSet
	CityID             string
	CurrentCityName    string
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
	Cities             []CitySummary
	WeatherJSON        template.JS
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
	TodayLabel       string
	TodayMin         float64
	TodayMax         float64
	TomorrowLabel    string
	TomorrowMin      float64
	TomorrowMax      float64
	TrendText        string
	Cities           []CitySummary
	Hourly           []HourlyView
	WeatherJSON      template.JS
	DuplicatesCount  int
	DuplicatesLabel  string
	DuplicatesURL    string
}

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	lang := detectLang(r)
	text := texts(lang)

	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	cities := weather.AllCities()
	summaries := make([]CitySummary, 0, len(cities))

	var heroData *weather.WeatherData

	for _, city := range cities {
		data, err := s.weather.GetWeather(ctx, city.ID)
		if err != nil {
			s.logger.Printf("get weather for %s: %v", city.ID, err)
			continue
		}

		if heroData == nil {
			heroData = data
		}

		desc := weather.LocalizedDescription(data.Current.WeatherCode, lang)
		if data.IsFallback {
			desc = text.WeatherUnavailableMsg
		}
		summaries = append(summaries, CitySummary{
			ID:          data.CityID,
			Name:        weather.LocalizedCityName(data.CityID, lang),
			Temperature: data.Current.Temperature,
			Description: desc,
			Icon:        data.Current.Icon,
			WindSpeed:   data.Current.WindSpeed,
			Humidity:    data.Current.Humidity,
		})
	}

	if heroData == nil {
		http.Error(w, "service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	var todayLabel, tomorrowLabel string
	var todayMin, todayMax, tomorrowMin, tomorrowMax float64
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

	currentDesc := weather.LocalizedDescription(heroData.Current.WeatherCode, lang)
	if heroData.IsFallback {
		currentDesc = text.WeatherUnavailableMsg
	}
	page := IndexPageData{
		IsIndex:            true,
		WeatherCode:        heroData.Current.WeatherCode,
		IsNight:            heroData.Current.IsNight,
		Lang:               lang,
		Path:               r.URL.Path,
		Text:               text,
		CityID:             "",
		CurrentCityName:    weather.LocalizedCityName(heroData.CityID, lang),
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
		Cities:             summaries,
		WeatherJSON:        buildWeatherJSON(heroData.Sunrise, heroData.Sunset, heroData.Timezone, heroData.UTCOffsetSeconds),
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
	text := texts(lang)

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
			Description: weather.LocalizedDescription(d.WeatherCode, lang),
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
		trend = trendText(lang, diff)
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
		cardDesc := weather.LocalizedDescription(d.Current.WeatherCode, lang)
		if d.IsFallback {
			cardDesc = text.WeatherUnavailableMsg
		}
		cityCards = append(cityCards, CitySummary{
			ID:          d.CityID,
			Name:        weather.LocalizedCityName(d.CityID, lang),
			Temperature: d.Current.Temperature,
			Description: cardDesc,
			Icon:        d.Current.Icon,
			WindSpeed:   d.Current.WindSpeed,
			Humidity:    d.Current.Humidity,
		})
	}

	// hourly view (first 12 hours)
	hourly := make([]HourlyView, 0, len(data.Hourly))
	for _, h := range data.Hourly {
		hourly = append(hourly, HourlyView{
			TimeLabel:   h.Time.Format("15:04"),
			Temperature: h.Temperature,
			Description: weather.LocalizedDescription(h.WeatherCode, lang),
			Icon:        h.Icon,
		})
	}

	currentDesc := weather.LocalizedDescription(data.Current.WeatherCode, lang)
	if data.IsFallback {
		currentDesc = text.WeatherUnavailableMsg
	}
	page := CityPageData{
		IsIndex:            false,
		WeatherCode:        data.Current.WeatherCode,
		IsNight:            data.Current.IsNight,
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
	}

	if err := s.tmpl.ExecuteTemplate(w, "city-page", page); err != nil {
		s.logger.Printf("render city: %v", err)
	}
}

// --- API: weather by city id (existing) ---

type apiWeatherResponse struct {
	CityID   string        `json:"cityId"`
	CityName string        `json:"cityName"`
	Lang     string        `json:"lang"`
	Current  apiCurrent    `json:"current"`
	Hourly   []apiHourly   `json:"hourly,omitempty"`
}

type apiCurrent struct {
	Temperature float64 `json:"temperature"`
	Description string  `json:"description"`
	Icon        string  `json:"icon"`
	Wind        float64 `json:"wind"`
	Humidity    float64 `json:"humidity"`
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

	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	data, err := s.weather.GetWeather(ctx, cityID)
	if err != nil {
		s.logger.Printf("api get city weather %s: %v", cityID, err)
		http.Error(w, "failed to fetch weather", http.StatusBadGateway)
		return
	}

	resp := apiWeatherResponse{
		CityID:   data.CityID,
		CityName: weather.LocalizedCityName(data.CityID, lang),
		Lang:     lang,
		Current: apiCurrent{
			Temperature: data.Current.Temperature,
			Description: weather.LocalizedDescription(data.Current.WeatherCode, lang),
			Icon:        data.Current.Icon,
			Wind:        data.Current.WindSpeed,
			Humidity:    data.Current.Humidity,
		},
	}

	for _, h := range data.Hourly {
		resp.Hourly = append(resp.Hourly, apiHourly{
			Time:        h.Time.Format("15:04"),
			Temperature: h.Temperature,
			Description: weather.LocalizedDescription(h.WeatherCode, lang),
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
	if s.places == nil {
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

	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	place, err := s.places.GetByID(ctx, id)
	if err != nil {
		s.logger.Printf("api place weather get %d: %v", id, err)
		http.Error(w, "failed to load place", http.StatusInternalServerError)
		return
	}
	if place == nil {
		http.NotFound(w, r)
		return
	}

	displayName := pickPlaceDisplayName(place, lang)
	if displayName == "" {
		displayName = place.Name
	}

	cacheKey := "place:" + strconv.FormatInt(place.ID, 10)
	data, err := s.weather.GetWeatherForLocation(ctx, cacheKey, displayName, place.Lat, place.Lon)
	if err != nil {
		s.logger.Printf("api place weather %d: %v", id, err)
		http.Error(w, "failed to fetch weather", http.StatusBadGateway)
		return
	}

	resp := apiWeatherResponse{
		CityID:   cacheKey,
		CityName: displayName,
		Lang:     lang,
		Current: apiCurrent{
			Temperature: data.Current.Temperature,
			Description: weather.LocalizedDescription(data.Current.WeatherCode, lang),
			Icon:        data.Current.Icon,
			Wind:        data.Current.WindSpeed,
			Humidity:    data.Current.Humidity,
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
	if s.places != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 600*time.Millisecond)
		defer cancel()
		place, err := s.places.Nearest(ctx, lat, lon)
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

	if s.places == nil {
		// search сервис недоступен — возвращаем пустой список, чтобы не ломать UI
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte("[]"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	defer cancel()

	list, err := s.places.Search(ctx, q, limit)
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
		nameUK, nameRU, nameEN := derivePlaceNames(p)
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

// --- /place/{id}: weather for arbitrary settlement ---

func (s *Server) Place(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/place/") {
		http.NotFound(w, r)
		return
	}
	if s.places == nil {
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
	text := texts(lang)

	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	place, err := s.places.GetByID(ctx, id)
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
	displayName := pickPlaceDisplayName(place, lang)
	if displayName == "" {
		displayName = place.Name
	}
	locationSubtitle := formatPlaceLocation(place, lang)
	s.logger.Printf("place %d (%s) location subtitle: %s", place.ID, displayName, locationSubtitle)
	data, err := s.weather.GetWeatherForLocation(ctx, cacheKey, displayName, place.Lat, place.Lon)
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
			Description: weather.LocalizedDescription(d.WeatherCode, lang),
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
		trend = trendText(lang, diff)
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
		cardDesc := weather.LocalizedDescription(d.Current.WeatherCode, lang)
		if d.IsFallback {
			cardDesc = text.WeatherUnavailableMsg
		}
		cityCards = append(cityCards, CitySummary{
			ID:          d.CityID,
			Name:        weather.LocalizedCityName(d.CityID, lang),
			Temperature: d.Current.Temperature,
			Description: cardDesc,
			Icon:        d.Current.Icon,
			WindSpeed:   d.Current.WindSpeed,
			Humidity:    d.Current.Humidity,
		})
	}

	hourly := make([]HourlyView, 0, len(data.Hourly))
	for _, h := range data.Hourly {
		hourly = append(hourly, HourlyView{
			TimeLabel:   h.Time.Format("15:04"),
			Temperature: h.Temperature,
			Description: weather.LocalizedDescription(h.WeatherCode, lang),
			Icon:        h.Icon,
		})
	}

	dupCount, dupLabel, dupURL := s.computePlaceDuplicates(ctx, place, lang, displayName)

	currentDesc := weather.LocalizedDescription(data.Current.WeatherCode, lang)
	if data.IsFallback {
		currentDesc = text.WeatherUnavailableMsg
	}
	page := CityPageData{
		IsIndex:            false,
		WeatherCode:        data.Current.WeatherCode,
		IsNight:            data.Current.IsNight,
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

func isLatinLettersOnly(s string) bool {
	hasLetter := false
	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		hasLetter = true
		if !unicode.In(r, unicode.Latin) {
			return false
		}
	}
	return hasLetter
}

// derivePlaceNames возвращает best‑effort имена на укр/рус/англ
// на основе полей name_uk/name_ru в базе.
func derivePlaceNames(p places.Place) (string, string, string) {
	rawUK := strings.TrimSpace(p.NameUK)
	rawRU := strings.TrimSpace(p.NameRU)

	var nameUK, nameRU, nameEN string

	// RU: предпочитаем явное русское имя, иначе берём кириллицу из name_uk.
	if rawRU != "" {
		nameRU = rawRU
	} else if hasCyrillic(rawUK) {
		nameRU = rawUK
	}

	// UK: если name_uk уже на кириллице — используем его,
	// иначе fall back на русское (хотя бы кириллица), иначе то, что есть.
	if hasCyrillic(rawUK) {
		nameUK = rawUK
	} else if rawRU != "" {
		nameUK = rawRU
	} else {
		nameUK = rawUK
	}

	// EN: если name_uk уже в латинице — это наш лучший вариант,
	// иначе транслитеруем из кириллицы.
	if isLatinLettersOnly(rawUK) && rawUK != "" {
		nameEN = rawUK
	} else if rawRU != "" {
		nameEN = places.TranslitLatin(rawRU)
	} else if rawUK != "" {
		nameEN = places.TranslitLatin(rawUK)
	}

	return nameUK, nameRU, nameEN
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
		"vinnytsia":             {"Вінницька область", "Винницкая область", "Vinnytsia Oblast"},
		"vinnytskaoblast":       {"Вінницька область", "Винницкая область", "Vinnytsia Oblast"},
		"volyn":                 {"Волинська область", "Волынская область", "Volyn Oblast"},
		"volynskaoblast":        {"Волинська область", "Волынская область", "Volyn Oblast"},
		"dnipropetrovsk":        {"Дніпропетровська область", "Днепропетровская область", "Dnipropetrovsk Oblast"},
		"dnipropetrovska":       {"Дніпропетровська область", "Днепропетровская область", "Dnipropetrovsk Oblast"},
		"dnipropetrovskaoblast": {"Дніпропетровська область", "Днепропетровская область", "Dnipropetrovsk Oblast"},
		"donetsk":               {"Донецька область", "Донецкая область", "Donetsk Oblast"},
		"donetskaoblast":        {"Донецька область", "Донецкая область", "Donetsk Oblast"},
		"zhytomyr":              {"Житомирська область", "Житомирская область", "Zhytomyr Oblast"},
		"zhytomyrskaoblast":     {"Житомирська область", "Житомирская область", "Zhytomyr Oblast"},
		"zakarpattia":           {"Закарпатська область", "Закарпатская область", "Zakarpattia Oblast"},
		"zakarpatskaoblast":     {"Закарпатська область", "Закарпатская область", "Zakarpattia Oblast"},
		"zaporizhzhia":          {"Запорізька область", "Запорожская область", "Zaporizhzhia Oblast"},
		"zaporizkaoblast":       {"Запорізька область", "Запорожская область", "Zaporizhzhia Oblast"},
		"ivano-frankivsk":       {"Івано‑Франківська область", "Ивано‑Франковская область", "Ivano‑Frankivsk Oblast"},
		"ivanofrankivsk":        {"Івано‑Франківська область", "Ивано‑Франковская область", "Ivano‑Frankivsk Oblast"},
		"ivano frankivskaoblast": {"Івано‑Франківська область", "Ивано‑Франковская область", "Ivano‑Frankivsk Oblast"},
		"ivano-frankivskaoblast": {"Івано‑Франківська область", "Ивано‑Франковская область", "Ivano‑Frankivsk Oblast"},
		"kyiv":                  {"Київська область", "Киевская область", "Kyiv Oblast"},
		"kyivskaoblast":         {"Київська область", "Киевская область", "Kyiv Oblast"},
		"kirovohrad":            {"Кіровоградська область", "Кировоградская область", "Kirovohrad Oblast"},
		"kirovohradskaobla":     {"Кіровоградська область", "Кировоградская область", "Kirovohrad Oblast"},
		"kirovohradskaoblast":   {"Кіровоградська область", "Кировоградская область", "Kirovohrad Oblast"},
		"luhansk":               {"Луганська область", "Луганская область", "Luhansk Oblast"},
		"luhanskaoblast":        {"Луганська область", "Луганская область", "Luhansk Oblast"},
		"lviv":                  {"Львівська область", "Львовская область", "Lviv Oblast"},
		"lvivskaoblast":         {"Львівська область", "Львовская область", "Lviv Oblast"},
		"mykolaiv":              {"Миколаївська область", "Николаевская область", "Mykolaiv Oblast"},
		"mykolaivskaoblast":     {"Миколаївська область", "Николаевская область", "Mykolaiv Oblast"},
		"odesa":                 {"Одеська область", "Одесская область", "Odesa Oblast"},
		"odeskaoblast":          {"Одеська область", "Одесская область", "Odesa Oblast"},
		"poltava":               {"Полтавська область", "Полтавская область", "Poltava Oblast"},
		"poltavskaoblast":       {"Полтавська область", "Полтавская область", "Poltava Oblast"},
		"rivne":                 {"Рівненська область", "Ровенская область", "Rivne Oblast"},
		"rivnenskaoblast":       {"Рівненська область", "Ровенская область", "Rivne Oblast"},
		"sumy":                  {"Сумська область", "Сумская область", "Sumy Oblast"},
		"sumskaoblast":          {"Сумська область", "Сумская область", "Sumy Oblast"},
		"ternopil":              {"Тернопільська область", "Тернопольская область", "Ternopil Oblast"},
		"ternopilskaoblast":     {"Тернопільська область", "Тернопольская область", "Ternopil Oblast"},
		"kharkiv":               {"Харківська область", "Харьковская область", "Kharkiv Oblast"},
		"kharkivskaoblast":      {"Харківська область", "Харьковская область", "Kharkiv Oblast"},
		"kherson":               {"Херсонська область", "Херсонская область", "Kherson Oblast"},
		"khersonskaoblast":      {"Херсонська область", "Херсонская область", "Kherson Oblast"},
		"khmelnytskyi":          {"Хмельницька область", "Хмельницкая область", "Khmelnytskyi Oblast"},
		"khmelnytskaoblast":     {"Хмельницька область", "Хмельницкая область", "Khmelnytskyi Oblast"},
		"cherkasy":              {"Черкаська область", "Черкасская область", "Cherkasy Oblast"},
		"cherkaskaoblast":       {"Черкаська область", "Черкасская область", "Cherkasy Oblast"},
		"chernivtsi":            {"Чернівецька область", "Черновицкая область", "Chernivtsi Oblast"},
		"chernivetskaoblast":    {"Чернівецька область", "Черновицкая область", "Chernivtsi Oblast"},
		"chernihiv":             {"Чернігівська область", "Черниговская область", "Chernihiv Oblast"},
		"chernihivskaoblast":    {"Чернігівська область", "Черниговская область", "Chernihiv Oblast"},
		"crimea":                {"Автономна Республіка Крим", "Автономная Республика Крым", "Autonomous Republic of Crimea"},
		"respublikakrym":        {"Автономна Республіка Крим", "Автономная Республика Крым", "Autonomous Republic of Crimea"},
		"sevastopol":            {"Севастополь", "Севастополь", "Sevastopol"},
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
	switch b {
	case "місто":
		return "місто", "город", "city"
	case "селище":
		return "селище", "посёлок", "settlement"
	case "село":
		return "село", "село", "village"
	default:
		if base == "" {
			return "", "", ""
		}
		// Если тип неизвестен, возвращаем его как есть для всех языков.
		return base, base, base
	}
}

func pickPlaceDisplayName(p *places.Place, lang string) string {
	nameUK, nameRU, nameEN := derivePlaceNames(*p)
	switch lang {
	case "uk":
		if nameUK != "" {
			return nameUK
		}
	case "ru":
		if nameRU != "" {
			return nameRU
		}
	case "en":
		if nameEN != "" {
			return nameEN
		}
	}
	// Fallbacks
	if nameUK != "" {
		return nameUK
	}
	if nameRU != "" {
		return nameRU
	}
	if nameEN != "" {
		return nameEN
	}
	return strings.TrimSpace(p.Name)
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

func formatPlaceLocation(p *places.Place, lang string) string {
	var raion string
	if p.Raion != nil {
		raion = strings.TrimSpace(*p.Raion)
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

	parts := make([]string, 0, 3)
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
	if s.places == nil {
		return 0, "", ""
	}

	// Ищем все населённые пункты с таким же названием (в выбранном языке).
	q := displayName
	list, err := s.places.Search(ctx, q, 10)
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
		// сравниваем по локализованному имени
		nameUK, nameRU, nameEN := derivePlaceNames(other)
		var candidate string
		switch lang {
		case "uk":
			candidate = nameUK
		case "en":
			candidate = nameEN
		default:
			candidate = nameRU
		}
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

	var label string
	switch lang {
	case "uk":
		if count == 1 {
			label = "Є ще 1 варіант"
		} else if count >= 2 && count <= 4 {
			label = "Є ще " + strconv.Itoa(count) + " варіанти"
		} else {
			label = "Є ще " + strconv.Itoa(count) + " варіантів"
		}
	case "en":
		if count == 1 {
			label = "1 more result"
		} else {
			label = strconv.Itoa(count) + " more results"
		}
	default: // ru
		if count == 1 {
			label = "Есть ещё 1 вариант"
		} else if count >= 2 && count <= 4 {
			label = "Есть ещё " + strconv.Itoa(count) + " варианта"
		} else {
			label = "Есть ещё " + strconv.Itoa(count) + " вариантов"
		}
	}

	// Ссылка на главную с предзаполненным поиском
	dupURL := "/?lang=" + lang + "&query=" + url.QueryEscape(displayName)
	return count, label, dupURL
}

func detectLang(r *http.Request) string {
	lang := strings.ToLower(r.URL.Query().Get("lang"))
	switch lang {
	case "en", "uk", "ru":
		return lang
	default:
		return "ru"
	}
}

func texts(lang string) TextSet {
	switch lang {
	case "en":
		return TextSet{
			BrandTop:           "LIVE",
			BrandBottom:        "WEATHER",
			NavNow:             "Now",
			NavCities:          "Cities",
			BadgeNow:           "NOW",
			CitiesLabel:        "Cities",
			CitiesTitle:        "Weather in Ukraine",
			CityBadge:          "CITY",
			MetricWind:         "Wind",
			MetricHumidity:     "Humidity",
			MetricPressure:     "Pressure",
			Today:              "Today",
			Tomorrow:           "Tomorrow",
			RangeFrom:          "from",
			RangeTo:            "to",
			CurrentMetricsAria: "Current conditions",
			CurrentWeatherAria: "Current weather",
			ShortForecastAria:  "Short forecast",
			ForecastLabel:      "Forecast",
			ForecastTitle:      "Next 3 days",
			ForecastAria:       "3‑day forecast",
			TrendLabel:         "Trend",
			ChartTitle:         "Temperature chart",
			ChartAria:          "Temperature changes over 3 days",
			OtherCitiesAria:    "Other cities",
			QuickSwitchTitle:   "Quick switch",
			SearchPlaceholder:     "Type a settlement…",
			Footer:                "Data: Open‑Meteo · Updated every 10 minutes",
			MoreLink:              "Details",
			WeatherUnavailableMsg: "Data temporarily unavailable",
		}
	case "uk":
		return TextSet{
			BrandTop:           "ЖИВА",
			BrandBottom:        "ПОГОДА",
			NavNow:             "Зараз",
			NavCities:          "Міста",
			BadgeNow:           "ЗАРАЗ",
			CitiesLabel:        "Міста",
			CitiesTitle:        "Погода по Україні",
			CityBadge:          "МІСТО",
			MetricWind:         "Вітер",
			MetricHumidity:     "Вологість",
			MetricPressure:     "Тиск",
			Today:              "Сьогодні",
			Tomorrow:           "Завтра",
			RangeFrom:          "від",
			RangeTo:            "до",
			CurrentMetricsAria: "Поточні показники",
			CurrentWeatherAria: "Поточна погода",
			ShortForecastAria:  "Короткий прогноз",
			ForecastLabel:      "Прогноз",
			ForecastTitle:      "3 дні наперед",
			ForecastAria:       "Прогноз на 3 дні",
			TrendLabel:         "Тренд",
			ChartTitle:         "Графік температури",
			ChartAria:          "Зміна температури за 3 дні",
			OtherCitiesAria:    "Інші міста",
			QuickSwitchTitle:   "Швидкий перехід",
			SearchPlaceholder:     "Введи населений пункт…",
			Footer:                "Дані: Open‑Meteo · Оновлення кожні 10 хвилин",
			MoreLink:              "Докладніше",
			WeatherUnavailableMsg: "Дані тимчасово недоступні",
		}
	default: // ru
		return TextSet{
			BrandTop:           "ЖИВАЯ",
			BrandBottom:        "ПОГОДА",
			NavNow:             "Сейчас",
			NavCities:          "Города",
			BadgeNow:           "СЕЙЧАС",
			CitiesLabel:        "Города",
			CitiesTitle:        "Погода по Украине",
			CityBadge:          "ГОРОД",
			MetricWind:         "Ветер",
			MetricHumidity:     "Влажность",
			MetricPressure:     "Давление",
			Today:              "Сегодня",
			Tomorrow:           "Завтра",
			RangeFrom:          "от",
			RangeTo:            "до",
			CurrentMetricsAria: "Текущие показатели",
			CurrentWeatherAria: "Текущая погода",
			ShortForecastAria:  "Краткий прогноз",
			ForecastLabel:      "Прогноз",
			ForecastTitle:      "3 дня вперёд",
			ForecastAria:       "Прогноз на 3 дня",
			TrendLabel:         "Тренд",
			ChartTitle:         "График температуры",
			ChartAria:          "График изменения температуры за 3 дня",
			OtherCitiesAria:    "Другие города",
			QuickSwitchTitle:   "Быстрый переход",
			SearchPlaceholder:     "Введи населённый пункт…",
			Footer:                "Данные: Open‑Meteo · Обновление каждые 10 минут",
			MoreLink:              "Подробнее",
			WeatherUnavailableMsg: "Данные временно недоступны",
		}
	}
}

func trendText(lang string, diff float64) string {
	switch lang {
	case "en":
		switch {
		case diff > 4:
			return "A noticeable warm‑up is expected tomorrow."
		case diff > 2:
			return "Tomorrow will be a bit warmer than today."
		case diff < -4:
			return "A significant cool‑down is expected tomorrow."
		case diff < -2:
			return "Tomorrow will feel slightly cooler than today."
		default:
			return "Temperature will stay roughly the same in the coming days."
		}
	case "uk":
		switch {
		case diff > 4:
			return "Завтра очікується відчутне потепління."
		case diff > 2:
			return "Завтра буде трохи тепліше, ніж сьогодні."
		case diff < -4:
			return "Завтра очікується помітне похолодання."
		case diff < -2:
			return "Завтра буде трохи прохолодніше, ніж сьогодні."
		default:
			return "У найближчі дні температура буде приблизно на одному рівні."
		}
	default: // ru
		switch {
		case diff > 4:
			return "Завтра ожидается заметное потепление."
		case diff > 2:
			return "Завтра станет немного теплее, чем сегодня."
		case diff < -4:
			return "Завтра ожидается ощутимое похолодание."
		case diff < -2:
			return "Завтра будет чуть прохладнее, чем сегодня."
		default:
			return "Температура в ближайшие дни будет примерно на одном уровне."
		}
	}
}

