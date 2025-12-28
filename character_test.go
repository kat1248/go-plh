package main

import (
	"bytes"
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

	// Mock server for all API endpoints
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

	// Use a custom HTTP client pointing to the mock server
	mockClient := s.Client()

	r := fetchCharacterDataWithClient(context.Background(), "Mynxee", mockClient, s.URL+"/", s.URL+"/")
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
}

// Example wrapper for fetchCharacterData that injects a client and URLs
func fetchCharacterDataWithClient(ctx context.Context, name string, client *http.Client, ccpURL, zkillURL string) *characterResponse {
	oldClient := httpClient
	oldCcp := ccpEsiURL
	oldZkill := zkillAPIURL
	httpClient = client
	ccpEsiURL = ccpURL
	zkillAPIURL = zkillURL
	defer func() {
		httpClient = oldClient
		ccpEsiURL = oldCcp
		zkillAPIURL = oldZkill
	}()

	return fetchCharacterData(ctx, name)
}

func TestFetchCharacterData_NotFound(t *testing.T) {
	ccpCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	zkillCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	mockClient := s.Client()
	r := fetchCharacterDataWithClient(context.Background(), "NoSuch", mockClient, s.URL+"/", s.URL+"/")
	if r.err == nil {
		t.Fatalf("expected error for missing character, got nil")
	}
}

func TestFetchCharacterData_Timeout(t *testing.T) {
	ccpCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	zkillCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{"characters": []map[string]any{{"id": 77, "name": "Slow"}}})
	}))
	defer s.Close()

	mockClient := s.Client()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := fetchCharacterDataWithClient(ctx, "Slow", mockClient, s.URL+"/", s.URL+"/")
	if r.err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
}
