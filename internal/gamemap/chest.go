package gamemap

import "time"

// Chest holds items at a specific map tile.
type Chest struct {
	Items []ChestItem
}

// ChestItem is a single item stack in a chest.
type ChestItem struct {
	Slot   int
	ItemID int
	Amount int
}

type ChestSpawnDef struct {
	Slot      int
	ItemID    int
	Amount    int
	SpawnTime int
	LastTaken time.Time
}

// GetChestItems returns the items in a chest at the given coordinates.
func (m *GameMap) GetChestItems(x, y int) []ChestItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	chest := m.chests[[2]int{x, y}]
	if chest == nil {
		return nil
	}
	result := make([]ChestItem, len(chest.Items))
	copy(result, chest.Items)
	return result
}

// AddChestItem adds an item to a chest. Returns the updated item list.
func (m *GameMap) AddChestItem(x, y, itemID, amount int) []ChestItem {
	m.mu.Lock()
	defer m.mu.Unlock()
	chest := m.chests[[2]int{x, y}]
	if chest == nil {
		return nil
	}
	for i := range chest.Items {
		if chest.Items[i].ItemID == itemID {
			// Enforce max chest item limit
			if maxChest := m.cfg.Limits.MaxChest; maxChest > 0 && chest.Items[i].Amount+amount > maxChest {
				return nil
			}
			chest.Items[i].Amount += amount
			result := make([]ChestItem, len(chest.Items))
			copy(result, chest.Items)
			return result
		}
	}
	if len(chest.Items) >= m.cfg.Chest.Slots {
		return nil // chest full
	}
	chest.Items = append(chest.Items, ChestItem{ItemID: itemID, Amount: amount})
	result := make([]ChestItem, len(chest.Items))
	copy(result, chest.Items)
	return result
}

// TakeChestItem removes an item from a chest. Returns (amount taken, updated items).
func (m *GameMap) TakeChestItem(x, y, itemID int) (int, []ChestItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	chest := m.chests[[2]int{x, y}]
	if chest == nil {
		return 0, nil
	}
	for i := range chest.Items {
		if chest.Items[i].ItemID == itemID {
			amount := chest.Items[i].Amount
			slot := chest.Items[i].Slot
			chest.Items = append(chest.Items[:i], chest.Items[i+1:]...)
			spawns := m.chestSpawnDefs[[2]int{x, y}]
			for j := range spawns {
				if spawns[j].Slot == slot {
					spawns[j].LastTaken = time.Now()
				}
			}
			result := make([]ChestItem, len(chest.Items))
			copy(result, chest.Items)
			return amount, result
		}
	}
	return 0, nil
}

func (m *GameMap) tickChestRespawn() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for coords, defs := range m.chestSpawnDefs {
		chest := m.chests[coords]
		if chest == nil {
			continue
		}

		for i := range defs {
			def := &defs[i]
			if def.Slot <= 0 || def.ItemID <= 0 || def.Amount <= 0 {
				continue
			}
			if chestHasSlotItem(chest, def.Slot) {
				continue
			}
			if !def.LastTaken.IsZero() && def.SpawnTime > 0 && now.Sub(def.LastTaken) < time.Duration(def.SpawnTime)*time.Minute {
				continue
			}
			if len(chest.Items) >= m.cfg.Chest.Slots {
				continue
			}

			chest.Items = append(chest.Items, ChestItem{
				Slot:   def.Slot,
				ItemID: def.ItemID,
				Amount: def.Amount,
			})
		}
	}
}

func chestHasSlotItem(chest *Chest, slot int) bool {
	if chest == nil {
		return false
	}
	for _, item := range chest.Items {
		if item.Slot == slot {
			return true
		}
	}
	return false
}
