package main

import (
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

	weatherClient := weather.NewClient(10*time.Minute, 5*time.Second)

	logger := log.New(os.Stdout, "[weather-ua] ", log.LstdFlags|log.Lshortfile)

	var placesStore *places.Store
	const placesRelPath = "data/places.db"
	if _, err := os.Stat(placesRelPath); err == nil {
		abs, _ := filepath.Abs(placesRelPath)
		ps, err := places.NewStore(placesRelPath)
		if err != nil {
			logger.Printf("failed to init places store at %s: %v", abs, err)
		} else {
			placesStore = ps
			logger.Printf("places store initialized from %s", abs)
		}
	} else {
		wd, _ := os.Getwd()
		abs, _ := filepath.Abs(placesRelPath)
		logger.Printf("places db not found at %s (wd=%s): %v", abs, wd, err)
	}
	logger.Printf("search enabled: %v", placesStore != nil)

	srv := handlers.NewServer(tmpl, weatherClient, placesStore, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.Index)
	mux.HandleFunc("/city/", srv.City)
	mux.HandleFunc("/place/", srv.Place)
	mux.HandleFunc("/api/weather/", srv.APIWeather)
	mux.HandleFunc("/api/place_weather", srv.APIPlaceWeather)
	mux.HandleFunc("/api/places", srv.PlacesSuggest)
	mux.HandleFunc("/weather/geo", srv.GeoRedirect)

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	logger.Printf("starting server on %s", addr)

	return http.ListenAndServe(addr, mux)
}

