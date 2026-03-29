package handlers

import (
	"context"
	"strings"

	"github.com/avdoseferovic/geoserv/internal/player"
	"github.com/avdoseferovic/geoserv/internal/player/handlers/guild"
	"github.com/ethanmoffat/eolib-go/v3/protocol/net/client"
	"github.com/ethanmoffat/eolib-go/v3/protocol/net/server"
)

func handleGuildKick(ctx context.Context, p *player.Player, reader *player.EoReader) error {
	if p.State != player.StateInGame || p.CharacterID == nil {
		return nil
	}
	var pkt client.GuildKickClientPacket
	if err := pkt.Deserialize(reader); err != nil {
		return nil
	}
	if !p.ValidateSessionID(pkt.SessionId) {
		return nil
	}
	info, err := guild.LoadOwnInfo(ctx, p.DB, *p.CharacterID)
	if err != nil {
		return nil
	}
	if info.Rank > 2 {
		return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_NotRecruiter})
	}
	member, err := guild.LoadMemberByName(ctx, p.DB, strings.ToLower(pkt.MemberName))
	if err != nil || member.GuildID != info.ID {
		return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_RemoveNotMember})
	}
	if member.Rank == 1 {
		return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_RemoveLeader})
	}
	if member.Rank <= info.Rank {
		return nil
	}
	if err := p.DB.Execute(ctx, `UPDATE characters SET guild_id = NULL, guild_rank = NULL, guild_rank_string = NULL WHERE id = ?`, member.ID); err != nil {
		return nil
	}
	if id, found := p.World.FindPlayerByName(strings.ToLower(pkt.MemberName)); found {
		p.World.SendToPlayer(id, &server.GuildKickServerPacket{})
	}
	return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_Removed})
}

func handleGuildRank(ctx context.Context, p *player.Player, reader *player.EoReader) error {
	if p.State != player.StateInGame || p.CharacterID == nil {
		return nil
	}
	var pkt client.GuildRankClientPacket
	if err := pkt.Deserialize(reader); err != nil {
		return nil
	}
	if !p.ValidateSessionID(pkt.SessionId) {
		return nil
	}
	info, err := guild.LoadOwnInfo(ctx, p.DB, *p.CharacterID)
	if err != nil {
		return nil
	}
	if info.Rank != 1 {
		return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_RankingLeader})
	}
	if pkt.Rank < 1 || pkt.Rank > 9 {
		return nil
	}
	member, err := guild.LoadMemberByName(ctx, p.DB, strings.ToLower(pkt.MemberName))
	if err != nil || member.GuildID != info.ID {
		return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_RankingNotMember})
	}
	if member.Rank == 1 {
		return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_RankingLeader})
	}
	ranks, _ := guild.LoadRanks(ctx, p.DB, info.ID)
	norm := guild.NormalizeRanks(ranks)
	idx := pkt.Rank - 1
	if idx < 0 || idx >= len(norm) {
		return nil
	}
	if err := p.DB.Execute(ctx, `UPDATE characters SET guild_rank = ?, guild_rank_string = ? WHERE id = ?`, pkt.Rank, norm[idx], member.ID); err != nil {
		return nil
	}
	if id, found := p.World.FindPlayerByName(strings.ToLower(pkt.MemberName)); found {
		p.World.SendToPlayer(id, &server.GuildAcceptServerPacket{Rank: pkt.Rank})
	}
	return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_Updated})
}

func handleGuildRemove(ctx context.Context, p *player.Player, reader *player.EoReader) error {
	if p.State != player.StateInGame || p.CharacterID == nil {
		return nil
	}
	var pkt client.GuildRemoveClientPacket
	if err := pkt.Deserialize(reader); err != nil {
		return nil
	}
	if !p.ValidateSessionID(pkt.SessionId) {
		return nil
	}
	info, err := guild.LoadOwnInfo(ctx, p.DB, *p.CharacterID)
	if err != nil {
		return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_RemoveNotMember})
	}
	if info.Rank == 1 {
		return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_RemoveLeader})
	}
	if err := p.DB.Execute(ctx, `UPDATE characters SET guild_id = NULL, guild_rank = NULL, guild_rank_string = NULL WHERE id = ?`, *p.CharacterID); err != nil {
		return nil
	}
	p.GuildTag = ""
	return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_Removed})
}

func handleGuildJunk(ctx context.Context, p *player.Player, reader *player.EoReader) error {
	if p.State != player.StateInGame || p.CharacterID == nil {
		return nil
	}
	var pkt client.GuildJunkClientPacket
	if err := pkt.Deserialize(reader); err != nil {
		return nil
	}
	if !p.ValidateSessionID(pkt.SessionId) {
		return nil
	}
	info, err := guild.LoadOwnInfo(ctx, p.DB, *p.CharacterID)
	if err != nil || info.Rank != 1 {
		return nil
	}
	if err := p.DB.Execute(ctx, `UPDATE characters SET guild_id = NULL, guild_rank = NULL, guild_rank_string = NULL WHERE guild_id = ?`, info.ID); err != nil {
		return nil
	}
	if err := p.DB.Execute(ctx, `DELETE FROM guild_ranks WHERE guild_id = ?`, info.ID); err != nil {
		return nil
	}
	if err := p.DB.Execute(ctx, `DELETE FROM guilds WHERE id = ?`, info.ID); err != nil {
		return nil
	}
	p.GuildTag = ""
	return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_Removed})
}

func handleGuildAgree(ctx context.Context, p *player.Player, reader *player.EoReader) error {
	if p.State != player.StateInGame || p.CharacterID == nil {
		return nil
	}
	var pkt client.GuildAgreeClientPacket
	if err := pkt.Deserialize(reader); err != nil {
		return nil
	}
	if !p.ValidateSessionID(pkt.SessionId) {
		return nil
	}
	info, err := guild.LoadOwnInfo(ctx, p.DB, *p.CharacterID)
	if err != nil || info.Rank != 1 {
		return nil
	}
	switch pkt.InfoType {
	case client.GuildInfo_Description:
		data, _ := pkt.InfoTypeData.(*client.GuildAgreeInfoTypeDataDescription)
		if data == nil {
			return nil
		}
		description := strings.TrimSpace(data.Description)
		if len(description) > p.Cfg.Guild.MaxDescriptionLength {
			return nil
		}
		if err := p.DB.Execute(ctx, `UPDATE guilds SET description = ? WHERE id = ?`, description, info.ID); err != nil {
			return nil
		}
		return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_Updated})
	case client.GuildInfo_Ranks:
		data, _ := pkt.InfoTypeData.(*client.GuildAgreeInfoTypeDataRanks)
		if data == nil {
			return nil
		}
		ranks := guild.NormalizeRanks(data.Ranks)
		if !guild.ValidateRanks(p.Cfg.Guild.MaxRankLength, ranks) {
			return nil
		}
		if err := p.DB.Execute(ctx, `DELETE FROM guild_ranks WHERE guild_id = ?`, info.ID); err != nil {
			return nil
		}
		for i, rank := range ranks {
			_ = p.DB.Execute(ctx, `INSERT INTO guild_ranks (guild_id, index, rank) VALUES (?, ?, ?)`, info.ID, i+1, rank)
		}
		return p.Bus.SendPacket(&server.GuildReplyServerPacket{ReplyCode: server.GuildReply_RanksUpdated})
	default:
		return nil
	}
}
