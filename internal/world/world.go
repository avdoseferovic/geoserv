package world

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avdo/goeoserv/internal/config"
	"github.com/avdo/goeoserv/internal/db"
	"github.com/avdo/goeoserv/internal/gamemap"
	pubdata "github.com/avdo/goeoserv/internal/pub"
	"github.com/ethanmoffat/eolib-go/v3/data"
	eomap "github.com/ethanmoffat/eolib-go/v3/protocol/map"
	eonet "github.com/ethanmoffat/eolib-go/v3/protocol/net"
)

// World manages all game maps, online players, and the game tick loop.
type World struct {
	mu             sync.RWMutex
	maps           map[int]*gamemap.GameMap
	loggedAccounts map[int]bool // accountID -> logged in
	cfg            *config.Config
	db             *db.Database
}

func New(cfg *config.Config, database *db.Database) *World {
	return &World{
		maps:           make(map[int]*gamemap.GameMap),
		loggedAccounts: make(map[int]bool),
		cfg:            cfg,
		db:             database,
	}
}

// LoadMaps loads all EMF files from the data/maps directory.
func (w *World) LoadMaps() error {
	mapDir := "data/maps"
	entries, err := os.ReadDir(mapDir)
	if err != nil {
		return fmt.Errorf("reading map directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".emf") {
			continue
		}

		mapPath := filepath.Join(mapDir, entry.Name())
		mapData, err := os.ReadFile(mapPath)
		if err != nil {
			slog.Warn("failed to read map file", "path", mapPath, "err", err)
			continue
		}

		reader := data.NewEoReader(mapData)
		var emf eomap.Emf
		if err := emf.Deserialize(reader); err != nil {
			slog.Warn("failed to deserialize map", "path", mapPath, "err", err)
			continue
		}

		// Extract map ID from filename (e.g., "00001.emf" -> 1)
		baseName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		mapID, err := strconv.Atoi(baseName)
		if err != nil {
			slog.Warn("invalid map filename", "name", entry.Name())
			continue
		}

		gm := gamemap.New(mapID, &emf, w.cfg)
		gm.SpawnNPCs(w.cfg.NPCs.InstantSpawn)
		w.maps[mapID] = gm
		count++
	}

	slog.Info("maps loaded", "count", count)
	return nil
}

// InitNpcStats sets NPC HP/stats from ENF data for all loaded maps.
func (w *World) InitNpcStats() {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, m := range w.maps {
		m.InitNpcStats(func(npcID int) int {
			rec := pubdata.GetNpc(npcID)
			if rec != nil {
				return rec.Hp
			}
			return 1
		})
	}
}

// GetMap returns the map with the given ID, or nil if not found.
func (w *World) GetMap(mapID int) *gamemap.GameMap {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.maps[mapID]
}

// IsLoggedIn checks if an account is currently logged in.
func (w *World) IsLoggedIn(accountID int) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.loggedAccounts[accountID]
}

// AddLoggedInAccount marks an account as logged in.
func (w *World) AddLoggedInAccount(accountID int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.loggedAccounts[accountID] = true
}

// RemoveLoggedInAccount marks an account as logged out.
func (w *World) RemoveLoggedInAccount(accountID int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.loggedAccounts, accountID)
}

// EnterMap adds a player's character to a map and broadcasts their appearance.
func (w *World) EnterMap(mapID int, charInfo any) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		slog.Warn("map not found for enter", "map_id", mapID)
		return
	}
	if mc, ok := charInfo.(*gamemap.MapCharacter); ok {
		m.Enter(mc)
	}
}

// LeaveMap removes a player's character from a map.
func (w *World) LeaveMap(mapID, playerID int) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return
	}
	m.Leave(playerID)
}

// Walk handles a player walking on a map.
func (w *World) Walk(mapID, playerID int, direction int, coords [2]int) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return
	}
	m.Walk(playerID, direction, coords)
}

// Face handles a player changing direction on a map.
func (w *World) Face(mapID, playerID int, direction int) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return
	}
	m.Face(playerID, direction)
}

// Broadcast sends a packet to all players on a map except the sender.
func (w *World) Broadcast(mapID, excludePlayerID int, pkt eonet.Packet) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return
	}
	m.Broadcast(excludePlayerID, pkt)
}

// RunTickLoop starts the world tick loop. Call in a goroutine.
func (w *World) RunTickLoop(ctx context.Context) {
	tickRate := time.Duration(w.cfg.World.TickRate) * time.Millisecond
	ticker := time.NewTicker(tickRate)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick()
		}
	}
}

func (w *World) tick() {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, m := range w.maps {
		m.Tick()
	}
}

// BroadcastMap sends a packet to all players on a map except excludeID.
func (w *World) BroadcastMap(mapID, excludePlayerID int, pkt any) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return
	}
	if p, ok := pkt.(eonet.Packet); ok {
		m.Broadcast(excludePlayerID, p)
	}
}

// BroadcastGlobal sends a packet to all players on all maps except excludeID.
func (w *World) BroadcastGlobal(excludePlayerID int, pkt any) {
	p, ok := pkt.(eonet.Packet)
	if !ok {
		return
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, m := range w.maps {
		m.Broadcast(excludePlayerID, p)
	}
}

// SendToPlayer sends a packet to a specific player by searching all maps.
func (w *World) SendToPlayer(playerID int, pkt any) {
	p, ok := pkt.(eonet.Packet)
	if !ok {
		return
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, m := range w.maps {
		if bus := m.GetPlayerBus(playerID); bus != nil {
			_ = bus.SendPacket(p)
			return
		}
	}
}

// FindPlayerByName searches all maps for a player with the given name.
// Returns the playerID and true if found.
func (w *World) FindPlayerByName(name string) (int, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, m := range w.maps {
		if id, ok := m.FindPlayerByName(name); ok {
			return id, true
		}
	}
	return 0, false
}

// DamageNpc applies damage to an NPC on a map.
func (w *World) DamageNpc(mapID, npcIndex, playerID, damage int) (int, bool, int) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return 0, false, 0
	}
	return m.DamageNpc(npcIndex, playerID, damage)
}

// GetNpcAt returns the NPC index at the given coordinates on a map.
func (w *World) GetNpcAt(mapID, x, y int) int {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return -1
	}
	return m.IsNpcAt(x, y)
}

// DropItem drops an item on a map. Returns the ground item UID.
func (w *World) DropItem(mapID, itemID, amount, x, y, droppedBy int) int {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return 0
	}
	return m.DropItem(itemID, amount, x, y, droppedBy)
}

// PickupItem picks up a ground item. Returns (itemID, amount, ok).
func (w *World) PickupItem(mapID, uid int) (int, int, bool) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return 0, 0, false
	}
	item := m.PickupItem(uid)
	if item == nil {
		return 0, 0, false
	}
	return item.ItemID, item.Amount, true
}

// GetNearbyInfo returns the NearbyInfo for a given map.
func (w *World) GetNearbyInfo(mapID int) any {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return nil
	}
	info := m.GetNearbyInfo()
	return &info
}

// SendTo sends a packet to a specific player by looking across all maps.
func (w *World) SendTo(playerID int, pkt eonet.Packet) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, m := range w.maps {
		if bus := m.GetPlayerBus(playerID); bus != nil {
			_ = bus.SendPacket(pkt)
			return
		}
	}
}

// GetPlayerBus retrieves a player's PacketBus from whatever map they're on.
// Returns any to satisfy the WorldInterface; callers should type-assert to *protocol.PacketBus.
func (w *World) GetPlayerBus(playerID int) any {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, m := range w.maps {
		if bus := m.GetPlayerBus(playerID); bus != nil {
			return bus
		}
	}
	return nil
}

// GetPlayerPosition finds a player across all maps and returns their position.
func (w *World) GetPlayerPosition(playerID int) any {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, m := range w.maps {
		if pos := m.GetPlayerPosition(playerID); pos != nil {
			return pos
		}
	}
	return nil
}

// OnlinePlayerCount returns the total number of players across all maps.
func (w *World) OnlinePlayerCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	count := 0
	for _, m := range w.maps {
		count += m.PlayerCount()
	}
	return count
}

// GetOnlinePlayers returns info for all online players across all maps.
func (w *World) GetOnlinePlayers() any {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var result []gamemap.OnlinePlayerInfo
	for _, m := range w.maps {
		result = append(result, m.GetOnlinePlayers()...)
	}
	return result
}

// BroadcastToAdmins sends a packet to all players with admin level >= minAdmin.
func (w *World) BroadcastToAdmins(excludePlayerID int, minAdmin int, pkt any) {
	p, ok := pkt.(eonet.Packet)
	if !ok {
		return
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, m := range w.maps {
		m.BroadcastToAdmins(excludePlayerID, minAdmin, p)
	}
}

// WarpPlayer moves a player from one map to another. Returns NearbyInfo for the new map.
func (w *World) WarpPlayer(playerID, fromMapID, toMapID, toX, toY int) any {
	w.mu.RLock()
	fromMap := w.maps[fromMapID]
	toMap := w.maps[toMapID]
	w.mu.RUnlock()

	if fromMap == nil || toMap == nil {
		return nil
	}

	ch := fromMap.RemoveAndReturn(playerID)
	if ch == nil {
		return nil
	}

	ch.X = toX
	ch.Y = toY
	ch.MapID = toMapID
	toMap.Enter(ch)

	info := toMap.GetNearbyInfo()
	return &info
}

// BroadcastToGuild sends a packet to all online players in a guild (by tag).
func (w *World) BroadcastToGuild(excludePlayerID int, guildTag string, pkt any) {
	p, ok := pkt.(eonet.Packet)
	if !ok || guildTag == "" {
		return
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, m := range w.maps {
		m.BroadcastToGuild(excludePlayerID, guildTag, p)
	}
}

// BroadcastToParty sends a packet to all party members of the player's party.
func (w *World) BroadcastToParty(playerID int, pkt any) {
	p, ok := pkt.(eonet.Packet)
	if !ok {
		return
	}
	party := GetParty(playerID)
	if party != nil {
		party.BroadcastToParty(p)
	}
}

// GetChestItems returns items in a chest at given coords on a map.
func (w *World) GetChestItems(mapID, x, y int) any {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return nil
	}
	return m.GetChestItems(x, y)
}

// AddChestItem adds an item to a chest. Returns updated item list.
func (w *World) AddChestItem(mapID, x, y, itemID, amount int) any {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return nil
	}
	return m.AddChestItem(x, y, itemID, amount)
}

// TakeChestItem takes an item from a chest.
func (w *World) TakeChestItem(mapID, x, y, itemID int) (int, any) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return 0, nil
	}
	amt, items := m.TakeChestItem(x, y, itemID)
	return amt, items
}

// GetNpcEnfID returns the ENF record ID for an NPC at a given index on a map.
func (w *World) GetNpcEnfID(mapID, npcIndex int) int {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return 0
	}
	npc := m.GetNpc(npcIndex)
	if npc == nil {
		return 0
	}
	return npc.ID
}

// GetPlayerName returns a player's character name by searching all maps.
func (w *World) GetPlayerName(playerID int) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, m := range w.maps {
		if name := m.GetPlayerName(playerID); name != "" {
			return name
		}
	}
	return ""
}

// GetPendingWarp returns the pending warp destination for a player.
func (w *World) GetPendingWarp(mapID, playerID int) (int, int, int, bool) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return 0, 0, 0, false
	}
	warp := m.GetPendingWarp(playerID)
	if warp == nil {
		return 0, 0, 0, false
	}
	return warp.MapID, warp.X, warp.Y, true
}

// SetPendingWarp sets a pending warp on a player's map character.
func (w *World) SetPendingWarp(mapID, playerID, toMapID, toX, toY int) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return
	}
	m.SetPendingWarp(playerID, &gamemap.WarpDest{MapID: toMapID, X: toX, Y: toY})
}

// UpdateMapEquipment updates the visible equipment on a player's map character.
func (w *World) UpdateMapEquipment(mapID, playerID, boots, armor, hat, shield, weapon int) {
	w.mu.RLock()
	m := w.maps[mapID]
	w.mu.RUnlock()
	if m == nil {
		return
	}
	m.UpdateEquipment(playerID, boots, armor, hat, shield, weapon)
}
