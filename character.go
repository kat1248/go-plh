package main

import (
	"bytes"
	"context"
	"net/http"

	"fmt"
	"sync"
	"time"

	"dario.cat/mergo"
	json "github.com/goccy/go-json"
	log "github.com/sirupsen/logrus"
	cache "zgo.at/zcache/v2"
)

const (
	npcCorpThreshold = 2000000
)

var (
	ccpEsiURL   = "https://esi.evetech.net/latest/"
	zkillAPIURL = "https://zkillboard.com/api/"
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

// fetchCharacterData orchestrates retrieval and aggregation of all available data for a character name.
//
// It resolves the character ID, concurrently fetches CCP record, zKillboard record (when available),
// corporation start date, and additional dependent data (corporation danger, alliance/corporation names,
// last kill activity, kill histories, and favorite ship name) as applicable, then merges results into a
// single characterData. If a nickname mapping exists for the provided name, it is applied to the final
// character name. The function returns a characterResponse containing the aggregated characterData or an
// error if the character cannot be found or any fetch step fails.
func fetchCharacterData(ctx context.Context, client *http.Client, name string) *characterResponse {
	cd := characterData{Name: name}

	id, err := fetchCharacterID(ctx, client, name)
	if err != nil {
		return &characterResponse{&cd, fmt.Errorf("'%s' not found", name)}
	}

	cd.ZkillUsed = true

	cd.CharacterID = id

	cd.AnalyzeKills = analyzeKills

	ch := make(chan *characterResponse, 3)
	var wg sync.WaitGroup

	fetcher := func(f func(context.Context, *http.Client, int) *characterResponse, id int) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch <- f(ctx, client, id)
		}()
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

// fetchCCPRecord fetches the CCP (ESI) record for the given character ID and returns
// a characterResponse containing the populated fields Age, CorpID, Security,
// IsNpcCorp, and AllianceID.
// If the CCP data cannot be retrieved or parsed, the returned characterResponse
// contains the corresponding error.
func fetchCCPRecord(ctx context.Context, client *http.Client, id int) *characterResponse {
	cd := characterData{}

	ccpRec, err := fetchCharacterJSON(ctx, client, id)
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
	cd.IsNpcCorp = cd.CorpID < npcCorpThreshold
	cd.AllianceID = cr.AllianceID

	return &characterResponse{&cd, nil}
}

// fetchZKillRecord retrieves zKillboard statistics for the given character ID and returns them wrapped in a characterResponse.
// The returned characterData will include Danger, Gang, Kills, Losses, HasKillboard (true when kills or losses are non-zero) and mark ZkillUsed true.
// If fetching or unmarshaling the zKillboard JSON fails, the returned characterResponse will contain the corresponding error.
func fetchZKillRecord(ctx context.Context, client *http.Client, id int) *characterResponse {
	cd := characterData{ZkillUsed: false}

	zkillRec, err := fetchZKillJSON(ctx, client, id)
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

// fetchCharacterJSON returns the raw JSON payload for the character with the given ID,
// using an in-memory cache if a cached entry exists. If the record is not cached it
// fetches "characters/{id}/" from the CCP ESI endpoint, caches the response, and
// returns the JSON string or an error if the fetch fails.
func fetchCharacterJSON(ctx context.Context, client *http.Client, id int) (string, error) {
	ids := fmt.Sprint(id)

	rec, found := ccpCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	jsonPayload, err := ccpGet(ctx, client, "characters/"+ids+"/", nil)
	if err != nil {
		return "", err
	}

	ccpCache.Set(ids, string(jsonPayload))
	return string(jsonPayload), nil
}

// fetchZKillJSON retrieves the zKillboard stats JSON for the given character ID,
// using the in-memory zkillCache to return a cached response when available.
// It returns the JSON payload as a string, or a non-nil error if the underlying
// request fails.
func fetchZKillJSON(ctx context.Context, client *http.Client, id int) (string, error) {
	ids := fmt.Sprint(id)

	rec, found := zkillCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	jsonPayload, err := zkillGet(ctx, client, "stats/characterID/"+ids+"/")
	if err != nil {
		return "", err
	}

	zkillCache.Set(ids, string(jsonPayload))
	return string(jsonPayload), nil
}

// loadCharacterIds ensures EVE character IDs for the given names are present in the local cache.
// It skips empty names and any names already cached, POSTs the remaining names to the ESI
// "universe/ids/" endpoint (datasource=tranquility), and caches any returned character IDs.
// It returns true when one or more character entries were retrieved and cached, or false with
// an error if the lookup failed or no entries were found.
func loadCharacterIds(ctx context.Context, client *http.Client, names []string) (bool, error) {
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

	jsonPayload, err := ccpPost(ctx, client,
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

// fetchCharacterID looks up a character's EVE Online ID by character name.
// It first checks the local cache; if not present it POSTs the name to the ESI
// "universe/ids/" endpoint (datasource=tranquility), caches the discovered ID,
// and returns it. Returns 0 and a non-nil error if the name is not found or if
// there is a request/response marshaling or parsing error.
func fetchCharacterID(ctx context.Context, client *http.Client, name string) (int, error) {
	id, found := ccpCache.Get(name)
	if found {
		return id.(int), nil
	}

	nameList := []string{name}
	js, err := json.Marshal(nameList)
	if err != nil {
		return 0, fmt.Errorf("error marshaling %s", name)
	}

	jsonPayload, err := ccpPost(ctx, client,
		"universe/ids/",
		map[string]string{"datasource": "tranquility"},
		bytes.NewBuffer(js))
	if err != nil {
		return 0, err
	}

	var entries characterList

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		log.WithError(err).Error("error unmarshaling character IDs")
		return 0, err
	}

	if len(entries.Characters) == 0 {
		return 0, fmt.Errorf("not found %s", name)
	}

	cid := entries.Characters[0].ID

	ccpCache.SetWithExpire(name, cid, cache.NoExpiration)
	return cid, nil
}

// fetchCorporationName retrieves the corporation name for the given corporation ID.
// It returns a characterResponse whose characterData.CorpName is populated from the in-memory cache
// when available or fetched from the CCP API and then cached. If fetching or JSON unmarshalling fails,
// the returned characterResponse contains an error.
func fetchCorporationName(ctx context.Context, client *http.Client, id int) *characterResponse {
	ids := fmt.Sprint(id)

	name, found := ccpCache.Get(ids)
	if found {
		return &characterResponse{&characterData{CorpName: name.(string)}, nil}
	}

	cd := characterData{CorpName: ""}

	jsonPayload, err := ccpGet(ctx, client, "corporations/"+ids+"/", nil)
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

// fetchAllianceName retrieves the alliance name for the given alliance ID and returns it wrapped in a characterResponse.
// If id is 0, it returns an empty AllianceName. It consults an internal cache and caches successful lookups; on failure it returns a characterResponse containing the error.
func fetchAllianceName(ctx context.Context, client *http.Client, id int) *characterResponse {
	if id == 0 {
		return &characterResponse{&characterData{AllianceName: ""}, nil}
	}

	ids := fmt.Sprint(id)

	name, found := ccpCache.Get(ids)
	if found {
		return &characterResponse{&characterData{AllianceName: name.(string)}, nil}
	}

	cd := characterData{AllianceName: ""}

	jsonPayload, err := ccpGet(ctx, client, "alliances/"+ids+"/", map[string]string{"alliance_ids": ids})
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

// fetchCorpStartDate fetches a character's corporation history and computes the time since the character first joined a corporation.
// If a start date is found, CorpAge is set to a human-readable duration since that date; if no entries are present, CorpAge is left empty.
// It returns a characterResponse containing the populated characterData or an error encountered while fetching or parsing the data.
func fetchCorpStartDate(ctx context.Context, client *http.Client, id int) *characterResponse {
	cd := characterData{CorpAge: ""}

	ids := fmt.Sprint(id)

	jsonPayload, err := ccpGet(ctx, client, "characters/"+ids+"/corporationhistory", nil)
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

// fetchItemName resolves the human-readable name for a ship item ID and returns it in a characterResponse.
// On success the returned characterResponse contains characterData.FavoriteShipName set to the resolved name.
// If the ID cannot be resolved or a remote request/unmarshal fails, the response's error is non-nil.
// Successful lookups are cached under the key "ship:<id>".
func fetchItemName(ctx context.Context, client *http.Client, id int) *characterResponse {
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

	jsonPayload, err := ccpPost(ctx, client,
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

// fetchCorpDanger retrieves the danger score for a corporation from zKillboard and caches it.
// If a cached value exists it is returned immediately. Otherwise it requests zKillboard's
// stats/corporationID/{id}/ endpoint, parses the danger score into characterData.CorpDanger,
// caches the value, and returns it wrapped in a characterResponse. On error the returned
// characterResponse contains the error and a characterData with CorpDanger set to zero.
func fetchCorpDanger(ctx context.Context, client *http.Client, id int) *characterResponse {
	ids := fmt.Sprint(id)

	danger, found := zkillCache.Get(ids)
	if found {
		return &characterResponse{&characterData{CorpDanger: danger.(int)}, nil}
	}

	cd := characterData{CorpDanger: 0}

	jsonPayload, err := zkillGet(ctx, client, "stats/corporationID/"+ids+"/")
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
