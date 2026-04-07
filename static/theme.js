/**
 * @file Light/dark/auto theme from `localStorage`, SSR payload (`#__WEATHER__`), and sun times for auto mode.
 * Exposes `window.__theme` for imperative updates.
 */
(function () {
    /** Fallback day window when `sun` times missing (browser local clock). */
    var FALLBACK_DAY_START_MIN = 7 * 60; // 07:00
    var FALLBACK_DAY_END_MIN = 21 * 60; // 21:00
    var THEME_KEY = "weather:themeMode";
    var payload = null;
    var autoPollId = null;
    var themeTransitionClearId = null;

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
            metaEarly.setAttribute("content", storedEarly === "light" ? "#1a78c2" : "#020d18");
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

    /**
     * Parses embedded JSON from `#__WEATHER__` once; returns `{}` on missing/invalid data.
     * @returns {Record<string, unknown>}
     */
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

    /**
     * @returns {"light"|"dark"|"auto"} Збережені лише light/dark; без ключа — авто.
     */
    function getThemeMode() {
        try {
            var v = localStorage.getItem(THEME_KEY);
            if (v === "light" || v === "dark") return v;
            if (v === "auto") {
                localStorage.removeItem(THEME_KEY);
            }
        } catch (_) {}
        return "auto";
    }

    /**
     * @param {string} mode One of `light`, `dark`, `auto`.
     * @returns {void}
     */
    function saveThemeMode(mode) {
        try {
            if (mode === "auto") {
                localStorage.removeItem(THEME_KEY);
            } else {
                localStorage.setItem(THEME_KEY, mode);
            }
        } catch (_) {}
    }

    /**
     * Applies `data-theme`, body classes, theme-color meta, and dispatches `weather-theme-change`.
     * @param {"light"|"dark"} theme Resolved visual theme (not `auto`).
     * @returns {void}
     */
    function setBodyThemeClass(theme) {
        var body = document.body;
        var root = document.documentElement;
        var isDark = theme === "dark";
        var nextAttr = isDark ? "dark" : "light";
        var prevAttr = root ? root.getAttribute("data-theme") || "dark" : "dark";
        var changed = prevAttr !== nextAttr;

        if (changed && body) {
            body.classList.add("theme-transitioning");
            if (themeTransitionClearId) {
                clearTimeout(themeTransitionClearId);
                themeTransitionClearId = null;
            }
            themeTransitionClearId = setTimeout(function () {
                if (body) body.classList.remove("theme-transitioning");
                themeTransitionClearId = null;
            }, 400);
        }

        if (root) {
            root.setAttribute("data-theme", nextAttr);
            root.style.backgroundColor = isDark ? "#020d18" : "#1a78c2";
        }
        var themeMeta = document.querySelector('meta[name="theme-color"]');
        if (themeMeta) {
            themeMeta.setAttribute("content", isDark ? "#020d18" : "#1a78c2");
        }
        var fromDark = prevAttr === "dark";
        var toDark = isDark;
        try {
            window.dispatchEvent(
                new CustomEvent("weather-theme-change", {
                    detail: { fromDark: fromDark, toDark: toDark, changed: changed }
                })
            );
        } catch (_) {}
        if (!body) return;
        body.classList.remove("theme-light", "theme-dark");
        body.classList.add(isDark ? "theme-dark" : "theme-light");
    }

    /**
     * Хвилини від півночі в часовому поясі міста (`meta.offsetSeconds` від UTC), без зсуву getHours() браузера.
     * @param {{ offsetSeconds?: number }} [meta]
     * @returns {number}
     */
    function computeCityMinutesFromMidnight(meta) {
        var offsetSeconds = meta && typeof meta.offsetSeconds === "number" ? meta.offsetSeconds : null;
        if (offsetSeconds == null) {
            var loc = new Date();
            return loc.getHours() * 60 + loc.getMinutes();
        }
        var utcSec = Math.floor(Date.now() / 1000);
        var citySec = utcSec + offsetSeconds;
        var secInDay = ((citySec % 86400) + 86400) % 86400;
        return Math.floor(secInDay / 60);
    }

    /**
     * Авто: світло від сходу до заходу (дані з #__WEATHER__.sun), інакше fallback 07:00–21:00 локально браузера.
     * @returns {void}
     */
    function applyAutoTheme() {
        var data = readPayload();
        var sun = data && data.sun ? data.sun : {};
        var meta = data && data.meta ? data.meta : {};

        var sunriseMinutes = typeof sun.sunriseMinutes === "number" ? sun.sunriseMinutes : null;
        var sunsetMinutes = typeof sun.sunsetMinutes === "number" ? sun.sunsetMinutes : null;

        var minutesNow = computeCityMinutesFromMidnight(meta);
        var isNight;

        if (sunriseMinutes != null && sunsetMinutes != null && sunriseMinutes !== sunsetMinutes) {
            isNight = minutesNow < sunriseMinutes || minutesNow > sunsetMinutes;
        } else {
            isNight = minutesNow < FALLBACK_DAY_START_MIN || minutesNow >= FALLBACK_DAY_END_MIN;
            if (console && console.debug) {
                console.debug("[theme] auto: no sun in payload, using 07:00–21:00 local fallback");
            }
        }

        setBodyThemeClass(isNight ? "dark" : "light");
    }

    /** @returns {void} */
    function stopAutoPolling() {
        if (autoPollId) {
            clearInterval(autoPollId);
            autoPollId = null;
        }
    }

    /** Один одразу + кожні 5 хвилин, поки активний режим Авто. */
    function startAutoPolling() {
        stopAutoPolling();
        applyAutoTheme();
        autoPollId = setInterval(applyAutoTheme, 5 * 60 * 1000);
    }

    /**
     * @param {"light"|"dark"|"auto"} mode
     * @returns {void}
     */
    function applyThemeMode(mode) {
        if (mode === "light") {
            stopAutoPolling();
            setBodyThemeClass("light");
        } else if (mode === "dark") {
            stopAutoPolling();
            setBodyThemeClass("dark");
        } else {
            startAutoPolling();
        }
        syncThemeControls(mode);
    }

    /**
     * Highlights the active `.theme-switch__item` matching `data-theme-mode`.
     * @param {string} mode
     * @returns {void}
     */
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

    /** Binds theme switch buttons and applies stored mode on load. */
    function initThemeControls() {
        var items = document.querySelectorAll(".theme-switch__item");
        if (items && items.length) {
            items.forEach(function (btn) {
                btn.addEventListener("click", function () {
                    if (document.body && document.body.classList.contains("theme-transitioning")) {
                        return;
                    }
                    var mode = btn.getAttribute("data-theme-mode") || "auto";
                    saveThemeMode(mode);
                    applyThemeMode(mode);
                });
            });
        }
        var mode = getThemeMode();
        applyThemeMode(mode);
    }

    /**
     * Imperative theme API for other scripts.
     * @type {{ getMode: typeof getThemeMode, setMode: function(string): void, applyAuto: function(): void }}
     */
    window.__theme = {
        getMode: getThemeMode,
        /** @param {string} mode */
        setMode: function (mode) {
            saveThemeMode(mode);
            applyThemeMode(mode);
        },
        applyAuto: function () {
            if (getThemeMode() === "auto") {
                applyAutoTheme();
            }
        }
    };

    /** Mobile: gear toggles theme + language panel (desktop: always visible). */
    function initHeaderAppMenu() {
        var root = document.getElementById("js-header-actions");
        var toggle = document.getElementById("js-header-menu-toggle");
        var panel = document.getElementById("js-header-menu-panel");
        if (!root || !toggle || !panel) return;

        function setOpen(open) {
            root.classList.toggle("header-actions--open", open);
            toggle.setAttribute("aria-expanded", open ? "true" : "false");
        }

        toggle.addEventListener("click", function (e) {
            e.stopPropagation();
            setOpen(!root.classList.contains("header-actions--open"));
        });

        panel.addEventListener("click", function (e) {
            e.stopPropagation();
        });

        document.addEventListener("click", function () {
            setOpen(false);
        });
    }

    document.addEventListener("DOMContentLoaded", function () {
        initThemeControls();
        initHeaderAppMenu();
    });
})();
