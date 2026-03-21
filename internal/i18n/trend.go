package i18n

// TrendSentence describes tomorrow vs today max temperature trend.
func TrendSentence(lang string, diff float64) string {
	switch Normalize(lang) {
	case "en":
		switch {
		case diff > 4:
			return "A noticeable warm-up is expected tomorrow."
		case diff > 2:
			return "Tomorrow will be a bit warmer than today."
		case diff < -4:
			return "A significant cool-down is expected tomorrow."
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
