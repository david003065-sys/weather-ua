
(function () {
    const app = document.querySelector(".weather-app");
    if (!app) return;

    const weatherCode = parseInt(app.dataset.weatherCode || "0", 10);
    const isNight = app.dataset.isNight === "true";
    const cityId = app.dataset.cityId || "";
    const lang = document.documentElement.lang || "ru";

    function readClientI18n() {
        var el = document.getElementById("__CLIENT_I18N__");
        if (!el || !el.textContent) return {};
        try {
            return JSON.parse(String(el.textContent).trim() || "{}");
        } catch (e) {
            return {};
        }
    }
    var clientI18n = readClientI18n();

    var MOOD_CLASSES = ["weather-clear", "weather-cloudy", "weather-rain", "weather-snow", "weather-night"];

    function mapCodeToClass(code, night) {
        if (night) return "night";
        if (code === 0) return "sunny";
        if ([1, 2, 3].includes(code)) return "cloudy";
        if ((code >= 51 && code <= 67) || (code >= 80 && code <= 82)) return "rain";
        if ((code >= 71 && code <= 77) || code === 85 || code === 86) return "snow";
        if (code >= 95) return "rain";
        return "cloudy";
    }

    /* Ambient mood (CSS variables / gradients); night wins over condition */
    function mapCodeToMoodClass(code, night) {
        if (night) return "weather-night";
        if (code === 0) return "weather-clear";
        if ([1, 2, 3].includes(code)) return "weather-cloudy";
        if ((code >= 51 && code <= 67) || (code >= 80 && code <= 82) || code >= 95) return "weather-rain";
        if ((code >= 71 && code <= 77) || code === 85 || code === 86) return "weather-snow";
        return "weather-cloudy";
    }

    /**
     * @param engineCode опционально: OpenWeather id (data.weather[0].id) для Atmosphere; иначе тот же WMO
     */
    function syncWeatherAtmosphere(el, wmoCode, night, engineCode) {
        if (!el) return;
        var c = typeof wmoCode === "number" ? wmoCode : parseInt(wmoCode, 10);
        if (Number.isNaN(c)) c = 0;
        var n = night === true || night === "true";
        el.dataset.weatherCode = String(c);
        el.dataset.isNight = n ? "true" : "false";
        var vis = mapCodeToClass(c, n);
        el.classList.remove("sunny", "cloudy", "rain", "snow", "night");
        el.classList.add(vis);
        var mood = mapCodeToMoodClass(c, n);
        MOOD_CLASSES.forEach(function (m) {
            el.classList.remove(m);
        });
        el.classList.add(mood);
        var eng =
            engineCode !== undefined && engineCode !== null ? Number(engineCode) : c;
        if (typeof atmosphere !== "undefined" && atmosphere && typeof atmosphere.update === "function") {
            atmosphere.update(eng, n);
        }
    }

    function updateBackgroundByTemp(temp) {
        var from = "#1d4ed8";
        var to = "#f97316";
        if (temp < 0) {
            from = "#0ea5e9";
            to = "#1d4ed8";
        } else if (temp < 15) {
            from = "#38bdf8";
            to = "#0ea5e9";
        } else if (temp < 25) {
            from = "#22c55e";
            to = "#38bdf8";
        } else {
            from = "#f97316";
            to = "#fb923c";
        }
        var root = document.documentElement;
        root.style.setProperty("--temp-color-from", from);
        root.style.setProperty("--temp-color-to", to);
    }

    window.updateBackgroundByTemp = updateBackgroundByTemp;

    // Smart Advice (clothes tips) block on index page
    var adviceTextEl = document.getElementById("js-weather-advice-text");
    var adviceElement = adviceTextEl ? adviceTextEl.closest(".weather-advice") : null;

    function parseFirstNumber(text) {
        if (text == null) return NaN;
        var s = String(text).trim().replace(",", ".");
        var m = s.match(/-?\d+(\.\d+)?/);
        if (!m) return NaN;
        return parseFloat(m[0]);
    }

    function isRainWmo(code) {
        if (typeof code !== "number" || Number.isNaN(code)) return false;
        return (code >= 51 && code <= 67) || (code >= 80 && code <= 82) || code >= 95;
    }

    function updateSmartAdvice(tempC, windKph, humidityPct, isRain) {
        if (!adviceTextEl) return;
        if (typeof tempC !== "number" || Number.isNaN(tempC)) return;

        if (adviceElement) {
            adviceElement.classList.remove("animate-advice");
        }

        var base;
        if (tempC < 0) {
            base = "Одевайся максимально тепло! Пуховик, шарф и перчатки обязательны.";
        } else if (tempC < 10) {
            base = "Прохладно. Пальто или теплая куртка будут в самый раз.";
        } else if (tempC < 18) {
            base = "Свежо. Ветровка или плотное худи — идеальный выбор.";
        } else if (tempC < 25) {
            base = "Комфортно! Футболка и легкая кофта на вечер.";
        } else {
            base = "Жара! Выбирай легкую одежду из хлопка и пей больше воды.";
        }

        var extras = [];
        if (typeof windKph === "number" && !Number.isNaN(windKph)) {
            var windMs = windKph / 3.6; // API gives kph
            if (windMs > 7) extras.push("...но берегись ветра, он сегодня кусачий.");
        }
        if (typeof humidityPct === "number" && !Number.isNaN(humidityPct) && humidityPct > 80) {
            extras.push("Влажность высокая, будет казаться холоднее, чем есть.");
        }
        if (isRain) extras.push("И не забудь зонт — сегодня без него никак!");

        adviceTextEl.textContent = extras.length ? base + " " + extras.join(" ") : base;

        if (adviceElement) {
            void adviceElement.offsetWidth; // restart CSS animation
            adviceElement.classList.add("animate-advice");
        }
    }

    function updatePrecipChance(chance, isFallback) {
        var precipEl = document.getElementById("js-precip-chance");
        if (!precipEl) return;
        if (isFallback) {
            precipEl.textContent = "☔ —";
            return;
        }
        var ch = Number(chance);
        if (Number.isNaN(ch) || ch < 0 || ch > 100) {
            precipEl.textContent = "☔ —";
            return;
        }
        precipEl.textContent = "☔ " + Math.round(ch) + "%";
    }

    function getHourlyScrollEl() {
        return document.getElementById("js-hourly-scroll");
    }

    /** Почасовой график температуры (Chart.js) — точки из `.hourly-item` в `#js-hourly-scroll`. */
    function renderTempChart() {
        const canvas = document.getElementById("js-temp-chart");
        if (!canvas) {
            console.error("ОШИБКА: Холст #js-temp-chart не найден на странице!");
            return;
        }
        if (typeof Chart === "undefined") {
            console.error("ОШИБКА: Библиотека Chart.js не загрузилась!");
            return;
        }
        console.log("Canvas найден, Chart.js готов. Собираем данные...");

        const scroll = document.getElementById("js-hourly-scroll");
        if (!scroll) {
            console.error("ОШИБКА: Контейнер #js-hourly-scroll не найден.");
            return;
        }

        const items = scroll.querySelectorAll(".hourly-item");
        const labels = [];
        const data = [];
        for (let i = 0; i < items.length; i++) {
            const timeEl = items[i].querySelector(".hourly-item__time");
            const tempEl = items[i].querySelector(".hourly-item__temp");
            if (!timeEl || !tempEl) continue;
            const lab = String(timeEl.textContent || "").trim();
            let raw = String(tempEl.textContent || "")
                .trim()
                .replace(/\u2212/g, "-")
                .replace(/°/g, "")
                .replace(/−/g, "-")
                .trim();
            if (!raw || raw === "—" || raw === "-") continue;
            const n = parseFloat(raw.replace(",", "."));
            if (Number.isNaN(n)) continue;
            labels.push(lab);
            data.push(n);
        }

        const innerContainer = document.getElementById("js-hourly-inner");
        if (innerContainer && items.length > 0) {
            // Задаем точную ширину, чтобы Chart.js и flex-часы совпали
            innerContainer.style.minWidth = items.length * 55 + "px";
            innerContainer.style.width = items.length * 55 + "px";
        }

        const existing = typeof Chart.getChart === "function" ? Chart.getChart(canvas) : null;
        if (existing) {
            existing.destroy();
        }

        if (!labels.length) {
            console.warn("renderTempChart: нет почасовых точек в DOM (ожидается загрузка API).");
            return;
        }

        new Chart(canvas, {
            type: "line",
            data: {
                labels: labels,
                datasets: [
                    {
                        data: data,
                        borderColor: "rgba(255,255,255,0.8)",
                        backgroundColor: "rgba(255,255,255,0.08)",
                        borderWidth: 2,
                        tension: 0.4,
                        fill: true,
                        pointRadius: 3,
                        pointBackgroundColor: "rgba(255,255,255,0.9)",
                        pointBorderColor: "rgba(255,255,255,0.5)"
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false }
                },
                scales: {
                    x: { display: false },
                    y: { display: false }
                }
            }
        });
    }

    function renderHourlyForecast(hourly) {
        var hourlyScrollEl = getHourlyScrollEl();
        if (!hourlyScrollEl) return;
        hourlyScrollEl.innerHTML = "";
        hourlyScrollEl.removeAttribute("aria-hidden");
        if (Array.isArray(hourly) && hourly.length) {
            for (var i = 0; i < hourly.length; i++) {
                var h = hourly[i];
                if (!h) continue;

                var item = document.createElement("div");
                item.className = "hourly-item" + (i === 0 ? " hourly-item--now" : "");

                var timeEl = document.createElement("div");
                timeEl.className = "hourly-item__time";
                timeEl.textContent = h.time || "";

                var iconEl = document.createElement("div");
                iconEl.className = "hourly-item__icon";
                iconEl.setAttribute("aria-hidden", "true");
                iconEl.textContent = h.icon || "";

                var tempEl = document.createElement("div");
                tempEl.className = "hourly-item__temp";
                tempEl.textContent =
                    typeof h.temperature === "number" && !Number.isNaN(h.temperature)
                        ? Math.round(h.temperature) + "°"
                        : "—";

                item.appendChild(timeEl);
                item.appendChild(iconEl);
                item.appendChild(tempEl);
                hourlyScrollEl.appendChild(item);
            }
        }

        // Отрисовываем график только после того, как DOM наполнился данными
        if (typeof renderTempChart === "function") {
            // Небольшая задержка, чтобы браузер успел отрисовать элементы
            setTimeout(renderTempChart, 50);
        }
        if (typeof initDragToScroll === "function") {
            initDragToScroll();
        }
    }

    function numFromJSON(v) {
        if (typeof v === "number" && !Number.isNaN(v)) return v;
        if (typeof v === "string" && v.trim() !== "") {
            var n = parseFloat(String(v).replace(",", "."));
            return Number.isNaN(n) ? NaN : n;
        }
        return NaN;
    }

    /**
     * УФ, видимость, ощущается как, давление + рассвет/закат (index + city).
     * Сначала сброс, затем данные из свежего JSON /api/weather (WeatherAPI на бэкенде).
     */
    function renderMetrics(data) {
        if (!document.querySelector(".weather-details-grid") && !document.querySelector(".metrics-grid")) return;

        var uvIndexEl = document.getElementById("uv-index-val");
        var uvSubEl = document.getElementById("uv-index-sub");
        var visibilityEl = document.getElementById("visibility-val");
        var feelsEl = document.getElementById("feels-like-val");
        var pressureEl = document.getElementById("pressure-val");
        var pressureUnitEl = document.getElementById("pressure-unit");
        var sunriseEl = document.getElementById("sunrise-val");
        var sunsetEl = document.getElementById("sunset-val");
        var windTileEl = document.getElementById("js-current-wind");
        var humTileEl = document.getElementById("js-current-humidity");

        if (pressureUnitEl) pressureUnitEl.textContent = "мм рт. ст.";
        if (uvIndexEl) uvIndexEl.textContent = "—";
        if (uvSubEl) uvSubEl.textContent = "";
        if (visibilityEl) visibilityEl.textContent = "—";
        if (feelsEl) feelsEl.textContent = "—";
        if (pressureEl) pressureEl.textContent = "—";
        if (sunriseEl) sunriseEl.textContent = "—";
        if (sunsetEl) sunsetEl.textContent = "—";
        if (windTileEl) windTileEl.textContent = "—";
        if (humTileEl) humTileEl.textContent = "—";

        if (!data || !data.current) return;

        var current = data.current;
        var isFb = !!current.isFallback;
        if (windTileEl) {
            if (isFb) windTileEl.textContent = "—";
            else {
                var wSpd = numFromJSON(current.wind);
                if (!Number.isNaN(wSpd)) {
                    windTileEl.textContent =
                        Math.round(wSpd) + (clientI18n.windSuffix || " км/ч");
                }
            }
        }
        if (humTileEl) {
            if (isFb) humTileEl.textContent = "—";
            else {
                var humPct = numFromJSON(current.humidity);
                if (!Number.isNaN(humPct)) {
                    humTileEl.textContent =
                        Math.round(humPct) + (clientI18n.humiditySuffix || "%");
                }
            }
        }
        var uv = numFromJSON(current.uv_index);
        if (uvIndexEl && !Number.isNaN(uv)) {
            uvIndexEl.textContent = String(Math.round(uv));
        }
        if (uvSubEl && !Number.isNaN(uv)) {
            if (uv <= 2) uvSubEl.textContent = "Низкий";
            else if (uv <= 5) uvSubEl.textContent = "Средний";
            else uvSubEl.textContent = "Высокий";
        }

        var vis = numFromJSON(current.visibility);
        if (visibilityEl && !Number.isNaN(vis) && vis >= 0) {
            if (vis > 0) {
                visibilityEl.textContent = vis < 10 ? vis.toFixed(1) : String(Math.round(vis));
            }
        }

        var feels = numFromJSON(current.apparent_temperature);
        if (feelsEl && !Number.isNaN(feels)) {
            feelsEl.textContent = String(Math.round(feels));
        }

        var p = numFromJSON(current.pressure);
        if (pressureEl && !Number.isNaN(p) && p > 0) {
            pressureEl.textContent = String(Math.round(p));
        }

        var rise =
            (data.sunrise && String(data.sunrise).trim()) ||
            (data.daily && data.daily[0] && data.daily[0].sunrise && String(data.daily[0].sunrise).trim()) ||
            "";
        var set =
            (data.sunset && String(data.sunset).trim()) ||
            (data.daily && data.daily[0] && data.daily[0].sunset && String(data.daily[0].sunset).trim()) ||
            "";
        if (sunriseEl && rise) sunriseEl.textContent = rise;
        if (sunsetEl && set) sunsetEl.textContent = set;
    }

    function getRouteWeatherRequest() {
        var path = (window.location && window.location.pathname) || "/";
        // strip trailing slash (except root)
        if (path.length > 1 && path[path.length - 1] === "/") path = path.slice(0, -1);

        var mCity = path.match(/^\/city\/([^\/?#]+)$/);
        if (mCity && mCity[1]) {
            var cityId = decodeURIComponent(mCity[1]);
            return {
                kind: "city",
                url:
                    "/api/weather/" +
                    encodeURIComponent(cityId) +
                    "?lang=" +
                    encodeURIComponent(lang)
            };
        }

        var mPlace = path.match(/^\/place\/([^\/?#]+)$/);
        if (mPlace && mPlace[1]) {
            var placeId = decodeURIComponent(mPlace[1]);
            return {
                kind: "place",
                url:
                    "/api/place_weather?id=" +
                    encodeURIComponent(placeId) +
                    "&lang=" +
                    encodeURIComponent(lang)
            };
        }

        // default (index)
        return {
            kind: "default",
            url:
                "/api/weather/" +
                encodeURIComponent(DEFAULT_CITY_ID) +
                "?lang=" +
                encodeURIComponent(lang)
        };
    }

    async function refreshIndexFromRoute() {
        var hasMetrics =
            !!document.querySelector(".weather-details-grid") || !!document.querySelector(".metrics-grid");
        if (!getHourlyScrollEl() && !hasMetrics) return;

        var req = getRouteWeatherRequest();
        if (!req || !req.url) return;

        try {
            var res = await fetch(req.url);
            if (!res.ok) return;
            var json = await res.json();
            if (!json || !json.current) return;

            // Update main hero (temperature + condition) to prevent mismatch.
            var tempElNow = document.getElementById("js-current-temp");
            var descElNow = document.querySelector(".weather-hero__condition");
            var heroTitleEl = document.getElementById("hero-title");

            var isFallbackNow = !!json.current.isFallback;
            if (tempElNow) tempElNow.textContent = isFallbackNow ? "—" : Math.round(json.current.temperature);
            if (descElNow) descElNow.textContent = json.current.description || (isFallbackNow ? "—" : "");
            updatePrecipChance(json.current.precipitation_chance, isFallbackNow);
            if (heroTitleEl && json.cityName) heroTitleEl.textContent = json.cityName;

            // Update atmosphere / precipitation / clouds for the new code.
            if (json.current.weatherCode !== undefined && json.current.isNight !== undefined) {
                syncWeatherAtmosphere(app, json.current.weatherCode, json.current.isNight, json.current.weatherCode);
            }

            // Update background and smart advice.
            if (!isFallbackNow) {
                updateBackgroundByTemp(json.current.temperature);
                updateSmartAdvice(
                    json.current.temperature,
                    json.current.wind,
                    json.current.humidity,
                    isRainWmo(json.current.weatherCode)
                );
            }

            if (!Array.isArray(json.hourly) || !json.hourly.length) {
                console.error(
                    "[weather] hourly missing or empty from API (index/route)",
                    (req && req.url) || "",
                    json && json.cityId
                );
            }
            renderHourlyForecast(json.hourly);

            renderMetrics(json);
        } catch (e) {
            console.error("refreshIndexFromRoute failed", e);
        }
    }

    syncWeatherAtmosphere(app, weatherCode, isNight);

    var initialTempEl = document.getElementById("js-current-temp");
    if (initialTempEl) {
        var initialTemp = parseFloat(initialTempEl.textContent.replace(",", "."));
        if (!Number.isNaN(initialTemp)) {
            updateBackgroundByTemp(initialTemp);

            if (adviceTextEl) {
                var windEl = document.getElementById("js-current-wind");
                var humEl = document.getElementById("js-current-humidity");
                var initialWind = windEl ? parseFirstNumber(windEl.textContent) : NaN;
                var initialHumidity = humEl ? parseFirstNumber(humEl.textContent) : NaN;
                updateSmartAdvice(initialTemp, initialWind, initialHumidity, isRainWmo(weatherCode));
            }
        }
    }

    function initTempChart(canvas) {
        if (!window.Chart || !canvas) return;

        try {
            var existing = typeof Chart.getChart === "function" ? Chart.getChart(canvas) : null;
            if (existing) {
                existing.destroy();
            }
            /* Chart reads tokens from chart surface (updates when theme changes) */
            var host = canvas.closest(".chart-card") || document.body;
            var css = getComputedStyle(host);
            var textMuted = (css.getPropertyValue("--text-muted") || "#9ca3af").trim();
            var textStrong = (css.getPropertyValue("--text-strong") || "#e5e7eb").trim();
            var gridColor = (css.getPropertyValue("--chart-grid") || "rgba(148, 163, 184, 0.28)").trim();
            var lineMax = (css.getPropertyValue("--link") || "#38bdf8").trim();
            var lineMin = (css.getPropertyValue("--accent-warm") || "#f97316").trim();
            var fillMax = (css.getPropertyValue("--chart-fill-max") || "rgba(56, 189, 248, 0.16)").trim();
            var fillMin = (css.getPropertyValue("--chart-fill-min") || "rgba(249, 115, 22, 0.16)").trim();

            const labels = JSON.parse(canvas.dataset.labels || "[]");
            const min = JSON.parse(canvas.dataset.min || "[]");
            const max = JSON.parse(canvas.dataset.max || "[]");

            const ctx = canvas.getContext("2d");
            new Chart(ctx, {
                type: "line",
                data: {
                    labels: labels,
                    datasets: [
                        {
                            label: clientI18n.chartMax || "Макс",
                            data: max,
                            borderColor: lineMax,
                            backgroundColor: fillMax,
                            borderWidth: 2,
                            tension: 0.35,
                            fill: true,
                            pointRadius: 3,
                            pointBackgroundColor: lineMax,
                            pointBorderColor: textStrong
                        },
                        {
                            label: clientI18n.chartMin || "Мин",
                            data: min,
                            borderColor: lineMin,
                            backgroundColor: fillMin,
                            borderWidth: 2,
                            tension: 0.35,
                            fill: true,
                            pointRadius: 3,
                            pointBackgroundColor: lineMin,
                            pointBorderColor: textStrong
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: {
                            display: true,
                            labels: {
                                color: textStrong,
                                font: { size: 11 }
                            }
                        }
                    },
                    scales: {
                        x: {
                            ticks: { color: textMuted },
                            grid: { color: gridColor }
                        },
                        y: {
                            ticks: {
                                color: textMuted,
                                callback: function (value) {
                                    return value + "°";
                                }
                            },
                            grid: { color: gridColor }
                        }
                    }
                }
            });
        } catch (e) {
            console.error("Failed to init chart", e);
        }
    }

    const tempCanvas = document.querySelector(".js-temp-chart");
    if (tempCanvas) {
        function bootChart() {
            initTempChart(tempCanvas);
        }
        if (document.readyState === "complete") {
            bootChart();
        } else {
            window.addEventListener("load", bootChart);
        }
        window.addEventListener("weather-theme-change", bootChart);
    }

    // Geo/default city (index page only)
    function safeStorageGet(key) {
        try {
            return localStorage.getItem(key);
        } catch (_) {
            return null;
        }
    }

    function safeStorageSet(key, value) {
        try {
            localStorage.setItem(key, value);
        } catch (_) {}
    }

    var DEFAULT_CITY_ID = "kyiv"; // fallback when nothing is stored / geolocation fails

    function buildUrlFromCityName(cityName) {
        if (!cityName) return null;
        var s = String(cityName);
        var langQ = encodeURIComponent(lang);
        if (s.indexOf("place:") === 0) {
            var placeId = s.slice("place:".length);
            if (!placeId) return null;
            return "/place/" + encodeURIComponent(placeId) + "?lang=" + langQ;
        }
        return "/city/" + encodeURIComponent(s) + "?lang=" + langQ;
    }

    // On "/" keep defaultCity hint in localStorage (Kyiv) until user picks another route.
    var isIndexPage = window.location.pathname === "/";
    if (isIndexPage) {
        var saved = safeStorageGet("defaultCity");
        if (!saved) {
            safeStorageSet("defaultCity", DEFAULT_CITY_ID);
        }
    }

    // Геолокация: координаты → GET /geo → редирект на /place/… или /city/… (см. GeoRedirect).
    var geoBtn = document.getElementById("js-geo-btn");
    if (geoBtn) {
        geoBtn.addEventListener("click", function () {
            if (!navigator.geolocation) {
                alert(clientI18n.geoNoBrowserSupport || "Geolocation is not supported.");
                return;
            }
            geoBtn.classList.add("geo-btn--loading");
            var locLang = document.documentElement.lang || "ru";
            navigator.geolocation.getCurrentPosition(
                function (pos) {
                    var plat = pos.coords.latitude;
                    var plon = pos.coords.longitude;
                    window.location.href =
                        "/geo?lat=" +
                        encodeURIComponent(plat) +
                        "&lon=" +
                        encodeURIComponent(plon) +
                        "&lang=" +
                        encodeURIComponent(locLang);
                },
                function (err) {
                    if (typeof console !== "undefined" && console.error) {
                        console.error(err);
                    }
                    geoBtn.classList.remove("geo-btn--loading");
                    alert(clientI18n.geoLocationFailed || "Could not get your position.");
                },
                { enableHighAccuracy: false, timeout: 10000, maximumAge: 300000 }
            );
        });
    }

    // Index page: keep hourly + metrics in sync with the current route.
    if (
        getHourlyScrollEl() ||
        document.querySelector(".weather-details-grid") ||
        document.querySelector(".metrics-grid")
    ) {
        // Patch pushState so route changes without full navigation still refresh this UI.
        if (history && history.pushState && !history.__weatherRoutePatched) {
            history.__weatherRoutePatched = true;
            var __origPushState = history.pushState;
            history.pushState = function () {
                var ret = __origPushState.apply(this, arguments);
                window.dispatchEvent(new Event("weather:routeChanged"));
                return ret;
            };
            window.addEventListener("popstate", function () {
                window.dispatchEvent(new Event("weather:routeChanged"));
            });
        }

        setTimeout(function () {
            refreshIndexFromRoute();
        }, 800);

        window.addEventListener("weather:routeChanged", function () {
            refreshIndexFromRoute();
        });
    }

    // Auto refresh for city page
    if (cityId) {
        var tempEl = document.getElementById("js-current-temp");
        var descEl = document.querySelector(".weather-hero__condition");
        var windEl = document.getElementById("js-current-wind");
        var humEl = document.getElementById("js-current-humidity");

        function applyWeather(data) {
            if (!data || !data.current) return;
            var isFallback = !!data.current.isFallback;
            if (tempEl) tempEl.textContent = isFallback ? "—" : Math.round(data.current.temperature);
            if (descEl) descEl.textContent = data.current.description;
            updatePrecipChance(data.current.precipitation_chance, isFallback);
            if (windEl) {
                windEl.textContent = isFallback
                    ? "—"
                    : Math.round(data.current.wind) + (clientI18n.windSuffix || " км/ч");
            }
            if (humEl) {
                humEl.textContent = isFallback
                    ? "—"
                    : Math.round(data.current.humidity) + (clientI18n.humiditySuffix || "%");
            }
            if (!isFallback) updateBackgroundByTemp(data.current.temperature);
            if (!isFallback) {
                updateSmartAdvice(
                    data.current.temperature,
                    data.current.wind,
                    data.current.humidity,
                    isRainWmo(data.current.weatherCode)
                );
            }
            // Force sync precipitation/cloud dynamics for the NEW city.
            // (SyncWeatherAtmosphere below also calls atmosphere.update, but this guarantees it runs even if types mismatch.)
            try {
                if (
                    window.atmosphere &&
                    typeof window.atmosphere.update === "function" &&
                    data.current &&
                    typeof data.current.weatherCode !== "undefined"
                ) {
                    window.atmosphere.update(data.current.weatherCode, data.current.isNight);
                }
            } catch (_) {
                /* ignore */
            }
            if (!Array.isArray(data.hourly) || !data.hourly.length) {
                console.error("[weather] hourly missing or empty from API (city)", cityId, data);
            }
            renderHourlyForecast(data.hourly);
            if (
                data.current &&
                typeof data.current.weatherCode === "number" &&
                typeof data.current.isNight === "boolean"
            ) {
                var engineId =
                    data.weather &&
                    data.weather[0] &&
                    data.weather[0].id != null &&
                    !Number.isNaN(Number(data.weather[0].id))
                        ? Number(data.weather[0].id)
                        : data.current.weatherCode;
                syncWeatherAtmosphere(app, data.current.weatherCode, data.current.isNight, engineId);
            }
            renderMetrics(data);
        }

        function cacheKey(id, l) {
            return "weatherCache:v2:" + id + ":" + l;
        }

        async function fetchWeather() {
            try {
                var key = cacheKey(cityId, lang);
                var cachedRaw = localStorage.getItem(key);
                if (cachedRaw) {
                    var cached = JSON.parse(cachedRaw);
                    if (cached && cached.ts && Date.now() - cached.ts < 5 * 60 * 1000) {
                        applyWeather(cached.data);
                        return;
                    }
                }

                var res = await fetch("/api/weather/" + encodeURIComponent(cityId) + "?lang=" + encodeURIComponent(lang));
                if (!res.ok) return;
                var json = await res.json();
                applyWeather(json);
                try {
                    localStorage.setItem(key, JSON.stringify({ ts: Date.now(), data: json }));
                } catch (_) {
                    /* ignore */
                }
            } catch (e) {
                console.error("refresh weather failed", e);
            }
        }

        // initial refresh in background
        setTimeout(function () {
            if (document.visibilityState === "visible") {
                fetchWeather();
            }
        }, 2000);
        // every 5 минут, только для активной вкладки
        setInterval(function () {
            if (document.visibilityState === "visible") {
                fetchWeather();
            }
        }, 5 * 60 * 1000);
    }

    // PWA: service worker + install UI live in /static/pwa.js (loads first, independent of this bundle).

    // Place search autocomplete on index page
    (function initPlaceSearch() {
        var input = document.getElementById("js-place-search");
        var box = document.getElementById("js-place-suggestions");
        if (!input || !box) return;

        var lang = document.documentElement.lang || "ru";
        function t(key) {
            var fb = { empty: "No results", error: "Search error", tooShort: "Type at least 2 characters" };
            if (lang === "uk") fb = { empty: "Нічого не знайдено", error: "Помилка пошуку", tooShort: "Введи мінімум 2 символи" };
            if (lang === "ru") fb = { empty: "Ничего не найдено", error: "Ошибка поиска", tooShort: "Введите минимум 2 символа" };
            if (key === "empty") return clientI18n.searchEmpty || fb.empty;
            if (key === "error") return clientI18n.searchError || fb.error;
            if (key === "tooShort") return clientI18n.searchTooShort || fb.tooShort;
            return "";
        }

        var timer = null;
        var currentController = null;
        var lastRequestId = 0;
        var lastQuery = "";
        var items = [];
        var activeIndex = -1;
        var isLoading = false;

        // small spinner inside the input
        var spinner = document.createElement("div");
        spinner.className = "search-input-spinner";
        spinner.setAttribute("aria-hidden", "true");
        if (input.parentNode) {
            input.parentNode.appendChild(spinner);
        }
        function setLoading(v) {
            isLoading = v;
            spinner.style.opacity = v ? "1" : "0";
        }

        function clearSuggestions() {
            box.innerHTML = "";
            box.style.display = "none";
            items = [];
            activeIndex = -1;
        }

        function renderMessage(text) {
            box.innerHTML = "";
            var el = document.createElement("div");
            el.className = "search-suggestions__empty";
            el.textContent = text;
            box.appendChild(el);
            items = [];
            activeIndex = -1;
            box.style.display = "block";
        }

        function pickName(place) {
            if (!place) return "";
            if (lang === "uk") {
                return place.name_uk || place.name_ru || place.name_en || "";
            }
            if (lang === "ru") {
                return place.name_ru || place.name_uk || place.name_en || "";
            }
            // en
            return place.name_en || place.name_uk || place.name_ru || "";
        }

        function pickOblast(place) {
            if (!place) return "";
            if (lang === "uk") return place.oblast_uk || place.oblast_ru || place.oblast_en || "";
            if (lang === "ru") return place.oblast_ru || place.oblast_uk || place.oblast_en || "";
            return place.oblast_en || place.oblast_uk || place.oblast_ru || "";
        }

        function pickType(place) {
            if (!place) return "";
            var t = "";
            if (lang === "uk") t = place.type_uk || place.type_ru || place.type_en || "";
            else if (lang === "ru") t = place.type_ru || place.type_uk || place.type_en || "";
            else t = place.type_en || place.type_uk || place.type_ru || "";

            if (t) return t;
            return clientI18n.placeTypeFallback || (lang === "en" ? "settlement" : lang === "uk" ? "населений пункт" : "населённый пункт");
        }

        function highlightMatch(el, text, query) {
            el.textContent = "";
            if (!text) return;
            if (!query) {
                el.textContent = text;
                return;
            }
            var lowerText = text.toLowerCase();
            var lowerQuery = query.toLowerCase();
            var idx = lowerText.indexOf(lowerQuery);
            if (idx === -1) {
                el.textContent = text;
                return;
            }
            el.appendChild(document.createTextNode(text.slice(0, idx)));
            var mark = document.createElement("mark");
            mark.className = "search-suggestions__highlight";
            mark.textContent = text.slice(idx, idx + query.length);
            el.appendChild(mark);
            el.appendChild(document.createTextNode(text.slice(idx + query.length)));
        }

        function renderSuggestions(list) {
            if (!list || !list.length) {
                renderMessage(t("empty"));
                return;
            }
            box.innerHTML = "";
            list.forEach(function (p, idx) {
                var el = document.createElement("div");
                el.className = "search-suggestions__item";
                el.setAttribute("role", "option");
                el.dataset.id = String(p.id);
                el.dataset.lat = String(p.lat);
                el.dataset.lon = String(p.lon);

                var title = document.createElement("div");
                title.className = "search-suggestions__title";
                var titleText = pickName(p);
                highlightMatch(title, titleText, lastQuery);

                var meta = document.createElement("div");
                meta.className = "search-suggestions__meta";
                var type = pickType(p);
                var oblast = pickOblast(p);
                var raion = p.raion || "";
                var parts = [];
                if (type) parts.push(type);
                if (raion) parts.push(raion);
                if (oblast) parts.push(oblast);
                meta.textContent = parts.join(", ");

                el.appendChild(title);
                el.appendChild(meta);

                var favBtn = document.createElement("button");
                favBtn.type = "button";
                favBtn.className = "search-suggestions__fav";
                favBtn.setAttribute("aria-label", clientI18n.favAddAria || "Add to favourites");
                favBtn.textContent = "★";
                favBtn.addEventListener("click", function (e) {
                    e.stopPropagation();
                    if (window.__weatherLists && window.__weatherLists.addFavoriteFromPlace) {
                        window.__weatherLists.addFavoriteFromPlace(p, lang);
                    }
                });
                el.appendChild(favBtn);

                el.addEventListener("mousedown", function (e) {
                    if (e.target && e.target.closest(".search-suggestions__fav")) {
                        return;
                    }
                    e.preventDefault();
                    if (window.__weatherLists && window.__weatherLists.addRecentFromPlace) {
                        window.__weatherLists.addRecentFromPlace(p, lang);
                    }
                    goToPlace(p);
                });

                box.appendChild(el);
            });
            items = Array.prototype.slice.call(box.querySelectorAll(".search-suggestions__item"));
            box.style.display = "block";
        }

        function setActive(idx) {
            if (!items.length) return;
            if (idx < 0) idx = items.length - 1;
            if (idx >= items.length) idx = 0;
            items.forEach(function (el) {
                el.classList.remove("search-suggestions__item--active");
            });
            items[idx].classList.add("search-suggestions__item--active");
            activeIndex = idx;
        }

        async function goToPlace(place) {
            if (!place || !place.id) return;

            var fallbackUrl = "/place/" + encodeURIComponent(String(place.id));
            if (lang) {
                fallbackUrl += "?lang=" + encodeURIComponent(lang);
            }

            var lat = place.lat;
            var lon = place.lon;
            var hasCoords = typeof lat === "number" && typeof lon === "number" && !Number.isNaN(lat) && !Number.isNaN(lon);

            // Try to map the chosen place to a "known weather city" route (/city/<id>).
            if (hasCoords) {
                try {
                    var findUrl =
                        "/api/find-city?lat=" +
                        encodeURIComponent(lat) +
                        "&lon=" +
                        encodeURIComponent(lon) +
                        "&lang=" +
                        encodeURIComponent(lang);
                    var res = await fetch(findUrl);
                    if (res && res.ok) {
                        var data = await res.json();
                        if (data && data.cityName) {
                            var mappedUrl = buildUrlFromCityName(data.cityName);
                            if (mappedUrl) {
                                window.history.pushState({}, "", mappedUrl);
                                window.location.reload();
                                return;
                            }
                        }
                    }
                } catch (_) {
                    /* ignore and fallback */
                }
            }

            window.history.pushState({}, "", fallbackUrl);
            window.location.reload();
        }

        async function fetchSuggestions(q) {
            var requestId = ++lastRequestId;

            if (currentController) {
                currentController.abort();
            }
            currentController = ('AbortController' in window) ? new AbortController() : null;
            var signal = currentController ? currentController.signal : undefined;

            try {
                setLoading(true);
                var url = "/api/places?q=" + encodeURIComponent(q) + "&limit=10";
                if (lang) {
                    url += "&lang=" + encodeURIComponent(lang);
                }
                var res = await fetch(url, { signal: signal });
                if (!res.ok) {
                    renderMessage(t("error"));
                    return;
                }
                var json = await res.json();
                // если пришёл старый ответ — игнорируем
                if (requestId !== lastRequestId) {
                    return;
                }
                if (!Array.isArray(json)) {
                    renderMessage(t("error"));
                    return;
                }
                renderSuggestions(json);
            } catch (e) {
                if (e.name === "AbortError") {
                    return;
                }
                console.error("places search failed", e);
                renderMessage(t("error"));
            } finally {
                setLoading(false);
            }
        }

        input.addEventListener("input", function () {
            var val = input.value.trim();
            if (val === lastQuery) return;
            lastQuery = val;

            if (timer) clearTimeout(timer);

            if (!val || val.length < 2) {
                if (!val) {
                    clearSuggestions();
                } else {
                    renderMessage(t("tooShort"));
                }
                return;
            }
            if (val.length > 64) {
                renderMessage(t("tooShort"));
                return;
            }

            timer = setTimeout(function () {
                fetchSuggestions(val);
            }, 250);
        });

        input.addEventListener("keydown", function (e) {
            if (!items.length) return;
            if (e.key === "ArrowDown") {
                e.preventDefault();
                setActive(activeIndex + 1);
            } else if (e.key === "ArrowUp") {
                e.preventDefault();
                setActive(activeIndex - 1);
            } else if (e.key === "Enter") {
                if (activeIndex >= 0 && activeIndex < items.length) {
                    e.preventDefault();
                    var el = items[activeIndex];
                    var id = el && el.dataset ? el.dataset.id : null;
                    var lat = el && el.dataset ? parseFloat(el.dataset.lat) : NaN;
                    var lon = el && el.dataset ? parseFloat(el.dataset.lon) : NaN;
                    goToPlace({ id: id, lat: lat, lon: lon });
                }
            } else if (e.key === "Escape") {
                clearSuggestions();
            }
        });

        document.addEventListener("click", function (e) {
            if (!box.contains(e.target) && e.target !== input) {
                clearSuggestions();
            }
        });

        // Если пришли с /place с query-параметром ?query=..., префилим и открываем подсказки
        (function bootstrapFromQueryParam() {
            var search = window.location.search || "";
            var m = search.match(/[?&](query|q)=([^&]+)/);
            if (!m) return;
            var raw = m[2];
            try {
                var value = decodeURIComponent(raw.replace(/\+/g, " "));
                value = value.trim();
                if (!value) return;
                input.value = value;
                lastQuery = "";
                input.focus();
                var ev = new Event("input", { bubbles: true });
                input.dispatchEvent(ev);
            } catch (_) {
                // ignore
            }
        })();
    })();

    // Favorites and recent cities blocks on index page
    (function initFavoritesAndRecent() {
        var favRoot = document.getElementById("js-favorites-list");
        var recentRoot = document.getElementById("js-recent-list");
        if (!favRoot && !recentRoot) return;

        var lang = document.documentElement.lang || "ru";
        var FAVORITES_KEY = "weather:favorites:v1";
        var RECENT_KEY = "weather:recent:v1";

        function readList(key) {
            try {
                var raw = localStorage.getItem(key);
                if (!raw) return [];
                var parsed = JSON.parse(raw);
                if (!Array.isArray(parsed)) return [];
                return parsed;
            } catch (_) {
                return [];
            }
        }

        function saveList(key, items) {
            try {
                localStorage.setItem(key, JSON.stringify(items));
            } catch (_) {
                /* ignore */
            }
        }

        function upsert(items, place) {
            if (!place || !place.id) return items;
            var id = Number(place.id);
            var next = items.filter(function (p) { return p.id !== id; });
            next.unshift({
                id: id,
                name_uk: place.name_uk || "",
                name_ru: place.name_ru || "",
                name_en: place.name_en || "",
                oblast_uk: place.oblast_uk || "",
                oblast_ru: place.oblast_ru || "",
                oblast_en: place.oblast_en || ""
            });
            return next;
        }

        function removeById(items, id) {
            var num = Number(id);
            return items.filter(function (p) { return p.id !== num; });
        }

        function chooseName(p, l) {
            if (!p) return "";
            if (l === "uk") return p.name_uk || p.name_ru || p.name_en || "";
            if (l === "ru") return p.name_ru || p.name_uk || p.name_en || "";
            return p.name_en || p.name_uk || p.name_ru || "";
        }

        async function fetchPlaceWeather(id, l) {
            try {
                var url = "/api/place_weather?id=" + encodeURIComponent(String(id));
                if (l) url += "&lang=" + encodeURIComponent(l);
                var res = await fetch(url);
                if (!res.ok) return null;
                return await res.json();
            } catch (_) {
                return null;
            }
        }

        async function renderFavorites() {
            if (!favRoot) return;
            var items = readList(FAVORITES_KEY);
            favRoot.innerHTML = "";
            if (!items.length) {
                return;
            }
            var cards = document.createDocumentFragment();
            for (var i = 0; i < items.length; i++) {
                var p = items[i];
                var data = await fetchPlaceWeather(p.id, lang);
                if (!data || !data.current) continue;

                var card = document.createElement("a");
                card.href = "/place/" + encodeURIComponent(String(p.id)) + "?lang=" + encodeURIComponent(lang);
                card.className = "city-card city-card--favorite rail-card";

                var header = document.createElement("div");
                header.className = "city-card__header";
                var nameEl = document.createElement("span");
                nameEl.className = "city-card__name";
                nameEl.textContent = chooseName(p, lang);
                var iconEl = document.createElement("span");
                iconEl.className = "city-card__icon";
                iconEl.textContent = data.current.icon || "";
                header.appendChild(nameEl);
                header.appendChild(iconEl);

                var temp = document.createElement("div");
                temp.className = "city-card__temperature";
                var tempVal = document.createElement("span");
                tempVal.className = "city-card__temp-value";
                tempVal.textContent = data.current.isFallback ? "—" : Math.round(data.current.temperature);
                var tempUnit = document.createElement("span");
                tempUnit.className = "city-card__temp-unit";
                tempUnit.textContent = "°C";
                temp.appendChild(tempVal);
                temp.appendChild(tempUnit);

                var status = document.createElement("p");
                status.className = "city-card__status";
                status.textContent = data.current.description || "";

                var footer = document.createElement("div");
                footer.className = "city-card__footer";
                var removeBtn = document.createElement("button");
                removeBtn.type = "button";
                removeBtn.className = "city-card__remove";
                removeBtn.textContent = "×";
                removeBtn.title = clientI18n.favRemoveTitle || "Удалить из избранного";
                removeBtn.addEventListener("click", function (id) {
                    return function (e) {
                        e.preventDefault();
                        e.stopPropagation();
                        var current = readList(FAVORITES_KEY);
                        current = removeById(current, id);
                        saveList(FAVORITES_KEY, current);
                        renderFavorites();
                    };
                }(p.id));
                footer.appendChild(removeBtn);

                card.appendChild(header);
                card.appendChild(temp);
                card.appendChild(status);
                card.appendChild(footer);

                cards.appendChild(card);
            }
            favRoot.appendChild(cards);
        }

        async function renderRecent() {
            if (!recentRoot) return;
            var items = readList(RECENT_KEY);
            recentRoot.innerHTML = "";
            if (!items.length) return;
            var cards = document.createDocumentFragment();
            for (var i = 0; i < items.length && i < 8; i++) {
                var p = items[i];
                var data = await fetchPlaceWeather(p.id, lang);
                if (!data || !data.current) continue;

                var card = document.createElement("a");
                card.href = "/place/" + encodeURIComponent(String(p.id)) + "?lang=" + encodeURIComponent(lang);
                card.className = "city-card city-card--favorite rail-card";

                var header = document.createElement("div");
                header.className = "city-card__header";
                var nameEl = document.createElement("span");
                nameEl.className = "city-card__name";
                nameEl.textContent = chooseName(p, lang);
                var iconEl = document.createElement("span");
                iconEl.className = "city-card__icon";
                iconEl.textContent = data.current.icon || "";
                header.appendChild(nameEl);
                header.appendChild(iconEl);

                var temp = document.createElement("div");
                temp.className = "city-card__temperature";
                var tempVal = document.createElement("span");
                tempVal.className = "city-card__temp-value";
                tempVal.textContent = data.current.isFallback ? "—" : Math.round(data.current.temperature);
                var tempUnit = document.createElement("span");
                tempUnit.className = "city-card__temp-unit";
                tempUnit.textContent = "°C";
                temp.appendChild(tempVal);
                temp.appendChild(tempUnit);

                var status = document.createElement("p");
                status.className = "city-card__status";
                status.textContent = data.current.description || "";

                card.appendChild(header);
                card.appendChild(temp);
                card.appendChild(status);

                cards.appendChild(card);
            }
            recentRoot.appendChild(cards);
        }

        async function bootstrapFavorites() {
            var items = readList(FAVORITES_KEY);
            if (items.length) return;
            var seeds =
                lang === "en"
                    ? ["Dnipro", "Kyiv", "Kramatorsk"]
                    : lang === "ru"
                      ? ["Днепр", "Киев", "Краматорск"]
                      : ["Дніпро", "Київ", "Краматорськ"];
            var next = [];
            for (var i = 0; i < seeds.length; i++) {
                try {
                    var res = await fetch(
                        "/api/places?q=" + encodeURIComponent(seeds[i]) + "&limit=1&lang=" + encodeURIComponent(lang)
                    );
                    if (!res.ok) continue;
                    var arr = await res.json();
                    if (!arr || !arr.length) continue;
                    var p = arr[0];
                    if (!p || !p.id) continue;
                    next = upsert(next, p);
                } catch (_) {
                    // ignore single failure
                }
            }
            if (next.length) {
                saveList(FAVORITES_KEY, next);
            }
        }

        window.__weatherLists = {
            addFavoriteFromPlace: function (p) {
                var items = readList(FAVORITES_KEY);
                items = upsert(items, p);
                if (items.length > 12) {
                    items = items.slice(0, 12);
                }
                saveList(FAVORITES_KEY, items);
                renderFavorites();
            },
            addRecentFromPlace: function (p) {
                var items = readList(RECENT_KEY);
                items = upsert(items, p);
                if (items.length > 20) {
                    items = items.slice(0, 20);
                }
                saveList(RECENT_KEY, items);
                renderRecent();
            }
        };

        (async function () {
            await bootstrapFavorites();
            await renderFavorites();
            await renderRecent();
        })();
    })();

    // Homepage: load more / collapse city cards (4 initial, +8 per click)
    (function initCitiesListExpand() {
        var grid = document.getElementById("js-cities-grid");
        var btn = document.getElementById("js-cities-toggle");
        var wrap = document.querySelector(".cities-toggle-wrap");
        if (!grid || !btn || !wrap) return;

        var cards = grid.querySelectorAll("a.city-card");
        var total = cards.length;
        var INITIAL = 4;
        var STEP = 8;

        if (total <= INITIAL) {
            wrap.style.display = "none";
            return;
        }

        var visible = INITIAL;
        var pattern = (btn.getAttribute("data-pattern-show-more") || "").trim();
        var labelLess = btn.getAttribute("data-label-less") || "Show less";
        var fallbackMore = btn.getAttribute("data-fallback-more") || "More";
        var reduceMotion =
            typeof window.matchMedia === "function" &&
            window.matchMedia("(prefers-reduced-motion: reduce)").matches;

        function syncVisibility() {
            for (var i = 0; i < total; i++) {
                if (i < visible) {
                    cards[i].classList.remove("city-card--list-hidden");
                } else {
                    cards[i].classList.add("city-card--list-hidden");
                    cards[i].classList.remove("city-card--list-reveal");
                }
            }
        }

        function updateLabel() {
            if (visible >= total) {
                btn.textContent = labelLess;
                btn.setAttribute("aria-expanded", "true");
            } else {
                var nextBatch = Math.min(STEP, total - visible);
                if (pattern.indexOf("%d") !== -1) {
                    btn.textContent = pattern.replace("%d", String(nextBatch));
                } else {
                    btn.textContent = fallbackMore;
                }
                btn.setAttribute("aria-expanded", "false");
            }
        }

        for (var k = INITIAL; k < total; k++) {
            cards[k].classList.add("city-card--list-hidden");
        }
        updateLabel();

        btn.addEventListener("click", function () {
            if (visible >= total) {
                visible = INITIAL;
                syncVisibility();
                updateLabel();
                try {
                    var sec = document.getElementById("cities");
                    if (sec) sec.scrollIntoView({ behavior: "smooth", block: "nearest" });
                } catch (_) {}
                return;
            }
            var target = Math.min(total, visible + STEP);
            for (var j = visible; j < target; j++) {
                cards[j].classList.remove("city-card--list-hidden");
                if (!reduceMotion) {
                    cards[j].classList.add("city-card--list-reveal");
                    (function (idx) {
                        window.setTimeout(function () {
                            cards[idx].classList.remove("city-card--list-reveal");
                        }, 480);
                    })(j);
                }
            }
            visible = target;
            updateLabel();
        });
    })();

    // Избранные города (canonical city id) — делегирование + восстановление звезды после перезагрузки.
    function parseWeatherFavoritesArray() {
        try {
            var parsed = JSON.parse(localStorage.getItem("weather_favorites") || "[]");
            return Array.isArray(parsed) ? parsed : [];
        } catch (e) {
            return [];
        }
    }

    function resolveFavoriteCityId(btn) {
        let cityId = btn.getAttribute("data-city-id");
        if (!cityId || cityId.trim() === "") {
            const match = window.location.pathname.match(/\/place\/(\d+)/);
            if (match) cityId = match[1];
        }
        return cityId && cityId.trim() !== "" ? cityId.trim() : "";
    }

    document.addEventListener("click", (e) => {
        const btn = e.target.closest("#js-toggle-favorite");
        if (!btn) return;
        e.preventDefault();

        const cityId = resolveFavoriteCityId(btn);
        if (!cityId) {
            console.error("Не удалось найти ID города ни в кнопке, ни в URL");
            return;
        }

        let favs = parseWeatherFavoritesArray();
        const sId = String(cityId);
        const index = favs.indexOf(sId);

        if (index === -1) {
            favs.push(sId);
            btn.classList.add("fav-btn--active");
        } else {
            favs.splice(index, 1);
            btn.classList.remove("fav-btn--active");
        }
        try {
            localStorage.setItem("weather_favorites", JSON.stringify(favs));
        } catch (err) {
            /* quota / private mode */
        }
    });

    function initFavoriteState() {
        const btn = document.querySelector("#js-toggle-favorite");
        if (!btn) return;

        const cityId = resolveFavoriteCityId(btn);
        if (!cityId) return;

        const favs = parseWeatherFavoritesArray();
        if (favs.includes(String(cityId))) {
            btn.classList.add("fav-btn--active");
        }
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", initFavoriteState);
    } else {
        initFavoriteState();
    }

    function appendFavoriteCardHome(rail, item, lang, windLabel, humLabel) {
        var a = document.createElement("a");
        if (item.kind === "place") {
            a.href = "/place/" + encodeURIComponent(item.id) + "?lang=" + encodeURIComponent(lang);
        } else {
            a.href = "/city/" + encodeURIComponent(item.id) + "?lang=" + encodeURIComponent(lang);
        }
        a.className = "city-card rail-card";
        a.setAttribute("data-city-id", item.id);

        var top = document.createElement("div");
        top.className = "rail-card__top";
        var nameEl = document.createElement("span");
        nameEl.className = "rail-card__name";
        nameEl.textContent = item.name || "";
        var iconEl = document.createElement("span");
        iconEl.className = "rail-card__icon";
        iconEl.setAttribute("aria-hidden", "true");
        iconEl.textContent = item.icon || "";
        top.appendChild(nameEl);
        top.appendChild(iconEl);

        var tempWrap = document.createElement("div");
        tempWrap.className = "rail-card__temp";
        var tempVal = document.createElement("span");
        tempVal.className = "rail-card__temp-val";
        tempVal.textContent = item.weatherUnavailable ? "—" : String(Math.round(item.temperature));
        var tempUnit = document.createElement("span");
        tempUnit.className = "rail-card__temp-unit";
        tempUnit.textContent = "°";
        tempWrap.appendChild(tempVal);
        tempWrap.appendChild(tempUnit);

        var desc = document.createElement("p");
        desc.className = "rail-card__desc";
        desc.textContent = item.description || "";

        var footer = document.createElement("div");
        footer.className = "city-card__footer rail-card__meta";
        var metaWrap = document.createElement("div");
        metaWrap.className = "city-card__meta";

        var windPair = document.createElement("span");
        windPair.className = "meta-pair";
        var wLab = document.createElement("span");
        wLab.className = "meta-pair__label";
        wLab.textContent = windLabel || "";
        var wVal = document.createElement("span");
        wVal.className = "meta-pair__value";
        wVal.textContent = item.weatherUnavailable
            ? "—"
            : String(Math.round(item.windSpeed)) + (clientI18n.windSuffix || "");
        windPair.appendChild(wLab);
        windPair.appendChild(wVal);

        var humPair = document.createElement("span");
        humPair.className = "meta-pair";
        var hLab = document.createElement("span");
        hLab.className = "meta-pair__label";
        hLab.textContent = humLabel || "";
        var hVal = document.createElement("span");
        hVal.className = "meta-pair__value";
        hVal.textContent = item.weatherUnavailable
            ? "—"
            : String(Math.round(item.humidity)) + (clientI18n.humiditySuffix || "%");
        humPair.appendChild(hLab);
        humPair.appendChild(hVal);

        metaWrap.appendChild(windPair);
        metaWrap.appendChild(humPair);
        var chev = document.createElement("span");
        chev.className = "rail-card__chev";
        chev.setAttribute("aria-hidden", "true");
        chev.textContent = "→";
        footer.appendChild(metaWrap);
        footer.appendChild(chev);

        a.appendChild(top);
        a.appendChild(tempWrap);
        a.appendChild(desc);
        a.appendChild(footer);
        rail.appendChild(a);
    }

    function loadFavorites() {
        var rail = document.getElementById("js-favorites-rail");
        var emptyEl = document.getElementById("js-no-favorites");
        var section = document.getElementById("cities");
        if (!rail || !emptyEl) return;

        var ids;
        try {
            ids = JSON.parse(localStorage.getItem("weather_favorites") || "[]");
        } catch (e) {
            ids = [];
        }
        if (!Array.isArray(ids)) ids = [];
        ids = ids
            .map(function (x) {
                return String(x).trim();
            })
            .filter(Boolean);

        if (ids.length === 0) {
            rail.innerHTML = "";
            emptyEl.hidden = false;
            return;
        }

        emptyEl.hidden = true;
        rail.innerHTML = "";
        var lang = document.documentElement.lang || "ru";
        var qs =
            "ids=" + encodeURIComponent(ids.join(",")) + "&lang=" + encodeURIComponent(lang);

        fetch("/api/favorites?" + qs)
            .then(function (res) {
                if (!res.ok) throw new Error("bad status");
                return res.json();
            })
            .then(function (data) {
                if (!Array.isArray(data)) data = [];
                var windLabel = (section && section.dataset.metricWind) || "";
                var humLabel = (section && section.dataset.metricHumidity) || "";
                for (var i = 0; i < data.length; i++) {
                    appendFavoriteCardHome(rail, data[i], lang, windLabel, humLabel);
                }
                if (data.length === 0) {
                    emptyEl.hidden = false;
                }
            })
            .catch(function () {
                rail.innerHTML = "";
                emptyEl.hidden = false;
            });
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", loadFavorites);
    } else {
        loadFavorites();
    }

    window.addEventListener("weather-theme-change", function () {
        renderTempChart();
    });
})();

// --- Drag-to-Scroll для почасового прогноза ---
function initDragToScroll() {
    var slider =
        document.querySelector(".hourly-scroll") ||
        document.querySelector(".hourly-forecast") ||
        document.querySelector(".hourly-forecast-container");
    if (!slider) return;
    if (slider.getAttribute("data-drag-scroll-init") === "1") return;
    slider.setAttribute("data-drag-scroll-init", "1");

    var isDown = false;
    var startX = 0;
    var startScrollLeft = 0;

    slider.style.cursor = "grab";

    slider.addEventListener("mousedown", function (e) {
        isDown = true;
        slider.style.cursor = "grabbing";
        startX = e.pageX;
        startScrollLeft = slider.scrollLeft;
    });

    slider.addEventListener("mouseleave", function () {
        isDown = false;
        slider.style.cursor = "grab";
    });

    slider.addEventListener("mouseup", function () {
        isDown = false;
        slider.style.cursor = "grab";
    });

    slider.addEventListener("mousemove", function (e) {
        if (!isDown) return;
        e.preventDefault();
        var walk = (e.pageX - startX) * 1.5;
        slider.scrollLeft = startScrollLeft - walk;
    });
}

if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initDragToScroll);
} else {
    initDragToScroll();
}
