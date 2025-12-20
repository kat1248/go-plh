package main

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	json "github.com/goccy/go-json"
	cache "zgo.at/zcache/v2"
)

type zKillCharInfo struct {
	CharacterID   int `json:"character_id"`
	CorporationID int `json:"corporation_id"`
	AllianceID    int `json:"alliance_id"`
	ShipTypeID    int `json:"ship_type_id"`
}

type killMail struct {
	Time      string          `json:"killmail_time"`
	Victim    zKillCharInfo   `json:"victim"`
	Attackers []zKillCharInfo `json:"attackers"`
}

type zKillMailInfo struct {
	Hash string `json:"hash"`
}

type zKillMail struct {
	ID   int           `json:"killmail_id"`
	Info zKillMailInfo `json:"zkb"`
}

var (
	computeFavoriteShip = false
	killmailCache       = cache.New[string, any](1*time.Hour, 10*time.Minute)
)

// singleflight for killmail fetches to deduplicate inflight requests
var killmailSingleFlight struct {
	mu sync.Mutex
	m  map[string]*inflight
}

type inflight struct {
	wg  sync.WaitGroup
	res *killMail
	err error
}

func ccpGetKillMail(ctx context.Context, id int, hash string) *killMail {
	// check cache first
	key := fmt.Sprintf("%d:%s", id, hash)
	if rec, found := killmailCache.Get(key); found {
		km := rec.(killMail)
		return &km
	}

	// singleflight: check or join inflight
	killmailSingleFlight.mu.Lock()
	if killmailSingleFlight.m == nil {
		killmailSingleFlight.m = make(map[string]*inflight)
	}
	if in, ok := killmailSingleFlight.m[key]; ok {
		// join existing inflight - leader has already added to the WaitGroup
		killmailSingleFlight.mu.Unlock()
		in.wg.Wait()
		// even on error, return what the leader got
		return in.res
	}
	// become the leader
	in := &inflight{}
	in.wg.Add(1)
	killmailSingleFlight.m[key] = in
	killmailSingleFlight.mu.Unlock()

	// leader fetches the killmail
	km := killMail{}
	ids := fmt.Sprint(id)
	jsonPayload, err := ccpGet(ctx, "killmails/"+ids+"/"+hash+"/", nil)
	if err == nil {
		if err := json.Unmarshal(jsonPayload, &km); err != nil {
			err = err
		}
	}

	// store result and wake up waiters
	killmailSingleFlight.mu.Lock()
	in.res = &km
	in.err = err
	in.wg.Done()
	delete(killmailSingleFlight.m, key)
	killmailSingleFlight.mu.Unlock()

	if err == nil {
		killmailCache.Set(key, km)
	}

	return &km
}

func fetchLastKillActivity(ctx context.Context, id int) *characterResponse {
	cd := characterData{LastKill: ""}

	ids := fmt.Sprint(id)

	jsonPayload, err := zkillGet(ctx, "characterID/"+ids+"/")
	if err != nil {
		return &characterResponse{&cd, err}
	}

	entries := make([]zKillMail, 0)

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &characterResponse{&cd, fmt.Errorf("no kills for id %s", ids)}
	}

	km := ccpGetKillMail(ctx, entries[0].ID, entries[0].Info.Hash)

	when := getDate(km.Time)
	who := km.Victim.CharacterID

	var what string
	switch {
	case who == id:
		what = "loss"
	case who == 0:
		what = "struct"
	default:
		what = "kill"
	}

	cd.LastKill = when + " (" + what + ")"

	return &characterResponse{&cd, nil}
}

func fetchKillHistory(ctx context.Context, id int) *characterResponse {
	cd := characterData{
		RecentExplorerTotal: 0, RecentKillTotal: 0, LastKillTime: "",
		FavoriteShipID: 0, FavoriteShipCount: 0}

	ids := fmt.Sprint(id)

	jsonPayload, err := zkillGet(ctx, "kills/characterID/"+ids+"/")
	if err != nil {
		return &characterResponse{&cd, err}
	}

	entries := make([]zKillMail, 0)

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &characterResponse{&cd, fmt.Errorf("no kills for id %s", ids)}
	}

	cd.RecentKillTotal = len(entries)

	explorerShips := map[int]bool{
		29248: true, 11188: true, 11192: true,
		605: true, 11172: true, 607: true,
		11182: true, 586: true, 33468: true, 33470: true}

	explorerTotal := 0
	shipFreq := make(map[int]int)
	var mu sync.Mutex
	// cap concurrency to avoid rate limiting and spikes
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	for i, k := range entries {
		wg.Add(1)
		sem <- struct{}{}
		go func(entry zKillMail, last bool) {
			defer wg.Done()
			defer func() { <-sem }()
			km := ccpGetKillMail(ctx, entry.ID, entry.Info.Hash)
			if km == nil {
				return
			}
			localExplorer := 0
			localFreq := make(map[int]int)
			for _, attacker := range km.Attackers {
				if attacker.CharacterID == id {
					if explorerShips[km.Victim.ShipTypeID] {
						localExplorer++
					}
					localFreq[attacker.ShipTypeID]++
				}
			}
			mu.Lock()
			if last {
				cd.LastKillTime = getDate(km.Time)
			}
			explorerTotal += localExplorer
			for ship, cnt := range localFreq {
				shipFreq[ship] += cnt
			}
			mu.Unlock()
		}(k, i == len(entries)-1)
	}
	wg.Wait()

	cd.RecentExplorerTotal = explorerTotal

	if computeFavoriteShip {
		// pick the ship with the highest count
		bestID := 0
		bestCnt := 0
		for ship, cnt := range shipFreq {
			if cnt > bestCnt {
				bestCnt = cnt
				bestID = ship
			}
		}
		cd.FavoriteShipID = bestID
		cd.FavoriteShipCount = bestCnt
	}

	return &characterResponse{&cd, nil}
}

func fetchRecentKillHistory(ctx context.Context, id int) *characterResponse {
	cd := characterData{KillsLastWeek: 0}

	ids := fmt.Sprint(id)

	jsonPayload, err := zkillGet(ctx, "kills/characterID/"+ids+"/pastSeconds/"+fmt.Sprint(secondsInWeek)+"/")
	if err != nil {
		return &characterResponse{&cd, err}
	}

	// stream the JSON array and count elements without allocating the whole slice
	dec := json.NewDecoder(bytes.NewReader(jsonPayload))
	// expect an array
	tkn, err := dec.Token()
	if err != nil {
		return &characterResponse{&cd, err}
	}
	if delim, ok := tkn.(json.Delim); !ok || delim != '[' {
		// fallback to full unmarshal if not an array
		entries := make([]killMail, 0)
		if err := json.Unmarshal(jsonPayload, &entries); err != nil {
			return &characterResponse{&cd, err}
		}
		cd.KillsLastWeek = len(entries)
		return &characterResponse{&cd, nil}
	}

	count := 0
	for dec.More() {
		var km killMail
		if err := dec.Decode(&km); err != nil {
			return &characterResponse{&cd, err}
		}
		count++
	}
	cd.KillsLastWeek = count

	return &characterResponse{&cd, nil}
}
