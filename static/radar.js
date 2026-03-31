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

    function initRadar() {
        var mapEl = document.getElementById("weather-radar-map");
        if (!mapEl) return;
        if (typeof window.L === "undefined") return;

        var lat = parseCoord(mapEl.getAttribute("data-lat"));
        var lon = parseCoord(mapEl.getAttribute("data-lon"));
        if (!Number.isFinite(lat) || !Number.isFinite(lon)) return;

        var map = L.map("weather-radar-map", {
            zoomControl: true,
            attributionControl: true,
        }).setView([lat, lon], 7);

        L.tileLayer("https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png", {
            maxZoom: 18,
            attribution: "&copy; OpenStreetMap contributors &copy; CARTO",
        }).addTo(map);

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
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", initRadar);
    } else {
        initRadar();
    }
})();
