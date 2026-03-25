package quest

import (
	"strconv"
	"strings"
)

// Quest represents a parsed EO+ quest file.
type Quest struct {
	ID      int
	Name    string
	Version int
	States  map[string]*State
}

// State represents a quest state with actions and transition rules.
type State struct {
	Name        string
	Description string
	Actions     []Action
	Rules       []Rule
}

// Action is a quest action like AddNpcText, ShowHint, GiveItem, etc.
type Action struct {
	Name string
	Args []Arg
}

// Rule is a condition that triggers a state transition.
type Rule struct {
	Name string
	Args []Arg
	Goto string
}

// Arg is either an integer or string argument.
type Arg struct {
	IntVal int
	StrVal string
	IsStr  bool
}

// Parse parses an EO+ quest script string into a Quest struct.
func Parse(id int, input string) (*Quest, error) {
	q := &Quest{
		ID:     id,
		States: make(map[string]*State),
	}

	lines := strings.Split(input, "\n")
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		i++

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if strings.EqualFold(line, "main") {
			i = parseMain(q, lines, i)
		} else if len(line) > 6 && strings.EqualFold(line[:6], "state ") {
			stateName := strings.TrimSpace(line[6:])
			state := &State{Name: stateName}
			i = parseState(state, lines, i)
			q.States[stateName] = state
		}
	}

	return q, nil
}

func parseMain(q *Quest, lines []string, i int) int {
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		i++

		if line == "{" {
			continue
		}
		if line == "}" {
			return i
		}

		if len(line) >= 9 && strings.EqualFold(line[:9], "questname") {
			q.Name = extractQuotedString(line)
		} else if len(line) >= 7 && strings.EqualFold(line[:7], "version") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				q.Version, _ = strconv.Atoi(parts[1])
			}
		}
	}
	return i
}

func parseState(state *State, lines []string, i int) int {
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		i++

		if line == "{" {
			continue
		}
		if line == "}" {
			return i
		}
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if len(line) >= 4 && strings.EqualFold(line[:4], "desc") {
			state.Description = extractQuotedString(line)
		} else if len(line) >= 6 && strings.EqualFold(line[:6], "action") {
			action := parseAction(line)
			if action != nil {
				state.Actions = append(state.Actions, *action)
			}
		} else if len(line) >= 4 && strings.EqualFold(line[:4], "rule") {
			rule := parseRule(line)
			if rule != nil {
				state.Rules = append(state.Rules, *rule)
			}
		}
	}
	return i
}

func parseAction(line string) *Action {
	// Format: action FuncName( arg1 , arg2 , ... );
	idx := strings.Index(strings.ToLower(line), "action")
	if idx < 0 {
		return nil
	}
	rest := strings.TrimSpace(line[idx+6:])
	rest = strings.TrimSuffix(rest, ";")
	rest = strings.TrimSpace(rest)

	name, args := parseFuncCall(rest)
	if name == "" {
		return nil
	}
	return &Action{Name: name, Args: args}
}

func parseRule(line string) *Rule {
	// Format: rule Condition( args ) goto StateName
	idx := strings.Index(strings.ToLower(line), "rule")
	if idx < 0 {
		return nil
	}
	rest := strings.TrimSpace(line[idx+4:])

	// Find "goto"
	gotoIdx := strings.Index(strings.ToLower(rest), "goto")
	if gotoIdx < 0 {
		return nil
	}

	condPart := strings.TrimSpace(rest[:gotoIdx])
	gotoState := strings.TrimSpace(rest[gotoIdx+4:])

	name, args := parseFuncCall(condPart)
	if name == "" {
		return nil
	}

	return &Rule{Name: name, Args: args, Goto: gotoState}
}

func parseFuncCall(s string) (string, []Arg) {
	parenIdx := strings.Index(s, "(")
	if parenIdx < 0 {
		return strings.TrimSpace(s), nil
	}

	name := strings.TrimSpace(s[:parenIdx])
	closeIdx := strings.LastIndex(s, ")")
	if closeIdx < 0 {
		closeIdx = len(s)
	}

	argsStr := s[parenIdx+1 : closeIdx]
	parts := strings.Split(argsStr, ",")

	var args []Arg
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.HasPrefix(part, "\"") && strings.HasSuffix(part, "\"") {
			args = append(args, Arg{StrVal: part[1 : len(part)-1], IsStr: true})
		} else if n, err := strconv.Atoi(part); err == nil {
			args = append(args, Arg{IntVal: n})
		} else {
			// Try to handle quoted strings that span commas
			args = append(args, Arg{StrVal: part, IsStr: true})
		}
	}

	return name, args
}

func extractQuotedString(line string) string {
	first := strings.IndexByte(line, '"')
	if first < 0 {
		// No quotes — return the value after the keyword
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			return parts[1]
		}
		return ""
	}
	last := strings.LastIndexByte(line, '"')
	if last <= first {
		return ""
	}
	return line[first+1 : last]
}

// GetState returns a state by name, or nil.
func (q *Quest) GetState(name string) *State {
	return q.States[name]
}
