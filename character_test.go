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
	// reset caches for isolation
	ccpCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	zkillCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	// create test server for both CCP and zkill endpoints
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/universe/ids/":
			_ = json.NewEncoder(w).Encode(map[string]any{"characters": []map[string]any{{"id": 123, "name": "Mynxee"}}})
			return
		case "/characters/123/":
			_ = json.NewEncoder(w).Encode(ccpResponse{Name: "Mynxee", CorpID: 456, AllianceID: 789, Security: 1.23, Birthday: "2000-01-01T00:00:00Z"})
			return
		case "/stats/characterID/123/":
			_ = json.NewEncoder(w).Encode(zKillResponse{Danger: 10, Gang: 2, Kills: 0, Losses: 0})
			return
		case "/stats/corporationID/456/":
			_ = json.NewEncoder(w).Encode(zKillResponse{Danger: 5, Gang: 1, Kills: 0, Losses: 0})
			return
		case "/characters/123/corporationhistory":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"start_date": "2010-01-01T00:00:00Z"}})
			return
		case "/corporations/456/":
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "TestCorp"})
			return
		case "/alliances/789/":
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "TestAlliance"})
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
	// nickname mapping should apply
	if r.char.Name != "Space Mom" {
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

	// server with kills and killmails
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/universe/ids/":
			_ = json.NewEncoder(w).Encode(map[string]any{"characters": []map[string]any{{"id": 999, "name": "Pilot"}}})
		case "/characters/999/":
			_ = json.NewEncoder(w).Encode(ccpResponse{Name: "Pilot", CorpID: 10, AllianceID: 0, Security: 0.5, Birthday: "2005-01-01T00:00:00Z"})
		case "/stats/characterID/999/":
			_ = json.NewEncoder(w).Encode(zKillResponse{Danger: 1, Gang: 0, Kills: 2, Losses: 1})
		case "/characterID/999/":
			_ = json.NewEncoder(w).Encode([]zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}, {ID: 2, Info: zKillMailInfo{Hash: "h2"}}})
			return
		case "/stats/corporationID/10/":
			_ = json.NewEncoder(w).Encode(zKillResponse{Danger: 1, Gang: 0, Kills: 0, Losses: 0})
			return
		case "/characters/999/corporationhistory":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"start_date": "2012-05-01T00:00:00Z"}})
			return
		case "/kills/characterID/999/":
			_ = json.NewEncoder(w).Encode([]zKillMail{{ID: 1, Info: zKillMailInfo{Hash: "h1"}}, {ID: 2, Info: zKillMailInfo{Hash: "h2"}}})
		case "/killmails/1/h1/":
			_ = json.NewEncoder(w).Encode(killMail{Time: "2020-01-01T00:00:00Z", Victim: zKillCharInfo{ShipTypeID: 33468}, Attackers: []zKillCharInfo{{CharacterID: 999, ShipTypeID: 11188}}})
		case "/killmails/2/h2/":
			_ = json.NewEncoder(w).Encode(killMail{Time: "2020-01-02T00:00:00Z", Victim: zKillCharInfo{ShipTypeID: 605}, Attackers: []zKillCharInfo{{CharacterID: 999, ShipTypeID: 11172}}})
		case "/kills/characterID/999/pastSeconds/604800/":
			_ = json.NewEncoder(w).Encode([]killMail{{Time: "2020-01-01T00:00:00Z"}, {Time: "2020-01-02T00:00:00Z"}})
		case "/corporations/10/":
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "PilotCorp"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	origZkill := zkillAPIURL
	origCcp := ccpEsiURL
	oldAnalyze := analyzeKills
	zkillAPIURL = s.URL + "/"
	ccpEsiURL = s.URL + "/"
	analyzeKills = true
	defer func() { zkillAPIURL = origZkill; ccpEsiURL = origCcp; analyzeKills = oldAnalyze }()

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

func TestFetchCharacterID_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
		wantID  int
	}{
		{
			name: "valid response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/universe/ids/" {
					_ = json.NewEncoder(w).Encode(map[string]any{"characters": []map[string]any{{"id": 321, "name": "X"}}})
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: false,
			wantID:  321,
		},
		{
			name: "empty list",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/universe/ids/" {
					_ = json.NewEncoder(w).Encode(map[string]any{"characters": []any{}})
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
		{
			name: "malformed json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/universe/ids/" {
					w.Write([]byte("not-json"))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/universe/ids/" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ccpCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
			s := httptest.NewServer(tc.handler)
			defer s.Close()

			orig := ccpEsiURL
			ccpEsiURL = s.URL + "/"
			defer func() { ccpEsiURL = orig }()

			id, err := fetchCharacterID(context.Background(), "whatever")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tc.wantID {
				t.Fatalf("expected id %d got %d", tc.wantID, id)
			}
		})
	}
}

func TestFetchCharacterData_Timeout(t *testing.T) {
	ccpCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	zkillCache = cache.New[string, any](1*time.Hour, 10*time.Minute)

	// server returns slowly on stats endpoint to trigger a timeout
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/universe/ids/":
			_ = json.NewEncoder(w).Encode(map[string]any{"characters": []map[string]any{{"id": 77, "name": "Slow"}}})
			return
		case "/characters/77/":
			_ = json.NewEncoder(w).Encode(ccpResponse{Name: "Slow", CorpID: 5, AllianceID: 0, Security: 0.0, Birthday: "2001-01-01T00:00:00Z"})
			return
		case "/stats/characterID/77/":
			time.Sleep(200 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(zKillResponse{Danger: 0, Gang: 0, Kills: 0, Losses: 0})
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

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := fetchCharacterData(ctx, "Slow")
	if r.err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
}
