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
			names := r.URL.Query().Get("names")
			if names == "Mynxee" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"characters": []map[string]any{{"id": 123, "name": "Mynxee"}},
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case "/characters/123/":
			_ = json.NewEncoder(w).Encode(ccpResponse{
				Name:       "Mynxee",
				CorpID:     456,
				AllianceID: 789,
				Security:   1.23,
				Birthday:   "2000-01-01T00:00:00Z",
			})
		case "/stats/characterID/123/":
			_ = json.NewEncoder(w).Encode(zKillResponse{Kills: 0, Losses: 0})
		case "/stats/corporationID/456/":
			_ = json.NewEncoder(w).Encode(zKillResponse{})
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
	zkillAPIURL = s.URL
	ccpEsiURL = s.URL
	defer func() { zkillAPIURL = origZkill; ccpEsiURL = origCcp }()

	r := fetchCharacterData(context.Background(), "Mynxee")
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.char.CharacterID != 123 {
		t.Fatalf("expected id 123, got %d", r.char.CharacterID)
	}
	if r.char.CorpName != "TestCorp" {
		t.Fatalf("expected corp TestCorp, got %s", r.char.CorpName)
	}
	if r.char.AllianceName != "TestAlliance" {
		t.Fatalf("expected alliance TestAlliance, got %s", r.char.AllianceName)
	}
}
