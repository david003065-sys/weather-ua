/**
 * @file Secret "knock" UI: five quick clicks on the header brand open a monospace overlay that polls `/api/pulse`.
 * @module pulse
 */
(function () {
    "use strict";

    var MAX_GAP_MS = 400;
    var REQUIRED_KNOCKS = 5;
    var knockCount = 0;
    var clickResetTimer = null;

    var overlayEl = null;
    var outputEl = null;
    var pollTimer = null;

    /** Clears knock counter and pending navigation timeout. */
    function resetKnockState() {
        knockCount = 0;
        if (clickResetTimer) {
            clearTimeout(clickResetTimer);
            clickResetTimer = null;
        }
    }

    /**
     * Renders Go runtime stats from `/api/pulse` into the overlay `<pre>`.
     * @param {Record<string, unknown>|null|undefined} data Parsed JSON body.
     * @returns {void}
     */
    function renderPulse(data) {
        if (!outputEl) return;
        var goroutines = data && data.goroutines != null ? data.goroutines : "n/a";
        var memAlloc =
            data && typeof data.memory_alloc_mb === "number"
                ? data.memory_alloc_mb.toFixed(1) + " MB"
                : "n/a";
        var memSys =
            data && typeof data.memory_sys_mb === "number"
                ? data.memory_sys_mb.toFixed(1) + " MB"
                : "n/a";
        var memory = memAlloc === "n/a" ? "n/a" : memAlloc + " (Sys: " + memSys + ")";
        var gcCycles = data && data.gc_cycles != null ? data.gc_cycles : "n/a";
        var uptime = "n/a";
        if (data && typeof data.uptime === "string" && data.uptime) {
            var parts = data.uptime.split(".");
            uptime = (parts[0] || data.uptime) + "s";
        }

        var lines = [];
        lines.push("> PULSE STREAM ONLINE");
        lines.push("> ------------------------------");
        lines.push("> goroutines : " + goroutines);
        lines.push("> memory     : " + memory);
        lines.push("> gc         : " + gcCycles);
        lines.push("> uptime     : " + uptime);
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

    /** Stops the 1s `setInterval` used to poll `/api/pulse`. */
    function stopPolling() {
        if (pollTimer) {
            clearInterval(pollTimer);
            pollTimer = null;
        }
    }

    /** Removes overlay DOM and stops polling. */
    function closeDashboard() {
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

        fetchPulse();
        pollTimer = setInterval(fetchPulse, 1000);
    }

    /** Opens the dashboard if not already open (idempotent). */
    function togglePulseDashboard() {
        if (document.getElementById("pulse-dashboard")) return;
        openDashboard();
    }

    /**
     * Intercepts brand link: counts rapid clicks; on 5th opens pulse, else after gap follows the real `href`.
     * @param {MouseEvent} e Click event from `.header-brand a.brand-link`.
     * @returns {void}
     */
    function onBrandClick(e) {
        e.preventDefault();
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
            window.location.href = e.currentTarget.href || "/";
        }, MAX_GAP_MS);
    }

    /** Attaches `onBrandClick` to the header brand anchor if present. */
    function initPulseSecretKnock() {
        var brand = document.querySelector(".header-brand a.brand-link");
        if (!brand) return;
        brand.addEventListener("click", onBrandClick);
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", initPulseSecretKnock);
    } else {
        initPulseSecretKnock();
    }
})();
