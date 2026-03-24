/**
 * Живая Атмосфера — фиксированный слой #weather-bg.
 * Коды OpenWeather (200–899): дождь 5xx, снег 6xx, облака 801–804 (80x), ясно 800.
 * Иначе трактуем как WMO 0–99 (как в /api/weather).
 */
(function (global) {
    "use strict";

    var CLOUD_PATH =
        "M28 48h64c10 0 18-8 18-18 0-8-5-15-13-17 2-11-11-20-22-17-4-14-25-16-34-3-9-2-15 5-15 14v1c-8 2-14 9-14 17 0 10 8 18 18 18z";

    function prefersReducedMotion() {
        try {
            return global.matchMedia && global.matchMedia("(prefers-reduced-motion: reduce)").matches;
        } catch (e) {
            return false;
        }
    }

    /**
     * @returns {{ sky: string, fx: string }}
     * sky: clear | cloudy | rain | snow | storm
     * fx: none | rain | snow | svg-clouds
     */
    function resolveState(code, isNight) {
        var c = typeof code === "number" ? code : parseInt(code, 10);
        if (Number.isNaN(c)) c = 0;

        /* OpenWeather condition id (обычно ≥ 200) */
        if (c >= 200 && c < 900) {
            if (c >= 200 && c < 300) return { sky: "storm", fx: "rain" };
            if (c >= 300 && c < 400) return { sky: "cloudy", fx: "rain" };
            if (c >= 500 && c < 600) return { sky: "cloudy", fx: "rain" };
            if (c >= 600 && c < 700) return { sky: "cloudy", fx: "snow" };
            if (c >= 700 && c < 800) return { sky: "cloudy", fx: "none" };
            if (c === 800) return { sky: "clear", fx: "none" };
            if (c >= 801 && c <= 804) return { sky: "cloudy", fx: "svg-clouds" };
            return { sky: "cloudy", fx: "svg-clouds" };
        }

        /* WMO (Open-Meteo) */
        var n = isNight === true || isNight === "true";
        if (n) {
            if ((c >= 51 && c <= 67) || (c >= 80 && c <= 82) || c >= 95) return { sky: "rain", fx: "rain" };
            if ((c >= 71 && c <= 77) || c === 85 || c === 86) return { sky: "snow", fx: "snow" };
            if (c === 0) return { sky: "clear", fx: "none" };
            if ([1, 2, 3].indexOf(c) >= 0 || c === 45 || c === 48) return { sky: "cloudy", fx: "svg-clouds" };
            return { sky: "cloudy", fx: "none" };
        }
        if (c === 0) return { sky: "clear", fx: "none" };
        if ([1, 2, 3].indexOf(c) >= 0 || c === 45 || c === 48) return { sky: "cloudy", fx: "svg-clouds" };
        if ((c >= 51 && c <= 67) || (c >= 80 && c <= 82) || c >= 95) return { sky: "cloudy", fx: "rain" };
        if ((c >= 71 && c <= 77) || c === 85 || c === 86) return { sky: "cloudy", fx: "snow" };
        return { sky: "cloudy", fx: "none" };
    }

    function Atmosphere() {
        this._raf = null;
        this._precip = null;
        this._drops = [];
        this._root = null;
        this._canvas = null;
        this._cloudsLayer = null;
        this._resizeObs = null;
        this._ensureDom();
        this._syncFromWeatherApp();
    }

    /** Первичная отрисовка из data-weather-code / data-is-night на .weather-app (SSR). */
    Atmosphere.prototype._syncFromWeatherApp = function () {
        var app = document.querySelector(".weather-app");
        if (!app) {
            var self = this;
            if (document.readyState === "loading") {
                document.addEventListener("DOMContentLoaded", function () {
                    self._syncFromWeatherApp();
                });
            }
            return;
        }
        var code = parseInt(app.getAttribute("data-weather-code") || app.dataset.weatherCode || "0", 10);
        if (Number.isNaN(code)) code = 0;
        var night = app.dataset.isNight === "true";
        this.update(code, night);
    };

    Atmosphere.prototype._ensureDom = function () {
        var self = this;
        var el = document.getElementById("weather-bg");
        if (!el) {
            el = document.createElement("div");
            el.id = "weather-bg";
            el.className = "weather-bg weather-bg--sky-cloudy weather-bg--day weather-bg--fx-none";
            el.setAttribute("aria-hidden", "true");
            el.innerHTML =
                '<div class="weather-bg__clouds" aria-hidden="true"></div>' +
                '<canvas class="weather-bg__canvas" aria-hidden="true"></canvas>';
            function mount() {
                if (!el.parentNode && document.body) {
                    document.body.prepend(el);
                    self._resizeCanvas();
                }
            }
            if (document.body) {
                mount();
            } else {
                document.addEventListener("DOMContentLoaded", mount);
            }
        }
        this._root = el;
        this._cloudsLayer = el.querySelector(".weather-bg__clouds");
        this._canvas = el.querySelector(".weather-bg__canvas");
        if (global.ResizeObserver && this._canvas && this._root) {
            if (this._resizeObs) this._resizeObs.disconnect();
            this._resizeObs = new ResizeObserver(function () {
                self._resizeCanvas();
            });
            this._resizeObs.observe(this._root);
        }
        this._resizeCanvas();
    };

    Atmosphere.prototype._resizeCanvas = function () {
        var canvas = this._canvas;
        var container = this._root;
        if (!canvas || !container) return;
        var dpr = global.devicePixelRatio || 1;
        var w = container.clientWidth || global.innerWidth || 320;
        var h = container.clientHeight || global.innerHeight || 480;
        canvas.width = Math.max(2, Math.floor(w * dpr));
        canvas.height = Math.max(2, Math.floor(h * dpr));
        canvas.style.width = w + "px";
        canvas.style.height = h + "px";
        var ctx = canvas.getContext("2d");
        if (ctx) ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        this._drops = [];
    };

    Atmosphere.prototype._stopPrecip = function () {
        if (this._raf) {
            global.cancelAnimationFrame(this._raf);
            this._raf = null;
        }
        this._precip = null;
        this._drops = [];
        if (this._canvas) {
            var ctx = this._canvas.getContext("2d");
            if (ctx) {
                var w = this._root.clientWidth || global.innerWidth;
                var h = this._root.clientHeight || global.innerHeight;
                ctx.clearRect(0, 0, w, h);
            }
        }
    };

    Atmosphere.prototype._runPrecip = function (kind) {
        var self = this;
        this._stopPrecip();
        this._precip = kind;
        var canvas = this._canvas;
        var container = this._root;
        if (!canvas || !container || prefersReducedMotion()) return;

        function initDrops(w, h) {
            var n = kind === "snow" ? 80 : 120;
            var drops = [];
            var i;
            for (i = 0; i < n; i++) {
                drops.push({
                    x: Math.random() * w,
                    y: Math.random() * h,
                    len: kind === "rain" ? 10 + Math.random() * 22 : 0,
                    speed: kind === "rain" ? 15 + Math.random() * 26 : 0.7 + Math.random() * 2.6,
                    drift: kind === "snow" ? (Math.random() - 0.5) * 1.8 : 0,
                    r: kind === "snow" ? 0.9 + Math.random() * 2.2 : 0,
                });
            }
            self._drops = drops;
        }

        function frame() {
            if (!canvas.isConnected || self._precip !== kind) return;
            var w = container.clientWidth || global.innerWidth;
            var h = container.clientHeight || global.innerHeight;
            var ctx = canvas.getContext("2d");
            if (!ctx) return;
            if (self._drops.length === 0) initDrops(w, h);
            ctx.clearRect(0, 0, w, h);
            var drops = self._drops;
            var i;
            var d;
            if (kind === "rain") {
                ctx.globalAlpha = 0.42;
                ctx.strokeStyle = "rgba(186, 230, 253, 0.75)";
                ctx.lineWidth = 1.1;
                for (i = 0; i < drops.length; i++) {
                    d = drops[i];
                    ctx.beginPath();
                    ctx.moveTo(d.x, d.y);
                    ctx.lineTo(d.x - 1.5, d.y + d.len);
                    ctx.stroke();
                    d.y += d.speed;
                    d.x -= 0.5;
                    if (d.y > h + 20) {
                        d.y = -12;
                        d.x = Math.random() * w;
                    }
                }
            } else {
                ctx.globalAlpha = 0.52;
                for (i = 0; i < drops.length; i++) {
                    d = drops[i];
                    ctx.fillStyle = "rgba(248, 250, 252, 0.92)";
                    ctx.beginPath();
                    ctx.arc(d.x, d.y, d.r, 0, Math.PI * 2);
                    ctx.fill();
                    d.y += d.speed;
                    d.x += d.drift;
                    if (d.y > h + 10) {
                        d.y = -6;
                        d.x = Math.random() * w;
                    }
                }
            }
            ctx.globalAlpha = 1;
            self._raf = global.requestAnimationFrame(frame);
        }
        this._raf = global.requestAnimationFrame(frame);
    };

    Atmosphere.prototype._buildSvgClouds = function () {
        if (!this._cloudsLayer) return;
        this._cloudsLayer.innerHTML = "";
        var configs = [
            { top: "6%", left: "-5%", w: 180, dur: 72, delay: 0 },
            { top: "18%", left: "35%", w: 140, dur: 96, delay: -12 },
            { top: "32%", left: "10%", w: 200, dur: 84, delay: -24 },
            { top: "12%", left: "55%", w: 160, dur: 108, delay: -8 },
        ];
        var i;
        for (i = 0; i < configs.length; i++) {
            var cfg = configs[i];
            var wrap = document.createElement("div");
            wrap.className = "weather-bg__cloud-item";
            wrap.style.top = cfg.top;
            wrap.style.left = cfg.left;
            wrap.style.width = cfg.w + "px";
            wrap.style.animationDuration = cfg.dur + "s";
            wrap.style.animationDelay = cfg.delay + "s";
            var svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
            svg.setAttribute("class", "weather-bg__cloud-svg");
            svg.setAttribute("viewBox", "0 0 120 64");
            svg.setAttribute("preserveAspectRatio", "xMidYMid meet");
            var path = document.createElementNS("http://www.w3.org/2000/svg", "path");
            path.setAttribute("d", CLOUD_PATH);
            svg.appendChild(path);
            wrap.appendChild(svg);
            this._cloudsLayer.appendChild(wrap);
        }
    };

    Atmosphere.prototype._clearSvgClouds = function () {
        if (this._cloudsLayer) this._cloudsLayer.innerHTML = "";
    };

    /**
     * @param {number} weatherCode — OpenWeather id или WMO
     * @param {boolean} [isNight] — иначе берётся из .weather-app[data-is-night]
     */
    Atmosphere.prototype.update = function (weatherCode, isNight) {
        console.log("Atmosphere updating with code:", weatherCode);
        this._ensureDom();
        var app = document.querySelector(".weather-app");
        if (isNight === undefined && app) {
            isNight = app.dataset.isNight === "true";
        }
        var state = resolveState(weatherCode, isNight);
        var night = isNight === true || isNight === "true";
        var el = this._root;
        if (!el) return;

        var skies = ["clear", "cloudy", "rain", "snow", "storm"];
        var i;
        for (i = 0; i < skies.length; i++) {
            el.classList.remove("weather-bg--sky-" + skies[i]);
        }
        el.classList.remove("weather-bg--night", "weather-bg--day");
        el.classList.add("weather-bg--sky-" + state.sky);
        el.classList.add(night ? "weather-bg--night" : "weather-bg--day");

        el.classList.remove(
            "weather-bg--fx-none",
            "weather-bg--fx-rain",
            "weather-bg--fx-snow",
            "weather-bg--fx-clouds"
        );
        this._stopPrecip();
        this._clearSvgClouds();

        if (prefersReducedMotion()) {
            el.classList.add("weather-bg--fx-none");
            return;
        }

        if (state.fx === "rain") {
            el.classList.add("weather-bg--fx-rain");
            var self = this;
            global.requestAnimationFrame(function () {
                if (el.classList.contains("weather-bg--fx-rain")) self._runPrecip("rain");
            });
        } else if (state.fx === "snow") {
            el.classList.add("weather-bg--fx-snow");
            var self2 = this;
            global.requestAnimationFrame(function () {
                if (el.classList.contains("weather-bg--fx-snow")) self2._runPrecip("snow");
            });
        } else if (state.fx === "svg-clouds") {
            el.classList.add("weather-bg--fx-clouds");
            this._buildSvgClouds();
        } else {
            el.classList.add("weather-bg--fx-none");
        }
    };

    global.Atmosphere = Atmosphere;
    global.atmosphere = new Atmosphere();
})(typeof window !== "undefined" ? window : this);
