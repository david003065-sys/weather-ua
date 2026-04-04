/**
 * @file Secret "knock" UI: five quick clicks on `#js-pulse-knock` (thermo logo button) open a monospace overlay that polls `/api/pulse`.
 * The home link (`.brand-link`) is separate — normal navigation is not intercepted.
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

    /** Clears knock counter and pending reset timer. */
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
     * Counts rapid clicks on the logo button; on the 5th opens pulse.
     * `preventDefault` / `stopPropagation` run only on the 5th click (no navigation side effects on 1–4).
     * @param {MouseEvent} e Click from `.brand-pulse-knock` (type=button, not inside `<a>`).
     * @returns {void}
     */
    function onPulseKnockClick(e) {
        knockCount += 1;
        if (clickResetTimer) {
            clearTimeout(clickResetTimer);
            clickResetTimer = null;
        }

        if (knockCount >= REQUIRED_KNOCKS) {
            e.preventDefault();
            e.stopPropagation();
            resetKnockState();
            togglePulseDashboard();
            return;
        }

        clickResetTimer = setTimeout(function () {
            knockCount = 0;
            clickResetTimer = null;
        }, KNOCK_RESET_MS);
    }

    /** Attaches knock handler to the thermo logo button (`#js-pulse-knock` in layout.html). */
    function initPulseSecretKnock() {
        var knockEl =
            document.getElementById("js-pulse-knock") || document.querySelector("button.brand-pulse-knock");
        if (!knockEl) return;
        knockEl.addEventListener("click", onPulseKnockClick);
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", initPulseSecretKnock);
    } else {
        initPulseSecretKnock();
    }
})();
