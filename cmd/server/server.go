package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

	// Готовая БД из репозитория / артефакта сборки (см. cmd/build_db). Рабочая директория — корень проекта (Render: repo root).
	const placesRelPath = "data/places.db"
	if _, err := os.Stat(placesRelPath); err != nil {
		wd, _ := os.Getwd()
		abs, _ := filepath.Abs(placesRelPath)
		return fmt.Errorf("places database not found at %s (cwd=%s): %w — build with: go run ./cmd/build_db -input <csv-or-json> -output data/places.db", abs, wd, err)
	}
	abs, _ := filepath.Abs(placesRelPath)
	ps, err := places.NewStore(placesRelPath)
	if err != nil {
		return fmt.Errorf("open places store at %s: %w", abs, err)
	}
	logger.Printf("places store ready at %s", abs)

	srv := handlers.NewServer(tmpl, weatherClient, ps, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.Index)
	mux.HandleFunc("/city/", srv.City)
	mux.HandleFunc("/place/", srv.Place)
	mux.HandleFunc("/api/weather/", srv.APIWeather)
	mux.HandleFunc("/api/place_weather", srv.APIPlaceWeather)
	mux.HandleFunc("/api/places", srv.PlacesSuggest)
	mux.HandleFunc("/api/find-city", srv.APIFindCity)
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

