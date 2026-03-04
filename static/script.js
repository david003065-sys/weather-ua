
(function () {
    const app = document.querySelector(".weather-app");
    if (!app) return;

    const weatherCode = parseInt(app.dataset.weatherCode || "0", 10);
    const isNight = app.dataset.isNight === "true";
    const cityId = app.dataset.cityId || "";
    const lang = document.documentElement.lang || "ru";

    function mapCodeToClass(code, night) {
        if (night) return "night";
        if (code === 0) return "sunny";
        if ([1, 2, 3].includes(code)) return "cloudy";
        if ((code >= 51 && code <= 67) || (code >= 80 && code <= 82)) return "rain";
        if ((code >= 71 && code <= 77) || code === 85 || code === 86) return "snow";
        if (code >= 95) return "rain";
        return "cloudy";
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

    const weatherClass = mapCodeToClass(weatherCode, isNight);
    app.classList.remove("sunny", "cloudy", "rain", "snow", "night");
    app.classList.add(weatherClass);

    var initialTempEl = document.getElementById("js-current-temp");
    if (initialTempEl) {
        var initialTemp = parseFloat(initialTempEl.textContent.replace(",", "."));
        if (!Number.isNaN(initialTemp)) {
            updateBackgroundByTemp(initialTemp);
        }
    }

    function initTempChart(canvas) {
        if (!window.Chart || !canvas) return;

        try {
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
                            label: "Макс",
                            data: max,
                            borderColor: "#38bdf8",
                            backgroundColor: "rgba(56, 189, 248, 0.18)",
                            borderWidth: 2,
                            tension: 0.35,
                            fill: true,
                            pointRadius: 3,
                            pointBackgroundColor: "#38bdf8",
                            pointBorderColor: "#0f172a"
                        },
                        {
                            label: "Мин",
                            data: min,
                            borderColor: "#f97316",
                            backgroundColor: "rgba(249, 115, 22, 0.18)",
                            borderWidth: 2,
                            tension: 0.35,
                            fill: true,
                            pointRadius: 3,
                            pointBackgroundColor: "#f97316",
                            pointBorderColor: "#0f172a"
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
                                color: "#e5e7eb",
                                font: { size: 11 }
                            }
                        }
                    },
                    scales: {
                        x: {
                            ticks: { color: "#9ca3af" },
                            grid: { color: "rgba(55, 65, 81, 0.4)" }
                        },
                        y: {
                            ticks: {
                                color: "#9ca3af",
                                callback: function (value) {
                                    return value + "°";
                                }
                            },
                            grid: { color: "rgba(55, 65, 81, 0.4)" }
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
        if (document.readyState === "complete") {
            initTempChart(tempCanvas);
        } else {
            window.addEventListener("load", function () {
                initTempChart(tempCanvas);
            });
        }
    }

    // Geo button
    var geoBtn = document.getElementById("js-geo-btn");
    if (geoBtn && navigator.geolocation) {
        geoBtn.addEventListener("click", function () {
            geoBtn.disabled = true;
            geoBtn.classList.add("btn--loading");
            navigator.geolocation.getCurrentPosition(
                function (pos) {
                    var lat = pos.coords.latitude;
                    var lon = pos.coords.longitude;
                    window.location.href =
                        "/weather/geo?lat=" +
                        encodeURIComponent(lat) +
                        "&lon=" +
                        encodeURIComponent(lon) +
                        "&lang=" +
                        encodeURIComponent(lang);
                },
                function () {
                    geoBtn.disabled = false;
                    geoBtn.classList.remove("btn--loading");
                },
                { enableHighAccuracy: false, timeout: 8000, maximumAge: 300000 }
            );
        });
    }

    // Auto refresh for city page
    if (cityId) {
        var tempEl = document.getElementById("js-current-temp");
        var descEl = document.querySelector(".city-hero__subtitle");
        var windEl = document.getElementById("js-current-wind");
        var humEl = document.getElementById("js-current-humidity");

        function applyWeather(data) {
            if (!data || !data.current) return;
            if (tempEl) tempEl.textContent = Math.round(data.current.temperature);
            if (descEl) descEl.textContent = data.current.description;
            if (windEl) windEl.textContent = Math.round(data.current.wind) + " км/ч";
            if (humEl) humEl.textContent = Math.round(data.current.humidity) + "%";
            updateBackgroundByTemp(data.current.temperature);
        }

        function cacheKey(id, l) {
            return "weatherCache:" + id + ":" + l;
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

    // PWA: register service worker
    if ("serviceWorker" in navigator) {
        window.addEventListener("load", function () {
            navigator.serviceWorker.register("/static/service-worker.js").catch(function (e) {
                console.error("SW registration failed", e);
            });
        });
    }

    // Place search autocomplete on index page
    (function initPlaceSearch() {
        var input = document.getElementById("js-place-search");
        var box = document.getElementById("js-place-suggestions");
        if (!input || !box) return;

        var lang = document.documentElement.lang || "ru";
        var messages = {
            ru: {
                empty: "Ничего не найдено",
                error: "Ошибка поиска",
                tooShort: "Введите минимум 2 символа"
            },
            uk: {
                empty: "Нічого не знайдено",
                error: "Помилка пошуку",
                tooShort: "Введи мінімум 2 символи"
            },
            en: {
                empty: "No results",
                error: "Search error",
                tooShort: "Type at least 2 characters"
            }
        };
        function t(key) {
            var set = messages[lang] || messages.ru;
            return set[key] || messages.ru[key] || "";
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
            if (lang === "uk") return place.type_uk || place.type_ru || place.type_en || "";
            if (lang === "ru") return place.type_ru || place.type_uk || place.type_en || "";
            return place.type_en || place.type_uk || place.type_ru || "";
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
                favBtn.setAttribute("aria-label", "Add to favourites");
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
                    goToPlace(p.id);
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

        function goToPlace(id) {
            if (!id) return;
            var url = "/place/" + encodeURIComponent(String(id));
            if (lang) {
                url += "?lang=" + encodeURIComponent(lang);
            }
            window.location.href = url;
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
                    clearSuggestions();
                    return;
                }
                var json = await res.json();
                // если пришёл старый ответ — игнорируем
                if (requestId !== lastRequestId) {
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
                    var id = items[activeIndex].dataset.id;
                    goToPlace(id);
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
                card.className = "city-card city-card--favorite";

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
                tempVal.textContent = Math.round(data.current.temperature);
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
                removeBtn.title = "Удалить из избранного";
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
                card.className = "city-card city-card--favorite";

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
                tempVal.textContent = Math.round(data.current.temperature);
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
            var seeds = ["Дніпро", "Київ", "Краматорськ"];
            var next = [];
            for (var i = 0; i < seeds.length; i++) {
                try {
                    var res = await fetch("/api/places?q=" + encodeURIComponent(seeds[i]) + "&limit=1&lang=uk");
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
})();
