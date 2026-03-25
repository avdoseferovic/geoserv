package formula

import "testing"

func TestExpForLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		level int
		want  int
	}{
		{"level 0", 0, 0},
		{"level 1", 1, ExpTable[1]},
		{"level 10", 10, ExpTable[10]},
		{"level 253", 253, ExpTable[253]},
		{"negative level", -1, 0},
		{"level out of range", 254, 0},
		{"level way out of range", 999, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExpForLevel(tt.level)
			if got != tt.want {
				t.Errorf("ExpForLevel(%d) = %d, want %d", tt.level, got, tt.want)
			}
		})
	}
}

func TestLevelForExp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		exp  int
		want int
	}{
		{"zero exp", 0, 0},
		{"exactly level 1 exp", ExpTable[1], 1},
		{"between level 1 and 2", ExpTable[1] + 1, 1},
		{"exactly level 10 exp", ExpTable[10], 10},
		{"huge exp beyond max level", ExpTable[253] + 1, 253},
		{"negative exp", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := LevelForExp(tt.exp)
			if got != tt.want {
				t.Errorf("LevelForExp(%d) = %d, want %d", tt.exp, got, tt.want)
			}
		})
	}
}

func TestLevelForExp_RoundTrip(t *testing.T) {
	t.Parallel()
	// For every level, ExpForLevel -> LevelForExp should return the same level
	for level := range 254 {
		exp := ExpForLevel(level)
		got := LevelForExp(exp)
		if got != level {
			t.Errorf("round-trip failed: level %d -> exp %d -> level %d", level, exp, got)
		}
	}
}

func TestExpTableMonotonic(t *testing.T) {
	t.Parallel()
	// Exp requirements must strictly increase
	for i := 1; i < len(ExpTable); i++ {
		if ExpTable[i] <= ExpTable[i-1] {
			t.Errorf("ExpTable[%d] (%d) <= ExpTable[%d] (%d) — not strictly increasing",
				i, ExpTable[i], i-1, ExpTable[i-1])
		}
	}
}
