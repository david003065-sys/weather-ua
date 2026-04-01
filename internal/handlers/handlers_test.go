package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bss/internal/weather"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	// Изолируем тесты от .env / реального ключа: провайдер «выключен» → пустой ответ без сети.
	t.Setenv(weather.EnvWeatherAPIKey, "")
	wc := weather.NewClient(time.Minute, 2*time.Second)
	return NewServer(nil, wc, nil, log.New(io.Discard, "", 0))
}

func TestHandlePulse_GET(t *testing.T) {
	s := testServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/pulse", nil)
	s.HandlePulse(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, body %q", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"goroutines", "memory_alloc_mb", "memory_sys_mb", "gc_cycles", "uptime"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
}

func TestHandlePulse_MethodNotAllowed(t *testing.T) {
	s := testServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/pulse", nil)
	s.HandlePulse(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestHealth(t *testing.T) {
	s := testServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	s.Health(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if got := rr.Body.String(); got != "ok" {
		t.Fatalf("body %q", got)
	}
}

func TestAPIWeather(t *testing.T) {
	s := testServer(t)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		checkJSON  func(t *testing.T, body []byte)
	}{
		{
			name:       "GET known city without API key returns 200 fallback JSON",
			method:     http.MethodGet,
			path:       "/api/weather/kyiv",
			wantStatus: http.StatusOK,
			checkJSON: func(t *testing.T, body []byte) {
				t.Helper()
				var resp struct {
					CityID  string `json:"cityId"`
					Lang    string `json:"lang"`
					Current struct {
						IsFallback bool `json:"isFallback"`
					} `json:"current"`
				}
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatal(err)
				}
				if resp.CityID != "kyiv" {
					t.Fatalf("cityId %q", resp.CityID)
				}
				if resp.Lang != "ru" {
					t.Fatalf("default lang %q (i18n.Normalize default)", resp.Lang)
				}
				if !resp.Current.IsFallback {
					t.Fatal("expected isFallback without provider key")
				}
			},
		},
		{
			name:       "GET with lang query",
			method:     http.MethodGet,
			path:       "/api/weather/kyiv?lang=en",
			wantStatus: http.StatusOK,
			checkJSON: func(t *testing.T, body []byte) {
				t.Helper()
				var resp struct {
					Lang string `json:"lang"`
				}
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatal(err)
				}
				if resp.Lang != "en" {
					t.Fatalf("lang %q", resp.Lang)
				}
			},
		},
		{
			name:       "unknown city 404",
			method:     http.MethodGet,
			path:       "/api/weather/not-a-real-city-slug",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing city id 400",
			method:     http.MethodGet,
			path:       "/api/weather/",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "path without trailing slash not matched",
			method:     http.MethodGet,
			path:       "/api/weather",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			path:       "/api/weather/kyiv",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			s.APIWeather(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d, body %q", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if tt.checkJSON != nil {
				tt.checkJSON(t, rr.Body.Bytes())
			}
		})
	}
}

func TestAPIWeather_ContextCanceled(t *testing.T) {
	t.Setenv(weather.EnvWeatherAPIKey, "")
	s := NewServer(nil, weather.NewClient(time.Minute, 2*time.Second), nil, log.New(io.Discard, "", 0))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/weather/kyiv", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	s.APIWeather(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status %d, body %q", rr.Code, rr.Body.String())
	}
}

func TestAPIFavorites(t *testing.T) {
	s := testServer(t)

	tests := []struct {
		name       string
		method     string
		rawURL     string
		wantStatus int
		checkJSON  func(t *testing.T, body []byte)
	}{
		{
			name:       "empty ids returns empty array",
			method:     http.MethodGet,
			rawURL:     "/api/favorites",
			wantStatus: http.StatusOK,
			checkJSON: func(t *testing.T, body []byte) {
				t.Helper()
				var arr []apiFavoriteItem
				if err := json.Unmarshal(body, &arr); err != nil {
					t.Fatal(err)
				}
				if len(arr) != 0 {
					t.Fatalf("len %d", len(arr))
				}
			},
		},
		{
			name:       "unknown city omitted",
			method:     http.MethodGet,
			rawURL:     "/api/favorites?ids=notacity",
			wantStatus: http.StatusOK,
			checkJSON: func(t *testing.T, body []byte) {
				t.Helper()
				var arr []apiFavoriteItem
				if err := json.Unmarshal(body, &arr); err != nil {
					t.Fatal(err)
				}
				if len(arr) != 0 {
					t.Fatalf("expected empty, got %d", len(arr))
				}
			},
		},
		{
			name:       "known city without API key",
			method:     http.MethodGet,
			rawURL:     "/api/favorites?ids=kyiv",
			wantStatus: http.StatusOK,
			checkJSON: func(t *testing.T, body []byte) {
				t.Helper()
				var arr []apiFavoriteItem
				if err := json.Unmarshal(body, &arr); err != nil {
					t.Fatal(err)
				}
				if len(arr) != 1 || arr[0].ID != "kyiv" {
					t.Fatalf("got %+v", arr)
				}
				if arr[0].Kind != "city" {
					t.Fatalf("kind %q", arr[0].Kind)
				}
			},
		},
		{
			name:       "dedupe and order",
			method:     http.MethodGet,
			rawURL:     "/api/favorites?ids=kyiv,kyiv,dnipro",
			wantStatus: http.StatusOK,
			checkJSON: func(t *testing.T, body []byte) {
				t.Helper()
				var arr []apiFavoriteItem
				if err := json.Unmarshal(body, &arr); err != nil {
					t.Fatal(err)
				}
				if len(arr) != 2 {
					t.Fatalf("len %d", len(arr))
				}
				if arr[0].ID != "kyiv" || arr[1].ID != "dnipro" {
					t.Fatalf("order %+v", arr)
				}
			},
		},
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			rawURL:     "/api/favorites?ids=kyiv",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.rawURL, nil)
			s.APIFavorites(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("status %d, body %q", rr.Code, rr.Body.String())
			}
			if tt.checkJSON != nil {
				tt.checkJSON(t, rr.Body.Bytes())
			}
		})
	}
}

func TestAPIFavorites_ContextCanceled(t *testing.T) {
	t.Setenv(weather.EnvWeatherAPIKey, "")
	s := NewServer(nil, weather.NewClient(time.Minute, 2*time.Second), nil, log.New(io.Discard, "", 0))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/favorites?ids=kyiv", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	s.APIFavorites(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var arr []apiFavoriteItem
	if err := json.Unmarshal(rr.Body.Bytes(), &arr); err != nil {
		t.Fatal(err)
	}
	// Горутина с отменённым контекстом не заполняет слот — элемент пропускается.
	if len(arr) != 0 {
		t.Fatalf("expected empty on canceled ctx, got %d", len(arr))
	}
}

func TestIsAllDecimal(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"   ", false},
		{"0", true},
		{"123", true},
		{"12a3", false},
		{"-1", false},
		{"1.5", false},
		{" 42 ", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isAllDecimal(tt.in); got != tt.want {
				t.Fatalf("isAllDecimal(%q) = %v", tt.in, got)
			}
		})
	}
}

func TestCityCoordsByID(t *testing.T) {
	tests := []struct {
		id       string
		wantLat  float64
		wantLon  float64
		wantZero bool
	}{
		{"kyiv", 50.4501, 30.5234, false},
		{"dnipro", 48.467, 35.040, false},
		{"nosuch", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			lat, lon := cityCoordsByID(tt.id)
			if tt.wantZero {
				if lat != 0 || lon != 0 {
					t.Fatalf("got %v,%v", lat, lon)
				}
				return
			}
			if lat != tt.wantLat || lon != tt.wantLon {
				t.Fatalf("got %v,%v want %v,%v", lat, lon, tt.wantLat, tt.wantLon)
			}
		})
	}
}
