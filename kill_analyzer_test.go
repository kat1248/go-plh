package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchRecentKillHistory_Counts(t *testing.T) {
	// start a test server to serve zkill responses
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// a simple array of two killmails
		if r.URL.Path == "/kills/characterID/123/pastSeconds/604800/" {
			resp := []killMail{
				{Time: "2020-01-01T00:00:00Z", Victim: zKillCharInfo{CharacterID: 1, ShipTypeID: 33468}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}}},
				{Time: "2020-01-02T00:00:00Z", Victim: zKillCharInfo{CharacterID: 1, ShipTypeID: 605}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11172}}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	// override zkill API base
	origZkill := zkillAPIURL
	zkillAPIURL = s.URL + "/"
	defer func() { zkillAPIURL = origZkill }()

	r := fetchRecentKillHistory(context.Background(), 123)
	if r.err != nil {
		t.Fatalf("unexpected err: %v", r.err)
	}
	if r.char.KillsLastWeek != 2 {
		t.Fatalf("expected 2 kills, got %d", r.char.KillsLastWeek)
	}
}

func TestFetchKillHistory_ExplorerAndCounts(t *testing.T) {
	// start a test server to serve both zkill and ccp endpoints
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/123/":
			// two zKillMail entries
			resp := []zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}, {ID: 2, Info: zKillMailInfo{Hash: "h2"}}}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		case "/killmails/1/h1/":
			// attacker is our character 123 in an explorer ship
			resp := killMail{Time: "2020-01-01T00:00:00Z", Victim: zKillCharInfo{ShipTypeID: 33468}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}}}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		case "/killmails/2/h2/":
			resp := killMail{Time: "2020-01-02T00:00:00Z", Victim: zKillCharInfo{ShipTypeID: 605}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11172}}}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	origCcp := ccpEsiURL
	zkillAPIURL = s.URL + "/"
	ccpEsiURL = s.URL + "/"
	defer func() { zkillAPIURL = origZkill; ccpEsiURL = origCcp }()

	r := fetchKillHistory(context.Background(), 123)
	if r.err != nil {
		t.Fatalf("unexpected err: %v", r.err)
	}
	if r.char.RecentKillTotal != 2 {
		t.Fatalf("expected 2 recent kills, got %d", r.char.RecentKillTotal)
	}
	if r.char.RecentExplorerTotal != 2 {
		t.Fatalf("expected 2 explorer kills, got %d", r.char.RecentExplorerTotal)
	}
}

func TestFetchRecentKillHistory_ContextCancelled(t *testing.T) {
	// server that sleeps, to force timeout/cancel
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	zkillAPIURL = s.URL + "/"
	defer func() { zkillAPIURL = origZkill }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := fetchRecentKillHistory(ctx, 123)
	if r.err == nil {
		t.Fatalf("expected error due to context cancel, got nil")
	}
}

func BenchmarkFetchKillHistory_Small(b *testing.B) {
	// simple server returning small payloads
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/123/":
			resp := []zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/killmails/1/h1/":
			resp := killMail{Time: "2020-01-01T00:00:00Z", Victim: zKillCharInfo{ShipTypeID: 33468}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}}}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	origCcp := ccpEsiURL
	zkillAPIURL = s.URL + "/"
	ccpEsiURL = s.URL + "/"
	defer func() { zkillAPIURL = origZkill; ccpEsiURL = origCcp }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := fetchKillHistory(context.Background(), 123)
		if r.err != nil {
			b.Fatalf("unexpected err: %v", r.err)
		}
	}
}

func TestFetchKillHistory_ZkillError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kills/characterID/123/" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	orig := zkillAPIURL
	zkillAPIURL = s.URL + "/"
	defer func() { zkillAPIURL = orig }()

	r := fetchKillHistory(context.Background(), 123)
	if r.err == nil {
		t.Fatalf("expected error when zkill returns 500, got nil")
	}
}

func TestFetchKillHistory_ContextCancelled(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kills/characterID/123/" {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[]"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	orig := zkillAPIURL
	zkillAPIURL = s.URL + "/"
	defer func() { zkillAPIURL = orig }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := fetchKillHistory(ctx, 123)
	if r.err == nil {
		t.Fatalf("expected error due to context cancel, got nil")
	}
}
