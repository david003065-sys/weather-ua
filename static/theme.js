(function () {
    var NIGHT_START = 19;
    var NIGHT_END = 7;

    function applyThemeBySunTimes(payload) {
        var sun = payload && payload.sun;
        var sunriseISO = sun && sun.sunriseISO;
        var sunsetISO = sun && sun.sunsetISO;
        if (!sunriseISO || !sunsetISO) return false;
        var sunrise = new Date(sunriseISO);
        var sunset = new Date(sunsetISO);
        if (isNaN(sunrise.getTime()) || isNaN(sunset.getTime())) return false;
        var now = new Date();
        var isNight = now < sunrise || now > sunset;
        setBodyTheme(isNight);
        return true;
    }

    function applyThemeByClockFallback() {
        var hour = new Date().getHours();
        var isNight = hour >= NIGHT_START || hour < NIGHT_END;
        setBodyTheme(isNight);
    }

    function setBodyTheme(isNight) {
        var body = document.body;
        if (!body) return;
        body.classList.remove('theme-light', 'theme-dark');
        body.classList.add(isNight ? 'theme-dark' : 'theme-light');
    }

    function run() {
        var el = document.getElementById('__WEATHER__');
        var raw = el && el.textContent && el.textContent.trim();
        var applied = false;
        if (raw) {
            try {
                var payload = JSON.parse(raw);
                applied = applyThemeBySunTimes(payload);
            } catch (e) {}
        }
        if (!applied) applyThemeByClockFallback();
    }

    run();
    setInterval(run, 60000);
})();
