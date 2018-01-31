package character

import (
	"encoding/json"
	"fmt"
	"github.com/patrickmn/go-cache"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type CharacterData struct {
	Name                string
	CharacterId         int
	Security            float32
	Age                 string
	Danger              int
	Gang                int
	Kills               int
	Losses              int
	HasKillboard        bool
	LastKill            string
	CorpName            string
	CorpId              int
	CorpAge             int
	IsNpcCorp           bool
	CorpDanger          int
	AllianceName        string
	RecentExplorerTotal int
	RecentKillTotal     int
	LastKillTime        string
	KillsLastWeek       int
}

var ccpCache = cache.New(60*time.Minute, 10*time.Minute)
var zkillCache = cache.New(60*time.Minute, 10*time.Minute)

const ccpEsiURL = "https://esi.tech.ccp.is/latest/"
const zkillApiURL = "https://zkillboard.com/api/"
const userAgent = "kat1248@gmail.com - SC Little Helper - sclh.selfip.net"

func FetchCharacterData(name string) (CharacterData, error) {
	cd := CharacterData{Name: name}

	id, err := fetchCharacterId(name)
	if err != nil {
		return cd, fmt.Errorf("%s not found", name)
	}

	cd.CharacterId = id

	ccpRec, err := fetchCharacterRecord(id)
	if err != nil {
		return cd, fmt.Errorf("error fetching %d", id)
	}

	type CCPResponse struct {
		Name       string  `json:"name"`
		CorpId     int     `json:"corporation_id"`
		AllianceId int     `json:"alliance_id"`
		Security   float32 `json:"security_status"`
		Birthday   string  `json:""birthday"`
	}
	var cr CCPResponse

	err = json.Unmarshal([]byte(ccpRec), &cr)
	if err != nil {
		return cd, fmt.Errorf("error unmarshaling record for %s", name)
	}

	cd.Age = secondsToTimeString(secondsSince(cr.Birthday))
	cd.CorpId = cr.CorpId
	cd.CorpName, _ = fetchCorporationName(cd.CorpId)
	cd.Security = cr.Security
	cd.IsNpcCorp = cd.CorpId < 2000000
	cd.AllianceName, _ = fetchAllianceName(cr.AllianceId)
	startDate, _ := fetchCorpStartDate(cd.CharacterId)
	cd.CorpAge = daysSince(startDate)

	zkillRec, err := fetchZKillRecord(id)
	if err != nil {
		return cd, fmt.Errorf("error fetching zkill %d", id)
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
		return cd, fmt.Errorf("error unmarshaling zkill record for %s", name)
	}

	cd.Danger = zr.Danger
	cd.Gang = zr.Gang
	cd.Kills = zr.Kills
	cd.Losses = zr.Losses
	cd.HasKillboard = (cd.Kills != 0) || (cd.Losses != 0)
	cd.CorpDanger, _ = fetchCorpDanger(cd.CorpId)
	cd.LastKill = lastKillActivity(cd.CharacterId, cd.HasKillboard)
	/*
			Todo:
		        'recent_explorer_total': exp_total,
		        'recent_kill_total': kill_total,
		        'last_kill_time': last_kill_time,
		        'kills_last_week': kills_last_week
	*/

	return cd, nil
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

func fetchCharacterRecord(id int) (string, error) {
	ids := fmt.Sprint(id)
	rec, found := ccpCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	json_payload, _ := ccpFetch("characters/"+ids+"/", nil)
	ccpCache.Set(ids, string(json_payload), cache.DefaultExpiration)
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
		fmt.Println("error:", err)
		return 0, err
	}

	if len(f.Character) == 0 {
		return 0, fmt.Errorf("invalid character name %s", name)
	}
	// TODO: handle multiple hits
	cid := f.Character[0]
	ccpCache.Set(name, cid, cache.NoExpiration)
	return cid, nil
}

func fetchCorporationName(id int) (string, error) {
	ids := fmt.Sprint(id)
	name, found := ccpCache.Get(ids)
	if found {
		return name.(string), nil
	}

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
		fmt.Println("error:", err)
		return "", err
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("invalid corporation id %s", ids)
	}

	corporationName := entries[0].CorporationName
	ccpCache.Set(ids, corporationName, cache.NoExpiration)
	return corporationName, nil
}

func fetchAllianceName(id int) (string, error) {
	if id == 0 {
		return "", nil
	}
	ids := fmt.Sprint(id)
	name, found := ccpCache.Get(ids)
	if found {
		return name.(string), nil
	}

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
		fmt.Println("error:", err)
		return "", err
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("invalid alliance id %s", ids)
	}

	allianceName := entries[0].AllianceName
	ccpCache.Set(ids, allianceName, cache.NoExpiration)
	return allianceName, nil
}

func fetchCorpStartDate(id int) (string, error) {
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
		fmt.Println("error:", err)
		return "", err
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("invalid character id %s", ids)
	}

	date := entries[0].StartDate
	return date, nil
}

func zkillFetch(url string) ([]byte, error) {
	return fetchUrl(zkillApiURL+url, nil)
}

func fetchZKillRecord(id int) (string, error) {
	ids := fmt.Sprint(id)
	rec, found := zkillCache.Get(ids)
	if found {
		return rec.(string), nil
	}

	json_payload, _ := zkillFetch("stats/characterID/" + ids + "/")
	zkillCache.Set(ids, string(json_payload), cache.DefaultExpiration)
	return string(json_payload), nil
}

func fetchCorpDanger(id int) (int, error) {
	ids := fmt.Sprint(id)
	rec, found := zkillCache.Get(ids)
	if found {
		return rec.(int), nil
	}

	json_payload, _ := zkillFetch("stats/corporationID/" + ids + "/")
	type ZKillResponse struct {
		Danger int `json:"dangerRatio"`
	}

	var z ZKillResponse
	err := json.Unmarshal(json_payload, &z)
	if err != nil {
		fmt.Println("error:", err)
		return 0, err
	}
	zkillCache.Set(ids, z.Danger, cache.DefaultExpiration)
	return z.Danger, nil
}

func lastKillActivity(id int, has_killboard bool) string {
	if has_killboard {
		who, when := fetchLastKill(id)
		if who == id {
			return "died " + when
		} else if who == 0 {
			return "struct " + when
		} else {
			return "kill " + when
		}
	} else {
		return ""
	}
}

func fetchLastKill(id int) (int, string) {
	ids := fmt.Sprint(id)
	json_payload, _ := zkillFetch("api/characterID/" + ids + "/limit/1/")

	type VictimStruct struct {
		CharacterId int `json:"character_id"`
	}
	type KillMail struct {
		Victim       VictimStruct `json:"victim"`
		KillMailTime string       `json:"killmail_time"`
	}
	type KillMailList struct {
		KillMails []KillMail
	}

	entries := make([]KillMail, 0)
	err := json.Unmarshal(json_payload, &entries)
	if err != nil {
		fmt.Println("error:", err)
		return 0, ""
	}

	if len(entries) == 0 {
		return 0, ""
	}

	when := strings.Split(entries[0].KillMailTime, "T")[0]
	who := entries[0].Victim.CharacterId
	return who, when
}

func secondsToTimeString(seconds float64) string {
	s := int(seconds)
	years := s / 31104000
	s -= years * 31104000
	months := s / 2592000
	s -= months * 2592000
	days := s / 86400
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
