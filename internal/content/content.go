package content

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/avdoseferovic/geoserv/internal/config"
	"github.com/avdoseferovic/geoserv/internal/pub"
	eopub "github.com/ethanmoffat/eolib-go/v3/protocol/pub"
)

type ShopOffer struct {
	ItemID int `json:"item_id"`
	Cost   int `json:"cost"`
}

type CraftOffer struct {
	ItemID       int            `json:"item_id"`
	Cost         int            `json:"cost"`
	Ingredients  []CraftItem    `json:"ingredients"`
	Requirements CraftCondition `json:"requirements"`
}

type CraftItem struct {
	ItemID int `json:"item_id"`
	Amount int `json:"amount"`
}

type CraftCondition struct {
	MinLevel int `json:"min_level"`
	ClassID  int `json:"class_id"`
}

type Shop struct {
	NPCID int          `json:"npc_id"`
	Name  string       `json:"name"`
	Buy   []ShopOffer  `json:"buy"`
	Sell  []ShopOffer  `json:"sell"`
	Craft []CraftOffer `json:"craft"`
}

type SkillSpell struct {
	SpellID    int `json:"spell_id"`
	Cost       int `json:"cost"`
	MinLevel   int `json:"min_level"`
	ClassID    int `json:"class_id"`
	RequiredID int `json:"required_item_id"`
	RequiredN  int `json:"required_item_amount"`
}

type SkillMaster struct {
	NPCID  int          `json:"npc_id"`
	Name   string       `json:"name"`
	Spells []SkillSpell `json:"spells"`
}

type Registry struct {
	Shops        map[int]Shop
	SkillMasters map[int]SkillMaster
}

var current = &Registry{
	Shops:        map[int]Shop{},
	SkillMasters: map[int]SkillMaster{},
}

func Load(cfg *config.Config) (*Registry, error) {
	reg := &Registry{
		Shops:        map[int]Shop{},
		SkillMasters: map[int]SkillMaster{},
	}

	if pub.NpcDB != nil && pub.ShopFileDB != nil {
		for i, npc := range pub.NpcDB.Npcs {
			if npc.Type != eopub.Npc_Shop || npc.BehaviorId == 0 {
				continue
			}
			vendorID := npc.BehaviorId
			for _, s := range pub.ShopFileDB.Shops {
				if s.BehaviorId != vendorID {
					continue
				}
				shop := Shop{
					NPCID: i + 1,
					Name:  s.Name,
					Buy:   []ShopOffer{},
					Sell:  []ShopOffer{},
					Craft: []CraftOffer{},
				}

				for _, t := range s.Trades {
					if t.BuyPrice > 0 {
						shop.Buy = append(shop.Buy, ShopOffer{ItemID: t.ItemId, Cost: t.BuyPrice})
					}
					if t.SellPrice > 0 {
						shop.Sell = append(shop.Sell, ShopOffer{ItemID: t.ItemId, Cost: t.SellPrice})
					}
				}

				for _, c := range s.Crafts {
					craft := CraftOffer{
						ItemID: c.ItemId,
					}
					for _, ing := range c.Ingredients {
						craft.Ingredients = append(craft.Ingredients, CraftItem{ItemID: ing.ItemId, Amount: ing.Amount})
					}
					shop.Craft = append(shop.Craft, craft)
				}

				reg.Shops[shop.NPCID] = shop
				break
			}
		}
		slog.Info("shops loaded from pub data", "shops", len(reg.Shops))
	} else {
		slog.Warn("pub data for shops missing", "NpcDB_loaded", pub.NpcDB != nil, "ShopFileDB_loaded", pub.ShopFileDB != nil)
	}

	if cfg == nil {
		current = reg
		return reg, nil
	}

	if cfg.Content.ShopFile != "" {
		shops, err := loadJSONFile[[]Shop](cfg.Content.ShopFile)
		if err != nil {
			return nil, fmt.Errorf("loading shop file: %w", err)
		}
		for _, shop := range shops {
			reg.Shops[shop.NPCID] = shop
		}
		slog.Info("shops loaded from json", "shops", len(shops))
	}

	if cfg.Content.SkillMasterFile != "" {
		skillMasters, err := loadJSONFile[[]SkillMaster](cfg.Content.SkillMasterFile)
		if err != nil {
			return nil, fmt.Errorf("loading skill master file: %w", err)
		}
		for _, master := range skillMasters {
			reg.SkillMasters[master.NPCID] = master
		}
	}

	current = reg
	return reg, nil
}

func Current() *Registry {
	return current
}

func GetShop(npcID int) (Shop, bool) {
	shop, ok := current.Shops[npcID]
	return shop, ok
}

func GetSkillMaster(npcID int) (SkillMaster, bool) {
	master, ok := current.SkillMasters[npcID]
	return master, ok
}

func loadJSONFile[T any](path string) (T, error) {
	var result T
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	return result, nil
}
