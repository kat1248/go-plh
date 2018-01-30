package character

import (
	"encoding/json"
	"fmt"
	"github.com/patrickmn/go-cache"
	"io/ioutil"
	"net/http"
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
var ccpEsiURL = "https://esi.tech.ccp.is/latest/"

func fetchCharacterRecord(id int) (string, error) {
	rec, found := ccpCache.Get(fmt.Sprint(id))
	if found {
		fmt.Println("cache hit for", id)
		return rec.(string), nil
	}
	client := &http.Client{}
	req, _ := http.NewRequest("GET", ccpEsiURL+"characters/"+fmt.Sprint(id)+"/", nil)
	req.Header.Add("Accept", "application/json")
	resp, _ := client.Do(req)

	defer resp.Body.Close()
	resp_body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("http error %d", resp.StatusCode)
	}

	json_payload := string(resp_body)

	ccpCache.Set(fmt.Sprint(id), json_payload, cache.DefaultExpiration)
	return json_payload, nil
}

func fetchId(name string) (int, error) {
	id, found := ccpCache.Get(name)
	if found {
		fmt.Println("cache hit for", name)
		return id.(int), nil
	}

	client := &http.Client{}
	req, _ := http.NewRequest("GET", ccpEsiURL+"search/", nil)
	req.Header.Add("Accept", "application/json")

	q := req.URL.Query()
	q.Add("categories", "character")
	q.Add("search", name)
	q.Add("strict", "true")
	req.URL.RawQuery = q.Encode()
	resp, _ := client.Do(req)

	defer resp.Body.Close()
	resp_body, _ := ioutil.ReadAll(resp.Body)

	type Response struct {
		Character []int `json:"character"`
	}

	var f Response
	err := json.Unmarshal(resp_body, &f)
	if err != nil {
		fmt.Println("error:", err)
		return 0, err
	}

	fmt.Println(resp.Status)
	fmt.Println(string(resp_body))
	if len(f.Character) == 0 {
		return 0, fmt.Errorf("invalid character name %s", name)
	}

	id = f.Character[0]
	ccpCache.Set(name, id, cache.NoExpiration)
	return id.(int), nil
}

func FetchCharacterData(name string) (CharacterData, error) {
	cd := CharacterData{Name: name}

	id, err := fetchId(name)
	if err != nil {
		return cd, fmt.Errorf("%s not found", name)
	}

        cd.CharacterId = id

	rec, err := fetchCharacterRecord(id)
        if err != nil {
		return cd, fmt.Errorf("error fetching %d", id)
        }

        type Response struct {
		Name       string  `json:"name"`
		CorpId     int     `json:"corporation_id"`
		AllianceId int     `json:"alliance_id"`
		Security   float32 `json:"security_status"`
		Birthday   string  `json:""birthday"`
	}
	var f Response

        err = json.Unmarshal([]byte(rec), &f)
	if err != nil {
		return cd, fmt.Errorf("error unmarshaling record for %s", name)
	}

        cd.CorpId = f.CorpId
	cd.Security = f.Security
        cd.IsNpcCorp = cd.CorpId < 2000000
        
	return cd, nil
}
