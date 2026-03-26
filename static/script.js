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
        try { return JSON.parse(String(el.textContent).trim() || "{}"); } catch (e) { return {}; }
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

    function mapCodeToMoodClass(code, night) {
        if (night) return "weather-night";
        if (code === 0) return "weather-clear";
        if ([1, 2, 3].includes(code)) return "weather-cloudy";
        if ((code >= 51 && code <= 67) || (code >= 80 && code <= 82) || code >= 95) return "weather-rain";
        if ((code >= 71 && code <= 77) || code === 85 || code === 86) return "weather-snow";
        return "weather-cloudy";
    }

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
        MOOD_CLASSES.forEach(function (m) { el.classList.remove(m); });
        el.classList.add(mood);
        var eng = engineCode !== undefined && engineCode !== null ? Number(engineCode) : c;
        if (typeof atmosphere !== "undefined" && atmosphere && typeof atmosphere.update === "function") {
            atmosphere.update(eng, n);
        }
    }

    function updateBackgroundByTemp(temp) {
        var from = "#1d4ed8", to = "#f97316";
        if (temp < 0) { from = "#0ea5e9"; to = "#1d4ed8"; }
        else if (temp < 15) { from = "#38bdf8"; to = "#0ea5e9"; }
        else if (temp < 25) { from = "#22c55e"; to = "#38bdf8"; }
        else { from = "#f97316"; to = "#fb923c"; }
        var root = document.documentElement;
        root.style.setProperty("--temp-color-from", from);
        root.style.setProperty("--temp-color-to", to);
    }
    window.updateBackgroundByTemp = updateBackgroundByTemp;

    var adviceTextEl = document.getElementById("js-weather-advice-text");
    var adviceElement = adviceTextEl ? adviceTextEl.closest(".weather-advice") : null;

    function isRainWmo(code) {
        return (code >= 51 && code <= 67) || (code >= 80 && code <= 82) || code >= 95;
    }

    // --- ОБНОВЛЕННАЯ ЛОГИКА ОДЕЖДЫ ---
    function updateSmartAdvice(tempC, windKph, humidityPct, isRain) {
        if (!adviceTextEl) return;
        if (typeof tempC !== "number" || Number.isNaN(tempC)) return;
        if (adviceElement) adviceElement.classList.remove("animate-advice");

        let b = "", o = "", acc = [];
        let windMs = (windKph || 0) / 3.6;

        if (tempC < -15) { b = "Термобельё и свитер."; o = "Тяжёлый пуховик или шуба."; }
        else if (tempC < -5) { b = "Плотный джемпер или худи."; o = "Зимняя куртка или парка."; }
        else if (tempC < 5) { b = "Свитер или флиска."; o = "Пальто или лёгкий пуховик."; }
        else if (tempC < 12) { b = "Лонгслив или рубашка."; o = "Бомбер, кожанка или тренч."; }
        else if (tempC < 18) { b = "Футболка."; o = "Джинсовка, ветровка или плотное худи."; }
        else if (tempC < 25) { b = "Футболка или поло."; o = "Верхняя одежда не нужна."; }
        else if (tempC < 30) { b = "Майка или лён."; o = "Жара! Пей больше воды."; }
        else { b = "Минимум одежды."; o = "Очень жарко, ищи тень."; }

        if (tempC < 0) acc.push("шапка", "шарф", "перчатки");
        else if (tempC < 10) acc.push("лёгкая шапка");
        if (windMs > 7) acc.push("непродуваемая одежда");
        if (isRain) acc.push("зонт или дождевик");
        if (tempC > 20 && !isRain) acc.push("солнечные очки");
        if (humidityPct > 80 && tempC < 10) acc.push("термос с чаем");

        let res = `${b} ${o}`;
        if (acc.length > 0) res += " Не забудь: " + acc.join(", ") + ".";
        adviceTextEl.textContent = res;

        if (adviceElement) {
            void adviceElement.offsetWidth;
            adviceElement.classList.add("animate-advice");
        }
    }

    function renderHourlyForecast(hourly) {
        var el = document.getElementById("js-hourly-scroll");
        if (!el || !Array.isArray(hourly)) return;
        el.innerHTML = "";
        hourly.forEach((h, i) => {
            var item = document.createElement("div");
            item.className = "hourly-item" + (i === 0 ? " hourly-item--now" : "");
            item.innerHTML = `<div class="hourly-item__time">${h.time||""}</div>
                              <div class="hourly-item__icon">${h.icon||""}</div>
                              <div class="hourly-item__temp">${typeof h.temperature==='number'?Math.round(h.temperature)+"°":"—"}</div>`;
            el.appendChild(item);
        });
    }

    function numFromJSON(v) {
        if (typeof v === "number") return v;
        if (typeof v === "string") return parseFloat(v.replace(",", "."));
        return NaN;
    }

    function renderMetrics(data) {
        if (!data || !data.current) return;
        const cur = data.current;
        const set = (id, val) => { let e = document.getElementById(id); if(e) e.textContent = val; };
        
        set("uv-index-val", !isNaN(numFromJSON(cur.uv_index)) ? Math.round(numFromJSON(cur.uv_index)) : "—");
        set("visibility-val", !isNaN(numFromJSON(cur.visibility)) ? Math.round(numFromJSON(cur.visibility)) : "—");
        set("feels-like-val", !isNaN(numFromJSON(cur.apparent_temperature)) ? Math.round(numFromJSON(cur.apparent_temperature)) : "—");
        set("pressure-val", !isNaN(numFromJSON(cur.pressure)) ? Math.round(numFromJSON(cur.pressure)) : "—");
        set("js-current-wind", !isNaN(numFrom