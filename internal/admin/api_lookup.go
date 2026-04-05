package admin

import (
	"net/http"
	"strings"

	pubdata "github.com/avdoseferovic/geoserv/internal/pub"
	eopub "github.com/ethanmoffat/eolib-go/v3/protocol/pub"
)

type npcEntry struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type itemEntry struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Gender string `json:"gender,omitempty"`
}

var itemTypeLabels = map[eopub.ItemType]string{
	eopub.Item_General:   "General",
	eopub.Item_Currency:  "Currency",
	eopub.Item_Heal:      "Heal",
	eopub.Item_Teleport:  "Teleport",
	eopub.Item_ExpReward: "Exp Reward",
	eopub.Item_Key:       "Key",
	eopub.Item_Weapon:    "Weapon",
	eopub.Item_Shield:    "Shield",
	eopub.Item_Armor:     "Armor",
	eopub.Item_Hat:       "Hat",
	eopub.Item_Boots:     "Boots",
	eopub.Item_Gloves:    "Gloves",
	eopub.Item_Accessory: "Accessory",
	eopub.Item_Belt:      "Belt",
	eopub.Item_Necklace:  "Necklace",
	eopub.Item_Ring:      "Ring",
	eopub.Item_Armlet:    "Armlet",
	eopub.Item_Bracer:    "Bracer",
}

func itemTypeLabel(t eopub.ItemType) string {
	if l, ok := itemTypeLabels[t]; ok {
		return l
	}
	return "General"
}

func itemGender(rec *eopub.EifRecord) string {
	switch rec.Type {
	case eopub.Item_Armor, eopub.Item_Hat, eopub.Item_Boots, eopub.Item_Gloves,
		eopub.Item_Belt, eopub.Item_Accessory, eopub.Item_Bracer, eopub.Item_Armlet:
		switch rec.Spec2 {
		case 0:
			return "Male"
		case 1:
			return "Female"
		}
	}
	return ""
}

type vendorEntry struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	NpcType string `json:"npc_type,omitempty"`
}

var npcTypeLabels = map[eopub.NpcType]string{
	eopub.Npc_Friendly:   "Friendly",
	eopub.Npc_Passive:    "Passive",
	eopub.Npc_Aggressive: "Aggressive",
}

// handleGetVendorList returns NPCs that have a BehaviorId set (vendor NPCs).
func handleGetVendorList(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	seen := map[int]bool{}
	var out []vendorEntry
	n := pubdata.EnfLength()
	for i := 1; i <= n; i++ {
		rec := pubdata.GetNpc(i)
		if rec == nil || rec.BehaviorId == 0 || rec.Name == "" {
			continue
		}
		if seen[rec.BehaviorId] {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(rec.Name), q) {
			continue
		}
		seen[rec.BehaviorId] = true
		t := npcTypeLabels[rec.Type]
		out = append(out, vendorEntry{ID: rec.BehaviorId, Name: rec.Name, NpcType: t})
		if len(out) >= 50 {
			break
		}
	}
	if out == nil {
		out = []vendorEntry{}
	}
	writeJSON(w, out)
}

func handleGetNpcList(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	var out []npcEntry
	n := pubdata.EnfLength()
	for i := 1; i <= n; i++ {
		rec := pubdata.GetNpc(i)
		if rec == nil || rec.Name == "" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(rec.Name), q) {
			continue
		}
		out = append(out, npcEntry{ID: i, Name: rec.Name})
		if len(out) >= 50 {
			break
		}
	}
	if out == nil {
		out = []npcEntry{}
	}
	writeJSON(w, out)
}

func handleGetItemList(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	var out []itemEntry
	n := pubdata.EifLength()
	for i := 1; i <= n; i++ {
		rec := pubdata.GetItem(i)
		if rec == nil || rec.Name == "" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(rec.Name), q) {
			continue
		}
		out = append(out, itemEntry{
			ID:     i,
			Name:   rec.Name,
			Type:   itemTypeLabel(rec.Type),
			Gender: itemGender(rec),
		})
		if len(out) >= 50 {
			break
		}
	}
	if out == nil {
		out = []itemEntry{}
	}
	writeJSON(w, out)
}
