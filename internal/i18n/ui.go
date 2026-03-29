package i18n

// UI is the full SSR string bundle for one language (templates + meta).
type UI struct {
	BrandTop              string
	BrandBottom           string
	NavNow                string
	NavCities             string
	BadgeNow              string
	CitiesLabel           string
	CitiesTitle           string
	CityBadge             string
	MetricWind            string
	MetricHumidity        string
	MetricPressure        string
	Today                 string
	Tomorrow              string
	DayAfterTomorrow      string
	RangeFrom             string
	RangeTo               string
	CurrentMetricsAria    string
	CurrentWeatherAria    string
	ShortForecastAria     string
	ForecastLabel         string
	ForecastTitle         string
	ForecastAria          string
	TrendLabel            string
	ChartTitle            string
	ChartAria             string
	OtherCitiesAria       string
	QuickSwitchTitle      string
	SearchPlaceholder     string
	Footer                string
	MoreLink              string
	WeatherUnavailableMsg string

	// Units (value is rendered separately, e.g. "{{.Wind}}{{.Text.UnitWind}}")
	UnitWind     string
	UnitPressure string
	UnitHumidity string // usually "%"

	// Extra UI (formerly template conditionals)
	ThemeAuto               string
	ThemeLight              string
	ThemeDark               string
	ThemeSwitchAria         string
	LangSwitchAria          string
	NavAria                 string
	BrandHomeAria           string
	GeoDetectButton         string
	SearchSectionAria       string
	SuggestionsAria         string
	SunnyIconTitle          string
	CitiesToggleMore        string
	CitiesToggleLess        string
	CitiesToggleShowMoreN   string // "Show %d more" — %d replaced in JS
	RecentSectionLabel      string
	RecentSectionTitle      string
	FavoritesSectionLabel   string
	FavoritesSectionTitle   string
	FavoritesRailLabel      string // блок избранных на главной
	FavoritesRailTitle      string
	FavoritesRailEmpty      string
	ChooseAnotherDuplicate  string
	HourlySectionTitle      string
	MetaDescriptionTemplate string // may contain %s for city list
}

// For returns all UI strings for SSR.
func For(lang string) UI {
	switch Normalize(lang) {
	case "en":
		return UI{
			BrandTop:                "METEO",
			BrandBottom:             "UA",
			NavNow:                  "Now",
			NavCities:               "Cities",
			BadgeNow:                "NOW",
			CitiesLabel:             "Cities",
			CitiesTitle:             "Weather in Ukraine",
			CityBadge:               "CITY",
			MetricWind:              "Wind",
			MetricHumidity:          "Humidity",
			MetricPressure:          "Pressure",
			Today:                   "Today",
			Tomorrow:                "Tomorrow",
			DayAfterTomorrow:        "Day after",
			RangeFrom:               "from",
			RangeTo:                 "to",
			CurrentMetricsAria:      "Current conditions",
			CurrentWeatherAria:      "Current weather",
			ShortForecastAria:       "Short forecast",
			ForecastLabel:           "Forecast",
			ForecastTitle:           "Next 5 days",
			ForecastAria:            "5-day forecast",
			TrendLabel:              "Trend",
			ChartTitle:              "Temperature chart",
			ChartAria:               "Temperature changes over 5 days",
			OtherCitiesAria:         "Other cities",
			QuickSwitchTitle:        "Quick switch",
			SearchPlaceholder:       "Search for a place…",
			Footer:                  "Weather data · Updated every 30 minutes",
			MoreLink:                "Details",
			WeatherUnavailableMsg:   "Data temporarily unavailable",
			UnitWind:                " km/h",
			UnitPressure:            " mmHg",
			UnitHumidity:            "%",
			ThemeAuto:               "Auto",
			ThemeLight:              "Light",
			ThemeDark:               "Dark",
			ThemeSwitchAria:         "Theme mode",
			LangSwitchAria:          "Language",
			NavAria:                 "Main navigation",
			BrandHomeAria:           "MeteoUA — home",
			GeoDetectButton:         "Detect my location",
			SearchSectionAria:       "Search places",
			SuggestionsAria:         "Search suggestions",
			SunnyIconTitle:          "Clear sky",
			CitiesToggleMore:        "More",
			CitiesToggleLess:        "Show less",
			CitiesToggleShowMoreN:   "Show %d more",
			RecentSectionLabel:      "History",
			RecentSectionTitle:      "Recent places",
			FavoritesSectionLabel:   "Places",
			FavoritesSectionTitle:   "Favourites",
			FavoritesRailLabel:      "Favourite cities",
			FavoritesRailTitle:      "Your locations",
			FavoritesRailEmpty:      "You have not added any places yet. Search for a settlement and tap the star.",
			ChooseAnotherDuplicate:  "Choose another",
			HourlySectionTitle:      "Today — hourly",
			MetaDescriptionTemplate: "Weather in Ukraine: %s.",
		}
	case "uk":
		return UI{
			BrandTop:                "METEO",
			BrandBottom:             "UA",
			NavNow:                  "Зараз",
			NavCities:               "Міста",
			BadgeNow:                "ЗАРАЗ",
			CitiesLabel:             "Міста",
			CitiesTitle:             "Погода в Україні",
			CityBadge:               "МІСТО",
			MetricWind:              "Вітер",
			MetricHumidity:          "Вологість",
			MetricPressure:          "Тиск",
			Today:                   "Сьогодні",
			Tomorrow:                "Завтра",
			DayAfterTomorrow:        "Післязавтра",
			RangeFrom:               "від",
			RangeTo:                 "до",
			CurrentMetricsAria:      "Поточні показники",
			CurrentWeatherAria:      "Поточна погода",
			ShortForecastAria:       "Короткий прогноз",
			ForecastLabel:           "Прогноз",
			ForecastTitle:           "Наступні 5 днів",
			ForecastAria:            "Прогноз на 5 днів",
			TrendLabel:              "Тренд",
			ChartTitle:              "Графік температури",
			ChartAria:               "Зміна температури за 5 днів",
			OtherCitiesAria:         "Інші міста",
			QuickSwitchTitle:        "Швидкий перехід",
			SearchPlaceholder:       "Введи населений пункт…",
			Footer:                  "Дані про погоду · Оновлення кожні 30 хвилин",
			MoreLink:                "Докладніше",
			WeatherUnavailableMsg:   "Дані тимчасово недоступні",
			UnitWind:                " км/год",
			UnitPressure:            " мм рт. ст.",
			UnitHumidity:            "%",
			ThemeAuto:               "Авто",
			ThemeLight:              "Світла",
			ThemeDark:               "Темна",
			ThemeSwitchAria:         "Режим теми",
			LangSwitchAria:          "Мова інтерфейсу",
			NavAria:                 "Головна навігація",
			BrandHomeAria:           "MeteoUA — на головну",
			GeoDetectButton:         "Визначити моє місцезнаходження",
			SearchSectionAria:       "Пошук населеного пункту",
			SuggestionsAria:         "Підказки населених пунктів",
			SunnyIconTitle:          "Ясно",
			CitiesToggleMore:        "Ще",
			CitiesToggleLess:        "Згорнути",
			CitiesToggleShowMoreN:   "Показати ще %d",
			RecentSectionLabel:      "Історія",
			RecentSectionTitle:      "Недавні місця",
			FavoritesSectionLabel:   "Місця",
			FavoritesSectionTitle:   "Обрані",
			FavoritesRailLabel:      "Обрані міста",
			FavoritesRailTitle:      "Ваш список локацій",
			FavoritesRailEmpty:      "Ти ще не додав(ла) міста. Знайди населений пункт через пошук і натисни зірочку.",
			ChooseAnotherDuplicate:  "Обрати інший",
			HourlySectionTitle:      "Сьогодні — погодинно",
			MetaDescriptionTemplate: "Погода в Україні: %s.",
		}
	default: // ru
		return UI{
			BrandTop:                "METEO",
			BrandBottom:             "UA",
			NavNow:                  "Сейчас",
			NavCities:               "Города",
			BadgeNow:                "СЕЙЧАС",
			CitiesLabel:             "Города",
			CitiesTitle:             "Погода в Украине",
			CityBadge:               "ГОРОД",
			MetricWind:              "Ветер",
			MetricHumidity:          "Влажность",
			MetricPressure:          "Давление",
			Today:                   "Сегодня",
			Tomorrow:                "Завтра",
			DayAfterTomorrow:        "Послезавтра",
			RangeFrom:               "от",
			RangeTo:                 "до",
			CurrentMetricsAria:      "Текущие показатели",
			CurrentWeatherAria:      "Текущая погода",
			ShortForecastAria:       "Краткий прогноз",
			ForecastLabel:           "Прогноз",
			ForecastTitle:           "5 дней вперёд",
			ForecastAria:            "Прогноз на 5 дней",
			TrendLabel:              "Тренд",
			ChartTitle:              "График температуры",
			ChartAria:               "Изменение температуры за 5 дней",
			OtherCitiesAria:         "Другие города",
			QuickSwitchTitle:        "Быстрый переход",
			SearchPlaceholder:       "Введи населённый пункт…",
			Footer:                  "Данные о погоде · Обновление каждые 30 минут",
			MoreLink:                "Подробнее",
			WeatherUnavailableMsg:   "Данные временно недоступны",
			UnitWind:                " км/ч",
			UnitPressure:            " мм рт. ст.",
			UnitHumidity:            "%",
			ThemeAuto:               "Авто",
			ThemeLight:              "Светлая",
			ThemeDark:               "Тёмная",
			ThemeSwitchAria:         "Режим темы",
			LangSwitchAria:          "Выбор языка",
			NavAria:                 "Основная навигация",
			BrandHomeAria:           "MeteoUA — на главную",
			GeoDetectButton:         "Определить моё местоположение",
			SearchSectionAria:       "Поиск населённого пункта",
			SuggestionsAria:         "Подсказки населённых пунктов",
			SunnyIconTitle:          "Ясно",
			CitiesToggleMore:        "Ещё",
			CitiesToggleLess:        "Скрыть",
			CitiesToggleShowMoreN:   "Показать ещё %d",
			RecentSectionLabel:      "История",
			RecentSectionTitle:      "Недавние места",
			FavoritesSectionLabel:   "Места",
			FavoritesSectionTitle:   "Избранное",
			FavoritesRailLabel:      "Избранные города",
			FavoritesRailTitle:      "Ваш список локаций",
			FavoritesRailEmpty:      "Вы ещё не добавили города. Найдите город через поиск и нажмите на звёздочку.",
			ChooseAnotherDuplicate:  "Выбрать другой",
			HourlySectionTitle:      "Сегодня — по часам",
			MetaDescriptionTemplate: "Погода в Украине: %s.",
		}
	}
}
