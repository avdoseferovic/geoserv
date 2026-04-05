package admin

import (
	"net/http"
	"strconv"
	"strings"

	pubdata "github.com/avdoseferovic/geoserv/internal/pub"
)

// Shared paginated response wrapper.

func npcName(id int) string {
	if rec := pubdata.GetNpc(id); rec != nil {
		return rec.Name
	}
	return ""
}

func vendorName(behaviorID int) string {
	n := pubdata.EnfLength()
	for i := 1; i <= n; i++ {
		rec := pubdata.GetNpc(i)
		if rec != nil && rec.BehaviorId == behaviorID {
			return rec.Name
		}
	}
	return ""
}

func pageParams(r *http.Request) (page, size int, query string) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	size, _ = strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 || size > 100 {
		size = 20
	}
	if page < 0 {
		page = 0
	}
	query = strings.ToLower(r.URL.Query().Get("q"))
	return
}

func paginate[T any](all []T, page, size int, match func(T) bool) ([]T, int) {
	var filtered []T
	if match != nil {
		for _, item := range all {
			if match(item) {
				filtered = append(filtered, item)
			}
		}
	} else {
		filtered = all
	}
	total := len(filtered)
	start := page * size
	if start >= total {
		return nil, total
	}
	end := start + size
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

// --- Drops (EDF) ---

type dropItem struct {
	ItemID    int    `json:"item_id"`
	ItemName  string `json:"item_name,omitempty"`
	MinAmount int    `json:"min_amount"`
	MaxAmount int    `json:"max_amount"`
	Rate      int    `json:"rate"`
}

func itemName(id int) string {
	if rec := pubdata.GetItem(id); rec != nil {
		return rec.Name
	}
	return ""
}

type dropNpc struct {
	NpcID   int        `json:"npc_id"`
	NpcName string     `json:"npc_name,omitempty"`
	Drops   []dropItem `json:"drops"`
}

func getDropsData(page, size int, q string) ([]dropNpc, int) {
	if pubdata.DropDB == nil {
		return []dropNpc{}, 0
	}
	all := make([]dropNpc, 0, len(pubdata.DropDB.Npcs))
	for _, n := range pubdata.DropDB.Npcs {
		var drops []dropItem
		for _, d := range n.Drops {
			if d.ItemId == 0 && d.Rate == 0 {
				continue
			}
			drops = append(drops, dropItem{ItemID: d.ItemId, ItemName: itemName(d.ItemId), MinAmount: d.MinAmount, MaxAmount: d.MaxAmount, Rate: d.Rate})
		}
		if len(drops) == 0 && n.NpcId == 0 {
			continue
		}
		all = append(all, dropNpc{NpcID: n.NpcId, NpcName: npcName(n.NpcId), Drops: drops})
	}
	return paginate(all, page, size, func(d dropNpc) bool {
		if q == "" {
			return true
		}
		return strings.Contains(strconv.Itoa(d.NpcID), q) || strings.Contains(strings.ToLower(d.NpcName), q)
	})
}



// --- Talk (ETF) ---

type talkNpc struct {
	NpcID    int      `json:"npc_id"`
	NpcName  string   `json:"npc_name,omitempty"`
	Rate     int      `json:"rate"`
	Messages []string `json:"messages"`
}

func getTalkData(page, size int, q string) ([]talkNpc, int) {
	if pubdata.TalkDB == nil {
		return []talkNpc{}, 0
	}
	all := make([]talkNpc, len(pubdata.TalkDB.Npcs))
	for i, n := range pubdata.TalkDB.Npcs {
		msgs := make([]string, len(n.Messages))
		for j, m := range n.Messages {
			msgs[j] = m.Message
		}
		all[i] = talkNpc{NpcID: n.NpcId, NpcName: npcName(n.NpcId), Rate: n.Rate, Messages: msgs}
	}
	return paginate(all, page, size, func(t talkNpc) bool {
		if q == "" {
			return true
		}
		return strings.Contains(strconv.Itoa(t.NpcID), q) || strings.Contains(strings.ToLower(t.NpcName), q)
	})
}



// --- Inns (EID) ---

type innQuestion struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type innRecord struct {
	BehaviorID      int           `json:"behavior_id"`
	VendorName      string        `json:"vendor_name,omitempty"`
	Name            string        `json:"name"`
	SpawnMap        int           `json:"spawn_map"`
	SpawnX          int           `json:"spawn_x"`
	SpawnY          int           `json:"spawn_y"`
	SleepMap        int           `json:"sleep_map"`
	SleepX          int           `json:"sleep_x"`
	SleepY          int           `json:"sleep_y"`
	AltSpawnEnabled bool          `json:"alt_spawn_enabled"`
	AltSpawnMap     int           `json:"alt_spawn_map"`
	AltSpawnX       int           `json:"alt_spawn_x"`
	AltSpawnY       int           `json:"alt_spawn_y"`
	Questions       []innQuestion `json:"questions"`
}

func getInnsData(page, size int, q string) ([]innRecord, int) {
	if pubdata.InnDB == nil {
		return []innRecord{}, 0
	}
	all := make([]innRecord, len(pubdata.InnDB.Inns))
	for i, inn := range pubdata.InnDB.Inns {
		qs := make([]innQuestion, len(inn.Questions))
		for j, q := range inn.Questions {
			qs[j] = innQuestion{Question: q.Question, Answer: q.Answer}
		}
		all[i] = innRecord{
			BehaviorID: inn.BehaviorId, VendorName: vendorName(inn.BehaviorId), Name: inn.Name,
			SpawnMap: inn.SpawnMap, SpawnX: inn.SpawnX, SpawnY: inn.SpawnY,
			SleepMap: inn.SleepMap, SleepX: inn.SleepX, SleepY: inn.SleepY,
			AltSpawnEnabled: inn.AlternateSpawnEnabled,
			AltSpawnMap:     inn.AlternateSpawnMap, AltSpawnX: inn.AlternateSpawnX, AltSpawnY: inn.AlternateSpawnY,
			Questions: qs,
		}
	}
	return paginate(all, page, size, func(inn innRecord) bool {
		if q == "" {
			return true
		}
		return strings.Contains(strconv.Itoa(inn.BehaviorID), q) || strings.Contains(strings.ToLower(inn.Name), q)
	})
}



// --- Shops (ESF) ---

type shopTrade struct {
	ItemID    int    `json:"item_id"`
	ItemName  string `json:"item_name,omitempty"`
	BuyPrice  int    `json:"buy_price"`
	SellPrice int    `json:"sell_price"`
	MaxAmount int    `json:"max_amount"`
}

type shopIngredient struct {
	ItemID   int    `json:"item_id"`
	ItemName string `json:"item_name,omitempty"`
	Amount   int    `json:"amount"`
}

type shopCraft struct {
	ItemID      int              `json:"item_id"`
	ItemName    string           `json:"item_name,omitempty"`
	Ingredients []shopIngredient `json:"ingredients"`
}

type shopRecord struct {
	BehaviorID int         `json:"behavior_id"`
	VendorName string      `json:"vendor_name,omitempty"`
	Name       string      `json:"name"`
	Trades     []shopTrade `json:"trades"`
	Crafts     []shopCraft `json:"crafts"`
}

func getShopsData(page, size int, q string) ([]shopRecord, int) {
	if pubdata.ShopFileDB == nil {
		return []shopRecord{}, 0
	}
	all := make([]shopRecord, 0, len(pubdata.ShopFileDB.Shops))
	for _, s := range pubdata.ShopFileDB.Shops {
		var trades []shopTrade
		for _, t := range s.Trades {
			if t.ItemId == 0 {
				continue
			}
			trades = append(trades, shopTrade{ItemID: t.ItemId, ItemName: itemName(t.ItemId), BuyPrice: t.BuyPrice, SellPrice: t.SellPrice, MaxAmount: t.MaxAmount})
		}
		var crafts []shopCraft
		for _, c := range s.Crafts {
			if c.ItemId == 0 {
				continue
			}
			var ings []shopIngredient
			for _, ing := range c.Ingredients {
				if ing.ItemId != 0 {
					ings = append(ings, shopIngredient{ItemID: ing.ItemId, ItemName: itemName(ing.ItemId), Amount: ing.Amount})
				}
			}
			crafts = append(crafts, shopCraft{ItemID: c.ItemId, ItemName: itemName(c.ItemId), Ingredients: ings})
		}
		if s.BehaviorId == 0 && len(trades) == 0 && len(crafts) == 0 {
			continue
		}
		all = append(all, shopRecord{BehaviorID: s.BehaviorId, VendorName: vendorName(s.BehaviorId), Name: s.Name, Trades: trades, Crafts: crafts})
	}
	return paginate(all, page, size, func(s shopRecord) bool {
		if q == "" {
			return true
		}
		return strings.Contains(strconv.Itoa(s.BehaviorID), q) || strings.Contains(strings.ToLower(s.Name), q)
	})
}



// --- Skill Masters (EMF) ---

type skillEntry struct {
	SkillID   int   `json:"skill_id"`
	LevelReq  int   `json:"level_req"`
	ClassReq  int   `json:"class_req"`
	Price     int   `json:"price"`
	SkillReqs []int `json:"skill_reqs"`
	StrReq    int   `json:"str_req"`
	IntReq    int   `json:"int_req"`
	WisReq    int   `json:"wis_req"`
	AgiReq    int   `json:"agi_req"`
	ConReq    int   `json:"con_req"`
	ChaReq    int   `json:"cha_req"`
}

type masterRecord struct {
	BehaviorID int          `json:"behavior_id"`
	VendorName string       `json:"vendor_name,omitempty"`
	Name       string       `json:"name"`
	Skills     []skillEntry `json:"skills"`
}

func getMastersData(page, size int, q string) ([]masterRecord, int) {
	if pubdata.SkillMasterDB == nil {
		return []masterRecord{}, 0
	}
	all := make([]masterRecord, 0, len(pubdata.SkillMasterDB.SkillMasters))
	for _, m := range pubdata.SkillMasterDB.SkillMasters {
		var skills []skillEntry
		for _, s := range m.Skills {
			if s.SkillId == 0 {
				continue
			}
			skills = append(skills, skillEntry{
				SkillID: s.SkillId, LevelReq: s.LevelRequirement, ClassReq: s.ClassRequirement,
				Price: s.Price, SkillReqs: s.SkillRequirements,
				StrReq: s.StrRequirement, IntReq: s.IntRequirement, WisReq: s.WisRequirement,
				AgiReq: s.AgiRequirement, ConReq: s.ConRequirement, ChaReq: s.ChaRequirement,
			})
		}
		if m.BehaviorId == 0 && len(skills) == 0 {
			continue
		}
		all = append(all, masterRecord{BehaviorID: m.BehaviorId, VendorName: vendorName(m.BehaviorId), Name: m.Name, Skills: skills})
	}
	return paginate(all, page, size, func(m masterRecord) bool {
		if q == "" {
			return true
		}
		return strings.Contains(strconv.Itoa(m.BehaviorID), q) || strings.Contains(strings.ToLower(m.Name), q)
	})
}


