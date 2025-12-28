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

func TestFetchCharacterData_Basic(t *testing.T) {
	ccpCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	zkillCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/universe/ids/":
			_ = json.NewEncoder(w).Encode(map[string]any{"characters": []map[string]any{{"id": 123, "name": "Mynxee"}}})
		case "/characters/123/":
			_ = json.NewEncoder(w).Encode(ccpResponse{Name: "Mynxee", CorpID: 456, AllianceID: 789, Security: 1.23, Birthday: "2000-01-01T00:00:00Z"})
		case "/stats/characterID/123/":
			_ = json.NewEncoder(w).Encode(zKillResponse{Danger: 10, Gang: 2, Kills: 0, Losses: 0})
		case "/stats/corporationID/456/":
			_ = json.NewEncoder(w).Encode(zKillResponse{Danger: 5, Gang: 1, Kills: 0, Losses: 0})
		case "/characters/123/corporationhistory":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"start_date": "2010-01-01T00:00:00Z"}})
		case "/corporations/456/":
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "TestCorp"})
		case "/alliances/789/":
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "TestAlliance"})
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

	r := fetchCharacterData(context.Background(), "Mynxee")
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.char.CharacterID != 123 {
		t.Fatalf("expected id 123, got %d", r.char.CharacterID)
	}
	if r.char.CorpName != "TestCorp" {
		t.Fatalf("expected corp name TestCorp, got %s", r.char.CorpName)
	}
	if r.char.AllianceName != "TestAlliance" {
		t.Fatalf("expected alliance name TestAlliance, got %s", r.char.AllianceName)
	}
	if r.char.Name != "Space Mom" { // nickname mapping
		t.Fatalf("expected nickname mapping to Space Mom, got %s", r.char.Name)
	}
}

func TestFetchCharacterData_NotFound(t *testing.T) {
	ccpCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	zkillCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/universe/ids/" {
			_ = json.NewEncoder(w).Encode(map[string]any{"characters": []any{}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	origCcp := ccpEsiURL
	ccpEsiURL = s.URL + "/"
	defer func() { ccpEsiURL = origCcp }()

	r := fetchCharacterData(context.Background(), "NoSuch")
	if r.err == nil {
		t.Fatalf("expected error for missing character, got nil")
	}
}

func TestFetchCharacterData_WithKills(t *testing.T) {
	ccpCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	zkillCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	analyzeKills = true

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/universe/ids/":
			_ = json.NewEncoder(w).Encode(map[string]any{"characters": []map[string]any{{"id": 999, "name": "Pilot"}}})
		case "/characters/999/":
			_ = json.NewEncoder(w).Encode(ccpResponse{Name: "Pilot", CorpID: 10, AllianceID: 0, Security: 0.5, Birthday: "2005-01-01T00:00:00Z"})
		case "/stats/characterID/999/":
			_ = json.NewEncoder(w).Encode(zKillResponse{Danger: 1, Gang: 0, Kills: 2, Losses: 1})
		case "/kills/characterID/999/":
			_ = json.NewEncoder(w).Encode([]killMail{{Time: "2020-01-01T00:00:00Z"}, {Time: "2020-01-02T00:00:00Z"}})
		case "/stats/corporationID/10/":
			_ = json.NewEncoder(w).Encode(zKillResponse{Danger: 1, Gang: 0, Kills: 0, Losses: 0})
		case "/characters/999/corporationhistory":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"start_date": "2012-05-01T00:00:00Z"}})
		case "/corporations/10/":
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "PilotCorp"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	origCcp := ccpEsiURL
	zkillAPIURL = s.URL + "/"
	ccpEsiURL = s.URL + "/"
	defer func() { zkillAPIURL = origZkill; ccpEsiURL = origCcp; analyzeKills = false }()

	r := fetchCharacterData(context.Background(), "Pilot")
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
