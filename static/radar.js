document.addEventListener("DOMContentLoaded", function () {
    var container = document.getElementById("weather-radar-map");
    if (!container) return;
    if (typeof window.L === "undefined") return;

    container.style.minHeight = "300px";
    container.style.display = "block";
    container.style.width = "100%";

    var lat = parseFloat(container.dataset.lat) || 48.3794;
    var lon = parseFloat(container.dataset.lon) || 31.1656;

    var map = L.map("weather-radar-map", {
        zoomControl: false,
        attributionControl: true,
    }).setView([lat, lon], 6);

    L.tileLayer("https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png", {
        maxZoom: 18,
        attribution: "&copy; OpenStreetMap contributors &copy; CARTO",
    }).addTo(map);

    fetch("https://api.rainviewer.com/public/weather-maps.json")
        .then(function (res) {
            if (!res.ok) throw new Error("rainviewer status " + res.status);
            return res.json();
        })
        .then(function (json) {
            var radar = json && json.radar && Array.isArray(json.radar.past) ? json.radar.past : [];
            if (!radar.length) return;
            var last = radar[radar.length - 1];
            var ts = last && typeof last.time === "number" ? last.time : null;
            if (!ts) return;
            L.tileLayer(
                "https://tilecache.rainviewer.com/v2/radar/" + ts + "/256/{z}/{x}/{y}/2/1_1.png",
                {
                    opacity: 0.7,
                    tileSize: 256,
                    zIndex: 10,
                    attribution: "&copy; RainViewer",
                }
            ).addTo(map);
        })
        .catch(function () {
            /* ignore RainViewer errors; base map stays visible */
        });

    setTimeout(function () {
        map.invalidateSize();
    }, 500);
});
