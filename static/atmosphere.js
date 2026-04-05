/**
 * Живая Атмосфера — фиксированный слой #weather-bg.
 * Коды OpenWeather (200–899): дождь 5xx, снег 6xx, облака 801–804 (80x), ясно 800.
 * Иначе трактуем как WMO 0–99 (как в /api/weather).
 */
(function (global) {
    "use strict";

    var CLOUD_PATH =
        "M28 48h64c10 0 18-8 18-18 0-8-5-15-13-17 2-11-11-20-22-17-4-14-25-16-34-3-9-2-15 5-15 14v1c-8 2-14 9-14 17 0 10 8 18 18 18z";

    /**
     * @returns {boolean} True if user prefers reduced motion (skips canvas precipitation loop).
     */
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
     * fx: none | rain | snow | svg-clouds | cloud-mesh (WMO 3) | cloud-layers — 4×.cloud-fog (WMO 1,2,45,48)
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
            if (c === 3) return { sky: "cloudy", fx: "cloud-mesh" };
            if ([1, 2].indexOf(c) >= 0 || c === 45 || c === 48) return { sky: "cloudy", fx: "cloud-layers" };
            return { sky: "cloudy", fx: "none" };
        }
        if (c === 0) return { sky: "clear", fx: "none" };
        if (c === 3) return { sky: "cloudy", fx: "cloud-mesh" };
        if ([1, 2].indexOf(c) >= 0 || c === 45 || c === 48) return { sky: "cloudy", fx: "cloud-layers" };
        if ((c >= 51 && c <= 67) || (c >= 80 && c <= 82) || c >= 95) return { sky: "cloudy", fx: "rain" };
        if ((c >= 71 && c <= 77) || c === 85 || c === 86) return { sky: "cloudy", fx: "snow" };
        return { sky: "cloudy", fx: "none" };
    }

    /**
     * Background atmosphere controller: DOM layers + canvas for rain/snow particles.
     * @class
     */
    function Atmosphere() {
        this._raf = null;
        this._precip = null;
        this._rainHeavy = false;
        this._drops = [];
        this._root = null;
        this._canvas = null;
        this._starsCanvas = null;
        this._starsRaf = null;
        this._stars = [];
        this._lightCloudsCanvas = null;
        this._lightCloudsRaf = null;
        this._lightClouds = [];
        this._lightCloudsLastT = 0;
        this._cloudsLayer = null;
        this._meshLayer = null;
        this._resizeObs = null;
        this._themeCanvasFade = null;
        this._themeFadeRaf = null;
        this._ensureDom();
        var self = this;
        try {
            global.addEventListener("weather-theme-change", function (ev) {
                var d = ev && ev.detail;
                if (
                    prefersReducedMotion() ||
                    !d ||
                    d.changed === false ||
                    d.fromDark === d.toDark
                ) {
                    self._endThemeCanvasFadeImmediate();
                    self._syncFromWeatherApp();
                    return;
                }
                self._beginThemeCanvasFade(d.fromDark, d.toDark);
            });
        } catch (e) {}
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
        const isLightTheme =
            document.documentElement.classList.contains("theme-light") ||
            document.documentElement.getAttribute("data-theme") === "light";
        this.update(code, night, isLightTheme);
    };

    /** @returns {void} */
    Atmosphere.prototype._endThemeCanvasFadeImmediate = function () {
        if (this._themeFadeRaf) {
            global.cancelAnimationFrame(this._themeFadeRaf);
            this._themeFadeRaf = null;
        }
        this._themeCanvasFade = null;
        if (this._starsCanvas) {
            this._starsCanvas.style.opacity = "";
            this._starsCanvas.style.visibility = "";
        }
        if (this._lightCloudsCanvas) {
            this._lightCloudsCanvas.style.opacity = "";
            this._lightCloudsCanvas.style.visibility = "";
        }
    };

    /**
     * 300ms cross-fade stars ↔ light clouds (rAF + inline opacity; CSS transition disabled on these canvases).
     * @param {boolean} fromDark
     * @param {boolean} toDark
     * @returns {void}
     */
    Atmosphere.prototype._beginThemeCanvasFade = function (fromDark, toDark) {
        var self = this;
        self._endThemeCanvasFadeImmediate();
        self._ensureDom();
        var toLight = !toDark;
        self._themeCanvasFade = { start: global.performance.now(), toLight: toLight };
        if (toLight) {
            if (self._starsCanvas) {
                self._starsCanvas.style.visibility = "visible";
                self._starsCanvas.style.opacity = "1";
            }
            if (self._lightCloudsCanvas) {
                self._lightCloudsCanvas.style.visibility = "visible";
                self._lightCloudsCanvas.style.opacity = "0";
            }
        } else {
            if (self._lightCloudsCanvas) {
                self._lightCloudsCanvas.style.visibility = "visible";
                self._lightCloudsCanvas.style.opacity = "1";
            }
            if (self._starsCanvas) {
                self._starsCanvas.style.visibility = "visible";
                self._starsCanvas.style.opacity = "0";
            }
        }
        self._syncFromWeatherApp();

        function tick() {
            var fade = self._themeCanvasFade;
            if (!fade) return;
            var elapsed = global.performance.now() - fade.start;
            var t = Math.min(1, elapsed / 300);
            if (fade.toLight) {
                if (self._starsCanvas) self._starsCanvas.style.opacity = String(1 - t);
                if (self._lightCloudsCanvas) self._lightCloudsCanvas.style.opacity = String(t);
            } else {
                if (self._lightCloudsCanvas) self._lightCloudsCanvas.style.opacity = String(1 - t);
                if (self._starsCanvas) self._starsCanvas.style.opacity = String(t);
            }
            if (t < 1) {
                self._themeFadeRaf = global.requestAnimationFrame(tick);
            } else {
                self._themeFadeRaf = null;
                self._themeCanvasFade = null;
                if (self._starsCanvas) {
                    self._starsCanvas.style.opacity = "";
                    self._starsCanvas.style.visibility = "";
                }
                if (self._lightCloudsCanvas) {
                    self._lightCloudsCanvas.style.opacity = "";
                    self._lightCloudsCanvas.style.visibility = "";
                }
                self._syncFromWeatherApp();
            }
        }
        self._themeFadeRaf = global.requestAnimationFrame(tick);
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
                '<canvas class="weather-bg__stars-canvas" aria-hidden="true"></canvas>' +
                '<canvas class="weather-bg__light-clouds-canvas" aria-hidden="true"></canvas>' +
                '<div class="weather-bg__mesh" aria-hidden="true"></div>' +
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
        if (el && !el.querySelector(".weather-bg__stars-canvas")) {
            var starsEl = document.createElement("canvas");
            starsEl.className = "weather-bg__stars-canvas";
            starsEl.setAttribute("aria-hidden", "true");
            el.insertBefore(starsEl, el.firstChild);
        }
        if (el && !el.querySelector(".weather-bg__light-clouds-canvas")) {
            var lc = document.createElement("canvas");
            lc.className = "weather-bg__light-clouds-canvas";
            lc.setAttribute("aria-hidden", "true");
            var starsRef2 = el.querySelector(".weather-bg__stars-canvas");
            if (starsRef2) {
                el.insertBefore(lc, starsRef2.nextSibling);
            } else {
                el.insertBefore(lc, el.firstChild);
            }
        }
        if (el && !el.querySelector(".weather-bg__mesh")) {
            var mesh = document.createElement("div");
            mesh.className = "weather-bg__mesh";
            mesh.setAttribute("aria-hidden", "true");
            var lcMesh = el.querySelector(".weather-bg__light-clouds-canvas");
            if (lcMesh) {
                el.insertBefore(mesh, lcMesh.nextSibling);
            } else {
                var starsRef = el.querySelector(".weather-bg__stars-canvas");
                if (starsRef) {
                    el.insertBefore(mesh, starsRef.nextSibling);
                } else {
                    el.insertBefore(mesh, el.firstChild);
                }
            }
        }
        this._root = el;
        this._starsCanvas = el.querySelector(".weather-bg__stars-canvas");
        this._lightCloudsCanvas = el.querySelector(".weather-bg__light-clouds-canvas");
        this._meshLayer = el.querySelector(".weather-bg__mesh");
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

    /**
     * Sizes a canvas to the container with DPR scaling; optional 2d context transform.
     * @param {HTMLCanvasElement|null} canvas
     * @param {HTMLElement|null} container
     * @returns {void}
     */
    Atmosphere.prototype._sizeCanvasToRoot = function (canvas, container) {
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
    };

    /** @returns {void} */
    Atmosphere.prototype._reseedStars = function () {
        if (!this._starsCanvas || !this._root) return;
        var w = this._root.clientWidth || global.innerWidth || 320;
        var h = this._root.clientHeight || global.innerHeight || 480;
        var n = 60 + Math.floor(Math.random() * 21);
        var stars = [];
        var i;
        for (i = 0; i < n; i++) {
            stars.push({
                x: Math.random() * w,
                y: Math.random() * h * 0.6,
                r: 0.3 + Math.random() * 1.2,
                phase: Math.random() * Math.PI * 2,
            });
        }
        this._stars = stars;
    };

    /** @returns {void} */
    Atmosphere.prototype._stopStars = function () {
        if (this._starsRaf) {
            global.cancelAnimationFrame(this._starsRaf);
            this._starsRaf = null;
        }
        this._stars = [];
        if (this._starsCanvas && this._root) {
            var ctx = this._starsCanvas.getContext("2d");
            if (ctx) {
                var w = this._root.clientWidth || global.innerWidth;
                var h = this._root.clientHeight || global.innerHeight;
                ctx.save();
                ctx.setTransform(1, 0, 0, 1, 0, 0);
                ctx.clearRect(0, 0, this._starsCanvas.width, this._starsCanvas.height);
                ctx.restore();
            }
        }
    };

    /**
     * Twinkling starfield for dark theme only (see atmosphere.css).
     * @returns {void}
     */
    Atmosphere.prototype._starsFrame = function () {
        var self = this;
        var canvas = self._starsCanvas;
        var container = self._root;
        if (!canvas || !container || !canvas.isConnected) {
            self._starsRaf = null;
            return;
        }
        var isLight =
            document.documentElement.classList.contains("theme-light") ||
            document.documentElement.getAttribute("data-theme") === "light";
        var fadingStarsOut = self._themeCanvasFade && self._themeCanvasFade.toLight;
        if ((isLight && !fadingStarsOut) || prefersReducedMotion()) {
            self._stopStars();
            return;
        }
        var w = container.clientWidth || global.innerWidth;
        var h = container.clientHeight || global.innerHeight;
        var ctx = canvas.getContext("2d");
        if (!ctx) return;
        if (self._stars.length === 0) self._reseedStars();
        var t = performance.now() / 1000;
        ctx.clearRect(0, 0, w, h);
        var stars = self._stars;
        var i;
        for (i = 0; i < stars.length; i++) {
            var s = stars[i];
            var alpha = 0.25 + 0.55 * (0.5 + 0.5 * Math.sin(t + s.phase));
            ctx.fillStyle = "rgba(200,225,255," + alpha + ")";
            ctx.beginPath();
            ctx.arc(s.x, s.y, s.r, 0, Math.PI * 2);
            ctx.fill();
        }
        self._starsRaf = global.requestAnimationFrame(function () {
            self._starsFrame();
        });
    };

    /**
     * @param {boolean} isLightTheme
     * @returns {void}
     */
    Atmosphere.prototype._syncStars = function (isLightTheme) {
        if (isLightTheme || prefersReducedMotion()) {
            var fadingStarsOut = this._themeCanvasFade && this._themeCanvasFade.toLight;
            if (!fadingStarsOut) {
                this._stopStars();
                return;
            }
        }
        this._ensureDom();
        if (this._stars.length === 0) this._reseedStars();
        if (!this._starsRaf) this._starsFrame();
    };

    /**
     * Five soft drifting clouds (light theme only): multi-ellipse radial gradients.
     * @returns {void}
     */
    Atmosphere.prototype._initLightCloudModels = function () {
        if (!this._root) return;
        var w = this._root.clientWidth || global.innerWidth || 800;
        var h = this._root.clientHeight || global.innerHeight || 600;
        var templates = [
            {
                yPct: 0.06,
                scale: 1,
                speed: 6.5,
                alpha: 0.42,
                blobs: [
                    { dx: 0, dy: 0, rx: 58, ry: 34 },
                    { dx: -40, dy: 7, rx: 42, ry: 27 },
                    { dx: 44, dy: 9, rx: 38, ry: 24 },
                    { dx: -12, dy: -14, rx: 32, ry: 20 },
                ],
            },
            {
                yPct: 0.2,
                scale: 0.78,
                speed: 10,
                alpha: 0.35,
                blobs: [
                    { dx: 0, dy: 0, rx: 50, ry: 30 },
                    { dx: -34, dy: 5, rx: 36, ry: 22 },
                    { dx: 32, dy: 7, rx: 30, ry: 20 },
                ],
            },
            {
                yPct: 0.36,
                scale: 0.95,
                speed: 7.8,
                alpha: 0.4,
                blobs: [
                    { dx: 0, dy: 0, rx: 62, ry: 36 },
                    { dx: -48, dy: 10, rx: 44, ry: 28 },
                    { dx: 50, dy: 6, rx: 40, ry: 26 },
                    { dx: 8, dy: -16, rx: 34, ry: 22 },
                ],
            },
            {
                yPct: 0.52,
                scale: 0.65,
                speed: 12,
                alpha: 0.32,
                blobs: [
                    { dx: 0, dy: 0, rx: 44, ry: 26 },
                    { dx: -28, dy: 4, rx: 30, ry: 18 },
                    { dx: 26, dy: 6, rx: 26, ry: 16 },
                ],
            },
            {
                yPct: 0.68,
                scale: 0.88,
                speed: 8.8,
                alpha: 0.38,
                blobs: [
                    { dx: 0, dy: 0, rx: 54, ry: 32 },
                    { dx: -36, dy: 8, rx: 40, ry: 25 },
                    { dx: 38, dy: 5, rx: 36, ry: 23 },
                    { dx: -6, dy: -12, rx: 28, ry: 18 },
                ],
            },
        ];
        var clouds = [];
        var i;
        var j;
        for (i = 0; i < templates.length; i++) {
            var tm = templates[i];
            var minX = 0;
            var maxX = 0;
            for (j = 0; j < tm.blobs.length; j++) {
                var bl = tm.blobs[j];
                var sc = tm.scale;
                minX = Math.min(minX, bl.dx * sc - bl.rx * sc);
                maxX = Math.max(maxX, bl.dx * sc + bl.rx * sc);
            }
            var span = maxX - minX + 60;
            clouds.push({
                x: (i / templates.length) * (w + span) - span * 0.4 + Math.random() * 40,
                y: h * tm.yPct,
                scale: tm.scale,
                speed: tm.speed,
                alpha: tm.alpha,
                blobs: tm.blobs,
                span: span,
            });
        }
        this._lightClouds = clouds;
    };

    /** @returns {void} */
    Atmosphere.prototype._stopLightClouds = function () {
        if (this._lightCloudsRaf) {
            global.cancelAnimationFrame(this._lightCloudsRaf);
            this._lightCloudsRaf = null;
        }
        this._lightClouds = [];
        this._lightCloudsLastT = 0;
        if (this._lightCloudsCanvas && this._root) {
            var ctx = this._lightCloudsCanvas.getContext("2d");
            if (ctx) {
                ctx.save();
                ctx.setTransform(1, 0, 0, 1, 0, 0);
                ctx.clearRect(0, 0, this._lightCloudsCanvas.width, this._lightCloudsCanvas.height);
                ctx.restore();
            }
        }
    };

    /**
     * @returns {void}
     */
    Atmosphere.prototype._lightCloudsFrame = function () {
        var self = this;
        var canvas = self._lightCloudsCanvas;
        var container = self._root;
        if (!canvas || !container || !canvas.isConnected) {
            self._lightCloudsRaf = null;
            return;
        }
        var isLight =
            document.documentElement.classList.contains("theme-light") ||
            document.documentElement.getAttribute("data-theme") === "light";
        var fadingCloudsOut = self._themeCanvasFade && !self._themeCanvasFade.toLight;
        if ((!isLight && !fadingCloudsOut) || prefersReducedMotion()) {
            self._stopLightClouds();
            return;
        }
        var w = container.clientWidth || global.innerWidth;
        var h = container.clientHeight || global.innerHeight;
        var ctx = canvas.getContext("2d");
        if (!ctx) return;
        if (self._lightClouds.length === 0) self._initLightCloudModels();

        var nowMs = performance.now();
        var last = self._lightCloudsLastT || nowMs;
        var dt = Math.min(0.045, (nowMs - last) / 1000);
        self._lightCloudsLastT = nowMs;

        ctx.clearRect(0, 0, w, h);
        var clouds = self._lightClouds;
        var i;
        var b;
        for (i = 0; i < clouds.length; i++) {
            var c = clouds[i];
            c.x += c.speed * dt;
            if (c.x > w + c.span) {
                c.x = -c.span - 20 - Math.random() * 100;
                c.y = h * (0.05 + Math.random() * 0.65);
            }
            ctx.save();
            ctx.globalAlpha = c.alpha;
            for (b = 0; b < c.blobs.length; b++) {
                var bl = c.blobs[b];
                var cx = c.x + bl.dx * c.scale;
                var cy = c.y + bl.dy * c.scale;
                var rx = bl.rx * c.scale;
                var ry = bl.ry * c.scale;
                var rad = Math.max(rx, ry);
                var g = ctx.createRadialGradient(cx, cy, 0, cx, cy, rad);
                g.addColorStop(0, "rgba(255,255,255,0.95)");
                g.addColorStop(1, "rgba(200,235,255,0)");
                ctx.beginPath();
                ctx.ellipse(cx, cy, rx, ry, 0, 0, Math.PI * 2);
                ctx.fillStyle = g;
                ctx.fill();
            }
            ctx.restore();
        }

        self._lightCloudsRaf = global.requestAnimationFrame(function () {
            self._lightCloudsFrame();
        });
    };

    /**
     * @param {boolean} isLightTheme
     * @returns {void}
     */
    Atmosphere.prototype._syncLightClouds = function (isLightTheme) {
        if (prefersReducedMotion()) {
            this._stopLightClouds();
            return;
        }
        if (!isLightTheme) {
            var fadingCloudsOut = this._themeCanvasFade && !this._themeCanvasFade.toLight;
            if (!fadingCloudsOut) {
                this._stopLightClouds();
                return;
            }
        }
        this._ensureDom();
        if (this._lightClouds.length === 0) this._initLightCloudModels();
        if (!this._lightCloudsRaf) this._lightCloudsFrame();
    };

    Atmosphere.prototype._resizeCanvas = function () {
        var container = this._root;
        if (!container) return;
        this._sizeCanvasToRoot(this._canvas, container);
        this._sizeCanvasToRoot(this._starsCanvas, container);
        this._sizeCanvasToRoot(this._lightCloudsCanvas, container);
        this._drops = [];
        this._reseedStars();
        var lightNow =
            document.documentElement.getAttribute("data-theme") === "light" ||
            document.documentElement.classList.contains("theme-light");
        if (lightNow && !prefersReducedMotion()) {
            this._initLightCloudModels();
        } else {
            this._lightClouds = [];
        }
    };

    Atmosphere.prototype._stopPrecip = function () {
        if (this._raf) {
            global.cancelAnimationFrame(this._raf);
            this._raf = null;
        }
        this._precip = null;
        this._rainHeavy = false;
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

    /**
     * @param {"rain"|"snow"} kind
     * @param {{ rainHeavy?: boolean }} [opts] — OpenWeather 502/503: чуть быстрее обычного дождя
     */
    Atmosphere.prototype._runPrecip = function (kind, opts) {
        var self = this;
        opts = opts || {};
        this._stopPrecip();
        this._precip = kind;
        this._rainHeavy = kind === "rain" && !!opts.rainHeavy;
        var canvas = this._canvas;
        var container = this._root;
        if (!canvas || !container || prefersReducedMotion()) return;

        /**
         * Seeds particle array: rain uses vertical streaks (length + speed); snow uses small circles with drift.
         * Coordinates are in **CSS pixel space** of the container — `_resizeCanvas` sets ctx transform to DPR,
         * so drawing uses logical pixels while the backing store is high-DPI.
         * @param {number} w
         * @param {number} h
         * @returns {void}
         */
        function initDrops(w, h) {
            var n = kind === "snow" ? 80 : 120;
            var drops = [];
            var i;
            for (i = 0; i < n; i++) {
                if (kind === "rain") {
                    /* Умеренный дождь сверху вниз: ниже скорость, почти без ветрового сноса. */
                    var spdLo = self._rainHeavy ? 3.2 : 2.2;
                    var spdSpan = self._rainHeavy ? 1.8 : 1.4;
                    var lenLo = 14;
                    var lenSpan = 10;
                    drops.push({
                        x: Math.random() * w,
                        y: Math.random() * h,
                        len: lenLo + Math.random() * lenSpan,
                        speed: spdLo + Math.random() * spdSpan,
                        drift: 0,
                        r: 0,
                    });
                } else {
                    drops.push({
                        x: Math.random() * w,
                        y: Math.random() * h,
                        len: 0,
                        speed: 0.7 + Math.random() * 2.6,
                        drift: (Math.random() - 0.5) * 1.8,
                        r: 0.9 + Math.random() * 2.2,
                    });
                }
            }
            self._drops = drops;
        }

        /**
         * rAF loop: clears full logical viewport, draws streaks (rain) or arcs (snow), advances `y`/`x` each frame.
         * Particles wrap from bottom (`y > h`) back to top with randomized `x` for continuous effect.
         * @returns {void}
         */
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
                var heavy = self._rainHeavy;
                ctx.globalAlpha = heavy ? 0.33 : 0.28;
                ctx.strokeStyle = "rgba(200, 230, 255, 0.55)";
                ctx.lineWidth = heavy ? 0.62 : 0.48;
                var wind = heavy ? 0.05 : 0;
                for (i = 0; i < drops.length; i++) {
                    d = drops[i];
                    ctx.beginPath();
                    ctx.moveTo(d.x, d.y);
                    ctx.lineTo(d.x, d.y + d.len);
                    ctx.stroke();
                    d.y += d.speed;
                    d.x -= wind;
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

    /** WMO 3: mesh — три больших градиентных слоя (без cloud-fog). */
    Atmosphere.prototype._buildCloudMesh = function () {
        if (!this._cloudsLayer) return;
        this._cloudsLayer.innerHTML = "";
        var i;
        for (i = 1; i <= 3; i++) {
            var mesh = document.createElement("div");
            mesh.className = "sky-mesh sky-mesh--" + i;
            mesh.setAttribute("aria-hidden", "true");
            this._cloudsLayer.appendChild(mesh);
        }
    };

    /** WMO 1, 2, 45, 48: дымка (CSS), без SVG. */
    Atmosphere.prototype._buildCloudLayers = function () {
        if (!this._cloudsLayer) return;
        this._cloudsLayer.innerHTML = "";
        var i;
        for (i = 0; i < 4; i++) {
            var fog = document.createElement("div");
            fog.className = "cloud-fog";
            fog.setAttribute("aria-hidden", "true");
            this._cloudsLayer.appendChild(fog);
        }
    };

    /** Positions several animated SVG cloud puffs (OpenWeather-style broken clouds 801–804). */
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

    /** @returns {void} */
    Atmosphere.prototype._clearCloudsLayer = function () {
        if (this._cloudsLayer) this._cloudsLayer.innerHTML = "";
    };

    /** @returns {void} */
    Atmosphere.prototype._clearMeshLayer = function () {
        if (this._meshLayer) this._meshLayer.innerHTML = "";
    };

    /**
     * @param {number} weatherCode — OpenWeather id или WMO
     * @param {boolean} [isNight] — иначе берётся из .weather-app[data-is-night]
     */
    Atmosphere.prototype.update = function (weatherCode, isNight, isLightThemeCached) {
        this._ensureDom();
        var app = document.querySelector(".weather-app");
        if (isNight === undefined && app) {
            isNight = app.dataset.isNight === "true";
        }
        var state = resolveState(weatherCode, isNight);
        var night = isNight === true || isNight === "true";
        var el = this._root;
        if (!el) return;

        var cNum = typeof weatherCode === "number" ? weatherCode : parseInt(weatherCode, 10);
        if (Number.isNaN(cNum)) cNum = 0;

        var isLightTheme =
            isLightThemeCached !== undefined
                ? isLightThemeCached
                : document.documentElement.classList.contains("theme-light") ||
                  document.documentElement.getAttribute("data-theme") === "light";

        el.classList.remove("weather-bg--apple-light-mesh");
        this._clearMeshLayer();

        el.setAttribute("data-wmo", String(weatherCode));

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
            "weather-bg--fx-clouds",
            "weather-bg--fx-cloud-layers",
            "weather-bg--fx-cloud-mesh"
        );
        this._stopPrecip();
        this._clearCloudsLayer();

        // Bug-fix: ensure canvas is fully cleared on every update to avoid "stuck" drops.
        if (this._canvas) {
            try {
                var ctxClear = this._canvas.getContext("2d");
                if (ctxClear) {
                    ctxClear.save();
                    ctxClear.setTransform(1, 0, 0, 1, 0, 0);
                    ctxClear.clearRect(0, 0, this._canvas.width, this._canvas.height);
                    ctxClear.restore();
                }
            } catch (_) {
                /* ignore */
            }
        }

        if (prefersReducedMotion()) {
            this._endThemeCanvasFadeImmediate();
            this._stopStars();
            this._stopLightClouds();
            el.classList.add("weather-bg--fx-none");
            return;
        }

        if (isLightTheme) {
            if (state.sky === "cloudy" && (state.fx === "none" || !state.fx)) {
                state.fx = "none";
            }
            if (state.fx === "svg-clouds" || state.fx === "cloud-mesh" || state.fx === "cloud-layers") {
                state.fx = "none";
            }
        } else if (state.sky === "cloudy" && (state.fx === "none" || !state.fx)) {
            state.fx = "svg-clouds";
        }

        if (state.fx === "rain") {
            el.classList.add("weather-bg--fx-rain");
            var self = this;
            var cNum = typeof weatherCode === "number" ? weatherCode : parseInt(weatherCode, 10);
            var rainHeavy = !Number.isNaN(cNum) && (cNum === 502 || cNum === 503);
            global.requestAnimationFrame(function () {
                if (el.classList.contains("weather-bg--fx-rain")) self._runPrecip("rain", { rainHeavy: rainHeavy });
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
        } else if (state.fx === "cloud-mesh") {
            el.classList.add("weather-bg--fx-cloud-mesh");
            this._buildCloudMesh();
        } else if (state.fx === "cloud-layers") {
            el.classList.add("weather-bg--fx-cloud-layers");
            this._buildCloudLayers();
        } else {
            el.classList.add("weather-bg--fx-none");
        }

        this._syncStars(isLightTheme);
        this._syncLightClouds(isLightTheme);
    };

    global.Atmosphere = Atmosphere;
    global.atmosphere = new Atmosphere();
})(typeof window !== "undefined" ? window : this);
