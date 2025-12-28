package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	cache "zgo.at/zcache/v2"
)

// helper to inject mock client and URL for testing
func fetchKillHistoryWithClient(ctx context.Context, charID int, client *http.Client, zkillBase string) *killHistoryResponse {
	oldClient := httpClient
	oldZkill := zkillAPIURL
	httpClient = client
	zkillAPIURL = zkillBase
	defer func() {
		httpClient = oldClient
		zkillAPIURL = oldZkill
	}()
	return fetchKillHistory(ctx, charID)
}

func TestFetchRecentKillHistory_Counts(t *testing.T) {
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kills/characterID/123/pastSeconds/604800/" {
			resp := []killMail{
				{Time: "2020-01-01T00:00:00Z", Victim: zKillCharInfo{CharacterID: 1, ShipTypeID: 33468}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}}},
				{Time: "2020-01-02T00:00:00Z", Victim: zKillCharInfo{CharacterID: 1, ShipTypeID: 605}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11172}}},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	r := fetchRecentKillHistory(context.Background(), 123)
	if r.err != nil {
		t.Fatalf("unexpected err: %v", r.err)
	}
	if r.char.KillsLastWeek != 2 {
		t.Fatalf("expected 2 kills, got %d", r.char.KillsLastWeek)
	}
}

func TestFetchKillHistory_ExplorerAndCounts(t *testing.T) {
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	killmailHits := 0

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/123/":
			_ = json.NewEncoder(w).Encode([]zKillMail{
				{ID: 1, Info: zKillMailInfo{Hash: "h1"}},
				{ID: 2, Info: zKillMailInfo{Hash: "h2"}},
			})
		case "/killmails/1/h1/":
			killmailHits++
			_ = json.NewEncoder(w).Encode(killMail{
				Time: "2020-01-01T00:00:00Z",
				Victim: zKillCharInfo{ShipTypeID: 33468},
				Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}},
			})
		case "/killmails/2/h2/":
			killmailHits++
			_ = json.NewEncoder(w).Encode(killMail{
				Time: "2020-01-02T00:00:00Z",
				Victim: zKillCharInfo{ShipTypeID: 605},
				Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11172}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	mockClient := s.Client()
	r := fetchKillHistoryWithClient(context.Background(), 123, mockClient, s.URL+"/")
	if r.err != nil {
		t.Fatalf("unexpected err: %v", r.err)
	}
	if r.char.RecentKillTotal != 2 {
		t.Fatalf("expected 2 recent kills, got %d", r.char.RecentKillTotal)
	}
	if r.char.RecentExplorerTotal != 2 {
		t.Fatalf("expected 2 explorer kills, got %d", r.char.RecentExplorerTotal)
	}
	if killmailHits == 0 {
		t.Fatalf("expected killmail endpoints to be hit, got %d", killmailHits)
	}
}

func TestFetchRecentKillHistory_ContextCancelled(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode([]killMail{})
	}))
	defer s.Close()

	mockClient := s.Client()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := fetchKillHistoryWithClient(ctx, 123, mockClient, s.URL+"/")
	if r.err == nil {
		t.Fatalf("expected error due to context cancel, got nil")
	}
}

func TestKillmailCache_Basic(t *testing.T) {
	hits := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/123/":
			_ = json.NewEncoder(w).Encode([]zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}})
		case "/killmails/1/h1/":
			hits++
			_ = json.NewEncoder(w).Encode(killMail{
				Time: "2020-01-01T00:00:00Z",
				Victim: zKillCharInfo{ShipTypeID: 33468},
				Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	mockClient := s.Client()
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	r := fetchKillHistoryWithClient(context.Background(), 123, mockClient, s.URL+"/")
	if r.err != nil {
		t.Fatalf("unexpected err: %v", r.err)
	}
	r = fetchKillHistoryWithClient(context.Background(), 123, mockClient, s.URL+"/")
	if hits != 1 {
		t.Fatalf("expected killmail endpoint to be hit once, got %d", hits)
	}
}

func TestKillmailCache_Expiration(t *testing.T) {
	hits := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/123/":
			_ = json.NewEncoder(w).Encode([]zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}})
		case "/killmails/1/h1/":
			hits++
			_ = json.NewEncoder(w).Encode(killMail{
				Time: "2020-01-01T00:00:00Z",
				Victim: zKillCharInfo{ShipTypeID: 33468},
				Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	mockClient := s.Client()
	killmailCache = cache.New[string, any](50*time.Millisecond, 10*time.Millisecond)

	r := fetchKillHistoryWithClient(context.Background(), 123, mockClient, s.URL+"/")
	time.Sleep(100 * time.Millisecond)
	r = fetchKillHistoryWithClient(context.Background(), 123, mockClient, s.URL+"/")

	if hits != 2 {
		t.Fatalf("expected killmail endpoint to be hit twice after expiration, got %d", hits)
	}
}

func TestKillmailSingleflight_Deduplication(t *testing.T) {
	hits := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/1/":
			_ = json.NewEncoder(w).Encode([]zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}})
		case "/killmails/1/h1/":
			hits++
			time.Sleep(50 * time.Millisecond) // encourage concurrent join
			_ = json.NewEncoder(w).Encode(killMail{
				Time: "2020-01-01T00:00:00Z",
				Victim: zKillCharInfo{ShipTypeID: 33468},
				Attackers: []zKillCharInfo{{CharacterID: 1, ShipTypeID: 11188}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	mockClient := s.Client()
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	killmailSingleFlight = struct {
		mu sync.Mutex
		m  map[string]*inflight
	}{}

	var wg sync.WaitGroup
	n := 10
	results := make([]*killMail, 0, n)
	var mu sync.Mutex

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			km := ccpGetKillMail(context.Background(), 1, "h1")
			mu.Lock()
			results = append(results, km)
			mu.Unlock()
		}()
	}
	wg.Wait()

	if hits != 1 {
		t.Fatalf("expected 1 hit due to singleflight deduplication, got %d", hits)
	}
	for i, km := range results {
		if km == nil {
			t.Fatalf("result %d is nil", i)
		}
	}
}
