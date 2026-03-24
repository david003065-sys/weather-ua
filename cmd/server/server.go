package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"bss/internal/bootstrap"
	"bss/internal/handlers"
	"bss/internal/places"
	"bss/internal/weather"
)

func Run() error {
	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		return err
	}

	// Один клиент погоды на всё приложение; кэш в памяти, не пересоздаётся при запросах.
	weatherClient := weather.NewClient(15*time.Minute, 5*time.Second)

	logger := log.New(os.Stdout, "[weather-ua] ", log.LstdFlags|log.Lshortfile)
	weatherClient.SetLogger(logger)
	logger.Printf("weather client created once, cache TTL 15m")

	srv := handlers.NewServer(tmpl, weatherClient, nil, logger)
	srv.SetPlacesInitPending(true)
	go func() {
		defer srv.SetPlacesInitPending(false)

		// Тяжёлая часть: EnsureData (скачивание/импорт) и открытие SQLite + FTS — не блокируют health check.
		if err := bootstrap.EnsureData(logger); err != nil {
			logger.Printf("bootstrap data failed: %v", err)
		}

		const placesRelPath = "data/places.db"
		if _, err := os.Stat(placesRelPath); err == nil {
			abs, _ := filepath.Abs(placesRelPath)
			ps, err := places.NewStore(placesRelPath)
			if err != nil {
				logger.Printf("failed to init places store at %s: %v", abs, err)
			} else {
				srv.SetPlacesStore(ps)
				logger.Printf("places store initialized from %s", abs)
			}
		} else {
			wd, _ := os.Getwd()
			abs, _ := filepath.Abs(placesRelPath)
			logger.Printf("places db not found at %s (wd=%s): %v", abs, wd, err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.Index)
	mux.HandleFunc("/city/", srv.City)
	mux.HandleFunc("/place/", srv.Place)
	mux.HandleFunc("/api/weather/", srv.APIWeather)
	mux.HandleFunc("/api/place_weather", srv.APIPlaceWeather)
	mux.HandleFunc("/api/places", srv.PlacesSuggest)
	mux.HandleFunc("/weather/geo", srv.GeoRedirect)
	mux.HandleFunc("/health", srv.Health)

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

	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}

	addr := ":" + port
	logger.Printf("starting server on %s", addr)

	return http.ListenAndServe(addr, mux)
}

