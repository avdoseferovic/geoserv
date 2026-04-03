package gamemap

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/avdoseferovic/geoserv/internal/config"
	"github.com/avdoseferovic/geoserv/internal/protocol"
	pubdata "github.com/avdoseferovic/geoserv/internal/pub"
	"github.com/ethanmoffat/eolib-go/v3/data"
	eomap "github.com/ethanmoffat/eolib-go/v3/protocol/map"
	eonet "github.com/ethanmoffat/eolib-go/v3/protocol/net"
	"github.com/ethanmoffat/eolib-go/v3/protocol/net/server"
	eopub "github.com/ethanmoffat/eolib-go/v3/protocol/pub"
	eopubsrv "github.com/ethanmoffat/eolib-go/v3/protocol/pub/server"
)

func TestNpcNextChaseStepLockedRoutesAroundWalls(t *testing.T) {
	m := New(1, &eomap.Emf{
		Width:  5,
		Height: 3,
		TileSpecRows: []eomap.MapTileSpecRow{{
			Y: 1,
			Tiles: []eomap.MapTileSpecRowTile{{
				X:        2,
				TileSpec: eomap.MapTileSpec_Wall,
			}, {
				X:        3,
				TileSpec: eomap.MapTileSpec_Wall,
			}},
		}},
	}, &config.Config{NPCs: config.NPCs{ChaseDistance: 8}})

	npc := &NpcState{Index: 0, ID: 1, X: 1, Y: 1, Alive: true}
	target := &MapCharacter{PlayerID: 7, X: 4, Y: 1, HP: 10}

	m.npcs = []*NpcState{npc}
	m.players = map[int]*MapCharacter{target.PlayerID: target}

	nextX, nextY, dir, ok := m.npcNextChaseStepLocked(npc, target)
	if !ok {
		t.Fatal("expected NPC to find a chase step around the wall")
	}
	if nextX != 1 || nextY != 2 {
		t.Fatalf("next chase step = (%d,%d), want (1,2)", nextX, nextY)
	}
	if dir != 0 {
		t.Fatalf("first chase direction = %d, want 0 (down)", dir)
	}
}

func TestNpcAttackLockedRejectsDiagonalTargets(t *testing.T) {
	setNpcTestDB(t, map[int]eopub.EnfRecord{1: {MinDamage: 5, MaxDamage: 5}})

	m := New(1, &eomap.Emf{Width: 3, Height: 3}, &config.Config{NPCs: config.NPCs{ChaseDistance: 8}})
	npc := &NpcState{Index: 0, ID: 1, X: 1, Y: 1, Alive: true}
	target := &MapCharacter{PlayerID: 7, X: 2, Y: 2, HP: 10, MaxHP: 10}

	attack, ok := m.npcAttackLocked(npc, target)
	if ok {
		t.Fatalf("expected diagonal attack to fail, got %#v", attack)
	}
	if target.HP != 10 {
		t.Fatalf("target HP = %d after diagonal attack, want 10", target.HP)
	}
	if npcIsOrthogonallyAdjacent(npc.X, npc.Y, target.X, target.Y) {
		t.Fatal("expected diagonal tiles to not count as melee adjacency")
	}
}

func TestTickNPCs_BroadcastsAmbientNpcChat(t *testing.T) {
	setNpcTalkTestDB(t, map[int]eopubsrv.TalkRecord{
		1: {
			NpcId: 1,
			Rate:  100,
			Messages: []eopubsrv.TalkMessageRecord{
				{Message: "Stay awhile."},
			},
		},
	})

	bus, conn := newCaptureBus()
	m := New(1, &eomap.Emf{Width: 3, Height: 3}, &config.Config{
		NPCs: config.NPCs{
			ActRate:       1,
			TalkRate:      2,
			ChaseDistance: 8,
			Speed0:        8,
		},
	})
	m.npcs = []*NpcState{{Index: 0, ID: 1, X: 1, Y: 1, Alive: true}}
	m.players = map[int]*MapCharacter{
		7: {PlayerID: 7, X: 2, Y: 2, HP: 10, TP: 5, Bus: bus},
	}

	m.TickNPCs(1)
	if len(conn.writes) != 0 {
		t.Fatalf("unexpected packet after first tick: %d writes", len(conn.writes))
	}

	m.TickNPCs(1)

	pkt := decodeLastNpcPacket(t, conn)
	if got := len(pkt.Chats); got != 1 {
		t.Fatalf("chat update count = %d, want 1", got)
	}
	if got := pkt.Chats[0].NpcIndex; got != 0 {
		t.Fatalf("npc index = %d, want 0", got)
	}
	if got := pkt.Chats[0].Message; got != "Stay awhile." {
		t.Fatalf("chat message = %q, want %q", got, "Stay awhile.")
	}
}

func setNpcTestDB(t *testing.T, npcs map[int]eopub.EnfRecord) {
	t.Helper()

	prev := pubdata.NpcDB
	t.Cleanup(func() {
		pubdata.NpcDB = prev
	})

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

func setNpcTalkTestDB(t *testing.T, talks map[int]eopubsrv.TalkRecord) {
	t.Helper()

	prevDB := pubdata.TalkDB
	t.Cleanup(func() {
		pubdata.TalkDB = prevDB
	})

	maxID := 0
	for npcID := range talks {
		if npcID > maxID {
			maxID = npcID
		}
	}

	talkDB := &eopubsrv.TalkFile{Npcs: make([]eopubsrv.TalkRecord, 0, maxID)}
	for npcID, record := range talks {
		record.NpcId = npcID
		talkDB.Npcs = append(talkDB.Npcs, record)
	}

	pubdata.TalkDB = talkDB
}

type captureConn struct {
	net.Conn
	writes [][]byte
}

func (c *captureConn) Write(b []byte) (int, error) {
	c.writes = append(c.writes, append([]byte(nil), b...))
	return len(b), nil
}

func (*captureConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (*captureConn) SetWriteDeadline(time.Time) error { return nil }
func (*captureConn) SetReadDeadline(time.Time) error  { return nil }
func (*captureConn) Close() error                     { return nil }
func (*captureConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (*captureConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }

func newCaptureBus() (*protocol.PacketBus, *captureConn) {
	conn := &captureConn{}
	return protocol.NewPacketBus(protocol.NewTCPConn(conn)), conn
}

func decodeLastNpcPacket(t *testing.T, conn *captureConn) server.NpcPlayerServerPacket {
	t.Helper()

	if len(conn.writes) == 0 {
		t.Fatal("expected at least one packet write")
	}

	buf := conn.writes[len(conn.writes)-1]
	if len(buf) < 4 {
		t.Fatalf("packet too short: %d bytes", len(buf))
	}
	if got, want := data.DecodeNumber(buf[:2]), len(buf)-2; got != want {
		t.Fatalf("encoded packet length = %d, want %d", got, want)
	}
	if got := eonet.PacketAction(buf[2]); got != eonet.PacketAction_Player {
		t.Fatalf("packet action = %v, want %v", got, eonet.PacketAction_Player)
	}
	if got := eonet.PacketFamily(buf[3]); got != eonet.PacketFamily_Npc {
		t.Fatalf("packet family = %v, want %v", got, eonet.PacketFamily_Npc)
	}

	reader := data.NewEoReader(buf[4:])
	var pkt server.NpcPlayerServerPacket
	if err := pkt.Deserialize(reader); err != nil {
		t.Fatalf("deserialize npc packet: %v", err)
	}
	return pkt
}
