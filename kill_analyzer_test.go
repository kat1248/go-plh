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

func TestFetchRecentKillHistory_Counts(t *testing.T) {
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kills/characterID/123/pastSeconds/604800" {
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

	origZkill := zkillAPIURL
	zkillAPIURL = s.URL
	defer func() { zkillAPIURL = origZkill }()

	r := fetchRecentKillHistory(context.Background(), 123)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.char.KillsLastWeek != 2 {
		t.Fatalf("expected 2 kills, got %d", r.char.KillsLastWeek)
	}
}

func TestFetchKillHistory_ExplorerAndCounts(t *testing.T) {
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	killmailSingleFlight = struct {
		mu sync.Mutex
		m  map[string]*inflight
	}{m: map[string]*inflight{}}

	hits := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/123":
			resp := []zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}, {ID: 2, Info: zKillMailInfo{Hash: "h2"}}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/killmails/1/h1", "/killmails/2/h2":
			hits++
			var mail killMail
			if r.URL.Path == "/killmails/1/h1" {
				mail = killMail{Time: "2020-01-01T00:00:00Z", Victim: zKillCharInfo{ShipTypeID: 33468}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}}}
			} else {
				mail = killMail{Time: "2020-01-02T00:00:00Z", Victim: zKillCharInfo{ShipTypeID: 605}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11172}}}
			}
			_ = json.NewEncoder(w).Encode(mail)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	zkillAPIURL = s.URL
	defer func() { zkillAPIURL = origZkill }()

	r := fetchKillHistory(context.Background(), 123)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.char.RecentKillTotal != 2 {
		t.Fatalf("expected 2 recent kills, got %d", r.char.RecentKillTotal)
	}
	if r.char.RecentExplorerTotal != 2 {
		t.Fatalf("expected 2 explorer kills, got %d", r.char.RecentExplorerTotal)
	}
	if hits != 2 {
		t.Fatalf("expected killmail endpoints to be hit twice, got %d", hits)
	}
}

func TestKillmailCache_Basic(t *testing.T) {
	hits := 0
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	killmailSingleFlight = struct {
		mu sync.Mutex
		m  map[string]*inflight
	}{m: map[string]*inflight{}}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/123":
			_ = json.NewEncoder(w).Encode([]zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}})
		case "/killmails/1/h1":
			hits++
			_ = json.NewEncoder(w).Encode(killMail{Time: "2020-01-01T00:00:00Z", Victim: zKillCharInfo{ShipTypeID: 33468}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	zkillAPIURL = s.URL
	defer func() { zkillAPIURL = origZkill }()

	r := fetchKillHistory(context.Background(), 123)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	r = fetchKillHistory(context.Background(), 123)
	if r.err != nil {
		t.Fatalf("unexpected error on second fetch: %v", r.err)
	}

	if hits != 1 {
		t.Fatalf("expected killmail endpoint hit once, got %d", hits)
	}
}

func TestKillmailCache_Expiration(t *testing.T) {
	hits := 0
	killmailCache = cache.New[string, any](50*time.Millisecond, 10*time.Millisecond)
	killmailSingleFlight = struct {
		mu sync.Mutex
		m  map[string]*inflight
	}{m: map[string]*inflight{}}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/123":
			_ = json.NewEncoder(w).Encode([]zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}})
		case "/killmails/1/h1":
			hits++
			_ = json.NewEncoder(w).Encode(killMail{Time: "2020-01-01T00:00:00Z", Victim: zKillCharInfo{ShipTypeID: 33468}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	zkillAPIURL = s.URL
	defer func() { zkillAPIURL = origZkill }()

	r := fetchKillHistory(context.Background(), 123)
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}

	time.Sleep(100 * time.Millisecond)

	r = fetchKillHistory(context.Background(), 123)
	if r.err != nil {
		t.Fatalf("unexpected error on second fetch: %v", r.err)
	}

	if hits != 2 {
		t.Fatalf("expected killmail endpoint hit twice after expiration, got %d", hits)
	}
}

func TestFetchKillHistory_ContextCancelled(t *testing.T) {
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]zKillMail{})
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	zkillAPIURL = s.URL
	defer func() { zkillAPIURL = origZkill }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := fetchKillHistory(ctx, 123)
	if r.err == nil {
		t.Fatalf("expected error due to context cancel, got nil")
	}
}

func TestFetchRecentKillHistory_ContextCancelled(t *testing.T) {
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]killMail{})
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	zkillAPIURL = s.URL
	defer func() { zkillAPIURL = origZkill }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := fetchRecentKillHistory(ctx, 123)
	if r.err == nil {
		t.Fatalf("expected error due to context cancel, got nil")
	}
}
