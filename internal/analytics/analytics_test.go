package analytics

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAnalytics_TrackRequest(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "analytics.json")

	a := New(filePath)

	// Track some requests
	a.TrackRequest("/", "1.2.3.4")
	a.TrackRequest("/city/kyiv", "1.2.3.4")
	a.TrackRequest("/city/kyiv", "5.6.7.8")
	a.TrackRequest("/api/weather/kyiv", "")

	time.Sleep(100 * time.Millisecond) // let async operations complete

	summary := a.Summary()

	if summary.TotalRequests != 4 {
		t.Errorf("expected 4 total requests, got %d", summary.TotalRequests)
	}

	if summary.UniqueIPsToday != 2 {
		t.Errorf("expected 2 unique IPs, got %d", summary.UniqueIPsToday)
	}
}

func TestAnalytics_TrackCity(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "analytics.json")

	a := New(filePath)

	a.TrackCity("kyiv")
	a.TrackCity("kyiv")
	a.TrackCity("dnipro")
	a.TrackCity("kyiv")

	time.Sleep(100 * time.Millisecond)

	summary := a.Summary()

	if len(summary.TopCities) < 2 {
		t.Errorf("expected at least 2 cities in top, got %d", len(summary.TopCities))
	}

	// Kyiv should be first with 3 views
	if len(summary.TopCities) > 0 && summary.TopCities[0].Name != "kyiv" {
		t.Errorf("expected kyiv to be top city, got %s", summary.TopCities[0].Name)
	}
	if len(summary.TopCities) > 0 && summary.TopCities[0].Views != 3 {
		t.Errorf("expected kyiv to have 3 views, got %d", summary.TopCities[0].Views)
	}
}

func TestAnalytics_TrackSearch(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "analytics.json")

	a := New(filePath)

	a.TrackSearch("київ")
	a.TrackSearch("харків")
	a.TrackSearch("київ")
	a.TrackSearch("") // empty should be ignored

	time.Sleep(100 * time.Millisecond)

	summary := a.Summary()

	if len(summary.TopSearches) != 2 {
		t.Errorf("expected 2 searches in top, got %d", len(summary.TopSearches))
	}

	// Kyiv search should be first with 2 counts
	if len(summary.TopSearches) > 0 && summary.TopSearches[0].Count != 2 {
		t.Errorf("expected top search to have count 2, got %d", summary.TopSearches[0].Count)
	}
}

func TestAnalytics_PersistAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "analytics.json")

	// Create and populate
	a1 := New(filePath)
	a1.TrackRequest("/", "1.2.3.4")
	a1.TrackCity("kyiv")
	a1.TrackSearch("погода")
	a1.TrackError()

	// Force persist
	a1.persistToDisk()

	// Create new instance loading from disk
	a2 := New(filePath)

	summary := a2.Summary()

	if summary.TotalRequests != 1 {
		t.Errorf("expected 1 request after reload, got %d", summary.TotalRequests)
	}
	if summary.APIErrors != 1 {
		t.Errorf("expected 1 error after reload, got %d", summary.APIErrors)
	}
	if len(summary.TopCities) != 1 || summary.TopCities[0].Name != "kyiv" {
		t.Errorf("expected kyiv city after reload, got %v", summary.TopCities)
	}
	if len(summary.TopSearches) != 1 || summary.TopSearches[0].Query != "погода" {
		t.Errorf("expected 'погода' search after reload, got %v", summary.TopSearches)
	}
}

func TestAnalytics_CleanupOldIPs(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "analytics.json")

	a := New(filePath)

	// Add recent IP
	a.TrackRequest("/", "1.2.3.4")

	// Manually add old IP
	a.mu.Lock()
	a.UniqueIPs["5.6.7.8"] = time.Now().Add(-25 * time.Hour)
	a.mu.Unlock()

	// Run cleanup
	a.cleanupOldIPs()

	summary := a.Summary()

	if summary.UniqueIPsToday != 1 {
		t.Errorf("expected 1 unique IP after cleanup, got %d", summary.UniqueIPsToday)
	}
}

func TestAnalytics_TrimTopSearches(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "analytics.json")

	a := New(filePath)

	// Add 110 searches
	for i := 0; i < 110; i++ {
		a.TrackSearch(string(rune('a' + i%26)))
	}

	a.mu.Lock()
	if len(a.TopSearches) > 100 {
		t.Errorf("expected max 100 searches, got %d", len(a.TopSearches))
	}
	a.mu.Unlock()
}

func TestAnalytics_GetUptime(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "analytics.json")

	a := New(filePath)

	// Just check format - uptime should be short (just started)
	uptime := a.GetUptime()
	if uptime == "" {
		t.Error("expected non-empty uptime")
	}
}
