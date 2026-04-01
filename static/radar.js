(function () {
    "use strict";

    function parseCoord(value) {
        if (value == null) return NaN;
        var n = parseFloat(String(value).replace(",", "."));
        return Number.isFinite(n) ? n : NaN;
    }

    function latestRadarTimestamp(payload) {
        if (!payload || !payload.radar || !Array.isArray(payload.radar.past)) return null;
        if (payload.radar.past.length === 0) return null;
        var last = payload.radar.past[payload.radar.past.length - 1];
        if (!last) return null;
        return typeof last.time === "number" ? last.time : null;
    }

    function recentRadarTimestamps(payload, maxCount) {
        if (!payload || !payload.radar || !Array.isArray(payload.radar.past)) return [];
        var src = payload.radar.past;
        if (src.length === 0) return [];
        var from = Math.max(0, src.length - maxCount);
        var out = [];
        for (var i = from; i < src.length; i++) {
            var item = src[i];
            if (item && typeof item.time === "number") {
                out.push(item.time);
            }
        }
        return out;
    }

    function hideRadarContainer(mapEl) {
        if (!mapEl) return;
        var wrap = mapEl.closest(".radar-container");
        if (wrap) {
            wrap.style.display = "none";
        }
    }

    function initRadar() {
        var mapEl = document.getElementById("weather-radar-map");
        if (!mapEl) return false;
        if (typeof window.L === "undefined") return false;
        if (mapEl.dataset.radarInit === "1") return true;

        mapEl.style.minHeight = "300px";
        mapEl.style.display = "block";
        mapEl.style.width = "100%";

        var lat = parseCoord(mapEl.getAttribute("data-lat")) || 48.3794;
        var lon = parseCoord(mapEl.getAttribute("data-lon")) || 31.1656;

        var map = L.map("weather-radar-map", {
            zoomControl: true,
            attributionControl: true,
        }).setView([lat, lon], 7);
        mapEl.dataset.radarInit = "1";
        setTimeout(function () {
            map.invalidateSize();
        }, 0);
        window.addEventListener("resize", function () {
            map.invalidateSize();
        });

        var darkBase = L.tileLayer("https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png", {
            maxZoom: 18,
            attribution: "&copy; OpenStreetMap contributors &copy; CARTO",
        }).addTo(map);
        setTimeout(function () {
            map.invalidateSize();
        }, 500);
        var fallbackBaseAdded = false;
        darkBase.on("tileerror", function () {
            if (fallbackBaseAdded) return;
            fallbackBaseAdded = true;
            L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
                maxZoom: 18,
                attribution: "&copy; OpenStreetMap contributors",
            }).addTo(map);
        });

        var toggleBtn = document.getElementById("js-radar-toggle");
        var radarLayer = null;
        var radarFrames = [];
        var frameIndex = 0;
        var playTimer = null;

        function radarTileUrl(ts) {
            return "https://tilecache.rainviewer.com/v2/radar/" + ts + "/256/{z}/{x}/{y}/2/1_1.png";
        }

        function setFrame(idx) {
            if (!radarFrames.length) return;
            var i = idx % radarFrames.length;
            if (i < 0) i += radarFrames.length;
            frameIndex = i;
            if (radarLayer) {
                map.removeLayer(radarLayer);
            }
            radarLayer = L.tileLayer(radarTileUrl(radarFrames[frameIndex]), {
                opacity: 0.7,
                tileSize: 256,
                zIndex: 10,
                attribution: "&copy; RainViewer",
            }).addTo(map);
        }

        function stopPlayback() {
            if (playTimer) {
                clearInterval(playTimer);
                playTimer = null;
            }
            if (toggleBtn) {
                toggleBtn.textContent = "▶ Радар";
            }
        }

        function startPlayback() {
            if (playTimer || radarFrames.length < 2) return;
            if (toggleBtn) {
                toggleBtn.textContent = "⏸ Стоп";
            }
            playTimer = setInterval(function () {
                setFrame(frameIndex + 1);
            }, 800);
        }

        if (toggleBtn) {
            toggleBtn.addEventListener("click", function () {
                if (!radarFrames.length) return;
                if (playTimer) {
                    stopPlayback();
                } else {
                    startPlayback();
                }
            });
        }

        fetch("https://api.rainviewer.com/public/weather-maps.json")
            .then(function (res) {
                if (!res.ok) throw new Error("rainviewer status " + res.status);
                return res.json();
            })
            .then(function (json) {
                var ts = latestRadarTimestamp(json);
                if (!ts) return;
                radarFrames = recentRadarTimestamps(json, 5);
                if (!radarFrames.length) {
                    radarFrames = [ts];
                }
                setFrame(radarFrames.length - 1);
                if (toggleBtn && radarFrames.length < 2) {
                    toggleBtn.style.display = "none";
                }
            })
            .catch(function () {
                /* ignore radar API errors */
                if (toggleBtn) {
                    toggleBtn.style.display = "none";
                }
            });
        return true;
    }

    function bootRadarWithRetry() {
        var tries = 0;
        var maxTries = 20;
        function tryInit() {
            tries++;
            var mapEl = document.getElementById("weather-radar-map");
            if (!mapEl) return;
            if (initRadar()) return;
            if (tries >= maxTries) {
                hideRadarContainer(mapEl);
                return;
            }
            setTimeout(tryInit, 250);
        }
        tryInit();
    }

    document.addEventListener("DOMContentLoaded", bootRadarWithRetry);
})();
