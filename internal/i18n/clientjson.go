package i18n

import "encoding/json"

// ClientPayload is serialized to JSON for static/script.js (search, chart, favourites).
type ClientPayload struct {
	SearchEmpty    string `json:"searchEmpty"`
	SearchError    string `json:"searchError"`
	SearchTooShort string `json:"searchTooShort"`
	SWDataUpdated  string `json:"swDataUpdated"`
	WindUnit       string `json:"windUnit"`
	HumiditySuffix string `json:"humiditySuffix"`
	PressureUnit   string `json:"pressureUnit"`
	UVLow          string `json:"uvLow"`
	UVMedium       string `json:"uvMedium"`
	UVHigh         string `json:"uvHigh"`
	// SmartAdvice: [0–4] base text by temp band (<0, <10, <18, <25, else); [5] wind extra; [6] humidity; [7] rain.
	SmartAdvice       []string `json:"smartAdvice"`
	ChartMax          string   `json:"chartMax"`
	ChartMin          string   `json:"chartMin"`
	FavRemoveTitle    string   `json:"favRemoveTitle"`
	FavAddAria        string   `json:"favAddAria"`
	PlaceTypeFallback string   `json:"placeTypeFallback"`
	GeoNoBrowser      string   `json:"geoNoBrowserSupport"`
	GeoLocationFailed string   `json:"geoLocationFailed"`
	ShareTitleFmt     string   `json:"shareTitleFmt"`
	ShareTextFmt      string   `json:"shareTextFmt"`
	ShareCopied       string   `json:"shareCopied"`
	ShareButton       string   `json:"shareButton"`
}

func clientPayload(lang string) ClientPayload {
	switch Normalize(lang) {
	case "en":
		return ClientPayload{
			SearchEmpty:    "No results",
			SearchError:    "Search failed",
			SearchTooShort: "Type at least 2 characters",
			SWDataUpdated:  "Data updated",
			WindUnit:       "km/h",
			HumiditySuffix: "%",
			PressureUnit:   "mmHg",
			UVLow:          "Low",
			UVMedium:       "Moderate",
			UVHigh:         "High",
			SmartAdvice: []string{
				"Bundle up! A puffer, scarf, and gloves are a must.",
				"Chilly. A coat or warm jacket is perfect.",
				"Cool. A windbreaker or a warm hoodie works well.",
				"Comfortable! A T-shirt and a light layer for the evening.",
				"Hot! Wear light cotton clothes and drink more water.",
				"...but watch the wind — it's sharp today.",
				"High humidity will make it feel colder than it is.",
				"Don't forget an umbrella — you'll need it today!",
			},
			ChartMax:          "High",
			ChartMin:          "Low",
			FavRemoveTitle:    "Remove from favourites",
			FavAddAria:        "Add to favourites",
			PlaceTypeFallback: "Settlement",
			GeoNoBrowser:      "This browser does not support geolocation.",
			GeoLocationFailed: "Could not get your position. Check site permissions.",
			ShareTitleFmt:     "Weather in {city} — {temp}°C, {desc}",
			ShareTextFmt:      "{temp}°, {desc}. Today: {d0}. Tomorrow: {d1}.",
			ShareCopied:       "Link copied",
			ShareButton:       "Share",
		}
	case "uk":
		return ClientPayload{
			SearchEmpty:    "Нічого не знайдено",
			SearchError:    "Помилка пошуку",
			SearchTooShort: "Введи мінімум 2 символи",
			SWDataUpdated:  "Дані оновлено",
			WindUnit:       "км/год",
			HumiditySuffix: "%",
			PressureUnit:   "мм рт. ст.",
			UVLow:          "Низький",
			UVMedium:       "Помірний",
			UVHigh:         "Високий",
			SmartAdvice: []string{
				"Вдягайся максимально тепло! Пуховик, шарф і рукавички обов’язкові.",
				"Прохолодно. Пальто або тепла куртка — саме те.",
				"Свіжо. Вітровка або щільне худі — ідеальний вибір.",
				"Комфортно! Футболка й легкий светр на вечір.",
				"Спека! Легкий одяг із бавовни й пий більше води.",
				"...але остерігайся вітра, сьогодні він пронизливий.",
				"Висока вологість, здаватиметься холодніше, ніж є.",
				"І не забудь парасольку — сьогодні без неї ніяк!",
			},
			ChartMax:          "Макс",
			ChartMin:          "Мін",
			FavRemoveTitle:    "Прибрати з обраного",
			FavAddAria:        "Додати до обраного",
			PlaceTypeFallback: "населений пункт",
			GeoNoBrowser:      "Ваш браузер не підтримує геолокацію.",
			GeoLocationFailed: "Не вдалося отримати координати. Перевір дозволи сайту.",
			ShareTitleFmt:     "Погода в {city} — {temp}°C, {desc}",
			ShareTextFmt:      "{temp}°, {desc}. Сьогодні: {d0}. Завтра: {d1}.",
			ShareCopied:       "Посилання скопійовано",
			ShareButton:       "Поділитися",
		}
	default:
		return ClientPayload{
			SearchEmpty:    "Ничего не найдено",
			SearchError:    "Ошибка поиска",
			SearchTooShort: "Введите минимум 2 символа",
			SWDataUpdated:  "Данные обновлены",
			WindUnit:       "км/ч",
			HumiditySuffix: "%",
			PressureUnit:   "мм рт. ст.",
			UVLow:          "Низкий",
			UVMedium:       "Средний",
			UVHigh:         "Высокий",
			SmartAdvice: []string{
				"Одевайся максимально тепло! Пуховик, шарф и перчатки обязательны.",
				"Прохладно. Пальто или тёплая куртка будут в самый раз.",
				"Свежо. Ветровка или плотное худи — идеальный выбор.",
				"Комфортно! Футболка и лёгкая кофта на вечер.",
				"Жара! Выбирай лёгкую одежду из хлопка и пей больше воды.",
				"...но берегись ветра, он сегодня кусачий.",
				"Влажность высокая, будет казаться холоднее, чем есть.",
				"И не забудь зонт — сегодня без него никак!",
			},
			ChartMax:          "Макс",
			ChartMin:          "Мин",
			FavRemoveTitle:    "Удалить из избранного",
			FavAddAria:        "В избранное",
			PlaceTypeFallback: "населённый пункт",
			GeoNoBrowser:      "Ваш браузер не поддерживает геолокацию.",
			GeoLocationFailed: "Не удалось получить координаты. Проверьте разрешения.",
			ShareTitleFmt:     "Погода в {city} — {temp}°C, {desc}",
			ShareTextFmt:      "{temp}°, {desc}. Сегодня: {d0}. Завтра: {d1}.",
			ShareCopied:       "Ссылка скопирована",
			ShareButton:       "Поделиться",
		}
	}
}

// MarshalClientJSON returns UTF-8 JSON for embedding in <script type="application/json">.
func MarshalClientJSON(lang string) []byte {
	p := clientPayload(lang)
	b, err := json.Marshal(p)
	if err != nil {
		return []byte("{}")
	}
	return b
}
