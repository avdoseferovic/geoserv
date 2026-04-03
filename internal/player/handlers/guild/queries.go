package guild

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/avdoseferovic/geoserv/internal/config"
	"github.com/avdoseferovic/geoserv/internal/db"
	"github.com/ethanmoffat/eolib-go/v3/protocol/net/server"
)

type Info struct {
	ID          int
	Tag         string
	Name        string
	Description string
	Bank        int
	CreatedAt   string
	Rank        int
}

type MemberInfo struct {
	ID       int
	GuildID  int
	GuildTag string
	Rank     int
}

func LoadOwnInfo(ctx context.Context, d *db.Database, characterID int) (*Info, error) {
	var info Info
	err := d.QueryRow(ctx,
		`SELECT g.id, g.tag, g.name, COALESCE(g.description,''), g.bank, COALESCE(c.guild_rank, 0)
		 FROM characters c JOIN guilds g ON c.guild_id = g.id WHERE c.id = ?`,
		characterID).Scan(&info.ID, &info.Tag, &info.Name, &info.Description, &info.Bank, &info.Rank)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func LoadByTag(ctx context.Context, d *db.Database, tag string) (*Info, []string, error) {
	var info Info
	err := d.QueryRow(ctx,
		`SELECT id, tag, name, COALESCE(description,''), bank, COALESCE(created_at,'') FROM guilds WHERE tag = ?`,
		strings.TrimSpace(tag)).Scan(&info.ID, &info.Tag, &info.Name, &info.Description, &info.Bank, &info.CreatedAt)
	if err != nil {
		return nil, nil, err
	}
	ranks, _ := LoadRanks(ctx, d, info.ID)
	return &info, ranks, nil
}

func LoadRanks(ctx context.Context, d *db.Database, guildID int) ([]string, error) {
	rows, err := d.Query(ctx, `SELECT rank FROM guild_ranks WHERE guild_id = ? ORDER BY index`, guildID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ranks []string
	for rows.Next() {
		var rank string
		if err := rows.Scan(&rank); err == nil {
			ranks = append(ranks, rank)
		}
	}
	return ranks, nil
}

func MustLoadRanks(ctx context.Context, d *db.Database, guildID int) []string {
	ranks, _ := LoadRanks(ctx, d, guildID)
	return ranks
}

func DefaultRanks(cfg config.Guild) []string {
	leader := strings.TrimSpace(cfg.DefaultLeaderRankName)
	if leader == "" {
		leader = "Leader"
	}
	recruiter := strings.TrimSpace(cfg.DefaultRecruiterRank)
	if recruiter == "" {
		recruiter = "Recruiter"
	}
	newMember := strings.TrimSpace(cfg.DefaultNewMemberRank)
	if newMember == "" {
		newMember = "New Member"
	}

	return []string{leader, recruiter, "Officer", "Veteran", "Member", "Member", "Member", "Member", newMember}
}

func NormalizeRanks(cfg config.Guild, ranks []string) []string {
	base := DefaultRanks(cfg)
	for i := 0; i < len(ranks) && i < 9; i++ {
		if strings.TrimSpace(ranks[i]) != "" {
			base[i] = ranks[i]
		}
	}
	return base
}

func LoadStaff(ctx context.Context, d *db.Database, guildID int) ([]server.GuildStaff, error) {
	rows, err := d.Query(ctx,
		`SELECT COALESCE(guild_rank,0), name FROM characters WHERE guild_id = ? AND COALESCE(guild_rank,0) <= 2 ORDER BY guild_rank, name`,
		guildID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var staff []server.GuildStaff
	for rows.Next() {
		var rank int
		var name string
		if err := rows.Scan(&rank, &name); err == nil {
			staff = append(staff, server.GuildStaff{Rank: rank, Name: name})
		}
	}
	return staff, nil
}

func BankWealth(bank int) string {
	return fmt.Sprintf("%d gold", bank)
}

func LoadMemberByName(ctx context.Context, d *db.Database, name string) (*MemberInfo, error) {
	var info MemberInfo
	err := d.QueryRow(ctx,
		`SELECT id, COALESCE(guild_id,0), COALESCE(guild_rank,0) FROM characters WHERE LOWER(name) = ?`,
		strings.ToLower(name)).Scan(&info.ID, &info.GuildID, &info.Rank)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func LoadMemberByCharName(ctx context.Context, d *db.Database, charName string) (*MemberInfo, error) {
	var info MemberInfo
	err := d.QueryRow(ctx,
		`SELECT c.id, COALESCE(c.guild_id,0), COALESCE(c.guild_rank,0), COALESCE(g.tag,'')
		 FROM characters c LEFT JOIN guilds g ON c.guild_id = g.id WHERE LOWER(c.name) = ?`,
		strings.ToLower(charName)).Scan(&info.ID, &info.GuildID, &info.Rank, &info.GuildTag)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func Exists(ctx context.Context, d *db.Database, tag, name string) (bool, error) {
	var exists int
	err := d.QueryRow(ctx,
		`SELECT 1 FROM guilds WHERE UPPER(tag) = ? OR LOWER(name) = ? LIMIT 1`,
		strings.ToUpper(strings.TrimSpace(tag)), strings.ToLower(strings.TrimSpace(name))).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func ValidateRanks(maxRankLength int, ranks []string) bool {
	for _, rank := range ranks {
		if len(strings.TrimSpace(rank)) > maxRankLength {
			return false
		}
	}
	return true
}
