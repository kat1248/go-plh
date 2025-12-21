package main

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
	CorpAge             string  `json:"corp_age"`
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
	ZkillUsed           bool    `json:"zkill_used"`
	AnalyzeKills        bool    `json:"analyze_kills"`
}

type characterResponse struct {
	char *characterData
	err  error
}

// responses from the various api
type ccpResponse struct {
	Name       string  `json:"name"`
	CorpID     int     `json:"corporation_id"`
	AllianceID int     `json:"alliance_id"`
	Security   float32 `json:"security_status"`
	Birthday   string  `json:"birthday"`
}

type idEntry struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type characterList struct {
	Characters []idEntry `json:"characters"`
}

type zKillResponse struct {
	Danger int `json:"dangerRatio"`
	Gang   int `json:"gangRatio"`
	Kills  int `json:"shipsDestroyed"`
	Losses int `json:"shipsLost"`
}
