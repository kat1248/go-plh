package main

import (
	"fmt"
	"sort"
	"sync"

	json "github.com/goccy/go-json"
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

const (
	computeFavoriteShip = false
)

func ccpGetKillMail(id int, hash string) *killMail {
	km := killMail{}

	ids := fmt.Sprint(id)
	jsonPayload, err := ccpGet("killmails/"+ids+"/"+hash+"/", nil)
	if err != nil {
		return &km
	}
	if err := json.Unmarshal(jsonPayload, &km); err != nil {
		return &km
	}

	return &km
}

func fetchLastKillActivity(id int) *characterResponse {
	cd := characterData{LastKill: ""}

	ids := fmt.Sprint(id)

	jsonPayload, err := zkillGet("characterID/" + ids + "/")
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

	km := ccpGetKillMail(entries[0].ID, entries[0].Info.Hash)

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

	cd.LastKill = what + " " + when

	return &characterResponse{&cd, nil}
}

func fetchKillHistory(id int) *characterResponse {
	cd := characterData{
		RecentExplorerTotal: 0, RecentKillTotal: 0, LastKillTime: "",
		FavoriteShipID: 0, FavoriteShipCount: 0}

	ids := fmt.Sprint(id)

	jsonPayload, err := zkillGet("kills/characterID/" + ids + "/")
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
	var wg sync.WaitGroup
	for i, k := range entries {
		wg.Go(func() {
			func(entry zKillMail, last bool) {
				km := ccpGetKillMail(entry.ID, entry.Info.Hash)
				if last {
					cd.LastKillTime = getDate(km.Time)
				}
				for _, attacker := range km.Attackers {
					if attacker.CharacterID == id {
						if explorerShips[km.Victim.ShipTypeID] {
							explorerTotal++
						}
						shipFreq[attacker.ShipTypeID]++
					}
				}
			}(k, i == len(entries)-1)
		})
	}
	wg.Wait()

	cd.RecentExplorerTotal = explorerTotal

	if computeFavoriteShip {
		// sort the ships by freq
		hack := map[int]int{}
		hackkeys := []int{}
		for key, val := range shipFreq {
			hack[val] = key
			hackkeys = append(hackkeys, val)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(hackkeys)))

		//for _, val := range hackkeys {
		//	fmt.Println("ship", hack[val], "times", val)
		//}
		//fmt.Println("favorite ship type", hack[hackkeys[0]], "used", hackkeys[0], "times")

		cd.FavoriteShipID = hack[hackkeys[0]]
		cd.FavoriteShipCount = hackkeys[0]
	}

	return &characterResponse{&cd, nil}
}

func fetchRecentKillHistory(id int) *characterResponse {
	cd := characterData{KillsLastWeek: 0}

	ids := fmt.Sprint(id)

	jsonPayload, err := zkillGet("kills/characterID/" + ids + "/pastSeconds/" + fmt.Sprint(secondsInWeek) + "/")
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
