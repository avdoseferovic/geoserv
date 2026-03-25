package quest

import "testing"

func TestProcessRule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		rule      Rule
		npcChoice int
		ctx       *QuestPlayerContext
		wantGoto  string
		wantMatch bool
	}{
		{
			name:      "InputNpc matching choice",
			rule:      Rule{Name: "InputNpc", Args: []Arg{{IntVal: 2}}, Goto: "Next"},
			npcChoice: 2,
			wantGoto:  "Next",
			wantMatch: true,
		},
		{
			name:      "InputNpc wrong choice",
			rule:      Rule{Name: "InputNpc", Args: []Arg{{IntVal: 2}}, Goto: "Next"},
			npcChoice: 3,
			wantGoto:  "",
			wantMatch: false,
		},
		{
			name:      "TalkedToNpc always matches",
			rule:      Rule{Name: "TalkedToNpc", Args: []Arg{{IntVal: 5}}, Goto: "Talk"},
			npcChoice: 0,
			wantGoto:  "Talk",
			wantMatch: true,
		},
		{
			name:      "Always matches",
			rule:      Rule{Name: "Always", Goto: "End"},
			npcChoice: 0,
			wantGoto:  "End",
			wantMatch: true,
		},
		{
			name:      "KilledNpcs sufficient kills",
			rule:      Rule{Name: "KilledNpcs", Args: []Arg{{IntVal: 10}, {IntVal: 5}}, Goto: "Done"},
			npcChoice: 0,
			ctx:       &QuestPlayerContext{NpcKills: map[int]int{10: 7}},
			wantGoto:  "Done",
			wantMatch: true,
		},
		{
			name:      "KilledNpcs insufficient kills",
			rule:      Rule{Name: "KilledNpcs", Args: []Arg{{IntVal: 10}, {IntVal: 5}}, Goto: "Done"},
			npcChoice: 0,
			ctx:       &QuestPlayerContext{NpcKills: map[int]int{10: 3}},
			wantGoto:  "",
			wantMatch: false,
		},
		{
			name:      "KilledNpcs nil context",
			rule:      Rule{Name: "KilledNpcs", Args: []Arg{{IntVal: 10}, {IntVal: 5}}, Goto: "Done"},
			npcChoice: 0,
			ctx:       nil,
			wantGoto:  "",
			wantMatch: false,
		},
		{
			name:      "GotItems sufficient",
			rule:      Rule{Name: "GotItems", Args: []Arg{{IntVal: 100}, {IntVal: 3}}, Goto: "HasItems"},
			npcChoice: 0,
			ctx:       &QuestPlayerContext{Inventory: map[int]int{100: 5}},
			wantGoto:  "HasItems",
			wantMatch: true,
		},
		{
			name:      "GotItems insufficient",
			rule:      Rule{Name: "GotItems", Args: []Arg{{IntVal: 100}, {IntVal: 3}}, Goto: "HasItems"},
			npcChoice: 0,
			ctx:       &QuestPlayerContext{Inventory: map[int]int{100: 1}},
			wantGoto:  "",
			wantMatch: false,
		},
		{
			name:      "GotItems nil context",
			rule:      Rule{Name: "GotItems", Args: []Arg{{IntVal: 100}, {IntVal: 3}}, Goto: "HasItems"},
			npcChoice: 0,
			ctx:       nil,
			wantGoto:  "",
			wantMatch: false,
		},
		{
			name:      "unknown rule",
			rule:      Rule{Name: "FooBar", Goto: "X"},
			npcChoice: 0,
			wantGoto:  "",
			wantMatch: false,
		},
		{
			name:      "case insensitive rule name",
			rule:      Rule{Name: "ALWAYS", Goto: "End"},
			npcChoice: 0,
			wantGoto:  "End",
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotGoto, gotMatch := ProcessRuleWithContext(tt.rule, tt.npcChoice, tt.ctx)
			if gotGoto != tt.wantGoto || gotMatch != tt.wantMatch {
				t.Errorf("ProcessRuleWithContext() = (%q, %v), want (%q, %v)",
					gotGoto, gotMatch, tt.wantGoto, tt.wantMatch)
			}
		})
	}
}
