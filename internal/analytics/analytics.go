// Package analytics provides lightweight, self-hosted request analytics.
package analytics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Analytics holds in-memory counters and metrics.
type Analytics struct {
	mu            sync.RWMutex
	Requests      map[string]int64     // path → count
	TopCities     map[string]int64     // city slug → views
	TopSearches   map[string]int64     // search query → count
	UniqueIPs     map[string]time.Time // IP → last visit
	TodayRequests int64                // requests today (resets at midnight)
	TodayDate     string               // YYYY-MM-DD for tracking day changes
	Errors        int64
	StartedAt     time.Time

	// Extended tracking
	Referrers     map[string]int64 // source → count (google, telegram, direct, etc)
	Devices       map[string]int64 // device type → count (mobile, desktop, tablet)
	Browsers      map[string]int64 // browser → count
	OS            map[string]int64 // OS → count
	Countries     map[string]int64 // country code → count
	Languages     map[string]int64 // language → count
	BotVisits     int64            // search engine bot visits (Googlebot, etc)
	LastGoogleBot time.Time        // when Googlebot last visited

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

	// Extended data
	TopReferrers    []ReferrerCount `json:"top_referrers"`
	TopDevices      []DeviceCount   `json:"top_devices"`
	TopBrowsers     []BrowserCount  `json:"top_browsers"`
	TopCountries    []CountryCount  `json:"top_countries"`
	BotVisits       int64           `json:"bot_visits"`
	IndexedByGoogle bool            `json:"indexed_by_google"`
	LastGoogleBot   string          `json:"last_google_bot"`
}

type ReferrerCount struct {
	Source string `json:"source"`
	Count  int64  `json:"count"`
}

type DeviceCount struct {
	Device string `json:"device"`
	Count  int64  `json:"count"`
}

type BrowserCount struct {
	Browser string `json:"browser"`
	Count   int64  `json:"count"`
}

type CountryCount struct {
	Country string `json:"country"`
	Count   int64  `json:"count"`
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
		Referrers:   make(map[string]int64),
		Devices:     make(map[string]int64),
		Browsers:    make(map[string]int64),
		OS:          make(map[string]int64),
		Countries:   make(map[string]int64),
		Languages:   make(map[string]int64),
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

	// Check if day changed and reset counter
	today := time.Now().Format("2006-01-02")
	if a.TodayDate != today {
		a.TodayDate = today
		a.TodayRequests = 0
	}
	a.TodayRequests++

	a.Requests[path]++
	if ip != "" {
		a.UniqueIPs[ip] = time.Now()
	}
	a.schedulePersist()
}

// TrackExtended records detailed request metadata (referrer, device, browser, country).
func (a *Analytics) TrackExtended(referrer, userAgent, acceptLang, country string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Parse referrer to extract source domain
	source := a.parseReferrer(referrer)
	if source != "" {
		a.Referrers[source]++
	}

	// Parse User-Agent to extract device, browser, OS
	device, browser, os := a.parseUserAgent(userAgent)
	if device != "" {
		a.Devices[device]++
	}
	if browser != "" {
		a.Browsers[browser]++
	}
	if os != "" {
		a.OS[os]++
	}

	// Parse Accept-Language to extract primary language
	if acceptLang != "" {
		lang := a.parseLanguage(acceptLang)
		if lang != "" {
			a.Languages[lang]++
		}
	}

	// Track country if provided (from GeoIP)
	if country != "" {
		a.Countries[country]++
	}

	// Track search engine bot visits
	if a.IsGoogleBot(userAgent) {
		a.BotVisits++
		a.LastGoogleBot = time.Now()
	}

	a.schedulePersist()
}

// parseReferrer extracts source name from URL.
// Distinguishes between organic search and other properties.
func (a *Analytics) parseReferrer(ref string) string {
	if ref == "" {
		return "direct"
	}
	// Extract domain from referrer
	u, err := url.Parse(ref)
	if err != nil {
		return "other"
	}
	host := strings.ToLower(u.Hostname())
	path := strings.ToLower(u.Path)

	// Map known sources
	switch {
	// Google - distinguish search from other properties
	case strings.Contains(host, "google"):
		// Check if it's a search results page
		if strings.Contains(path, "/search") ||
			strings.Contains(path, "/url") ||
			u.Query().Get("q") != "" ||
			u.Query().Get("query") != "" {
			return "google-search"
		}
		return "google"
	case strings.Contains(host, "facebook") || strings.Contains(host, "fb.me"):
		return "facebook"
	case strings.Contains(host, "twitter") || strings.Contains(host, "t.co") || strings.Contains(host, "x.com"):
		return "twitter"
	case strings.Contains(host, "telegram") || strings.Contains(host, "t.me"):
		return "telegram"
	case strings.Contains(host, "instagram"):
		return "instagram"
	case strings.Contains(host, "youtube") || strings.Contains(host, "youtu.be"):
		return "youtube"
	// Search engines
	case strings.Contains(host, "bing"):
		return "bing"
	case strings.Contains(host, "yahoo"):
		return "yahoo"
	case strings.Contains(host, "duckduckgo"):
		return "duckduckgo"
	case strings.Contains(host, "yandex"):
		return "yandex"
	case strings.Contains(host, "baidu"):
		return "baidu"
	case host == "" || host == "localhost":
		return "direct"
	default:
		return "other"
	}
}

// IsGoogleBot checks if User-Agent is Googlebot crawler.
func (a *Analytics) IsGoogleBot(userAgent string) bool {
	ua := strings.ToLower(userAgent)
	return strings.Contains(ua, "googlebot") ||
		strings.Contains(ua, "google-web-preview")
}

// parseUserAgent extracts device type, browser and OS from User-Agent string.
func (a *Analytics) parseUserAgent(ua string) (device, browser, os string) {
	ua = strings.ToLower(ua)
	if ua == "" {
		return "", "", ""
	}

	// Detect device type
	switch {
	case strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone"):
		device = "mobile"
	case strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad"):
		device = "tablet"
	case strings.Contains(ua, "bot") || strings.Contains(ua, "crawler") || strings.Contains(ua, "spider"):
		device = "bot"
	default:
		device = "desktop"
	}

	// Detect browser
	switch {
	case strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg"):
		browser = "chrome"
	case strings.Contains(ua, "firefox"):
		browser = "firefox"
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		browser = "safari"
	case strings.Contains(ua, "edg"):
		browser = "edge"
	case strings.Contains(ua, "opera") || strings.Contains(ua, "opr"):
		browser = "opera"
	default:
		browser = "other"
	}

	// Detect OS
	switch {
	case strings.Contains(ua, "windows"):
		os = "windows"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os"):
		os = "macos"
	case strings.Contains(ua, "linux"):
		os = "linux"
	case strings.Contains(ua, "android"):
		os = "android"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		os = "ios"
	default:
		os = "other"
	}

	return device, browser, os
}

// parseLanguage extracts primary language from Accept-Language header.
func (a *Analytics) parseLanguage(lang string) string {
	if lang == "" {
		return ""
	}
	// Take first language before comma or semicolon
	lang = strings.ToLower(lang)
	if idx := strings.IndexAny(lang, ",;"); idx != -1 {
		lang = lang[:idx]
	}
	lang = strings.TrimSpace(lang)
	// Normalize to 2-letter code
	if len(lang) >= 2 {
		return lang[:2]
	}
	return lang
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

	// Check if day changed (for reading without write lock)
	todayDate := time.Now().Format("2006-01-02")
	todayRequests := a.TodayRequests
	if a.TodayDate != todayDate {
		todayRequests = 0 // Day changed, counter will reset on next TrackRequest
	}

	// Count unique IPs from today
	today := time.Now().Truncate(24 * time.Hour)
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

	// Top 5 referrers
	topReferrers := a.topReferrersLocked(5)

	// Top devices
	topDevices := a.topDevicesLocked(5)

	// Top browsers
	topBrowsers := a.topBrowsersLocked(5)

	// Top countries
	topCountries := a.topCountriesLocked(5)

	// Format last Googlebot visit time
	lastBot := "never"
	if !a.LastGoogleBot.IsZero() {
		since := time.Since(a.LastGoogleBot)
		if since < time.Hour {
			lastBot = fmt.Sprintf("%dm ago", int(since.Minutes()))
		} else if since < 24*time.Hour {
			lastBot = fmt.Sprintf("%dh ago", int(since.Hours()))
		} else {
			lastBot = fmt.Sprintf("%dd ago", int(since.Hours()/24))
		}
	}

	return AnalyticsSummary{
		TotalRequests:   total,
		TodayRequests:   todayRequests,
		TopCities:       topCities,
		TopSearches:     topSearches,
		UniqueIPsToday:  uniqueToday,
		APIErrors:       a.Errors,
		TopReferrers:    topReferrers,
		TopDevices:      topDevices,
		TopBrowsers:     topBrowsers,
		TopCountries:    topCountries,
		BotVisits:       a.BotVisits,
		IndexedByGoogle: a.BotVisits > 0,
		LastGoogleBot:   lastBot,
	}
}

// Helper functions for top N queries
type kv struct {
	key   string
	value int64
}

// topReferrersLocked returns top N referrers. Must be called with RLock held.
func (a *Analytics) topReferrersLocked(n int) []ReferrerCount {
	kvs := make([]kv, 0, len(a.Referrers))
	for k, v := range a.Referrers {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].value > kvs[j].value
	})
	if len(kvs) > n {
		kvs = kvs[:n]
	}
	result := make([]ReferrerCount, len(kvs))
	for i, kv := range kvs {
		result[i] = ReferrerCount{Source: kv.key, Count: kv.value}
	}
	return result
}

// topDevicesLocked returns top N devices. Must be called with RLock held.
func (a *Analytics) topDevicesLocked(n int) []DeviceCount {
	kvs := make([]kv, 0, len(a.Devices))
	for k, v := range a.Devices {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].value > kvs[j].value
	})
	if len(kvs) > n {
		kvs = kvs[:n]
	}
	result := make([]DeviceCount, len(kvs))
	for i, kv := range kvs {
		result[i] = DeviceCount{Device: kv.key, Count: kv.value}
	}
	return result
}

// topBrowsersLocked returns top N browsers. Must be called with RLock held.
func (a *Analytics) topBrowsersLocked(n int) []BrowserCount {
	kvs := make([]kv, 0, len(a.Browsers))
	for k, v := range a.Browsers {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].value > kvs[j].value
	})
	if len(kvs) > n {
		kvs = kvs[:n]
	}
	result := make([]BrowserCount, len(kvs))
	for i, kv := range kvs {
		result[i] = BrowserCount{Browser: kv.key, Count: kv.value}
	}
	return result
}

// topCountriesLocked returns top N countries. Must be called with RLock held.
func (a *Analytics) topCountriesLocked(n int) []CountryCount {
	kvs := make([]kv, 0, len(a.Countries))
	for k, v := range a.Countries {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].value > kvs[j].value
	})
	if len(kvs) > n {
		kvs = kvs[:n]
	}
	result := make([]CountryCount, len(kvs))
	for i, kv := range kvs {
		result[i] = CountryCount{Country: kv.key, Count: kv.value}
	}
	return result
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
		Requests      map[string]int64 `json:"requests"`
		TopCities     map[string]int64 `json:"top_cities"`
		TopSearches   map[string]int64 `json:"top_searches"`
		TodayRequests int64            `json:"today_requests"`
		TodayDate     string           `json:"today_date"`
		Errors        int64            `json:"errors"`
		StartedAt     time.Time        `json:"started_at"`
		Referrers     map[string]int64 `json:"referrers"`
		Devices       map[string]int64 `json:"devices"`
		Browsers      map[string]int64 `json:"browsers"`
		OS            map[string]int64 `json:"os"`
		Countries     map[string]int64 `json:"countries"`
		Languages     map[string]int64 `json:"languages"`
		BotVisits     int64            `json:"bot_visits"`
		LastGoogleBot time.Time        `json:"last_google_bot"`
	}{
		Requests:      a.Requests,
		TopCities:     a.TopCities,
		TopSearches:   a.TopSearches,
		TodayRequests: a.TodayRequests,
		TodayDate:     a.TodayDate,
		Errors:        a.Errors,
		StartedAt:     a.StartedAt,
		Referrers:     a.Referrers,
		Devices:       a.Devices,
		Browsers:      a.Browsers,
		OS:            a.OS,
		Countries:     a.Countries,
		Languages:     a.Languages,
		BotVisits:     a.BotVisits,
		LastGoogleBot: a.LastGoogleBot,
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
		Requests      map[string]int64 `json:"requests"`
		TopCities     map[string]int64 `json:"top_cities"`
		TopSearches   map[string]int64 `json:"top_searches"`
		TodayRequests int64            `json:"today_requests"`
		TodayDate     string           `json:"today_date"`
		Errors        int64            `json:"errors"`
		StartedAt     time.Time        `json:"started_at"`
		Referrers     map[string]int64 `json:"referrers"`
		Devices       map[string]int64 `json:"devices"`
		Browsers      map[string]int64 `json:"browsers"`
		OS            map[string]int64 `json:"os"`
		Countries     map[string]int64 `json:"countries"`
		Languages     map[string]int64 `json:"languages"`
		BotVisits     int64            `json:"bot_visits"`
		LastGoogleBot time.Time        `json:"last_google_bot"`
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

	// Check if loaded data is from today
	todayDate := time.Now().Format("2006-01-02")
	if data.TodayDate == todayDate {
		a.TodayRequests = data.TodayRequests
		a.TodayDate = data.TodayDate
	} else {
		// Reset today's counter if data is from previous day
		a.TodayRequests = 0
		a.TodayDate = todayDate
	}

	// Load extended data
	if data.Referrers != nil {
		a.Referrers = data.Referrers
	}
	if data.Devices != nil {
		a.Devices = data.Devices
	}
	if data.Browsers != nil {
		a.Browsers = data.Browsers
	}
	if data.OS != nil {
		a.OS = data.OS
	}
	if data.Countries != nil {
		a.Countries = data.Countries
	}
	if data.Languages != nil {
		a.Languages = data.Languages
	}
	a.BotVisits = data.BotVisits
	if !data.LastGoogleBot.IsZero() {
		a.LastGoogleBot = data.LastGoogleBot
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
