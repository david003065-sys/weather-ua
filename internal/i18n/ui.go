package i18n

import "time"

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
	HeaderMenuAria          string // mobile: opens theme + language panel
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

	AppName          string
	SmartAdviceTitle string
	FeelsLike        string
	Pressure         string
	UVIndex          string
	Visibility       string
	Sunrise          string
	Sunset           string
	SunTimesAria     string // aria-label for sunrise/sunset row
	ShareWeather     string // city page: share action (verbose aria, optional)
	ShareButton      string // city page: share label (matches client JSON shareButton)

	NotFoundTitle    string
	NotFoundSubtitle string
	NotFoundDesc     string
	NotFoundHome     string
	NotFoundSearch   string
	SarcasticAdvice  []string
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
			ForecastTitle:           "Next 3 days",
			ForecastAria:            "3-day forecast",
			TrendLabel:              "Trend",
			ChartTitle:              "Temperature chart",
			ChartAria:               "Temperature changes over 3 days",
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
			HeaderMenuAria:          "Theme and language",
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
			AppName:                 "Live Weather",
			SmartAdviceTitle:        "Clothing tips",
			FeelsLike:               "Feels like",
			Pressure:                "Pressure",
			UVIndex:                 "UV index",
			Visibility:              "Visibility",
			Sunrise:                 "Sunrise",
			Sunset:                  "Sunset",
			SunTimesAria:            "Sunrise and sunset",
			ShareWeather:            "Share weather",
			ShareButton:             "Share",
			NotFoundTitle:           "404",
			NotFoundSubtitle:        "City not found",
			NotFoundDesc:            "Try searching or go back to home",
			NotFoundHome:            "Home",
			NotFoundSearch:          "Search",
			SarcasticAdvice: []string{
				"Perfect day to cancel all plans and pretend you're not home.",
				"Going outside today is as big a mistake as many of your life decisions.",
				"Precipitation expected in the form of your tears over missed opportunities.",
				"The sun shines so bright it almost hides your depression.",
				"Ideal weather conditions for a quality existential crisis.",
				"Today's weather matches your mood: unpredictable and disappointing.",
				"At least the weather doesn't judge you like your mother does.",
				"Perfect weather to stay inside and question all your life choices.",
				"The forecast calls for 100% chance of regret.",
				"Today's weather: cold, gloomy, and emotionally unavailable.",
				"Great day to contemplate the meaningless void of existence.",
				"Weather so bad even your therapist would cancel the session.",
				"At least it's not raining inside your apartment. Yet.",
				"Today's forecast: cloudy with a chance of existential dread.",
				"Perfect weather for staying in bed and avoiding responsibilities.",
			},
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
			ForecastTitle:           "Наступні 3 дні",
			ForecastAria:            "Прогноз на 3 дні",
			TrendLabel:              "Тренд",
			ChartTitle:              "Графік температури",
			ChartAria:               "Зміна температури за 3 дні",
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
			HeaderMenuAria:          "Тема та мова",
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
			AppName:                 "Жива погода",
			SmartAdviceTitle:        "Поради щодо одягу",
			FeelsLike:               "Відчувається",
			Pressure:                "Тиск",
			UVIndex:                 "УФ-індекс",
			Visibility:              "Видимість",
			Sunrise:                 "Схід",
			Sunset:                  "Захід",
			SunTimesAria:            "Схід і захід сонця",
			ShareWeather:            "Поділитися погодою",
			ShareButton:             "Поділитися",
			NotFoundTitle:           "404",
			NotFoundSubtitle:        "Місто не знайдено",
			NotFoundDesc:            "Спробуйте пошук або поверніться на головну",
			NotFoundHome:            "На головну",
			NotFoundSearch:          "Пошук",
			SarcasticAdvice: []string{
				"Чудовий день, щоб скасувати всі плани і робити вигляд, що тебе немає вдома.",
				"Виходити на вулицю сьогодні — така ж помилка, як і багато твоїх життєвих рішень.",
				"Очікуються опади у вигляді твоїх сліз по пропущених можливостях.",
				"Сонце світить так яскраво, що майже приховує твою депресію.",
				"Ідеальні погодні умови для якісного екзистенційного кризису.",
				"Погода сьогодні збігається з твоїм настроєм: непередбачувана і розчаровуюча.",
				"Щоправда, погода не засуджує тебе так, як твоя мама.",
				"Чудова погода, щоб сидіти вдома і сумніватися у всіх життєвих виборах.",
				"Прогноз обіцяє 100% шансів на жаль.",
				"Погода сьогодні: холодна, похмура і емоційно недоступна.",
				"Чудовий день, щоб роздумувати про безглузду порожнечу буття.",
				"Погода настільки погана, що навіть твій психотерапевт скасував би сесію.",
				"Щоправда, поки що не дощить у твоїй квартирі. Ще.",
				"Прогноз на сьогодні: хмарно з можливістю екзистенційного жаху.",
				"Чудова погода, щоб лежати в ліжку і уникати відповідальності.",
			},
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
			ForecastTitle:           "3 дня вперёд",
			ForecastAria:            "Прогноз на 3 дня",
			TrendLabel:              "Тренд",
			ChartTitle:              "График температуры",
			ChartAria:               "Изменение температуры за 3 дня",
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
			HeaderMenuAria:          "Тема и язык",
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
			AppName:                 "Живая погода",
			SmartAdviceTitle:        "Советы по одежде",
			FeelsLike:               "Ощущается",
			Pressure:                "Давление",
			UVIndex:                 "УФ-индекс",
			Visibility:              "Видимость",
			Sunrise:                 "Рассвет",
			Sunset:                  "Закат",
			SunTimesAria:            "Восход и закат",
			ShareWeather:            "Поделиться погодой",
			ShareButton:             "Поделиться",
			NotFoundTitle:           "404",
			NotFoundSubtitle:        "Город не найден",
			NotFoundDesc:            "Попробуйте поиск или вернитесь на главную",
			NotFoundHome:            "На главную",
			NotFoundSearch:          "Поиск",
			SarcasticAdvice: []string{
				"Отличный день, чтобы отменить все планы и сделать вид, что тебя нет дома.",
				"Выходить на улицу сегодня — это такая же ошибка, как и многие твои жизненные решения.",
				"Ожидаются осадки в виде твоих слез по упущенным возможностям.",
				"Солнце светит так ярко, что почти скрывает твою депрессию.",
				"Идеальные погодные условия для качественного экзистенциального кризиса.",
				"Погода сегодня совпадает с твоим настроем: непредсказуемая и разочаровывающая.",
				"Зато погода не осуждает тебя так, как твоя мама.",
				"Отличная погода, чтобы сидеть дома и сомневаться во всех жизненных выборах.",
				"Прогноз обещает 100% шанс сожаления.",
				"Погода сегодня: холодная, унылая и эмоционально недоступная.",
				"Чудный день, чтобы размышлять о бессмысленной пустоте бытия.",
				"Погода настолько плохая, что даже твой психотерапевт отменил бы сеанс.",
				"Зато пока что не льёт у тебя в квартире дождь. Пока.",
				"Прогноз на сегодня: облачно с возможностью экзистенциального ужаса.",
				"Отличная погода, чтобы лежать в кровати и избегать ответственности.",
			},
		}
	}
}

// GetRandomAdvice returns a random sarcastic advice phrase from the UI bundle.
// Returns SmartAdviceTitle if SarcasticAdvice is empty or unavailable.
func (ui UI) GetRandomAdvice() string {
	if len(ui.SarcasticAdvice) == 0 {
		return ui.SmartAdviceTitle
	}
	// Use math/rand for simplicity (not crypto-secure needed for UI text)
	n := len(ui.SarcasticAdvice)
	return ui.SarcasticAdvice[time.Now().UnixNano()%int64(n)]
}
