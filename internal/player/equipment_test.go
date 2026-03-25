package player

import (
	"testing"

	eopub "github.com/ethanmoffat/eolib-go/v3/protocol/pub"
)

func TestEquip_SingleSlot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		itemType eopub.ItemType
		getField func(*Equipment) int
	}{
		{"Weapon", eopub.Item_Weapon, func(e *Equipment) int { return e.Weapon }},
		{"Shield", eopub.Item_Shield, func(e *Equipment) int { return e.Shield }},
		{"Armor", eopub.Item_Armor, func(e *Equipment) int { return e.Armor }},
		{"Hat", eopub.Item_Hat, func(e *Equipment) int { return e.Hat }},
		{"Boots", eopub.Item_Boots, func(e *Equipment) int { return e.Boots }},
		{"Gloves", eopub.Item_Gloves, func(e *Equipment) int { return e.Gloves }},
		{"Accessory", eopub.Item_Accessory, func(e *Equipment) int { return e.Accessory }},
		{"Belt", eopub.Item_Belt, func(e *Equipment) int { return e.Belt }},
		{"Necklace", eopub.Item_Necklace, func(e *Equipment) int { return e.Necklace }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := &Equipment{}

			// Equip item 100 into empty slot
			old := e.Equip(tt.itemType, 100, 0)
			if old != 0 {
				t.Errorf("equipping into empty slot returned old=%d, want 0", old)
			}
			if tt.getField(e) != 100 {
				t.Errorf("slot value = %d, want 100", tt.getField(e))
			}

			// Replace with item 200
			old = e.Equip(tt.itemType, 200, 0)
			if old != 100 {
				t.Errorf("replacing returned old=%d, want 100", old)
			}
			if tt.getField(e) != 200 {
				t.Errorf("slot value = %d, want 200", tt.getField(e))
			}
		})
	}
}

func TestEquip_DualSlot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		itemType eopub.ItemType
		getSlot  func(*Equipment, int) int
	}{
		{"Ring", eopub.Item_Ring, func(e *Equipment, i int) int { return e.Ring[i] }},
		{"Armlet", eopub.Item_Armlet, func(e *Equipment, i int) int { return e.Armlet[i] }},
		{"Bracer", eopub.Item_Bracer, func(e *Equipment, i int) int { return e.Bracer[i] }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := &Equipment{}

			// Slot 0
			old := e.Equip(tt.itemType, 10, 0)
			if old != 0 {
				t.Errorf("slot 0 equip old = %d, want 0", old)
			}
			if tt.getSlot(e, 0) != 10 {
				t.Errorf("slot 0 = %d, want 10", tt.getSlot(e, 0))
			}

			// Slot 1
			old = e.Equip(tt.itemType, 20, 1)
			if old != 0 {
				t.Errorf("slot 1 equip old = %d, want 0", old)
			}
			if tt.getSlot(e, 1) != 20 {
				t.Errorf("slot 1 = %d, want 20", tt.getSlot(e, 1))
			}

			// Both slots independent
			if tt.getSlot(e, 0) != 10 {
				t.Errorf("slot 0 changed to %d after equipping slot 1", tt.getSlot(e, 0))
			}
		})
	}
}

func TestUnequip(t *testing.T) {
	t.Parallel()
	e := &Equipment{Weapon: 50, Hat: 30, Ring: [2]int{10, 20}}

	removed := e.Unequip(eopub.Item_Weapon, 0)
	if removed != 50 || e.Weapon != 0 {
		t.Errorf("Unequip Weapon: removed=%d, slot=%d", removed, e.Weapon)
	}

	removed = e.Unequip(eopub.Item_Ring, 1)
	if removed != 20 || e.Ring[1] != 0 {
		t.Errorf("Unequip Ring[1]: removed=%d, slot=%d", removed, e.Ring[1])
	}
	if e.Ring[0] != 10 {
		t.Errorf("Ring[0] changed to %d", e.Ring[0])
	}
}

func TestFindItemType(t *testing.T) {
	t.Parallel()
	e := &Equipment{
		Weapon: 100,
		Armor:  200,
		Ring:   [2]int{0, 300},
	}

	tests := []struct {
		name     string
		itemID   int
		wantType eopub.ItemType
		wantSub  int
	}{
		{"weapon", 100, eopub.Item_Weapon, 0},
		{"armor", 200, eopub.Item_Armor, 0},
		{"ring slot 1", 300, eopub.Item_Ring, 1},
		{"not equipped", 999, eopub.Item_General, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotType, gotSub := e.FindItemType(tt.itemID)
			if gotType != tt.wantType || gotSub != tt.wantSub {
				t.Errorf("FindItemType(%d) = (%v, %d), want (%v, %d)",
					tt.itemID, gotType, gotSub, tt.wantType, tt.wantSub)
			}
		})
	}
}

func TestForEachID(t *testing.T) {
	t.Parallel()
	e := &Equipment{
		Boots:  1,
		Armor:  2,
		Weapon: 3,
		Ring:   [2]int{4, 0}, // slot 1 empty
		Armlet: [2]int{0, 5},
	}

	var collected []int
	e.ForEachID(func(id int) {
		collected = append(collected, id)
	})

	// Should have exactly 5 non-zero items
	if len(collected) != 5 {
		t.Errorf("ForEachID called fn %d times, want 5 (collected: %v)", len(collected), collected)
	}

	// Verify all expected IDs are present
	want := map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true}
	for _, id := range collected {
		if !want[id] {
			t.Errorf("unexpected ID %d in ForEachID results", id)
		}
	}
}

func TestIsEquipable(t *testing.T) {
	t.Parallel()
	equipable := []eopub.ItemType{
		eopub.Item_Weapon, eopub.Item_Shield, eopub.Item_Armor, eopub.Item_Hat,
		eopub.Item_Boots, eopub.Item_Gloves, eopub.Item_Accessory, eopub.Item_Belt,
		eopub.Item_Necklace, eopub.Item_Ring, eopub.Item_Armlet, eopub.Item_Bracer,
	}
	for _, typ := range equipable {
		if !IsEquipable(typ) {
			t.Errorf("IsEquipable(%v) = false, want true", typ)
		}
	}

	notEquipable := []eopub.ItemType{
		eopub.Item_General, eopub.Item_Heal, eopub.Item_Teleport,
		eopub.Item_HairDye, eopub.Item_ExpReward,
	}
	for _, typ := range notEquipable {
		if IsEquipable(typ) {
			t.Errorf("IsEquipable(%v) = true, want false", typ)
		}
	}
}
