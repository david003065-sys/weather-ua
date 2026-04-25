package main

import (
	"context"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"bss/internal/analytics"
	"bss/internal/handlers"
	"bss/internal/middleware"
	"bss/internal/places"
	"bss/internal/weather"

	"golang.org/x/time/rate"
)

func Run() error {
	logger := log.New(os.Stdout, "[weather-ua] ", log.LstdFlags|log.Lshortfile)
	var srvPtr atomic.Pointer[handlers.Server]
	var weatherClientPtr atomic.Pointer[weather.Client]

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
	rlDone := make(chan struct{})
	apiRL.StartCleanup(5*time.Minute, rlDone)
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

	// SEO: sitemap and robots.txt (no rate limiting required)
	mux.HandleFunc("/sitemap.xml", readyOr503(func(s *handlers.Server, w http.ResponseWriter, r *http.Request) {
		s.Sitemap(w, r)
	}))
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "https://weather-ua.onrender.com"
		}
		robots := "User-agent: *\nAllow: /\nSitemap: " + baseURL + "/sitemap.xml\n"
		_, _ = w.Write([]byte(robots))
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
		weatherClientPtr.Store(weatherClient)
		weatherClient.SetLogger(logger)
		logger.Printf("weather client created once, cache TTL 15m")

		// Analytics: lightweight self-hosted tracking
		analyticsTracker := analytics.New("data/analytics.json")
		weatherClient.SetAnalytics(analyticsTracker)
		logger.Printf("analytics initialized")

		srv := handlers.NewServer(tmpl, weatherClient, nil, logger)
		srv.SetAnalytics(analyticsTracker)
		srvPtr.Store(srv)
		logger.Printf("core server dependencies are ready")

		go weatherClient.WarmCache(context.Background())

		// Heavy: open SQLite / load FTS indexes.
		const placesRelPath = "data/places.db"
		abs, _ := filepath.Abs(placesRelPath)
		if _, err := os.Stat(placesRelPath); err != nil {
			logger.Printf("[places] places.db not found at %s: %v", abs, err)
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
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: middleware.Recovery(logger, mux),
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		close(rlDone)
		return err
	case <-sigCtx.Done():
		logger.Printf("shutdown signal received, starting graceful shutdown")
		close(rlDone)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Printf("graceful shutdown error: %v", err)
		}
		if wc := weatherClientPtr.Load(); wc != nil {
			wc.FlushCacheToDisk()
		}
		return nil
	}
}
