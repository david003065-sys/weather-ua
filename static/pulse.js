/**
 * @file Secret "knock" UI: five quick clicks on `#js-pulse-knock` (thermo logo button) open a monospace overlay that polls `/api/pulse`.
 * Knock is detected via delegated capture on `.header-brand` + `closest("#js-pulse-knock")` so the link never sees those clicks.
 * @module pulse
 */
(function () {
    "use strict";

    /** If no further knock click within this window, the knock counter resets. */
    var KNOCK_RESET_MS = 2000;
    var REQUIRED_KNOCKS = 5;
    var knockCount = 0;
    var clickResetTimer = null;

    var overlayEl = null;
    var outputEl = null;
    var pollTimer = null;
    /** Document listeners for closing overlay (removed in closeDashboard). */
    var pulseDocClickHandler = null;
    var pulseEscapeHandler = null;

    /** Clears knock counter and pending reset timer. */
    function resetKnockState() {
        knockCount = 0;
        if (clickResetTimer) {
            clearTimeout(clickResetTimer);
            clickResetTimer = null;
        }
    }

    /**
     * Coerces API numeric fields to a finite number (JSON numbers, numeric strings, BigInt).
     * @param {unknown} v
     * @returns {number|null}
     */
    function asFiniteNumber(v) {
        if (v == null) return null;
        if (typeof v === "number" && Number.isFinite(v)) return v;
        if (typeof v === "bigint") {
            var bi = Number(v);
            return Number.isFinite(bi) ? bi : null;
        }
        if (typeof v === "string" && String(v).trim() !== "") {
            var n = parseFloat(String(v).replace(/,/g, "."));
            return Number.isFinite(n) ? n : null;
        }
        return null;
    }

    /**
     * Renders Go runtime stats from `/api/pulse` into the overlay `<pre>`.
     * @param {Record<string, unknown>|null|undefined} data Parsed JSON body.
     * @returns {void}
     */
    function renderPulse(data) {
        if (!outputEl) return;
        var g = data ? asFiniteNumber(data.goroutines) : null;
        var goroutines = g != null ? String(Math.floor(g)) : "n/a";

        var alloc = data ? asFiniteNumber(data.memory_alloc_mb) : null;
        var sys = data ? asFiniteNumber(data.memory_sys_mb) : null;
        var memory = "n/a";
        if (alloc != null || sys != null) {
            var memAlloc = alloc != null ? alloc.toFixed(1) + " MB" : "n/a";
            var memSys = sys != null ? sys.toFixed(1) + " MB" : "n/a";
            memory = alloc != null && sys != null ? memAlloc + " (Sys: " + memSys + ")" : memAlloc !== "n/a" ? memAlloc : memSys;
        }

        var gcN = data ? asFiniteNumber(data.gc_cycles) : null;
        var gcCycles = gcN != null ? String(Math.floor(gcN)) : "n/a";

        var uptime = "n/a";
        if (data && typeof data.uptime === "string" && data.uptime) {
            var parts = data.uptime.split(".");
            uptime = (parts[0] || data.uptime) + "s";
        }

        // Analytics section
        var analytics = data && data.analytics ? data.analytics : {};
        var totalReq = asFiniteNumber(analytics.total_requests);
        var todayReq = asFiniteNumber(analytics.today_requests);
        var uniqueIPs = asFiniteNumber(analytics.unique_ips_today);
        var apiErrors = asFiniteNumber(analytics.api_errors);

        var lines = [];
        lines.push("> PULSE STREAM ONLINE");
        lines.push("> ------------------------------");
        lines.push("> goroutines : " + goroutines);
        lines.push("> memory     : " + memory);
        lines.push("> gc         : " + gcCycles);
        lines.push("> uptime     : " + uptime);

        // Analytics block
        lines.push(">");
        lines.push("> ANALYTICS");
        lines.push("> ------------------------------");
        lines.push("> requests   : " + (totalReq != null ? totalReq.toLocaleString() : "n/a"));
        lines.push("> today      : " + (todayReq != null ? todayReq.toLocaleString() : "n/a"));
        lines.push("> unique IPs : " + (uniqueIPs != null ? uniqueIPs.toLocaleString() : "n/a"));
        lines.push("> API errors : " + (apiErrors != null ? apiErrors.toLocaleString() : "n/a"));

        // Top cities
        var topCities = analytics.top_cities || [];
        if (topCities.length > 0) {
            lines.push(">");
            lines.push("> TOP CITIES");
            lines.push("> ------------------------------");
            for (var i = 0; i < Math.min(5, topCities.length); i++) {
                var city = topCities[i];
                var name = city.name || "unknown";
                var views = asFiniteNumber(city.views);
                lines.push("> " + String(i + 1) + ". " + name + " : " + (views != null ? views.toLocaleString() : "n/a"));
            }
        }

        // Top searches
        var topSearches = analytics.top_searches || [];
        if (topSearches.length > 0) {
            lines.push(">");
            lines.push("> TOP SEARCHES");
            lines.push("> ------------------------------");
            for (var j = 0; j < Math.min(5, topSearches.length); j++) {
                var search = topSearches[j];
                var query = search.query || "unknown";
                var count = asFiniteNumber(search.count);
                lines.push("> " + String(j + 1) + ". " + query + " : " + (count != null ? count.toLocaleString() : "n/a"));
            }
        }

        outputEl.textContent = lines.join("\n");
    }

    /**
     * @param {string} errText Human-readable error message.
     * @returns {void}
     */
    function renderError(errText) {
        if (!outputEl) return;
        outputEl.textContent = [
            "> PULSE STREAM ERROR",
            "> ------------------------------",
            "> " + errText,
        ].join("\n");
    }

    /** Stops the 3s `setInterval` used to poll `/api/pulse`. */
    function stopPolling() {
        if (pollTimer) {
            clearInterval(pollTimer);
            pollTimer = null;
        }
    }

    function removePulseDocumentListeners() {
        if (pulseDocClickHandler) {
            document.removeEventListener("click", pulseDocClickHandler, true);
            pulseDocClickHandler = null;
        }
        if (pulseEscapeHandler) {
            document.removeEventListener("keydown", pulseEscapeHandler, true);
            pulseEscapeHandler = null;
        }
    }

    /** Removes overlay DOM, stops polling, and detaches document-level close handlers. */
    function closeDashboard() {
        removePulseDocumentListeners();
        stopPolling();
        if (overlayEl && overlayEl.parentNode) {
            overlayEl.parentNode.removeChild(overlayEl);
        }
        overlayEl = null;
        outputEl = null;
    }

    /**
     * Fetches `/api/pulse` and updates the overlay; non-OK responses become `renderError`.
     * @returns {void}
     */
    function fetchPulse() {
        fetch("/api/pulse", { method: "GET" })
            .then(function (res) {
                if (!res.ok) throw new Error("HTTP " + res.status);
                return res.json();
            })
            .then(function (json) {
                renderPulse(json);
            })
            .catch(function (err) {
                renderError(String((err && err.message) || err || "unknown error"));
            });
    }

    /**
     * Builds the full-screen terminal-style overlay and starts polling.
     * @returns {void}
     */
    function openDashboard() {
        if (overlayEl) return;

        overlayEl = document.createElement("div");
        overlayEl.id = "pulse-dashboard";
        overlayEl.style.position = "fixed";
        overlayEl.style.top = "0";
        overlayEl.style.left = "0";
        overlayEl.style.width = "100vw";
        overlayEl.style.height = "100vh";
        overlayEl.style.background = "rgba(0,0,0,0.95)";
        overlayEl.style.color = "#00ff00";
        overlayEl.style.fontFamily = "monospace";
        overlayEl.style.zIndex = "99999";
        overlayEl.style.padding = "20px";
        overlayEl.style.boxSizing = "border-box";

        var header = document.createElement("div");
        header.style.display = "flex";
        header.style.alignItems = "center";
        header.style.justifyContent = "space-between";
        header.style.marginBottom = "14px";

        var title = document.createElement("div");
        title.textContent = "SYSTEM PULSE [///]";
        title.style.fontSize = "18px";
        title.style.fontWeight = "700";

        var closeBtn = document.createElement("button");
        closeBtn.type = "button";
        closeBtn.textContent = "[X] CLOSE";
        closeBtn.style.background = "transparent";
        closeBtn.style.border = "1px solid #00ff00";
        closeBtn.style.color = "#00ff00";
        closeBtn.style.fontFamily = "monospace";
        closeBtn.style.padding = "6px 10px";
        closeBtn.style.cursor = "pointer";
        closeBtn.addEventListener("click", closeDashboard);

        header.appendChild(title);
        header.appendChild(closeBtn);

        outputEl = document.createElement("pre");
        outputEl.style.margin = "0";
        outputEl.style.whiteSpace = "pre-wrap";
        outputEl.style.lineHeight = "1.5";
        outputEl.textContent = "> BOOTSTRAP...";

        overlayEl.appendChild(header);
        overlayEl.appendChild(outputEl);
        document.body.appendChild(overlayEl);

        pulseEscapeHandler = function (e) {
            if (!overlayEl) return;
            if (e.key !== "Escape") return;
            e.preventDefault();
            closeDashboard();
        };
        document.addEventListener("keydown", pulseEscapeHandler, true);

        setTimeout(function () {
            var ignoreNextBackdropClick = true;
            pulseDocClickHandler = function (e) {
                if (!overlayEl) return;
                if (overlayEl.contains(e.target)) return;
                if (ignoreNextBackdropClick) {
                    ignoreNextBackdropClick = false;
                    return;
                }
                closeDashboard();
            };
            document.addEventListener("click", pulseDocClickHandler, true);
        }, 50);

        fetchPulse();
        pollTimer = setInterval(fetchPulse, 3000);
    }

    /** Opens the dashboard if not already open (idempotent). */
    function togglePulseDashboard() {
        if (document.getElementById("pulse-dashboard")) return;
        openDashboard();
    }

    /**
     * Handles knock only when the click target is inside `#js-pulse-knock` (delegated from `.header-brand`).
     * Runs in **capture** on `.header-brand` so the gesture is handled before `<a class="brand-link">` or other scripts.
     * @param {MouseEvent} e
     * @returns {void}
     */
    function onHeaderBrandClickCapture(e) {
        if (!e.target || typeof e.target.closest !== "function") return;
        var knockBtn = e.target.closest("#js-pulse-knock");
        if (!knockBtn) return;

        e.preventDefault();
        e.stopPropagation();
        if (typeof e.stopImmediatePropagation === "function") {
            e.stopImmediatePropagation();
        }

        knockCount += 1;
        if (clickResetTimer) {
            clearTimeout(clickResetTimer);
            clickResetTimer = null;
        }

        if (knockCount >= REQUIRED_KNOCKS) {
            resetKnockState();
            togglePulseDashboard();
            return;
        }

        clickResetTimer = setTimeout(function () {
            knockCount = 0;
            clickResetTimer = null;
        }, KNOCK_RESET_MS);
    }

    /** Delegated capture listener on `.header-brand` (stable ancestor of knock + link). */
    function initPulseSecretKnock() {
        var host =
            document.querySelector(".site-header .header-brand") || document.querySelector(".header-brand");
        if (!host) return;
        host.addEventListener("click", onHeaderBrandClickCapture, true);
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", initPulseSecretKnock);
    } else {
        initPulseSecretKnock();
    }
})();
