/**
 * PWA bootstrap — runs independently of weather-app / chart.js so SW always registers.
 * Debug logs: [PWA] prefix (remove or gate in production if desired).
 */
(function () {
    "use strict";

    var manifestLink = document.querySelector('link[rel="manifest"]');
    var manifestURL = manifestLink ? manifestLink.href : "/manifest.webmanifest";

    fetch(manifestURL, { credentials: "same-origin" })
        .then(function (res) {
            console.log("[PWA] manifest loaded:", res.status, res.ok ? "OK" : "FAIL", manifestURL);
            if (!res.ok) {
                console.warn("[PWA] manifest HTTP not OK — install may fail in Chrome");
            }
        })
        .catch(function (err) {
            console.warn("[PWA] manifest fetch error — install may fail:", err);
        });

    if ("serviceWorker" in navigator) {
        window.addEventListener("load", function () {
            navigator.serviceWorker
                .register("/sw.js", { scope: "/", updateViaCache: "none" })
                .then(function (reg) {
                    console.log(
                        "[PWA] service worker registration success:",
                        "scope=" + reg.scope,
                        "script=" + (reg.active ? reg.active.scriptURL : reg.installing && reg.installing.scriptURL)
                    );
                    return reg.update();
                })
                .catch(function (err) {
                    console.error("[PWA] service worker registration failed:", err);
                });
        });
    } else {
        console.log("[PWA] serviceWorker not supported in this browser");
    }

    var deferredPrompt = null;
    var btn = document.getElementById("js-pwa-install-btn");

    /**
     * @returns {boolean} True if the app runs as an installed PWA (standalone / WCO / iOS).
     */
    function isStandalone() {
        return (
            window.matchMedia("(display-mode: standalone)").matches ||
            window.matchMedia("(display-mode: window-controls-overlay)").matches ||
            window.navigator.standalone === true
        );
    }

    /**
     * Localizes the install button label from `data-label-*` attributes to match `<html lang>`.
     * @returns {void}
     */
    function setInstallLabel() {
        if (!btn) return;
        var lang = (document.documentElement.getAttribute("lang") || "ru").toLowerCase();
        var key = "data-label-" + (lang === "uk" || lang === "en" ? lang : "ru");
        var text = btn.getAttribute(key) || btn.getAttribute("data-label-ru") || "Install";
        var labelEl = btn.querySelector(".pwa-install-btn__label");
        if (labelEl) {
            labelEl.textContent = text;
        }
        btn.setAttribute("aria-label", text);
    }

    /** Hides the install CTA and marks it aria-hidden. */
    function hideInstall() {
        if (!btn) return;
        btn.style.display = "none";
        btn.setAttribute("aria-hidden", "true");
    }

    /** Shows the install CTA and refreshes its localized label. */
    function showInstall() {
        if (!btn) return;
        setInstallLabel();
        btn.style.display = "flex";
        btn.removeAttribute("aria-hidden");
    }

    if (isStandalone()) {
        console.log("[PWA] already running as installed app — install UI hidden");
        hideInstall();
    } else {
        hideInstall();
    }

    if (btn) {
        setInstallLabel();
    }

    window.addEventListener("beforeinstallprompt", function (e) {
        console.log("[PWA] beforeinstallprompt fired — install is available");
        e.preventDefault();
        deferredPrompt = e;
        if (!isStandalone()) {
            showInstall();
        }
    });

    window.addEventListener("appinstalled", function () {
        console.log("[PWA] appinstalled event — PWA was installed");
        deferredPrompt = null;
        hideInstall();
    });

    /* After load: explain if Chrome has not fired beforeinstallprompt yet */
    window.addEventListener("load", function () {
        setTimeout(function () {
            if (isStandalone()) return;
            if (deferredPrompt) {
                console.log("[PWA] beforeinstallprompt: deferred prompt is ready (use Install button)");
            } else {
                console.log(
                    "[PWA] beforeinstallprompt: not fired yet. Chrome needs HTTPS, valid manifest + SW, and usually a short engagement (reload or second visit). Check DevTools → Application → Manifest."
                );
            }
        }, 4000);
    });

    if (btn) {
        btn.addEventListener("click", function () {
            if (!deferredPrompt) {
                console.log("[PWA] install click ignored — no deferred prompt (criteria not met or already installed)");
                return;
            }
            deferredPrompt.prompt();
            deferredPrompt.userChoice
                .then(function (choice) {
                    console.log("[PWA] install prompt userChoice:", choice.outcome);
                    deferredPrompt = null;
                    hideInstall();
                })
                .catch(function () {
                    deferredPrompt = null;
                    hideInstall();
                });
        });
    }
})();
