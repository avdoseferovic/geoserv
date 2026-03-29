package gamemap

import (
	"log/slog"

	"github.com/avdoseferovic/geoserv/internal/protocol"
	eonet "github.com/ethanmoffat/eolib-go/v3/protocol/net"
)

// Broadcast sends a packet to all players on this map except excludeID.
// Collects player bus refs under lock, then sends outside the lock
// to avoid blocking map operations during network I/O.
func (m *GameMap) Broadcast(excludeID int, pkt eonet.Packet) {
	m.mu.RLock()
	buses := make([]*protocol.PacketBus, 0, len(m.players))
	for pid, ch := range m.players {
		if pid != excludeID {
			buses = append(buses, ch.Bus)
		}
	}
	m.mu.RUnlock()

	for _, bus := range buses {
		if err := bus.SendPacket(pkt); err != nil {
			slog.Debug("broadcast send error", "err", err)
		}
	}
}

// BroadcastToAdmins sends a packet to players with admin level >= minAdmin.
func (m *GameMap) BroadcastToAdmins(excludeID int, minAdmin int, pkt eonet.Packet) {
	m.mu.RLock()
	buses := make([]*protocol.PacketBus, 0, len(m.players))
	for pid, ch := range m.players {
		if pid != excludeID && ch.Admin >= minAdmin {
			buses = append(buses, ch.Bus)
		}
	}
	m.mu.RUnlock()

	for _, bus := range buses {
		_ = bus.SendPacket(pkt)
	}
}

// BroadcastToGuild sends a packet to players in the specified guild.
func (m *GameMap) BroadcastToGuild(excludeID int, guildTag string, pkt eonet.Packet) {
	m.mu.RLock()
	buses := make([]*protocol.PacketBus, 0, len(m.players))
	for pid, ch := range m.players {
		if pid != excludeID && ch.GuildTag == guildTag {
			buses = append(buses, ch.Bus)
		}
	}
	m.mu.RUnlock()

	for _, bus := range buses {
		_ = bus.SendPacket(pkt)
	}
}
