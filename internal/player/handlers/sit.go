package handlers

import (
	"context"
	"log/slog"

	"github.com/avdo/goeoserv/internal/gamemap"
	"github.com/avdo/goeoserv/internal/player"
	eoproto "github.com/ethanmoffat/eolib-go/v3/protocol"
	eonet "github.com/ethanmoffat/eolib-go/v3/protocol/net"
	"github.com/ethanmoffat/eolib-go/v3/protocol/net/client"
	"github.com/ethanmoffat/eolib-go/v3/protocol/net/server"
)

func init() {
	player.Register(eonet.PacketFamily_Sit, eonet.PacketAction_Request, handleSitRequest)
	player.Register(eonet.PacketFamily_Chair, eonet.PacketAction_Request, handleChairRequest)
}

func handleSitRequest(ctx context.Context, p *player.Player, reader *player.EoReader) error {
	if p.State != player.StateInGame || p.World == nil {
		return nil
	}

	var pkt client.SitRequestClientPacket
	if err := pkt.Deserialize(reader); err != nil {
		slog.Error("failed to deserialize sit request", "id", p.ID, "err", err)
		return nil
	}

	// Get current sit state from map (0 = standing, 2 = floor)
	currentSitState := 0
	if pos := p.World.GetPlayerPosition(p.ID); pos != nil {
		if mc, ok := pos.(*gamemap.MapCharacter); ok {
			currentSitState = mc.SitState
		}
	}

	// Toggle sit/stand
	newSitState := 2 // sitting on floor
	if currentSitState == 2 {
		newSitState = 0 // stand up
	}

	p.World.UpdatePlayerSitState(p.MapID, p.ID, newSitState)

	p.World.BroadcastMap(p.MapID, p.ID, &server.SitPlayerServerPacket{
		PlayerId:  p.ID,
		Coords:    eoproto.Coords{X: p.CharX, Y: p.CharY},
		Direction: eoproto.Direction(p.CharDirection),
	})

	return nil
}

func handleChairRequest(ctx context.Context, p *player.Player, reader *player.EoReader) error {
	if p.State != player.StateInGame || p.World == nil {
		return nil
	}

	var pkt client.ChairRequestClientPacket
	if err := pkt.Deserialize(reader); err != nil {
		slog.Error("failed to deserialize chair request", "id", p.ID, "err", err)
		return nil
	}

	// Handle sit/stand action from packet
	// SitState: 0 = standing, 1 = chair, 2 = floor
	switch pkt.SitAction {
	case client.SitAction_Sit: // Sit
		// Use coordinates from packet if available, otherwise use player position
		x, y := p.CharX, p.CharY
		if sitData, ok := pkt.SitActionData.(*client.ChairRequestSitActionDataSit); ok {
			x = int(sitData.Coords.X)
			y = int(sitData.Coords.Y)
		}

		p.World.UpdatePlayerSitState(p.MapID, p.ID, 1) // 1 = sitting on chair

		p.World.BroadcastMap(p.MapID, p.ID, &server.SitPlayerServerPacket{
			PlayerId:  p.ID,
			Coords:    eoproto.Coords{X: x, Y: y},
			Direction: eoproto.Direction(p.CharDirection),
		})

	case client.SitAction_Stand: // Stand
		p.World.UpdatePlayerSitState(p.MapID, p.ID, 0) // 0 = standing

		p.World.BroadcastMap(p.MapID, p.ID, &server.SitPlayerServerPacket{
			PlayerId:  p.ID,
			Coords:    eoproto.Coords{X: p.CharX, Y: p.CharY},
			Direction: eoproto.Direction(p.CharDirection),
		})
	}

	return nil
}
