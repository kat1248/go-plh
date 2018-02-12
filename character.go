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

// output data type, this is what is sent to the webpage to represent a character
type characterData struct {
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

type characterResponse struct {
	char *characterData
	err  error
}

type zKillCharInfo struct {
	CharacterId   int `json:"character_id"`
	CorporationId int `json:"corporation_id"`
	AllianceId    int `json:"alliance_id"`
	ShipTypeId    int `json:"ship_type_id"`
}

type killMail struct {
	Time      string          `json:"killMail_time"`
	Victim    zKillCharInfo   `json:"victim"`
	Attackers []zKillCharInfo `json:"attackers"`
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
)

func fetchcharacterData(name string) *characterResponse {
	cd := characterData{Name: name}

	id, err := fetchCharacterId(name)
	if err != nil {
		return &characterResponse{&cd, fmt.Errorf("'%s' not found", name)}
	}

	cd.CharacterId = id

	ch := make(chan *characterResponse, 3)
	var wg sync.WaitGroup

	fetcher := func(f func(int) *characterResponse, id int) {
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
		return &characterResponse{&cd, err}
	}

	ch = make(chan *characterResponse, 6)

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
		return &characterResponse{&cd, err}
	}

	if computeFavoriteShip && cd.FavoriteShipId != 0 {
		ch = make(chan *characterResponse, 1)

		fetcher(fetchItemName, cd.FavoriteShipId)

		wg.Wait()
		close(ch)

		if err := handleMerges(&cd, ch); err != nil {
			return &characterResponse{&cd, err}
		}
	}

	if n, ok := nicknames[name]; ok {
		cd.Name = n
	}

	return &characterResponse{&cd, nil}
}

func handleMerges(cd *characterData, ch chan *characterResponse) error {
	for r := range ch {
		if r.err != nil {
			return r.err
		}
		mergo.Merge(cd, r.char)
	}
	return nil
}

func fetchCCPRecord(id int) *characterResponse {
	cd := characterData{}

	ccpRec, err := fetchCharacterJson(id)
	if err != nil {
		return &characterResponse{&cd, err}
	}

	type ccpResponse struct {
		Name       string  `json:"name"`
		CorpId     int     `json:"corporation_id"`
		AllianceId int     `json:"alliance_id"`
		Security   float32 `json:"security_status"`
		Birthday   string  `json:"birthday"`
	}
	var cr ccpResponse

	if err = json.Unmarshal([]byte(ccpRec), &cr); err != nil {
		return &characterResponse{&cd, err}
	}

	cd.Age = secondsToTimeString(secondsSince(cr.Birthday))
	cd.CorpId = cr.CorpId
	cd.Security = cr.Security
	cd.IsNpcCorp = cd.CorpId < 2000000
	cd.AllianceId = cr.AllianceId

	return &characterResponse{&cd, nil}
}

func fetchZKillRecord(id int) *characterResponse {
	cd := characterData{}

	zkillRec, err := fetchZKillJson(id)
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

func fetchUrl(method, url string, params map[string]string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

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
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	return respBody, nil
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

	jsonPayload, err := ccpGet("characters/"+ids+"/", nil)
	if err != nil {
		return "", err
	}
	ccpCache.Set(ids, string(jsonPayload), cache.DefaultExpiration)
	return string(jsonPayload), nil
}

func fetchZKillJson(id int) (string, error) {
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

func fetchCharacterId(name string) (int, error) {
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

	type charIdResponse struct {
		Character []int `json:"character"`
	}
	var f charIdResponse

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
			rec, err := fetchCharacterJson(v)
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

	jsonPayload, err := ccpGet("corporations/names/", map[string]string{"corporation_ids": ids})
	if err != nil {
		return &characterResponse{&cd, err}
	}

	type corpEntry struct {
		CorporationName string `json:"corporation_name"`
		CorporationId   int    `json:"corporation_id"`
	}

	entries := make([]corpEntry, 0)

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &characterResponse{&cd, fmt.Errorf("invalid corporation id %s", ids)}
	}

	cd.CorpName = entries[0].CorporationName
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

	jsonPayload, err := ccpGet("alliances/names/", map[string]string{"alliance_ids": ids})
	if err != nil {
		return &characterResponse{&cd, err}
	}

	type allianceEntry struct {
		AllianceName string `json:"alliance_name"`
		AllianceId   int    `json:"alliance_id"`
	}

	entries := make([]allianceEntry, 0)

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &characterResponse{&cd, fmt.Errorf("invalid alliance id %s", ids)}
	}

	cd.AllianceName = entries[0].AllianceName
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
		Id       int    `json:"id"`
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

func fetchLastKillActivity(id int) *characterResponse {
	cd := characterData{LastKill: ""}

	ids := fmt.Sprint(id)

	jsonPayload, err := zkillGet("api/characterID/" + ids + "/limit/1/")
	if err != nil {
		return &characterResponse{&cd, err}
	}

	entries := make([]killMail, 0)

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &characterResponse{&cd, fmt.Errorf("no kills for id %s", ids)}
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

	return &characterResponse{&cd, nil}
}

func fetchKillHistory(id int) *characterResponse {
	cd := characterData{RecentExplorerTotal: 0, RecentKillTotal: 0, LastKillTime: ""}

	ids := fmt.Sprint(id)

	jsonPayload, err := zkillGet("api/kills/characterID/" + ids + "/")
	if err != nil {
		return &characterResponse{&cd, err}
	}

	entries := make([]killMail, 0)

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &characterResponse{&cd, fmt.Errorf("no kills for id %s", ids)}
	}

	explorerShips := map[int]bool{
		29248: true, 11188: true, 11192: true,
		605: true, 11172: true, 607: true,
		11182: true, 586: true, 33468: true, 33470: true}

	explorerTotal := 0
	shipFreq := make(map[int]int)
	for _, k := range entries {
		if explorerShips[k.Victim.ShipTypeId] {
			explorerTotal++
		}
		for _, attacker := range k.Attackers {
			if attacker.CharacterId == id {
				if _, ok := shipFreq[attacker.ShipTypeId]; ok {
					shipFreq[attacker.ShipTypeId] += 1
				} else {
					shipFreq[attacker.ShipTypeId] = 1
				}
			}
		}
	}

	// sort the ships by freq
	hack := map[int]int{}
	hackkeys := []int{}
	for key, val := range shipFreq {
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

	return &characterResponse{&cd, nil}
}

func fetchRecentKillHistory(id int) *characterResponse {
	cd := characterData{KillsLastWeek: 0}

	ids := fmt.Sprint(id)

	jsonPayload, err := zkillGet("api/kills/characterID/" + ids + "/pastSeconds/" + fmt.Sprint(SecondsInWeek) + "/")
	if err != nil {
		return &characterResponse{&cd, err}
	}

	entries := make([]killMail, 0)

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}
	cd.KillsLastWeek = len(entries)

	return &characterResponse{&cd, nil}
}
