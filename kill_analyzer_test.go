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
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kills/characterID/123/pastSeconds/604800/" {
			resp := []killMail{
				{Time: "2020-01-01T00:00:00Z"},
				{Time: "2020-01-02T00:00:00Z"},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	orig := zkillAPIURL
	zkillAPIURL = s.URL
	defer func() { zkillAPIURL = orig }()

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
			resp := []zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}, {ID: 2, Info: zKillMailInfo{Hash: "h2"}}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/killmails/1/h1/":
			killmailHits++
			_ = json.NewEncoder(w).Encode(killMail{Time: "2020-01-01T00:00:00Z"})
		case "/killmails/2/h2/":
			killmailHits++
			_ = json.NewEncoder(w).Encode(killMail{Time: "2020-01-02T00:00:00Z"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	orig := zkillAPIURL
	zkillAPIURL = s.URL
	defer func() { zkillAPIURL = orig }()

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
	if killmailHits != 2 {
		t.Fatalf("expected killmail endpoints to be hit twice, got %d", killmailHits)
	}
}

func TestKillmailCache_Basic(t *testing.T) {
	killmailCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	hits := 0

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kills/characterID/123/":
			_ = json.NewEncoder(w).Encode([]zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}})
		case "/killmails/1/h1/":
			hits++
			_ = json.NewEncoder(w).Encode(killMail{Time: "2020-01-01T00:00:00Z"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	orig := zkillAPIURL
	zkillAPIURL = s.URL
	defer func() { zkillAPIURL = orig }()

	_ = fetchKillHistory(context.Background(), 123)
	_ = fetchKillHistory(context.Background(), 123)

	if hits != 1 {
		t.Fatalf("expected killmail endpoint to be hit once, got %d", hits)
	}
}
