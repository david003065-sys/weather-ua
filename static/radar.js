document.addEventListener('DOMContentLoaded', function() {
    const container = document.getElementById('weather-radar-map');
    if (!container) return;

    // Жестко задаем стили для диагностического экрана
    container.style.minHeight = "300px";
    container.style.display = "flex";
    container.style.alignItems = "center";
    container.style.justifyContent = "center";
    container.style.background = "#1a1a1a";
    container.style.border = "1px solid #333";
    container.style.color = "#fff";
    container.style.padding = "20px";
    container.style.fontFamily = "monospace";
    container.style.textAlign = "center";

    try {
        // 1. Проверка загрузки библиотеки
        if (typeof L === 'undefined') {
            throw new Error("Библиотека Leaflet (L) не найдена! Проверь, есть ли <script src='...leaflet.js'> в HTML (layout.html или city.html).");
        }

        // 2. Проверка координат
        const lat = parseFloat(container.dataset.lat);
        const lon = parseFloat(container.dataset.lon);
        if (isNaN(lat) || isNaN(lon)) {
            throw new Error(`Неверные координаты от Go-бэкенда: lat=${container.dataset.lat}, lon=${container.dataset.lon}`);
        }

        // Если дошли сюда — всё ок, пробуем запустить базовую карту
        container.innerHTML = ""; // очищаем текст
        container.style.display = "block"; // возвращаем блочный вид для карты
        
        const map = L.map('weather-radar-map').setView([lat, lon], 6);
        L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png').addTo(map);
        
        setTimeout(() => { map.invalidateSize(); }, 500);

    } catch (e) {
        // ВЫВОДИМ ОШИБКУ НА ЭКРАН
        container.innerHTML = `<div style="color: #ff4444; font-size: 16px;"><b>КРИТИЧЕСКАЯ ОШИБКА РАДАРА:</b><br><br>${e.message}</div>`;
        console.error(e);
    }
});
