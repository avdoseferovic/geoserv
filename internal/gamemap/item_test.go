package gamemap

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/avdoseferovic/geoserv/internal/config"
	"github.com/avdoseferovic/geoserv/internal/protocol"
	eomap "github.com/ethanmoffat/eolib-go/v3/protocol/map"
)

func TestDropNpcItem_UsesNpcProtectionAndAllowsKillerPickup(t *testing.T) {
	killerBus, killerConn := newItemCaptureBus()
	otherBus, _ := newItemCaptureBus()

	m := New(1, &eomap.Emf{Width: 3, Height: 3}, &config.Config{
		World: config.World{DropProtectNPC: 5},
		Map:   config.Map{MaxItems: 200},
	})
	m.players = map[int]*MapCharacter{
		1: {PlayerID: 1, X: 1, Y: 1, Bus: killerBus},
		2: {PlayerID: 2, X: 2, Y: 2, Bus: otherBus},
	}

	uid := m.DropNpcItem(100, 2, 1, 1, 1)
	if uid == 0 {
		t.Fatal("expected NPC drop UID")
	}
	if got := len(killerConn.writes); got == 0 {
		t.Fatal("killer did not receive NPC item add packet")
	}

	if item := m.PickupItem(uid, 2); item != nil {
		t.Fatal("non-killer should not pick protected NPC loot")
	}

	item := m.PickupItem(uid, 1)
	if item == nil {
		t.Fatal("killer should pick protected NPC loot")
	}
	if item.ItemID != 100 || item.Amount != 2 {
		t.Fatalf("picked item = %#v, want itemID=100 amount=2", item)
	}
}

func TestDropItem_UsesPlayerProtectionAndExcludesDropperBroadcast(t *testing.T) {
	dropperBus, dropperConn := newItemCaptureBus()
	otherBus, otherConn := newItemCaptureBus()

	m := New(1, &eomap.Emf{Width: 3, Height: 3}, &config.Config{
		World: config.World{DropProtectPlayer: 5},
		Map:   config.Map{MaxItems: 200},
	})
	m.players = map[int]*MapCharacter{
		1: {PlayerID: 1, X: 1, Y: 1, Bus: dropperBus},
		2: {PlayerID: 2, X: 2, Y: 2, Bus: otherBus},
	}

	uid := m.DropItem(200, 3, 1, 1, 1)
	if uid == 0 {
		t.Fatal("expected player drop UID")
	}
	if got := len(dropperConn.writes); got != 0 {
		t.Fatalf("dropper writes = %d, want 0 because ItemDropServerPacket is sent elsewhere", got)
	}
	if got := len(otherConn.writes); got == 0 {
		t.Fatal("other player did not receive item add packet")
	}

	if item := m.PickupItem(uid, 2); item != nil {
		t.Fatal("non-owner should not pick protected player drop")
	}

	item := m.PickupItem(uid, 1)
	if item == nil {
		t.Fatal("dropper should pick protected player drop")
	}
	if item.ItemID != 200 || item.Amount != 3 {
		t.Fatalf("picked item = %#v, want itemID=200 amount=3", item)
	}
}

type itemCaptureConn struct {
	net.Conn
	writes [][]byte
}

func (c *itemCaptureConn) Write(b []byte) (int, error) {
	c.writes = append(c.writes, append([]byte(nil), b...))
	return len(b), nil
}

func (*itemCaptureConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (*itemCaptureConn) SetWriteDeadline(time.Time) error { return nil }
func (*itemCaptureConn) SetReadDeadline(time.Time) error  { return nil }
func (*itemCaptureConn) Close() error                     { return nil }
func (*itemCaptureConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (*itemCaptureConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }

func newItemCaptureBus() (*protocol.PacketBus, *itemCaptureConn) {
	conn := &itemCaptureConn{}
	return protocol.NewPacketBus(protocol.NewTCPConn(conn)), conn
}
