package main

import (
	"bytes"
	"context"

	"fmt"
	"sync"
	"time"

	"dario.cat/mergo"
	json "github.com/goccy/go-json"
	cache "zgo.at/zcache/v2"
)

const (
	ccpEsiURL   = "https://esi.evetech.net/latest/"
	zkillAPIURL = "https://zkillboard.com/api/"
	zkillURL    = "https://zkillboard.com/"
)

var (
	ccpCache   = cache.New[string, any](1*time.Hour, 10*time.Minute)
	zkillCache = cache.New[string, any](1*time.Hour, 10*time.Minute)
	nicknames  = map[string]string{
		"Mynxee":        "Space Mom",
		"Portia Tigana": "Tiggs"}
)

func (c characterData) String() string {
	return c.Name
}

func fetchCharacterData(ctx context.Context, name string) *characterResponse {
	cd := characterData{Name: name}

	id, err := fetchCharacterID(ctx, name)
	if err != nil {
		return &characterResponse{&cd, fmt.Errorf("'%s' not found", name)}
	}

	cd.ZkillUsed = true

	cd.CharacterID = id

	ch := make(chan *characterResponse, 3)
	var wg sync.WaitGroup

	fetcher := func(f func(context.Context, int) *characterResponse, id int) {
		wg.Add(1)
		go func(f func(context.Context, int) *characterResponse, id int) {
			defer wg.Done()
			ch <- f(ctx, id)
		}(f, id)
	}

	fetcher(fetchCCPRecord, cd.CharacterID)
	if cd.ZkillUsed {
		fetcher(fetchZKillRecord, cd.CharacterID)
	}
	fetcher(fetchCorpStartDate, cd.CharacterID)

	wg.Wait()
	close(ch)

	if err := cd.handleMerges(ch); err != nil {
		return &characterResponse{&cd, err}
	}

	ch = make(chan *characterResponse, 6)

	if cd.ZkillUsed {
		fetcher(fetchCorpDanger, cd.CorpID)
	}
	fetcher(fetchAllianceName, cd.AllianceID)
	fetcher(fetchCorporationName, cd.CorpID)

	if cd.HasKillboard {
		fetcher(fetchLastKillActivity, cd.CharacterID)
	}

	if analyzeKills && cd.Kills != 0 {
		fetcher(fetchKillHistory, cd.CharacterID)
		fetcher(fetchRecentKillHistory, cd.CharacterID)
	}

	wg.Wait()
	close(ch)

	if err := cd.handleMerges(ch); err != nil {
		return &characterResponse{&cd, err}
	}

	if cd.FavoriteShipID != 0 {
		ch = make(chan *characterResponse, 1)

		fetcher(fetchItemName, cd.FavoriteShipID)

		wg.Wait()
		close(ch)

		if err := cd.handleMerges(ch); err != nil {
			return &characterResponse{&cd, err}
		}
	}

	if n, ok := nicknames[name]; ok {
		cd.Name = n
	}

	return &characterResponse{&cd, nil}
}

func (c *characterData) handleMerges(ch chan *characterResponse) error {
	for r := range ch {
		if r.err != nil {
			return r.err
		}
		mergo.Merge(c, r.char)
	}
	return nil
}

func fetchCCPRecord(ctx context.Context, id int) *characterResponse {
	cd := characterData{}

	ccpRec, err := fetchCharacterJSON(ctx, id)
	if err != nil {
		return &characterResponse{&cd, err}
	}

	var cr ccpResponse

	if err = json.Unmarshal([]byte(ccpRec), &cr); err != nil {
		return &characterResponse{&cd, err}
	}

	cd.Age = secondsToTimeString(secondsSince(cr.Birthday))
	cd.CorpID = cr.CorpID
	cd.Security = cr.Security
	cd.IsNpcCorp = cd.CorpID < 2000000
	cd.AllianceID = cr.AllianceID

	return &characterResponse{&cd, nil}
}

func fetchZKillRecord(ctx context.Context, id int) *characterResponse {
	cd := characterData{ZkillUsed: false}

	zkillRec, err := fetchZKillJSON(ctx, id)
	if err != nil {
		return &characterResponse{&cd, err}
	}

	var zr zKillResponse

	if err = json.Unmarshal([]byte(zkillRec), &zr); err != nil {
		return &characterResponse{&cd, err}
	}

	cd.Danger = zr.Danger
	cd.Gang = zr.Gang
	cd.Kills = zr.Kills
	cd.Losses = zr.Losses
	cd.HasKillboard = (cd.Kills != 0) || (cd.Losses != 0)
	cd.ZkillUsed = true

	return &characterResponse{&cd, nil}
}

func fetchCharacterJSON(ctx context.Context, id int) (string, error) {
	ids := fmt.Sprint(id)

	rec, found := ccpCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	jsonPayload, err := ccpGet(ctx, "characters/"+ids+"/", nil)
	if err != nil {
		return "", err
	}

	ccpCache.Set(ids, string(jsonPayload))
	return string(jsonPayload), nil
}

func fetchZKillJSON(ctx context.Context, id int) (string, error) {
	ids := fmt.Sprint(id)

	rec, found := zkillCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	jsonPayload, err := zkillGet(ctx, "stats/characterID/"+ids+"/")
	if err != nil {
		return "", err
	}

	zkillCache.Set(ids, string(jsonPayload))
	return string(jsonPayload), nil
}

func loadCharacterIds(ctx context.Context, names []string) (bool, error) {
	findNames := []string{}

	for _, name := range names {
		if len(name) == 0 {
			continue
		}
		_, found := ccpCache.Get(name)
		if !found {
			findNames = append(findNames, name)
		}
	}

	// nothing to do, we've already found the ids
	if len(findNames) == 0 {
		return true, nil
	}

	js, err := json.Marshal(findNames)
	if err != nil {
		return false, fmt.Errorf("error marshaling names")
	}

	jsonPayload, err := ccpPost(ctx,
		"universe/ids/",
		map[string]string{"datasource": "tranquility"},
		bytes.NewBuffer(js))
	if err != nil {
		return false, err
	}

	var entries characterList

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return false, err
	}

	if len(entries.Characters) == 0 {
		return false, fmt.Errorf("no entries found")
	}

	for _, entry := range entries.Characters {
		ccpCache.SetWithExpire(entry.Name, entry.ID, cache.NoExpiration)
	}

	return true, nil
}

func fetchCharacterID(ctx context.Context, name string) (int, error) {
	id, found := ccpCache.Get(name)
	if found {
		return id.(int), nil
	}

	nameList := []string{name}
	js, err := json.Marshal(nameList)
	if err != nil {
		return 0, fmt.Errorf("error marshaling %s", name)
	}

	jsonPayload, err := ccpPost(ctx,
		"universe/ids/",
		map[string]string{"datasource": "tranquility"},
		bytes.NewBuffer(js))
	if err != nil {
		return 0, err
	}

	var entries characterList

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		fmt.Println("error = ", err)
		return 0, err
	}

	if len(entries.Characters) == 0 {
		return 0, fmt.Errorf("not found %s", name)
	}

	cid := 0
	cid = entries.Characters[0].ID

	ccpCache.SetWithExpire(name, cid, cache.NoExpiration)
	return cid, nil
}

func fetchCorporationName(ctx context.Context, id int) *characterResponse {
	ids := fmt.Sprint(id)

	name, found := ccpCache.Get(ids)
	if found {
		return &characterResponse{&characterData{CorpName: name.(string)}, nil}
	}

	cd := characterData{CorpName: ""}

	jsonPayload, err := ccpGet(ctx, "corporations/"+ids+"/", nil)
	if err != nil {
		return &characterResponse{&cd, err}
	}

	type corpEntry struct {
		CorporationName string `json:"name"`
	}

	var entry corpEntry

	if err := json.Unmarshal(jsonPayload, &entry); err != nil {
		return &characterResponse{&cd, err}
	}

	cd.CorpName = entry.CorporationName
	ccpCache.SetWithExpire(ids, cd.CorpName, cache.NoExpiration)

	return &characterResponse{&cd, nil}
}

func fetchAllianceName(ctx context.Context, id int) *characterResponse {
	if id == 0 {
		return &characterResponse{&characterData{AllianceName: ""}, nil}
	}

	ids := fmt.Sprint(id)

	name, found := ccpCache.Get(ids)
	if found {
		return &characterResponse{&characterData{AllianceName: name.(string)}, nil}
	}

	cd := characterData{AllianceName: ""}

	jsonPayload, err := ccpGet(ctx, "alliances/"+ids+"/", map[string]string{"alliance_ids": ids})
	if err != nil {
		return &characterResponse{&cd, err}
	}

	type allianceEntry struct {
		AllianceName string `json:"name"`
	}

	var entry allianceEntry

	if err := json.Unmarshal(jsonPayload, &entry); err != nil {
		return &characterResponse{&cd, err}
	}

	cd.AllianceName = entry.AllianceName
	ccpCache.SetWithExpire(ids, cd.AllianceName, cache.NoExpiration)

	return &characterResponse{&cd, nil}
}

func fetchCorpStartDate(ctx context.Context, id int) *characterResponse {
	cd := characterData{CorpAge: ""}

	ids := fmt.Sprint(id)

	jsonPayload, err := ccpGet(ctx, "characters/"+ids+"/corporationhistory", nil)
	if err != nil {
		return &characterResponse{&cd, err}
	}

	type corporationEntry struct {
		StartDate string `json:"start_date"`
	}

	var entries []corporationEntry

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &characterResponse{&cd, nil}
	}

	cd.CorpAge = secondsToTimeString(secondsSince(entries[0].StartDate))

	return &characterResponse{&cd, nil}
}

func fetchItemName(ctx context.Context, id int) *characterResponse {
	ids := fmt.Sprint(id)

	name, found := ccpCache.Get("ship:" + ids)
	if found {
		return &characterResponse{&characterData{FavoriteShipName: name.(string)}, nil}
	}

	cd := characterData{FavoriteShipName: ""}

	idList := []int{id}
	js, err := json.Marshal(idList)
	if err != nil {
		return &characterResponse{&cd, err}
	}

	jsonPayload, err := ccpPost(ctx,
		"universe/names/",
		map[string]string{"datasource": "tranquility"},
		bytes.NewBuffer(js))
	if err != nil {
		return &characterResponse{&cd, err}
	}

	type typeEntry struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Category string `json:"category"`
	}

	var entries []typeEntry

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &characterResponse{&cd, fmt.Errorf("invalid ship id %s", ids)}
	}

	cd.FavoriteShipName = entries[0].Name
	ccpCache.SetWithExpire("ship:"+ids, cd.FavoriteShipName, cache.NoExpiration)

	return &characterResponse{&cd, nil}
}

func fetchCorpDanger(ctx context.Context, id int) *characterResponse {
	ids := fmt.Sprint(id)

	danger, found := zkillCache.Get(ids)
	if found {
		return &characterResponse{&characterData{CorpDanger: danger.(int)}, nil}
	}

	cd := characterData{CorpDanger: 0}

	jsonPayload, err := zkillGet(ctx, "stats/corporationID/"+ids+"/")
	if err != nil {
		return &characterResponse{&cd, err}
	}

	var z zKillResponse

	if err := json.Unmarshal(jsonPayload, &z); err != nil {
		return &characterResponse{&cd, err}
	}

	cd.CorpDanger = z.Danger
	zkillCache.Set(ids, cd.CorpDanger)

	return &characterResponse{&cd, nil}
}
