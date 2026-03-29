package gamemap

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/avdoseferovic/geoserv/internal/config"
	"github.com/avdoseferovic/geoserv/internal/protocol"
	pubdata "github.com/avdoseferovic/geoserv/internal/pub"
	eodata "github.com/ethanmoffat/eolib-go/v3/data"
	eomap "github.com/ethanmoffat/eolib-go/v3/protocol/map"
	eonet "github.com/ethanmoffat/eolib-go/v3/protocol/net"
	eopub "github.com/ethanmoffat/eolib-go/v3/protocol/pub"
)

// discardConn is a net.Conn that discards all writes and never blocks.
// Used to benchmark game logic without network I/O overhead.
type discardConn struct{ net.Conn }

func (discardConn) Write(b []byte) (int, error)      { return len(b), nil }
func (discardConn) SetWriteDeadline(time.Time) error { return nil }
func (discardConn) SetReadDeadline(time.Time) error  { return nil }
func (discardConn) Close() error                     { return nil }
func (discardConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (discardConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (discardConn) Read([]byte) (int, error)         { select {} } //nolint:all

func newDiscardBus() *protocol.PacketBus {
	conn := protocol.NewTCPConn(discardConn{})
	return protocol.NewPacketBus(conn)
}

func setBenchNpcDB(tb testing.TB, npcs map[int]eopub.EnfRecord) {
	tb.Helper()

	prev := pubdata.NpcDB
	tb.Cleanup(func() { pubdata.NpcDB = prev })

	maxID := 0
	for npcID := range npcs {
		if npcID > maxID {
			maxID = npcID
		}
	}
	npcDB := &eopub.Enf{Npcs: make([]eopub.EnfRecord, maxID)}
	for npcID, record := range npcs {
		npcDB.Npcs[npcID-1] = record
	}
	pubdata.NpcDB = npcDB
}

func benchCfg() *config.Config {
	return &config.Config{
		World: config.World{
			RecoverRate:    720,
			NPCRecoverRate: 840,
			SpikeRate:      12,
			DrainRate:      125,
			WarpSuckRate:   15,
			GhostRate:      5,
			SpikeDamage:    0.1,
			TickRate:       125,
		},
		NPCs: config.NPCs{
			FreezeOnEmptyMap: true,
			ChaseDistance:    10,
			BoredTimer:       240,
			ActRate:          5,
			Speed0:           8,
			Speed1:           7,
			Speed2:           6,
			Speed3:           5,
			Speed4:           4,
			Speed5:           3,
			Speed6:           2,
		},
		Map: config.Map{
			DoorCloseRate: 3,
		},
	}
}

// benchMap creates a large open map with the given number of players scattered across it.
func benchMap(b *testing.B, playerCount int) *GameMap {
	b.Helper()

	// 100x100 map — large enough to spread players without excessive collisions
	m := New(1, &eomap.Emf{Width: 100, Height: 100}, benchCfg())

	for i := range playerCount {
		ch := &MapCharacter{
			PlayerID:  i + 1,
			Name:      fmt.Sprintf("player%d", i+1),
			X:         (i % 100),
			Y:         (i / 100) % 100,
			Direction: i % 4,
			HP:        100,
			MaxHP:     200,
			TP:        50,
			MaxTP:     100,
			SP:        30,
			MaxSP:     60,
			Evade:     10,
			Armor:     15,
			Level:     20,
			Bus:       newDiscardBus(),
		}
		m.players[ch.PlayerID] = ch
	}

	return m
}

// benchMapWithNPCs creates a map with players and aggressive NPCs.
func benchMapWithNPCs(b *testing.B, playerCount, npcCount int) *GameMap {
	b.Helper()

	setBenchNpcDB(b, map[int]eopub.EnfRecord{
		1: {
			Type:      eopub.Npc_Aggressive,
			Hp:        500,
			MinDamage: 10,
			MaxDamage: 20,
			Accuracy:  50,
		},
	})

	m := benchMap(b, playerCount)

	for i := range npcCount {
		npc := &NpcState{
			Index:     i,
			ID:        1,
			X:         (i*3 + 50) % 100,
			Y:         (i*7 + 50) % 100,
			Direction: i % 4,
			SpawnX:    50,
			SpawnY:    50,
			SpawnType: 0,
			SpawnTime: 60,
			HP:        500,
			MaxHP:     500,
			Alive:     true,
			Opponents: make(map[int]*NpcOpponent),
		}
		// Give each NPC a few opponents to trigger chase/attack logic
		for j := range min(3, playerCount) {
			pid := (i+j)%playerCount + 1
			npc.Opponents[pid] = &NpcOpponent{DamageDealt: 10}
		}
		m.npcs = append(m.npcs, npc)
	}

	return m
}

// --- Tick Benchmarks ---

func BenchmarkTick(b *testing.B) {
	for _, pc := range []int{10, 50, 100, 500, 1000} {
		b.Run(fmt.Sprintf("players=%d", pc), func(b *testing.B) {
			m := benchMap(b, pc)
			// Force recovery tick every iteration
			m.tickCount = m.cfg.World.RecoverRate - 1
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				m.Tick()
			}
		})
	}
}

func BenchmarkTickWithNPCs(b *testing.B) {
	for _, tc := range []struct {
		players, npcs int
	}{
		{50, 10},
		{100, 20},
		{200, 50},
		{500, 100},
	} {
		b.Run(fmt.Sprintf("players=%d/npcs=%d", tc.players, tc.npcs), func(b *testing.B) {
			m := benchMapWithNPCs(b, tc.players, tc.npcs)
			m.tickCount = m.cfg.World.RecoverRate - 1
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				m.Tick()
			}
		})
	}
}

// --- NPC AI Benchmarks ---

func BenchmarkTickNPCs(b *testing.B) {
	for _, tc := range []struct {
		players, npcs int
	}{
		{10, 5},
		{50, 20},
		{100, 50},
		{500, 100},
	} {
		b.Run(fmt.Sprintf("players=%d/npcs=%d", tc.players, tc.npcs), func(b *testing.B) {
			m := benchMapWithNPCs(b, tc.players, tc.npcs)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				m.TickNPCs(m.cfg.NPCs.ActRate)
			}
		})
	}
}

// --- Broadcast Benchmarks ---

func BenchmarkBroadcast(b *testing.B) {
	for _, pc := range []int{10, 50, 100, 500, 1000} {
		b.Run(fmt.Sprintf("players=%d", pc), func(b *testing.B) {
			m := benchMap(b, pc)
			pkt := &fakePacket{}
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				m.Broadcast(0, pkt)
			}
		})
	}
}

// --- Walk Benchmarks (includes collision check + broadcast) ---

func BenchmarkWalk(b *testing.B) {
	for _, pc := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("players=%d", pc), func(b *testing.B) {
			m := benchMap(b, pc)
			// Walk player 1 back and forth
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				x := (b.N % 98) + 1
				m.Walk(1, 3, [2]int{x, 0})
			}
		})
	}
}

// --- Recovery Tick (HP/TP regen for all players) ---

func BenchmarkTickRecovery(b *testing.B) {
	for _, pc := range []int{10, 50, 100, 500, 1000} {
		b.Run(fmt.Sprintf("players=%d", pc), func(b *testing.B) {
			m := benchMap(b, pc)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				m.tickRecovery()
			}
		})
	}
}

// --- NPC Combat (DamageNpc under contention) ---

func BenchmarkDamageNpc(b *testing.B) {
	for _, pc := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("players=%d", pc), func(b *testing.B) {
			m := benchMapWithNPCs(b, pc, 20)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				// Multiple players hitting the same NPC
				m.DamageNpc(0, 1, 5)
				// Heal NPC back so it doesn't die
				m.mu.Lock()
				if len(m.npcs) > 0 {
					m.npcs[0].HP = m.npcs[0].MaxHP
					m.npcs[0].Alive = true
				}
				m.mu.Unlock()
			}
		})
	}
}

// --- Enter/Leave churn (players joining and leaving) ---

func BenchmarkEnterLeave(b *testing.B) {
	for _, pc := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("existing=%d", pc), func(b *testing.B) {
			m := benchMap(b, pc)
			newPlayer := &MapCharacter{
				PlayerID: pc + 1,
				Name:     "benchplayer",
				X:        50,
				Y:        50,
				HP:       100,
				MaxHP:    200,
				Bus:      newDiscardBus(),
			}
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				m.Enter(newPlayer)
				m.Leave(newPlayer.PlayerID)
			}
		})
	}
}

// --- Concurrent Tick + Walk (simulates real server load) ---

func BenchmarkConcurrentTickAndWalk(b *testing.B) {
	for _, pc := range []int{50, 100, 500} {
		b.Run(fmt.Sprintf("players=%d", pc), func(b *testing.B) {
			m := benchMapWithNPCs(b, pc, 10)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				// Simulate a tick + several concurrent walks
				done := make(chan struct{})
				go func() {
					m.Tick()
					close(done)
				}()
				// Concurrent player walks
				for j := range min(10, pc) {
					pid := j + 1
					x := (j*3 + 1) % 99
					m.Walk(pid, 3, [2]int{x, (j * 7) % 99})
				}
				<-done
			}
		})
	}
}

// fakePacket implements eonet.Packet for benchmarking broadcasts.
type fakePacket struct{}

func (fakePacket) Family() eonet.PacketFamily { return 1 }
func (fakePacket) Action() eonet.PacketAction { return 1 }
func (fakePacket) Serialize(w *eodata.EoWriter) error {
	return w.AddByte(0x01)
}
func (fakePacket) Deserialize(r *eodata.EoReader) error { return nil }
func (fakePacket) ByteSize() int                        { return 1 }
