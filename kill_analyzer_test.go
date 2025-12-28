package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cache "zgo.at/zcache/v2"
)

func TestFetchRecentKillHistory_Counts(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kills/characterID/123/pastSeconds/604800/" {
			_ = json.NewEncoder(w).Encode([]killMail{
				{Time: "2020-01-01T00:00:00Z", Victim: zKillCharInfo{CharacterID: 1, ShipTypeID: 33468}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}}},
				{Time: "2020-01-02T00:00:00Z", Victim: zKillCharInfo{CharacterID: 1, ShipTypeID: 605}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11172}}},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

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
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	hits := 0

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/123/":
			_ = json.NewEncoder(w).Encode([]zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}, {ID: 2, Info: zKillMailInfo{Hash: "h2"}}})
		case "/killmails/1/h1/", "/killmails/2/h2/":
			hits++
			_ = json.NewEncoder(w).Encode(killMail{Time: "2020-01-01T00:00:00Z", Victim: zKillCharInfo{ShipTypeID: 33468}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	zkillAPIURL = s.URL + "/"
	defer func() { zkillAPIURL = origZkill }()

	r := fetchKillHistory(context.Background(), 123)
	if r.err != nil {
		t.Fatalf("unexpected err: %v", r.err)
	}
	if r.char.RecentKillTotal != 2 {
		t.Fatalf("expected 2 recent kills, got %d", r.char.RecentKillTotal)
	}
	if hits == 0 {
		t.Fatalf("expected killmail endpoints to be hit, got %d", hits)
	}
	if r.char.RecentExplorerTotal != 2 {
		t.Fatalf("expected 2 explorer kills, got %d", r.char.RecentExplorerTotal)
	}
}

func TestKillmailCache_Basic(t *testing.T) {
	hits := 0
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/123/":
			_ = json.NewEncoder(w).Encode([]zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}})
		case "/killmails/1/h1/":
			hits++
			_ = json.NewEncoder(w).Encode(killMail{Time: "2020-01-01T00:00:00Z", Victim: zKillCharInfo{ShipTypeID: 33468}, Attackers: []zKillCharInfo{{CharacterID: 123, ShipTypeID: 11188}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	zkillAPIURL = s.URL + "/"
	defer func() { zkillAPIURL = origZkill }()

	_ = fetchKillHistory(context.Background(), 123)
	_ = fetchKillHistory(context.Background(), 123)

	if hits != 1 {
		t.Fatalf("expected killmail endpoint to be hit once, got %d", hits)
	}
}
