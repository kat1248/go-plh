package main

import (
	"encoding/json"
	"fmt"
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

func fetchLastKillActivity(id int) *characterResponse {
	cd := characterData{LastKill: ""}

	ids := fmt.Sprint(id)

	jsonPayload, err := zkillGet("characterID/" + ids + "/")
	if err != nil {
		return &characterResponse{&cd, err}
	}

	// fmt.Println("json", string(jsonPayload[:]))

	type zKillMailInfo struct {
		KillMailHash string `json:"hash"`
	}

	type zKillMail struct {
		KillMailID   int           `json:"killmail_id"`
		KillMailInfo zKillMailInfo `json:"zkb"`
	}

	entries := make([]zKillMail, 0)

	if err := json.Unmarshal(jsonPayload, &entries); err != nil {
		return &characterResponse{&cd, err}
	}

	if len(entries) == 0 {
		return &characterResponse{&cd, fmt.Errorf("no kills for id %s", ids)}
	}

	killmail_ids := fmt.Sprint(entries[0].KillMailID)
	killmail_hash := entries[0].KillMailInfo.KillMailHash

	// fetch killmail from ccp
	jsonPayload2, err := ccpGet("killmails/"+killmail_ids+"/"+killmail_hash+"/", nil)
	if err != nil {
		return &characterResponse{&cd, err}
	}

	// fmt.Println("json2", string(jsonPayload2[:]))

	var km killMail

	if err := json.Unmarshal(jsonPayload2, &km); err != nil {
		return &characterResponse{&cd, err}
	}

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
	cd := characterData{RecentExplorerTotal: 0, RecentKillTotal: 0, LastKillTime: ""}

	ids := fmt.Sprint(id)

	jsonPayload, err := zkillGet("kills/characterID/" + ids + "/")
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
		if explorerShips[k.Victim.ShipTypeID] {
			explorerTotal++
		}
		for _, attacker := range k.Attackers {
			if attacker.CharacterID == id {
				if _, ok := shipFreq[attacker.ShipTypeID]; ok {
					shipFreq[attacker.ShipTypeID]++
				} else {
					shipFreq[attacker.ShipTypeID] = 1
				}
			}
		}
	}

	// sort the ships by freq
	//hack := map[int]int{}
	//hackkeys := []int{}
	//for key, val := range shipFreq {
	//	hack[val] = key
	//	hackkeys = append(hackkeys, val)
	//}
	//sort.Sort(sort.Reverse(sort.IntSlice(hackkeys)))

	// for _, val := range hackkeys {
	// 	fmt.Println("ship", hack[val], "times", val)
	// }
	// fmt.Println("favorite ship type", hack[hackkeys[0]], "used", hackkeys[0], "times")

	cd.RecentExplorerTotal = explorerTotal
	cd.RecentKillTotal = len(entries)
	cd.LastKillTime = getDate(entries[len(entries)-1].Time)
	//cd.FavoriteShipID = hack[hackkeys[0]]
	//cd.FavoriteShipCount = hackkeys[0]

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
