package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/imdario/mergo"
	"github.com/patrickmn/go-cache"
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
}

type CharacterResponse struct {
	Char *CharacterData
	Err  error
}

type VictimStruct struct {
	CharacterId int `json:"character_id"`
	ShipTypeId  int `json:"ship_type_id"`
}

type KillMail struct {
	Victim       VictimStruct `json:"victim"`
	KillMailTime string       `json:"killmail_time"`
}

type KillMailList struct {
	KillMails []KillMail
}

const (
	ccpEsiURL   = "https://esi.tech.ccp.is/latest/"
	zkillApiURL = "https://zkillboard.com/api/"
	userAgent   = "kat1248@gmail.com - SC Little Helper - sclh.selfip.net"
)

var (
	ccpCache   = cache.New(60*time.Minute, 10*time.Minute)
	zkillCache = cache.New(60*time.Minute, 10*time.Minute)
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

	return &CharacterResponse{&cd, nil}
}

func handleMerges(cd *CharacterData, ch chan *CharacterResponse) error {
	for r := range ch {
		if r.Err != nil {
			return r.Err
		}
		mergo.Merge(cd, r.Char)
	}
	return nil
}

func fetchCCPRecord(id int) *CharacterResponse {
	cd := CharacterData{}

	ccpRec, err := fetchCharacterJson(id)
	if err != nil {
		return &CharacterResponse{&cd, fmt.Errorf("error fetching %d", id)}
	}

	type CCPResponse struct {
		Name       string  `json:"name"`
		CorpId     int     `json:"corporation_id"`
		AllianceId int     `json:"alliance_id"`
		Security   float32 `json:"security_status"`
		Birthday   string  `json:"birthday"`
	}
	var cr CCPResponse

	err = json.Unmarshal([]byte(ccpRec), &cr)
	if err != nil {
		return &CharacterResponse{&cd, fmt.Errorf("error fetching %d", id)}
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
		return &CharacterResponse{&cd, fmt.Errorf("error fetching zkill %d", id)}
	}

	type ZKillResponse struct {
		Danger int `json:"dangerRatio"`
		Gang   int `json:"gangRatio"`
		Kills  int `json:"shipsDestroyed"`
		Losses int `json:"shipsLost"`
	}
	var zr ZKillResponse

	err = json.Unmarshal([]byte(zkillRec), &zr)
	if err != nil {
		return &CharacterResponse{&cd, fmt.Errorf("error unmarshaling zkill record for %d", id)}
	}

	cd.Danger = zr.Danger
	cd.Gang = zr.Gang
	cd.Kills = zr.Kills
	cd.Losses = zr.Losses
	cd.HasKillboard = (cd.Kills != 0) || (cd.Losses != 0)

	return &CharacterResponse{&cd, nil}
}

func fetchUrl(url string, params map[string]string) ([]byte, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", userAgent)

	if len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, _ := client.Do(req)

	defer resp.Body.Close()
	resp_body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	return resp_body, nil
}

func ccpFetch(url string, params map[string]string) ([]byte, error) {
	return fetchUrl(ccpEsiURL+url, params)
}

func zkillFetch(url string) ([]byte, error) {
	return fetchUrl(zkillApiURL+url, nil)
}

func fetchCharacterJson(id int) (string, error) {
	ids := fmt.Sprint(id)
	rec, found := ccpCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	json_payload, _ := ccpFetch("characters/"+ids+"/", nil)
	ccpCache.Set(ids, string(json_payload), cache.DefaultExpiration)
	return string(json_payload), nil
}

func fetchZKillJson(id int) (string, error) {
	ids := fmt.Sprint(id)
	rec, found := zkillCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	json_payload, _ := zkillFetch("stats/characterID/" + ids + "/")
	zkillCache.Set(ids, string(json_payload), cache.DefaultExpiration)
	return string(json_payload), nil
}

func fetchCharacterId(name string) (int, error) {
	id, found := ccpCache.Get(name)
	if found {
		return id.(int), nil
	}

	json_payload, _ := ccpFetch("search/", map[string]string{"categories": "character", "search": name, "strict": "true"})

	type Response struct {
		Character []int `json:"character"`
	}
	var f Response

	err := json.Unmarshal(json_payload, &f)
	if err != nil {
		return 0, err
	}

	cid := 0
	if len(f.Character) == 0 {
		return cid, fmt.Errorf("invalid character name %s", name)
	} else if len(f.Character) == 1 {
		cid = f.Character[0]
	} else {
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
	cd := CharacterData{CorpName: ""}

	ids := fmt.Sprint(id)
	name, found := ccpCache.Get(ids)
	if found {
		cd.CorpName = name.(string)
	} else {
		json_payload, _ := ccpFetch("corporations/names/", map[string]string{"corporation_ids": ids})

		type CorpEntry struct {
			CorporationName string `json:"corporation_name"`
			CorporationId   int    `json:"corporation_id"`
		}
		type Response struct {
			Entries []CorpEntry
		}

		entries := make([]CorpEntry, 0)
		err := json.Unmarshal(json_payload, &entries)
		if err != nil {
			return &CharacterResponse{&cd, err}
		}

		if len(entries) == 0 {
			return &CharacterResponse{&cd, fmt.Errorf("invalid corporation id %s", ids)}
		}

		cd.CorpName = entries[0].CorporationName
		ccpCache.Set(ids, cd.CorpName, cache.NoExpiration)
	}

	return &CharacterResponse{&cd, nil}
}

func fetchAllianceName(id int) *CharacterResponse {
	cd := CharacterData{AllianceName: ""}

	if id == 0 {
		return &CharacterResponse{&cd, nil}
	}

	ids := fmt.Sprint(id)
	name, found := ccpCache.Get(ids)
	if found {
		cd.AllianceName = name.(string)
	} else {
		json_payload, _ := ccpFetch("alliances/names/", map[string]string{"alliance_ids": ids})

		type AllianceEntry struct {
			AllianceName string `json:"alliance_name"`
			AllianceId   int    `json:"alliance_id"`
		}
		type Response struct {
			Entries []AllianceEntry
		}

		entries := make([]AllianceEntry, 0)
		err := json.Unmarshal(json_payload, &entries)
		if err != nil {
			return &CharacterResponse{&cd, err}
		}

		if len(entries) == 0 {
			return &CharacterResponse{&cd, fmt.Errorf("invalid alliance id %s", ids)}
		}

		cd.AllianceName = entries[0].AllianceName
		ccpCache.Set(ids, cd.AllianceName, cache.NoExpiration)
	}

	return &CharacterResponse{&cd, nil}
}

func fetchCorpStartDate(id int) *CharacterResponse {
	cd := CharacterData{CorpAge: 0}

	ids := fmt.Sprint(id)

	json_payload, _ := ccpFetch("characters/"+ids+"/corporationhistory", nil)

	type CorporationEntry struct {
		StartDate string `json:"start_date"`
	}
	type Response struct {
		Entries []CorporationEntry
	}

	entries := make([]CorporationEntry, 0)
	err := json.Unmarshal(json_payload, &entries)
	if err != nil {
		return &CharacterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &CharacterResponse{&cd, fmt.Errorf("invalid character id %s", ids)}
	}

	cd.CorpAge = daysSince(entries[0].StartDate)

	return &CharacterResponse{&cd, nil}
}

func fetchCorpDanger(id int) *CharacterResponse {
	cd := CharacterData{CorpDanger: 0}

	ids := fmt.Sprint(id)
	danger, found := zkillCache.Get(ids)
	if found {
		cd.CorpDanger = danger.(int)
	} else {
		json_payload, _ := zkillFetch("stats/corporationID/" + ids + "/")

		type ZKillResponse struct {
			Danger int `json:"dangerRatio"`
		}
		var z ZKillResponse

		err := json.Unmarshal(json_payload, &z)
		if err != nil {
			return &CharacterResponse{&cd, err}
		}

		cd.CorpDanger = z.Danger
		zkillCache.Set(ids, cd.CorpDanger, cache.DefaultExpiration)
	}

	return &CharacterResponse{&cd, nil}
}

func fetchLastKillActivity(id int) *CharacterResponse {
	cd := CharacterData{LastKill: ""}

	ids := fmt.Sprint(id)
	json_payload, _ := zkillFetch("api/characterID/" + ids + "/limit/1/")

	entries := make([]KillMail, 0)
	err := json.Unmarshal(json_payload, &entries)
	if err != nil {
		return &CharacterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &CharacterResponse{&cd, err}
	}

	when := getDate(entries[0].KillMailTime)
	who := entries[0].Victim.CharacterId
	what := "kill"
	if who == id {
		what = "died"
	} else if who == 0 {
		what = "struct"
	}
	cd.LastKill = what + " " + when

	return &CharacterResponse{&cd, nil}
}

func fetchKillHistory(id int) *CharacterResponse {
	cd := CharacterData{RecentExplorerTotal: 0, RecentKillTotal: 0, LastKillTime: ""}

	ids := fmt.Sprint(id)
	json_payload, _ := zkillFetch("api/kills/characterID/" + ids + "/")

	entries := make([]KillMail, 0)
	err := json.Unmarshal(json_payload, &entries)
	if err != nil {
		return &CharacterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &CharacterResponse{&cd, err}
	}

	explorerShips := map[int]bool{
		29248: true, 11188: true, 11192: true, 605: true, 11172: true,
		607: true, 11182: true, 586: true, 33468: true, 33470: true}

	explorerTotal := 0
	for _, k := range entries {
		if explorerShips[k.Victim.ShipTypeId] {
			explorerTotal++
		}
	}

	cd.RecentExplorerTotal = explorerTotal
	cd.RecentKillTotal = len(entries)
	cd.LastKillTime = getDate(entries[len(entries)-1].KillMailTime)

	return &CharacterResponse{&cd, nil}
}

func fetchRecentKillHistory(id int) *CharacterResponse {
	cd := CharacterData{KillsLastWeek: 0}

	ids := fmt.Sprint(id)
	json_payload, _ := zkillFetch("api/kills/characterID/" + ids + "/pastSeconds/604800/")

	entries := make([]KillMail, 0)
	err := json.Unmarshal(json_payload, &entries)
	if err != nil {
		return &CharacterResponse{&cd, err}
	}
	cd.KillsLastWeek = len(entries)

	return &CharacterResponse{&cd, nil}
}

const (
	SecondsInMinute = 60
	SecondsInHour   = 60 * SecondsInMinute
	SecondsInDay    = 24 * SecondsInHour
	SecondsInMonth  = 30 * SecondsInDay
	SecondsInYear   = 365 * SecondsInDay
)

func secondsToTimeString(seconds float64) string {
	s := int(seconds)
	years := s / SecondsInYear
	s -= years * SecondsInYear
	months := s / SecondsInMonth
	s -= months * SecondsInMonth
	days := s / SecondsInDay
	ts := ""
	if years > 0 {
		ts += fmt.Sprint(years) + "y"
	}
	if months > 0 {
		ts += fmt.Sprint(months) + "m"
	}
	if days > 0 {
		ts += fmt.Sprint(days) + "d"
	}
	if ts == "" {
		ts = "today"
	}
	return ts
}

func secondsSince(dt string) float64 {
	t, _ := time.Parse("2006-01-02T15:04:05Z", dt)
	duration := time.Since(t)

	return duration.Seconds()
}

func daysSince(dt string) int {
	return int(secondsSince(dt)) / 86400
}

func getDate(dt string) string {
	return strings.Split(dt, "T")[0]
}
