(function () {
    var NIGHT_FALLBACK_START = 20; // 20:00
    var NIGHT_FALLBACK_END = 6;    // 06:00
    var THEME_KEY = "weather:themeMode";
    var payload = null;
    var autoTimerId = null;

    /* Apply stored light/dark before paint (html data-theme + body class). */
    try {
        var storedEarly = localStorage.getItem(THEME_KEY);
        var root = document.documentElement;
        if (storedEarly === "light") {
            root.setAttribute("data-theme", "light");
        } else if (storedEarly === "dark") {
            root.setAttribute("data-theme", "dark");
        }
        var metaEarly = document.querySelector('meta[name="theme-color"]');
        if (metaEarly && (storedEarly === "light" || storedEarly === "dark")) {
            metaEarly.setAttribute("content", storedEarly === "light" ? "#f8fafc" : "#0f172a");
        }
        /* auto / missing: keep <html data-theme="dark"> from template */
        var bodyEarly = document.body;
        if (bodyEarly) {
            if (storedEarly === "light") {
                bodyEarly.classList.remove("theme-dark");
                bodyEarly.classList.add("theme-light");
            } else if (storedEarly === "dark") {
                bodyEarly.classList.remove("theme-light");
                bodyEarly.classList.add("theme-dark");
            }
        }
    } catch (_) {}

    function readPayload() {
        if (payload !== null) return payload;
        var el = document.getElementById("__WEATHER__");
        var raw = el && el.textContent && el.textContent.trim();
        if (!raw) {
            payload = {};
            return payload;
        }
        try {
            payload = JSON.parse(raw);
        } catch (e) {
            payload = {};
        }
        return payload;
    }

    function getThemeMode() {
        try {
            var v = localStorage.getItem(THEME_KEY);
            if (v === "light" || v === "dark" || v === "auto") return v;
        } catch (_) {}
        /* First visit: polished dark theme; Auto/Light only after explicit choice */
        return "dark";
    }

    function saveThemeMode(mode) {
        try {
            localStorage.setItem(THEME_KEY, mode);
        } catch (_) {}
    }

    function setBodyThemeClass(theme) {
        var body = document.body;
        var root = document.documentElement;
        var isDark = theme === "dark";
        if (root) {
            root.setAttribute("data-theme", isDark ? "dark" : "light");
        }
        var themeMeta = document.querySelector('meta[name="theme-color"]');
        if (themeMeta) {
            themeMeta.setAttribute("content", isDark ? "#0f172a" : "#f8fafc");
        }
        try {
            window.dispatchEvent(new CustomEvent("weather-theme-change"));
        } catch (_) {}
        if (!body) return;
        body.classList.remove("theme-light", "theme-dark");
        body.classList.add(isDark ? "theme-dark" : "theme-light");
    }

    function computeCityNow(meta) {
        var now = new Date();
        var offsetSeconds = meta && typeof meta.offsetSeconds === "number" ? meta.offsetSeconds : null;
        if (offsetSeconds == null) {
            return now;
        }
        // current UTC time in ms
        var utcMs = now.getTime() + now.getTimezoneOffset() * 60000;
        var cityMs = utcMs + offsetSeconds * 1000;
        return new Date(cityMs);
    }

    function applyAutoTheme() {
        var data = readPayload();
        var sun = data && data.sun ? data.sun : {};
        var meta = data && data.meta ? data.meta : {};

        var sunriseMinutes = typeof sun.sunriseMinutes === "number" ? sun.sunriseMinutes : null;
        var sunsetMinutes = typeof sun.sunsetMinutes === "number" ? sun.sunsetMinutes : null;

        var cityNow = computeCityNow(meta);
        var minutesNow = cityNow.getHours() * 60 + cityNow.getMinutes();
        var isNight;

        if (sunriseMinutes != null && sunsetMinutes != null && sunriseMinutes !== sunsetMinutes) {
            isNight = minutesNow < sunriseMinutes || minutesNow > sunsetMinutes;
        } else {
            // fallback: 20:00–06:00
            isNight = cityNow.getHours() >= NIGHT_FALLBACK_START || cityNow.getHours() < NIGHT_FALLBACK_END;
            if (console && console.debug) {
                console.debug("[theme] fallback clock mode, no sun times");
            }
        }

        setBodyThemeClass(isNight ? "dark" : "light");
        scheduleNextAutoSwitch(cityNow, sunriseMinutes, sunsetMinutes);
    }

    function scheduleNextAutoSwitch(cityNow, sunriseMinutes, sunsetMinutes) {
        if (autoTimerId) {
            clearTimeout(autoTimerId);
            autoTimerId = null;
        }
        if (sunriseMinutes == null || sunsetMinutes == null || sunriseMinutes === sunsetMinutes) {
            // simple fallback: пересчитать через 30 минут
            autoTimerId = setTimeout(applyAutoTheme, 30 * 60 * 1000);
            return;
        }
        var minutesNow = cityNow.getHours() * 60 + cityNow.getMinutes();
        var nextMinutes;
        var isCurrentlyNight = minutesNow < sunriseMinutes || minutesNow > sunsetMinutes;
        if (isCurrentlyNight) {
            if (minutesNow < sunriseMinutes) {
                nextMinutes = sunriseMinutes;
            } else {
                nextMinutes = sunriseMinutes + 24 * 60;
            }
        } else {
            nextMinutes = sunsetMinutes;
            if (minutesNow >= sunsetMinutes) {
                nextMinutes = sunriseMinutes + 24 * 60;
            }
        }
        var diffMinutes = nextMinutes - minutesNow;
        if (diffMinutes <= 0) {
            diffMinutes = 15; // safety
        }
        autoTimerId = setTimeout(applyAutoTheme, diffMinutes * 60 * 1000);
    }

    function applyThemeMode(mode) {
        if (mode === "light") {
            if (autoTimerId) {
                clearTimeout(autoTimerId);
                autoTimerId = null;
            }
            setBodyThemeClass("light");
        } else if (mode === "dark") {
            if (autoTimerId) {
                clearTimeout(autoTimerId);
                autoTimerId = null;
            }
            setBodyThemeClass("dark");
        } else {
            applyAutoTheme();
        }
        syncThemeControls(mode);
    }

    function syncThemeControls(mode) {
        var items = document.querySelectorAll(".theme-switch__item");
        if (!items) return;
        items.forEach(function (btn) {
            btn.classList.remove("theme-switch__item--active");
            var m = btn.getAttribute("data-theme-mode");
            if (m === mode) {
                btn.classList.add("theme-switch__item--active");
            }
        });
    }

    function initThemeControls() {
        var items = document.querySelectorAll(".theme-switch__item");
        if (items && items.length) {
            items.forEach(function (btn) {
                btn.addEventListener("click", function () {
                    var mode = btn.getAttribute("data-theme-mode") || "auto";
                    saveThemeMode(mode);
                    applyThemeMode(mode);
                });
            });
        }
        var mode = getThemeMode();
        applyThemeMode(mode);
    }

    // expose small API if нужно вызывать извне
    window.__theme = {
        getMode: getThemeMode,
        setMode: function (mode) {
            saveThemeMode(mode);
            applyThemeMode(mode);
        },
        applyAuto: applyAutoTheme
    };

    document.addEventListener("DOMContentLoaded", function () {
        initThemeControls();
    });
})();
