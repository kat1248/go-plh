package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/imdario/mergo"
	cache "github.com/patrickmn/go-cache"
)

type CharacterData struct {
	Name                string  `json:"name"`
	CharacterId         int     `json:"character_id"`
	Security            float32 `json:"security"`
	Age                 string  `json:"age"`
	Danger              int     `json:"danger"`
	Gang                int     `json:"gang"`
	Kills               int     `json:"kills"`
	Losses              int     `json:"losses"`
	HasKillboard        bool    `json:"has_killboard"`
	LastKill            string  `json:"last_kill"`
	CorpName            string  `json:"corp_name"`
	CorpId              int     `json:"corp_id"`
	CorpAge             int     `json:"corp_age"`
	IsNpcCorp           bool    `json:"is_npc_corp"`
	CorpDanger          int     `json:"corp_danger"`
	AllianceId          int     `json:"alliance_id"`
	AllianceName        string  `json:"alliance_name"`
	RecentExplorerTotal int     `json:"recent_explorer_total"`
	RecentKillTotal     int     `json:"recent_kill_total"`
	LastKillTime        string  `json:"last_kill_time"`
	KillsLastWeek       int     `json:"kills_last_week"`
	FavoriteShipId      int     `json:"favorite_ship_id"`
	FavoriteShipCount   int     `json:"favorite_ship_count"`
	FavoriteShipName    string  `json:"favorite_ship_name"`
}

type CharacterResponse struct {
	char *CharacterData
	err  error
}

type ZKillCharInfo struct {
	CharacterId   int `json:"character_id"`
	CorporationId int `json:"corporation_id"`
	AllianceId    int `json:"alliance_id"`
	ShipTypeId    int `json:"ship_type_id"`
}

type KillMail struct {
	Time      string          `json:"killmail_time"`
	Victim    ZKillCharInfo   `json:"victim"`
	Attackers []ZKillCharInfo `json:"attackers"`
}

const (
	ccpEsiURL           = "https://esi.tech.ccp.is/latest/"
	zkillApiURL         = "https://zkillboard.com/api/"
	userAgent           = "kat1248@gmail.com - SC Little Helper - sclh.selfip.net"
	computeFavoriteShip = false
)

var (
	ccpCache   = cache.New(60*time.Minute, 10*time.Minute)
	zkillCache = cache.New(60*time.Minute, 10*time.Minute)
	nicknames  = map[string]string{
		"Mynxee":        "Space Mom",
		"Portia Tigana": "Tiggs"}
	localTransport = &http.Transport{DisableKeepAlives: true}
	localClient    = &http.Client{Transport: localTransport}
)

func fetchCharacterData(name string) *CharacterResponse {
	cd := CharacterData{Name: name}

	if len(name) <= 3 {
		return &CharacterResponse{&cd, fmt.Errorf("'%s' invalid", name)}
	}

	id, err := fetchCharacterId(name)
	if err != nil {
		return &CharacterResponse{&cd, fmt.Errorf("'%s' not found", name)}
	}

	cd.CharacterId = id

	ch := make(chan *CharacterResponse, 3)
	var wg sync.WaitGroup

	fetcher := func(f func(int) *CharacterResponse, id int) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch <- f(id)
		}()
	}

	fetcher(fetchCCPRecord, cd.CharacterId)
	fetcher(fetchZKillRecord, cd.CharacterId)
	fetcher(fetchCorpStartDate, cd.CharacterId)

	wg.Wait()
	close(ch)

	if err := handleMerges(&cd, ch); err != nil {
		return &CharacterResponse{&cd, err}
	}

	ch = make(chan *CharacterResponse, 6)

	fetcher(fetchCorpDanger, cd.CorpId)
	fetcher(fetchAllianceName, cd.AllianceId)
	fetcher(fetchCorporationName, cd.CorpId)

	if cd.HasKillboard {
		fetcher(fetchLastKillActivity, cd.CharacterId)
	}

	if cd.Kills != 0 {
		fetcher(fetchKillHistory, cd.CharacterId)
		fetcher(fetchRecentKillHistory, cd.CharacterId)
	}

	wg.Wait()
	close(ch)

	if err := handleMerges(&cd, ch); err != nil {
		return &CharacterResponse{&cd, err}
	}

	if computeFavoriteShip && cd.FavoriteShipId != 0 {
		ch = make(chan *CharacterResponse, 1)

		fetcher(fetchItemName, cd.FavoriteShipId)

		wg.Wait()
		close(ch)

		if err := handleMerges(&cd, ch); err != nil {
			return &CharacterResponse{&cd, err}
		}
	}

	if n, ok := nicknames[name]; ok {
		cd.Name = n
	}

	return &CharacterResponse{&cd, nil}
}

func handleMerges(cd *CharacterData, ch chan *CharacterResponse) error {
	for r := range ch {
		if r.err != nil {
			return r.err
		}
		mergo.Merge(cd, r.char)
	}
	return nil
}

func fetchCCPRecord(id int) *CharacterResponse {
	cd := CharacterData{}

	ccpRec, err := fetchCharacterJson(id)
	if err != nil {
		return &CharacterResponse{&cd, err}
	}

	type CCPResponse struct {
		Name       string  `json:"name"`
		CorpId     int     `json:"corporation_id"`
		AllianceId int     `json:"alliance_id"`
		Security   float32 `json:"security_status"`
		Birthday   string  `json:"birthday"`
	}
	var cr CCPResponse

	if err = json.Unmarshal([]byte(ccpRec), &cr); err != nil {
		return &CharacterResponse{&cd, err}
	}

	cd.Age = secondsToTimeString(secondsSince(cr.Birthday))
	cd.CorpId = cr.CorpId
	cd.Security = cr.Security
	cd.IsNpcCorp = cd.CorpId < 2000000
	cd.AllianceId = cr.AllianceId

	return &CharacterResponse{&cd, nil}
}

func fetchZKillRecord(id int) *CharacterResponse {
	cd := CharacterData{}

	zkillRec, err := fetchZKillJson(id)
	if err != nil {
		return &CharacterResponse{&cd, err}
	}

	type ZKillResponse struct {
		Danger int `json:"dangerRatio"`
		Gang   int `json:"gangRatio"`
		Kills  int `json:"shipsDestroyed"`
		Losses int `json:"shipsLost"`
	}
	var zr ZKillResponse

	if err = json.Unmarshal([]byte(zkillRec), &zr); err != nil {
		return &CharacterResponse{&cd, err}
	}

	cd.Danger = zr.Danger
	cd.Gang = zr.Gang
	cd.Kills = zr.Kills
	cd.Losses = zr.Losses
	cd.HasKillboard = (cd.Kills != 0) || (cd.Losses != 0)

	return &CharacterResponse{&cd, nil}
}

func fetchUrl(method, url string, params map[string]string, body io.Reader) ([]byte, error) {
	req, _ := http.NewRequest(method, url, body)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", userAgent)

	if len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := localClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	return resp_body, nil
}

func ccpGet(url string, params map[string]string) ([]byte, error) {
	return fetchUrl("GET", ccpEsiURL+url, params, nil)
}

func ccpPost(url string, params map[string]string, body io.Reader) ([]byte, error) {
	return fetchUrl("POST", ccpEsiURL+url, params, body)
}

func zkillGet(url string) ([]byte, error) {
	return fetchUrl("GET", zkillApiURL+url, nil, nil)
}

func fetchCharacterJson(id int) (string, error) {
	ids := fmt.Sprint(id)

	rec, found := ccpCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	json_payload, _ := ccpGet("characters/"+ids+"/", nil)
	ccpCache.Set(ids, string(json_payload), cache.DefaultExpiration)
	return string(json_payload), nil
}

func fetchZKillJson(id int) (string, error) {
	ids := fmt.Sprint(id)

	rec, found := zkillCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	json_payload, _ := zkillGet("stats/characterID/" + ids + "/")
	zkillCache.Set(ids, string(json_payload), cache.DefaultExpiration)
	return string(json_payload), nil
}

func fetchCharacterId(name string) (int, error) {
	id, found := ccpCache.Get(name)
	if found {
		return id.(int), nil
	}

	json_payload, _ := ccpGet(
		"search/",
		map[string]string{
			"categories": "character",
			"search":     name,
			"strict":     "true"})

	type Response struct {
		Character []int `json:"character"`
	}
	var f Response

	if err := json.Unmarshal(json_payload, &f); err != nil {
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

	type FetchData struct {
		json string
		id   int
	}

	ch := make(chan *FetchData, len(ids))
	var wg sync.WaitGroup

	for _, v := range ids {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			rec, _ := fetchCharacterJson(v)
			ch <- &FetchData{json: rec, id: v}
		}(v)
	}

	wg.Wait()
	close(ch)

	type CCPResponse struct {
		Name string `json:"name"`
	}
	var cr CCPResponse
	for r := range ch {
		_ = json.Unmarshal([]byte(r.json), &cr)
		if cr.Name == name {
			cid = r.id
			break
		}
	}

	return cid
}

func fetchCorporationName(id int) *CharacterResponse {
	ids := fmt.Sprint(id)

	name, found := ccpCache.Get(ids)
	if found {
		return &CharacterResponse{&CharacterData{CorpName: name.(string)}, nil}
	}

	cd := CharacterData{CorpName: ""}

	json_payload, _ := ccpGet("corporations/names/", map[string]string{"corporation_ids": ids})

	type CorpEntry struct {
		CorporationName string `json:"corporation_name"`
		CorporationId   int    `json:"corporation_id"`
	}

	entries := make([]CorpEntry, 0)

	if err := json.Unmarshal(json_payload, &entries); err != nil {
		return &CharacterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &CharacterResponse{&cd, fmt.Errorf("invalid corporation id %s", ids)}
	}

	cd.CorpName = entries[0].CorporationName
	ccpCache.Set(ids, cd.CorpName, cache.NoExpiration)

	return &CharacterResponse{&cd, nil}
}

func fetchAllianceName(id int) *CharacterResponse {
	if id == 0 {
		return &CharacterResponse{&CharacterData{AllianceName: ""}, nil}
	}

	ids := fmt.Sprint(id)

	name, found := ccpCache.Get(ids)
	if found {
		return &CharacterResponse{&CharacterData{AllianceName: name.(string)}, nil}
	}

	cd := CharacterData{AllianceName: ""}

	json_payload, _ := ccpGet("alliances/names/", map[string]string{"alliance_ids": ids})

	type AllianceEntry struct {
		AllianceName string `json:"alliance_name"`
		AllianceId   int    `json:"alliance_id"`
	}

	entries := make([]AllianceEntry, 0)

	if err := json.Unmarshal(json_payload, &entries); err != nil {
		return &CharacterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &CharacterResponse{&cd, fmt.Errorf("invalid alliance id %s", ids)}
	}

	cd.AllianceName = entries[0].AllianceName
	ccpCache.Set(ids, cd.AllianceName, cache.NoExpiration)

	return &CharacterResponse{&cd, nil}
}

func fetchCorpStartDate(id int) *CharacterResponse {
	cd := CharacterData{CorpAge: 0}

	ids := fmt.Sprint(id)

	json_payload, _ := ccpGet("characters/"+ids+"/corporationhistory", nil)

	type CorporationEntry struct {
		StartDate string `json:"start_date"`
	}

	entries := make([]CorporationEntry, 0)

	if err := json.Unmarshal(json_payload, &entries); err != nil {
		return &CharacterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &CharacterResponse{&cd, fmt.Errorf("invalid character id %s", ids)}
	}

	cd.CorpAge = daysSince(entries[0].StartDate)

	return &CharacterResponse{&cd, nil}
}

func fetchItemName(id int) *CharacterResponse {
	ids := fmt.Sprint(id)

	name, found := ccpCache.Get("ship:" + ids)
	if found {
		return &CharacterResponse{&CharacterData{FavoriteShipName: name.(string)}, nil}
	}

	cd := CharacterData{FavoriteShipName: ""}

	id_list := []int{id}
	js, _ := json.Marshal(id_list)

	json_payload, _ := ccpPost(
		"universe/names/",
		map[string]string{"datasource": "tranquility"},
		bytes.NewBuffer(js))

	type TypeEntry struct {
		Id       int    `json:"id"`
		Name     string `json:"name"`
		Category string `json:"category"`
	}

	entries := make([]TypeEntry, 0)

	if err := json.Unmarshal(json_payload, &entries); err != nil {
		return &CharacterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &CharacterResponse{&cd, fmt.Errorf("invalid ship id %s", ids)}
	}

	cd.FavoriteShipName = entries[0].Name

	ccpCache.Set("ship:"+ids, cd.FavoriteShipName, cache.NoExpiration)

	return &CharacterResponse{&cd, nil}
}

func fetchCorpDanger(id int) *CharacterResponse {
	ids := fmt.Sprint(id)

	danger, found := zkillCache.Get(ids)
	if found {
		return &CharacterResponse{&CharacterData{CorpDanger: danger.(int)}, nil}
	}

	cd := CharacterData{CorpDanger: 0}

	json_payload, _ := zkillGet("stats/corporationID/" + ids + "/")

	type ZKillResponse struct {
		Danger int `json:"dangerRatio"`
	}
	var z ZKillResponse

	if err := json.Unmarshal(json_payload, &z); err != nil {
		return &CharacterResponse{&cd, err}
	}

	cd.CorpDanger = z.Danger
	zkillCache.Set(ids, cd.CorpDanger, cache.DefaultExpiration)

	return &CharacterResponse{&cd, nil}
}

func fetchLastKillActivity(id int) *CharacterResponse {
	cd := CharacterData{LastKill: ""}

	ids := fmt.Sprint(id)

	json_payload, _ := zkillGet("api/characterID/" + ids + "/limit/1/")

	entries := make([]KillMail, 0)

	if err := json.Unmarshal(json_payload, &entries); err != nil {
		return &CharacterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &CharacterResponse{&cd, fmt.Errorf("no kills for id %s", ids)}
	}

	when := getDate(entries[0].Time)
	who := entries[0].Victim.CharacterId

	var what string
	switch {
	case who == id:
		what = "died"
	case who == 0:
		what = "struct"
	default:
		what = "kill"
	}

	cd.LastKill = what + " " + when

	return &CharacterResponse{&cd, nil}
}

func fetchKillHistory(id int) *CharacterResponse {
	cd := CharacterData{RecentExplorerTotal: 0, RecentKillTotal: 0, LastKillTime: ""}

	ids := fmt.Sprint(id)

	json_payload, _ := zkillGet("api/kills/characterID/" + ids + "/")

	entries := make([]KillMail, 0)

	if err := json.Unmarshal(json_payload, &entries); err != nil {
		return &CharacterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &CharacterResponse{&cd, fmt.Errorf("no kills for id %s", ids)}
	}

	explorerShips := map[int]bool{
		29248: true, 11188: true, 11192: true,
		605: true, 11172: true, 607: true,
		11182: true, 586: true, 33468: true, 33470: true}

	explorerTotal := 0
	ship_freq := make(map[int]int)
	for _, k := range entries {
		if explorerShips[k.Victim.ShipTypeId] {
			explorerTotal++
		}
		for _, attacker := range k.Attackers {
			if attacker.CharacterId == id {
				if _, ok := ship_freq[attacker.ShipTypeId]; ok {
					ship_freq[attacker.ShipTypeId] += 1
				} else {
					ship_freq[attacker.ShipTypeId] = 1
				}
			}
		}
	}

	// sort the ships by freq
	hack := map[int]int{}
	hackkeys := []int{}
	for key, val := range ship_freq {
		hack[val] = key
		hackkeys = append(hackkeys, val)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(hackkeys)))

	// for _, val := range hackkeys {
	// 	fmt.Println("ship", hack[val], "times", val)
	// }
	// fmt.Println("favorite ship type", hack[hackkeys[0]], "used", hackkeys[0], "times")

	cd.RecentExplorerTotal = explorerTotal
	cd.RecentKillTotal = len(entries)
	cd.LastKillTime = getDate(entries[len(entries)-1].Time)
	cd.FavoriteShipId = hack[hackkeys[0]]
	cd.FavoriteShipCount = hackkeys[0]

	return &CharacterResponse{&cd, nil}
}

func fetchRecentKillHistory(id int) *CharacterResponse {
	cd := CharacterData{KillsLastWeek: 0}

	ids := fmt.Sprint(id)

	json_payload, _ := zkillGet("api/kills/characterID/" + ids + "/pastSeconds/" + fmt.Sprint(SecondsInWeek) + "/")

	entries := make([]KillMail, 0)

	if err := json.Unmarshal(json_payload, &entries); err != nil {
		return &CharacterResponse{&cd, err}
	}
	cd.KillsLastWeek = len(entries)

	return &CharacterResponse{&cd, nil}
}
