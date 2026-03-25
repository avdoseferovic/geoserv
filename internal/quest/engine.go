package quest

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// QuestDB holds all loaded quests indexed by quest ID.
var QuestDB = make(map[int]*Quest)

// LoadQuests loads all .eqf files from the given directory.
func LoadQuests(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading quest directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".eqf") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("failed to read quest file", "path", path, "err", err)
			continue
		}

		// Extract quest ID from filename (e.g., "00001.eqf" -> 1)
		baseName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		questID, err := strconv.Atoi(baseName)
		if err != nil {
			slog.Warn("invalid quest filename", "name", entry.Name())
			continue
		}

		quest, err := Parse(questID, string(content))
		if err != nil {
			slog.Warn("failed to parse quest", "path", path, "err", err)
			continue
		}

		QuestDB[questID] = quest
		count++
	}

	slog.Info("quests loaded", "count", count)
	return nil
}

// QuestPlayerContext provides player state needed for quest rule evaluation.
type QuestPlayerContext struct {
	NpcKills  map[int]int // npcID -> kill count (from active quest state)
	Inventory map[int]int // itemID -> amount
}

// ProcessRuleWithContext checks if a rule condition is met, using player context for inventory/kill checks.
func ProcessRuleWithContext(rule Rule, npcInputChoice int, ctx *QuestPlayerContext) (string, bool) {
	lower := strings.ToLower(rule.Name)
	switch lower {
	case "inputnpc":
		// InputNpc(choice_id) — player selected a dialog option
		if len(rule.Args) > 0 && !rule.Args[0].IsStr && rule.Args[0].IntVal == npcInputChoice {
			return rule.Goto, true
		}
	case "talkedtonpc":
		// TalkedToNpc(npc_id) — always true when talking to the NPC
		return rule.Goto, true
	case "killednpcs":
		// KilledNpcs(npc_id, count)
		if ctx == nil || len(rule.Args) < 2 {
			return "", false
		}
		npcID := rule.Args[0].IntVal
		required := rule.Args[1].IntVal
		if ctx.NpcKills[npcID] >= required {
			return rule.Goto, true
		}
		return "", false
	case "gotitems":
		// GotItems(item_id, count)
		if ctx == nil || len(rule.Args) < 2 {
			return "", false
		}
		itemID := rule.Args[0].IntVal
		required := rule.Args[1].IntVal
		if ctx.Inventory[itemID] >= required {
			return rule.Goto, true
		}
		return "", false
	case "always":
		return rule.Goto, true
	}
	return "", false
}
