package gamemap

import (
	"math/rand/v2"

	"github.com/avdoseferovic/geoserv/internal/protocol"
	eoproto "github.com/ethanmoffat/eolib-go/v3/protocol"
	"github.com/ethanmoffat/eolib-go/v3/protocol/net/server"
)

// npcTickBatch collects NPC updates during a tick for batched broadcasting.
type npcTickBatch struct {
	positions []server.NpcUpdatePosition
	attacks   []server.NpcUpdateAttack
}

// TickNPCs processes NPC logic for one tick: respawning, movement, actions.
// Updates are collected during the locked phase and broadcast in a single
// batched packet per player after unlocking, reducing serializations from
// (NPCs * players) to just (players).
func (m *GameMap) TickNPCs(actRate int) {
	m.mu.Lock()

	if len(m.players) == 0 && m.cfg.NPCs.FreezeOnEmptyMap {
		m.mu.Unlock()
		return
	}

	var batch npcTickBatch

	for _, npc := range m.npcs {
		if !npc.Alive {
			npc.SpawnTicks--
			if npc.SpawnTicks <= 0 {
				sx, sy := m.findFreeSpawnTile(npc.SpawnX, npc.SpawnY)
				npc.Alive = true
				npc.HP = npc.MaxHP
				npc.X = sx
				npc.Y = sy
				npc.Direction = rand.IntN(4)
				npc.Opponents = nil

				batch.positions = append(batch.positions, npcPositionUpdate(npc))
			}
			continue
		}

		npc.ActTicks++
		npc.TalkTicks++

		npcActRate := m.npcSpeedForType(npc.SpawnType)
		if npcActRate <= 0 {
			continue
		}

		if npc.ActTicks < npcActRate {
			continue
		}
		npc.ActTicks = 0

		boredTimer := m.cfg.NPCs.BoredTimer
		if boredTimer > 0 {
			for pid, opp := range npc.Opponents {
				opp.BoredTicks += actRate
				if opp.BoredTicks >= boredTimer {
					delete(npc.Opponents, pid)
				}
			}
		}

		if target, _ := m.npcClosestOpponentLocked(npc); target == nil {
			m.npcAcquireAggroLocked(npc)
		}

		if target, _ := m.npcClosestOpponentLocked(npc); target != nil {
			m.npcActCollect(npc, &batch)
		} else {
			if rand.IntN(4) == 0 {
				if moved := m.npcRandomWalk(npc); moved {
					batch.positions = append(batch.positions, npcPositionUpdate(npc))
				}
			}
		}
	}

	if len(batch.positions) == 0 && len(batch.attacks) == 0 {
		m.mu.Unlock()
		return
	}

	// Snapshot player state while still locked
	type playerSnapshot struct {
		id     int
		hp, tp int
		bus    *protocol.PacketBus
	}
	snapshots := make([]playerSnapshot, 0, len(m.players))
	for _, ch := range m.players {
		snapshots = append(snapshots, playerSnapshot{
			id: ch.PlayerID, hp: ch.HP, tp: ch.TP, bus: ch.Bus,
		})
	}
	m.mu.Unlock()

	// Send one batched packet per player outside the lock
	for i := range snapshots {
		ps := &snapshots[i]
		pkt := &server.NpcPlayerServerPacket{
			Positions: batch.positions,
			Attacks:   batch.attacks,
		}
		if npcAttackTargetsPlayer(batch.attacks, ps.id) {
			pkt.Hp = &ps.hp
			pkt.Tp = &ps.tp
		}
		_ = ps.bus.SendPacket(pkt)
	}
}

func npcPositionUpdate(npc *NpcState) server.NpcUpdatePosition {
	return server.NpcUpdatePosition{
		NpcIndex:  npc.Index,
		Coords:    eoproto.Coords{X: npc.X, Y: npc.Y},
		Direction: eoproto.Direction(npc.Direction),
	}
}
