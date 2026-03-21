package i18n

import "encoding/json"

// ClientPayload is serialized to JSON for static/script.js (search, chart, favourites).
type ClientPayload struct {
	SearchEmpty       string `json:"searchEmpty"`
	SearchError       string `json:"searchError"`
	SearchTooShort    string `json:"searchTooShort"`
	WindSuffix        string `json:"windSuffix"`
	HumiditySuffix    string `json:"humiditySuffix"`
	ChartMax          string `json:"chartMax"`
	ChartMin          string `json:"chartMin"`
	FavRemoveTitle    string `json:"favRemoveTitle"`
	FavAddAria        string `json:"favAddAria"`
	PlaceTypeFallback string `json:"placeTypeFallback"`
}

func clientPayload(lang string) ClientPayload {
	switch Normalize(lang) {
	case "en":
		return ClientPayload{
			SearchEmpty:        "No results",
			SearchError:        "Search failed",
			SearchTooShort:     "Type at least 2 characters",
			WindSuffix:         " km/h",
			HumiditySuffix:     "%",
			ChartMax:           "High",
			ChartMin:           "Low",
			FavRemoveTitle:     "Remove from favourites",
			FavAddAria:         "Add to favourites",
			PlaceTypeFallback:  "Settlement",
		}
	case "uk":
		return ClientPayload{
			SearchEmpty:        "Нічого не знайдено",
			SearchError:        "Помилка пошуку",
			SearchTooShort:     "Введи мінімум 2 символи",
			WindSuffix:         " км/год",
			HumiditySuffix:     "%",
			ChartMax:           "Макс",
			ChartMin:           "Мін",
			FavRemoveTitle:     "Прибрати з обраного",
			FavAddAria:         "Додати до обраного",
			PlaceTypeFallback:  "населений пункт",
		}
	default:
		return ClientPayload{
			SearchEmpty:        "Ничего не найдено",
			SearchError:        "Ошибка поиска",
			SearchTooShort:     "Введите минимум 2 символа",
			WindSuffix:         " км/ч",
			HumiditySuffix:     "%",
			ChartMax:           "Макс",
			ChartMin:           "Мин",
			FavRemoveTitle:     "Удалить из избранного",
			FavAddAria:         "В избранное",
			PlaceTypeFallback:  "населённый пункт",
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
