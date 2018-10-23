package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/imdario/mergo"
	cache "github.com/patrickmn/go-cache"
)

// output data type, this is what is sent to the webpage to represent a character
type characterData struct {
	Name                string  `json:"name"`
	CharacterID         int     `json:"character_id"`
	Security            float32 `json:"security"`
	Age                 string  `json:"age"`
	Danger              int     `json:"danger"`
	Gang                int     `json:"gang"`
	Kills               int     `json:"kills"`
	Losses              int     `json:"losses"`
	HasKillboard        bool    `json:"has_killboard"`
	LastKill            string  `json:"last_kill"`
	CorpName            string  `json:"corp_name"`
	CorpID              int     `json:"corp_id"`
	CorpAge             int     `json:"corp_age"`
	IsNpcCorp           bool    `json:"is_npc_corp"`
	CorpDanger          int     `json:"corp_danger"`
	AllianceID          int     `json:"alliance_id"`
	AllianceName        string  `json:"alliance_name"`
	RecentExplorerTotal int     `json:"recent_explorer_total"`
	RecentKillTotal     int     `json:"recent_kill_total"`
	LastKillTime        string  `json:"last_kill_time"`
	KillsLastWeek       int     `json:"kills_last_week"`
	FavoriteShipID      int     `json:"favorite_ship_id"`
	FavoriteShipCount   int     `json:"favorite_ship_count"`
	FavoriteShipName    string  `json:"favorite_ship_name"`
}

type characterResponse struct {
	char *characterData
	err  error
}

const (
	ccpEsiURL   = "https://esi.evetech.net/latest/"
	zkillAPIURL = "https://zkillboard.com/api/"
	userAgent   = "kat1248@gmail.com - SC Little Helper - sclh.selfip.net"
)

var (
	ccpCache   = cache.New(60*time.Minute, 10*time.Minute)
	zkillCache = cache.New(60*time.Minute, 10*time.Minute)
	nicknames  = map[string]string{
		"Mynxee":        "Space Mom",
		"Portia Tigana": "Tiggs"}
)

func (c characterData) String() string {
	return c.Name
}

func fetchcharacterData(name string) *characterResponse {
	cd := characterData{Name: name}

	id, err := fetchCharacterID(name)
	if err != nil {
		return &characterResponse{&cd, fmt.Errorf("'%s' not found", name)}
	}

	cd.CharacterID = id

	ch := make(chan *characterResponse, 3)
	var wg sync.WaitGroup

	fetcher := func(f func(int) *characterResponse, id int) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch <- f(id)
		}()
	}

	fetcher(fetchCCPRecord, cd.CharacterID)
	fetcher(fetchZKillRecord, cd.CharacterID)
	fetcher(fetchCorpStartDate, cd.CharacterID)

	wg.Wait()
	close(ch)

	if err := cd.handleMerges(ch); err != nil {
		return &characterResponse{&cd, err}
	}

	ch = make(chan *characterResponse, 6)

	fetcher(fetchCorpDanger, cd.CorpID)
	fetcher(fetchAllianceName, cd.AllianceID)
	fetcher(fetchCorporationName, cd.CorpID)

	if cd.HasKillboard {
		fetcher(fetchLastKillActivity, cd.CharacterID)
	}

	if cd.Kills != 0 {
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

func fetchCCPRecord(id int) *characterResponse {
	cd := characterData{}

	ccpRec, err := fetchCharacterJSON(id)
	if err != nil {
		return &characterResponse{&cd, err}
	}

	type ccpResponse struct {
		Name       string  `json:"name"`
		CorpID     int     `json:"corporation_id"`
		AllianceID int     `json:"alliance_id"`
		Security   float32 `json:"security_status"`
		Birthday   string  `json:"birthday"`
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

func fetchZKillRecord(id int) *characterResponse {
	cd := characterData{}

	zkillRec, err := fetchZKillJSON(id)
	if err != nil {
		return &characterResponse{&cd, err}
	}

	type zKillResponse struct {
		Danger int `json:"dangerRatio"`
		Gang   int `json:"gangRatio"`
		Kills  int `json:"shipsDestroyed"`
		Losses int `json:"shipsLost"`
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

	return &characterResponse{&cd, nil}
}

func fetchCharacterJSON(id int) (string, error) {
	ids := fmt.Sprint(id)

	rec, found := ccpCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	jsonPayload, err := ccpGet("characters/"+ids+"/", nil)
	if err != nil {
		return "", err
	}
	ccpCache.Set(ids, string(jsonPayload), cache.DefaultExpiration)
	return string(jsonPayload), nil
}

func fetchZKillJSON(id int) (string, error) {
	ids := fmt.Sprint(id)

	rec, found := zkillCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	jsonPayload, err := zkillGet("stats/characterID/" + ids + "/")
	if err != nil {
		return "", err
	}
	zkillCache.Set(ids, string(jsonPayload), cache.DefaultExpiration)
	return string(jsonPayload), nil
}

func fetchCharacterID(name string) (int, error) {
	id, found := ccpCache.Get(name)
	if found {
		return id.(int), nil
	}

	jsonPayload, err := ccpGet(
		"search/",
		map[string]string{
			"categories": "character",
			"search":     name,
			"strict":     "true"})
	if err != nil {
		return 0, err
	}

	type charIDResponse struct {
		Character []int `json:"character"`
	}
	var f charIDResponse

	if err := json.Unmarshal(jsonPayload, &f); err != nil {
		return 0, err
	}

	cid := 0
	switch len(f.Character) {
	case 0:
		return cid, fmt.Errorf("invalid character name %s", name)
	case 1:
		cid = f.Character[0]
	default:
		cid = fetchMultipleIds(name, f.Character)
	}

	ccpCache.Set(name, cid, cache.NoExpiration)
	return cid, nil
}

func fetchMultipleIds(name string, ids []int) int {
	cid := 0
	type fetchData struct {
		json string
		id   int
	}

	ch := make(chan *fetchData, len(ids))
	var wg sync.WaitGroup

	for _, v := range ids {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			rec, err := fetchCharacterJSON(v)
			if err != nil {
				return
			}
			ch <- &fetchData{json: rec, id: v}
		}(v)
	}

	wg.Wait()
	close(ch)

	type ccpResponse struct {
		Name string `json:"name"`
	}
	var cr ccpResponse
	for r := range ch {
		err := json.Unmarshal([]byte(r.json), &cr)
		if err != nil {
			continue
		}
		if cr.Name == name {
			cid = r.id
			break
		}
	}

	return cid
}

func fetchCorporationName(id int) *characterResponse {
	ids := fmt.Sprint(id)

	name, found := ccpCache.Get(ids)
	if found {
		return &characterResponse{&characterData{CorpName: name.(string)}, nil}
	}

	cd := characterData{CorpName: ""}

	jsonPayload, err := ccpGet("corporations/"+ids+"/", nil)
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
	ccpCache.Set(ids, cd.CorpName, cache.NoExpiration)

	return &characterResponse{&cd, nil}
}

func fetchAllianceName(id int) *characterResponse {
	if id == 0 {
		return &characterResponse{&characterData{AllianceName: ""}, nil}
	}

	ids := fmt.Sprint(id)

	name, found := ccpCache.Get(ids)
	if found {
		return &characterResponse{&characterData{AllianceName: name.(string)}, nil}
	}

	cd := characterData{AllianceName: ""}

	jsonPayload, err := ccpGet("alliances/"+ids+"/", map[string]string{"alliance_ids": ids})
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
	ccpCache.Set(ids, cd.AllianceName, cache.NoExpiration)

	return &characterResponse{&cd, nil}
}

func fetchCorpStartDate(id int) *characterResponse {
	cd := characterData{CorpAge: 0}

	ids := fmt.Sprint(id)

	jsonPayload, err := ccpGet("characters/"+ids+"/corporationhistory", nil)
	if err != nil {
		return &characterResponse{&cd, err}
	}

	type corporationEntry struct {
		StartDate string `json:"start_date"`
	}

	entries := make([]corporationEntry, 0)

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &characterResponse{&cd, nil}
	}

	cd.CorpAge = daysSince(entries[0].StartDate)

	return &characterResponse{&cd, nil}
}

func fetchItemName(id int) *characterResponse {
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

	jsonPayload, err := ccpPost(
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

	entries := make([]typeEntry, 0)

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &characterResponse{&cd, fmt.Errorf("invalid ship id %s", ids)}
	}

	cd.FavoriteShipName = entries[0].Name
	fmt.Println("favorite ship =", cd.FavoriteShipName)
	ccpCache.Set("ship:"+ids, cd.FavoriteShipName, cache.NoExpiration)

	return &characterResponse{&cd, nil}
}

func fetchCorpDanger(id int) *characterResponse {
	ids := fmt.Sprint(id)

	danger, found := zkillCache.Get(ids)
	if found {
		return &characterResponse{&characterData{CorpDanger: danger.(int)}, nil}
	}

	cd := characterData{CorpDanger: 0}

	jsonPayload, err := zkillGet("stats/corporationID/" + ids + "/")
	if err != nil {
		return &characterResponse{&cd, err}
	}

	type zKillResponse struct {
		Danger int `json:"dangerRatio"`
	}
	var z zKillResponse

	if err := json.Unmarshal(jsonPayload, &z); err != nil {
		return &characterResponse{&cd, err}
	}

	cd.CorpDanger = z.Danger
	zkillCache.Set(ids, cd.CorpDanger, cache.DefaultExpiration)

	return &characterResponse{&cd, nil}
}
