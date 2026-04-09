package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"bss/internal/bootstrap"
	"bss/internal/handlers"
	"bss/internal/middleware"
	"bss/internal/places"
	"bss/internal/weather"

	"golang.org/x/time/rate"
)

func Run() error {
	logger := log.New(os.Stdout, "[weather-ua] ", log.LstdFlags|log.Lshortfile)
	var srvPtr atomic.Pointer[handlers.Server]

	mux := http.NewServeMux()
	// Render health check must pass even while heavy warm-up is still running.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	readyOr503 := func(h func(*handlers.Server, http.ResponseWriter, *http.Request)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			srv := srvPtr.Load()
			if srv == nil {
				http.Error(w, "service warming up", http.StatusServiceUnavailable)
				return
			}
			h(srv, w, r)
		}
	}

	// Per-IP and global caps apply only to HTTP /api/* — not to in-process weather.WarmCache (direct provider calls).
	// Slightly relaxed vs original 10/60 so one browser tab opening several API endpoints in parallel is less likely to 429.
	apiRL := middleware.NewRateLimiter(rate.Every(time.Minute/20), 20, rate.Every(time.Minute/120), 120, 5*time.Minute)
	apiRL.StartCleanup(5*time.Minute, make(chan struct{}))
	logger.Printf("api rate limiter: per-ip 20/min, global 120/min (WarmCache bypasses this)")

	apiRoute := func(h func(*handlers.Server, http.ResponseWriter, *http.Request)) http.Handler {
		return apiRL.WrapFunc(readyOr503(h))
	}

	// SSR pages — no rate limit.
	mux.HandleFunc("/", readyOr503(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.Index(w, r)
	}))
	mux.HandleFunc("/city/", readyOr503(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.City(w, r)
	}))
	mux.HandleFunc("/place/", readyOr503(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.Place(w, r)
	}))
	mux.HandleFunc("/geo", readyOr503(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.GeoRedirect(w, r)
	}))
	mux.HandleFunc("/weather/geo", readyOr503(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.GeoRedirect(w, r)
	}))

	// API routes — rate limited.
	mux.Handle("/api/weather/", apiRoute(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.APIWeather(w, r)
	}))
	mux.Handle("/api/place_weather", apiRoute(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.APIPlaceWeather(w, r)
	}))
	mux.Handle("/api/favorites", apiRoute(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.APIFavorites(w, r)
	}))
	mux.Handle("/api/places", apiRoute(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.PlacesSuggest(w, r)
	}))
	mux.Handle("/api/find-city", apiRoute(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.APIFindCity(w, r)
	}))
	mux.Handle("/api/pulse", apiRoute(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.HandlePulse(w, r)
	}))

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// PWA: canonical manifest URL with correct MIME (Chrome installability).
	mux.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		http.ServeFile(w, r, "static/manifest.json")
	})

	// PWA: service worker at root scope so navigations (/, /city/*, /place/*) are controlled.
	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, "static/sw.js")
	})

	// Heavy init in background so ListenAndServe starts immediately.
	go func() {
		tmpl, err := template.ParseGlob("templates/*.html")
		if err != nil {
			logger.Printf("template parse failed: %v", err)
			return
		}

		// Один клиент погоды на всё приложение; кэш в памяти, не пересоздаётся при запросах.
		weatherClient := weather.NewClient(15*time.Minute, 5*time.Second)
		weatherClient.SetLogger(logger)
		logger.Printf("weather client created once, cache TTL 15m")

		srv := handlers.NewServer(tmpl, weatherClient, nil, logger)
		srvPtr.Store(srv)
		logger.Printf("core server dependencies are ready")

		go weatherClient.WarmCache(context.Background())

		// Download GeoNames (UA + alternateNamesV2 + admin1), build cities_ua.csv, import SQLite if missing or stale.
		if err := bootstrap.EnsureData(logger); err != nil {
			logger.Printf("[bootstrap] ensure data failed: %v", err)
			return
		}

		// Heavy: open SQLite / load FTS indexes.
		const placesRelPath = "data/places.db"
		abs, _ := filepath.Abs(placesRelPath)
		if _, err := os.Stat(placesRelPath); err != nil {
			logger.Printf("[places] places.db not found at %s after bootstrap: %v", abs, err)
			return
		}
		ps, err := places.NewStore(placesRelPath)
		if err != nil {
			logger.Printf("[places] open places store failed at %s: %v", abs, err)
			return
		}
		logger.Printf("[places] places store ready at %s", abs)
		srv.SetPlacesStore(ps)
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}

	addr := ":" + port
	logger.Printf("starting server on %s", addr)

	return http.ListenAndServe(addr, mux)
}
