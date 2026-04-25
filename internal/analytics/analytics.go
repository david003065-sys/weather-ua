// Package analytics provides lightweight, self-hosted request analytics.
package analytics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Analytics holds in-memory counters and metrics.
type Analytics struct {
	mu          sync.RWMutex
	Requests    map[string]int64     // path → count
	TopCities   map[string]int64     // city slug → views
	TopSearches map[string]int64     // search query → count
	UniqueIPs   map[string]time.Time // IP → last visit
	Errors      int64
	StartedAt   time.Time

	persistCh chan struct{}
	filePath  string
}

// AnalyticsSummary is the JSON-exportable snapshot.
type AnalyticsSummary struct {
	TotalRequests  int64         `json:"total_requests"`
	TodayRequests  int64         `json:"today_requests"`
	TopCities      []CityView    `json:"top_cities"`
	TopSearches    []SearchCount `json:"top_searches"`
	UniqueIPsToday int64         `json:"unique_ips_today"`
	APIErrors      int64         `json:"api_errors"`
}

type CityView struct {
	Name  string `json:"name"`
	Views int64  `json:"views"`
}

type SearchCount struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}

// New creates an Analytics instance and starts the background persist loop.
func New(filePath string) *Analytics {
	a := &Analytics{
		Requests:    make(map[string]int64),
		TopCities:   make(map[string]int64),
		TopSearches: make(map[string]int64),
		UniqueIPs:   make(map[string]time.Time),
		StartedAt:   time.Now(),
		persistCh:   make(chan struct{}, 1),
		filePath:    filePath,
	}
	a.loadFromDisk()
	go a.persistLoop()
	go a.cleanupLoop()
	return a
}

// TrackRequest records a request to a path from an IP.
func (a *Analytics) TrackRequest(path string, ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.Requests[path]++
	if ip != "" {
		a.UniqueIPs[ip] = time.Now()
	}
	a.schedulePersist()
}

// TrackCity records a city page view.
func (a *Analytics) TrackCity(slug string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.TopCities[slug]++
	a.schedulePersist()
}

// TrackSearch records a search query.
func (a *Analytics) TrackSearch(query string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if query == "" {
		return
	}
	a.TopSearches[query]++

	// Keep only top 100 searches
	if len(a.TopSearches) > 100 {
		a.trimTopSearchesLocked()
	}
	a.schedulePersist()
}

// TrackError records an API error.
func (a *Analytics) TrackError() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.Errors++
	a.schedulePersist()
}

// Summary returns a snapshot of analytics data.
func (a *Analytics) Summary() AnalyticsSummary {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Count today's requests (from midnight)
	today := time.Now().Truncate(24 * time.Hour)
	todayRequests := int64(0)

	// Count unique IPs from today
	uniqueToday := int64(0)
	for _, t := range a.UniqueIPs {
		if t.After(today) {
			uniqueToday++
		}
	}

	// Sum all requests for total
	total := int64(0)
	for _, c := range a.Requests {
		total += c
	}

	// Top 5 cities
	topCities := a.topCitiesLocked(5)

	// Top 5 searches
	topSearches := a.topSearchesLocked(5)

	return AnalyticsSummary{
		TotalRequests:  total,
		TodayRequests:  todayRequests, // Simplified - would need per-day tracking for accurate count
		TopCities:      topCities,
		TopSearches:    topSearches,
		UniqueIPsToday: uniqueToday,
		APIErrors:      a.Errors,
	}
}

// topCitiesLocked returns top N cities by views. Must be called with RLock held.
func (a *Analytics) topCitiesLocked(n int) []CityView {
	type kv struct {
		key   string
		value int64
	}
	kvs := make([]kv, 0, len(a.TopCities))
	for k, v := range a.TopCities {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].value > kvs[j].value
	})
	if len(kvs) > n {
		kvs = kvs[:n]
	}
	result := make([]CityView, len(kvs))
	for i, kv := range kvs {
		result[i] = CityView{Name: kv.key, Views: kv.value}
	}
	return result
}

// topSearchesLocked returns top N searches by count. Must be called with RLock held.
func (a *Analytics) topSearchesLocked(n int) []SearchCount {
	type kv struct {
		key   string
		value int64
	}
	kvs := make([]kv, 0, len(a.TopSearches))
	for k, v := range a.TopSearches {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].value > kvs[j].value
	})
	if len(kvs) > n {
		kvs = kvs[:n]
	}
	result := make([]SearchCount, len(kvs))
	for i, kv := range kvs {
		result[i] = SearchCount{Query: kv.key, Count: kv.value}
	}
	return result
}

// trimTopSearchesLocked removes low-count searches to keep only top 100.
func (a *Analytics) trimTopSearchesLocked() {
	type kv struct {
		key   string
		value int64
	}
	kvs := make([]kv, 0, len(a.TopSearches))
	for k, v := range a.TopSearches {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].value > kvs[j].value
	})
	if len(kvs) > 100 {
		// Remove bottom entries
		newMap := make(map[string]int64, 100)
		for i := 0; i < 100 && i < len(kvs); i++ {
			newMap[kvs[i].key] = kvs[i].value
		}
		a.TopSearches = newMap
	}
}

// schedulePersist signals the background persist loop.
func (a *Analytics) schedulePersist() {
	select {
	case a.persistCh <- struct{}{}:
	default: // already scheduled
	}
}

// persistLoop coalesces persist signals and writes to disk every 5 minutes max.
func (a *Analytics) persistLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-a.persistCh:
			time.Sleep(time.Second) // coalesce burst writes
			a.persistToDisk()
		case <-ticker.C:
			a.persistToDisk()
		}
	}
}

// cleanupLoop removes old IP entries every hour.
func (a *Analytics) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		a.cleanupOldIPs()
	}
}

// cleanupOldIPs removes IP entries older than 24 hours.
func (a *Analytics) cleanupOldIPs() {
	a.mu.Lock()
	defer a.mu.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour)
	for ip, t := range a.UniqueIPs {
		if t.Before(cutoff) {
			delete(a.UniqueIPs, ip)
		}
	}
}

// persistToDisk atomically writes analytics to JSON file.
func (a *Analytics) persistToDisk() {
	a.mu.RLock()

	data := struct {
		Requests    map[string]int64 `json:"requests"`
		TopCities   map[string]int64 `json:"top_cities"`
		TopSearches map[string]int64 `json:"top_searches"`
		Errors      int64            `json:"errors"`
		StartedAt   time.Time        `json:"started_at"`
	}{
		Requests:    a.Requests,
		TopCities:   a.TopCities,
		TopSearches: a.TopSearches,
		Errors:      a.Errors,
		StartedAt:   a.StartedAt,
	}
	a.mu.RUnlock()

	// Create directory if needed
	dir := filepath.Dir(a.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	// Write to temp file
	tmp := a.filePath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}

	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return
	}

	if err := w.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return
	}

	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return
	}

	// Atomic rename
	_ = os.Rename(tmp, a.filePath)
}

// loadFromDisk loads persisted analytics data.
func (a *Analytics) loadFromDisk() {
	f, err := os.Open(a.filePath)
	if err != nil {
		return // file doesn't exist yet
	}
	defer f.Close()

	var data struct {
		Requests    map[string]int64 `json:"requests"`
		TopCities   map[string]int64 `json:"top_cities"`
		TopSearches map[string]int64 `json:"top_searches"`
		Errors      int64            `json:"errors"`
		StartedAt   time.Time        `json:"started_at"`
	}

	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if data.Requests != nil {
		a.Requests = data.Requests
	}
	if data.TopCities != nil {
		a.TopCities = data.TopCities
	}
	if data.TopSearches != nil {
		a.TopSearches = data.TopSearches
	}
	a.Errors = data.Errors
	if !data.StartedAt.IsZero() {
		a.StartedAt = data.StartedAt
	}
}

// GetUptime returns server uptime as a formatted string.
func (a *Analytics) GetUptime() string {
	d := time.Since(a.StartedAt)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
