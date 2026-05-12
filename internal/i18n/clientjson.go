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
				"Perfect day to cancel all plans and pretend you're not home.",
				"Going outside today is as big a mistake as many of your life decisions.",
				"Precipitation expected in the form of your tears over missed opportunities.",
				"The sun shines so bright it almost hides your depression.",
				"Ideal weather conditions for a quality existential crisis.",
				"Today's weather matches your mood: unpredictable and disappointing.",
				"At least the weather doesn't judge you like your mother does.",
				"Perfect weather to stay inside and question all your life choices.",
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
				"Чудовий день, щоб скасувати всі плани і робити вигляд, що тебе немає вдома.",
				"Виходити на вулицю сьогодні — така ж помилка, як і багато твоїх життєвих рішень.",
				"Очікуються опади у вигляді твоїх сліз по пропущених можливостях.",
				"Сонце світить так яскраво, що майже приховує твою депресію.",
				"Ідеальні погодні умови для якісного екзистенційного кризису.",
				"Погода сьогодні збігається з твоїм настроєм: непередбачувана і розчаровуюча.",
				"Щоправда, погода не засуджує тебе так, як твоя мама.",
				"Чудова погода, щоб сидіти вдома і сумніватися у всіх життєвих виборах.",
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
				"Чудовий день, щоб скасувати всі плани і робити вигляд, що тебе немає вдома.",
				"Виходити на вулицю сьогодні — така ж помилка, як і багато твоїх життєвих рішень.",
				"Очікуються опади у вигляді твоїх сліз по пропущених можливостях.",
				"Сонце світить так яскраво, що майже приховує твою депресію.",
				"Ідеальні погодні умови для якісного екзистенційного кризису.",
				"Погода сьогодні збігається з твоїм настроєм: непередбачувана і розчаровуюча.",
				"Щоправда, погода не засуджує тебе так, як твоя мама.",
				"Чудова погода, щоб сидіти вдома і сумніватися у всіх життєвих виборах.",
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
