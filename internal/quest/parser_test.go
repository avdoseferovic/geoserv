package quest

import "testing"

func TestParse_BasicQuest(t *testing.T) {
	t.Parallel()
	input := `
main
{
  questname "Test Quest"
  version 1
}

state Begin
{
  desc "Starting state"
  action AddNpcText(1, "Hello!")
  rule TalkedToNpc(1) goto Next
}

state Next
{
  desc "Second state"
  rule Always() goto Done
}
`
	q, err := Parse(42, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if q.ID != 42 {
		t.Errorf("ID = %d, want 42", q.ID)
	}
	if q.Name != "Test Quest" {
		t.Errorf("Name = %q, want %q", q.Name, "Test Quest")
	}
	if q.Version != 1 {
		t.Errorf("Version = %d, want 1", q.Version)
	}
	if len(q.States) != 2 {
		t.Fatalf("len(States) = %d, want 2", len(q.States))
	}

	begin := q.GetState("Begin")
	if begin == nil {
		t.Fatal("state Begin not found")
	}
	if begin.Description != "Starting state" {
		t.Errorf("Begin.Description = %q, want %q", begin.Description, "Starting state")
	}
	if len(begin.Actions) != 1 {
		t.Errorf("len(Begin.Actions) = %d, want 1", len(begin.Actions))
	}
	if len(begin.Rules) != 1 {
		t.Fatalf("len(Begin.Rules) = %d, want 1", len(begin.Rules))
	}
	if begin.Rules[0].Name != "TalkedToNpc" {
		t.Errorf("rule name = %q, want %q", begin.Rules[0].Name, "TalkedToNpc")
	}
	if begin.Rules[0].Goto != "Next" {
		t.Errorf("rule goto = %q, want %q", begin.Rules[0].Goto, "Next")
	}

	next := q.GetState("Next")
	if next == nil {
		t.Fatal("state Next not found")
	}
	if len(next.Rules) != 1 || next.Rules[0].Goto != "Done" {
		t.Errorf("Next state rules incorrect")
	}
}

func TestParse_CaseInsensitive(t *testing.T) {
	t.Parallel()
	input := `
MAIN
{
  QUESTNAME "CI Quest"
  VERSION 2
}

STATE TestState
{
  DESC "test"
  ACTION AddNpcText(1, "hi")
  RULE Always() goto End
}
`
	q, err := Parse(1, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if q.Name != "CI Quest" {
		t.Errorf("Name = %q, want %q", q.Name, "CI Quest")
	}
	if q.Version != 2 {
		t.Errorf("Version = %d, want 2", q.Version)
	}
	if q.GetState("TestState") == nil {
		t.Error("state TestState not found")
	}
}

func TestParse_EmptyInput(t *testing.T) {
	t.Parallel()
	q, err := Parse(1, "")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(q.States) != 0 {
		t.Errorf("expected 0 states, got %d", len(q.States))
	}
}

func TestParse_CommentsSkipped(t *testing.T) {
	t.Parallel()
	input := `
// This is a comment
main
{
  questname "Commented"
  // version 99
}
`
	q, err := Parse(1, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if q.Name != "Commented" {
		t.Errorf("Name = %q, want %q", q.Name, "Commented")
	}
	if q.Version != 0 {
		t.Errorf("Version = %d, want 0 (commented out)", q.Version)
	}
}

func TestParse_ActionArgs(t *testing.T) {
	t.Parallel()
	input := `
state Begin
{
  action AddNpcText(123, "Hello world")
  action GiveItem(456, 10)
}
`
	q, err := Parse(1, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	begin := q.GetState("Begin")
	if begin == nil {
		t.Fatal("state Begin not found")
	}
	if len(begin.Actions) != 2 {
		t.Fatalf("len(Actions) = %d, want 2", len(begin.Actions))
	}

	// First action: AddNpcText(123, "Hello world")
	a0 := begin.Actions[0]
	if a0.Name != "AddNpcText" {
		t.Errorf("action[0].Name = %q, want %q", a0.Name, "AddNpcText")
	}
	if len(a0.Args) != 2 {
		t.Fatalf("action[0] has %d args, want 2", len(a0.Args))
	}
	if a0.Args[0].IntVal != 123 || a0.Args[0].IsStr {
		t.Errorf("action[0].Args[0] = %+v, want int 123", a0.Args[0])
	}
	if a0.Args[1].StrVal != "Hello world" || !a0.Args[1].IsStr {
		t.Errorf("action[0].Args[1] = %+v, want string 'Hello world'", a0.Args[1])
	}

	// Second action: GiveItem(456, 10)
	a1 := begin.Actions[1]
	if a1.Args[0].IntVal != 456 || a1.Args[1].IntVal != 10 {
		t.Errorf("GiveItem args = (%d, %d), want (456, 10)", a1.Args[0].IntVal, a1.Args[1].IntVal)
	}
}

func TestParse_RuleWithGoto(t *testing.T) {
	t.Parallel()
	input := `
state Begin
{
  rule InputNpc(2) goto AfterChoice
  rule KilledNpcs(5, 10) goto Victory
}
`
	q, err := Parse(1, input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	begin := q.GetState("Begin")
	if len(begin.Rules) != 2 {
		t.Fatalf("len(Rules) = %d, want 2", len(begin.Rules))
	}

	r0 := begin.Rules[0]
	if r0.Name != "InputNpc" || r0.Goto != "AfterChoice" {
		t.Errorf("rule[0] = {%q, goto %q}, want {InputNpc, AfterChoice}", r0.Name, r0.Goto)
	}
	if len(r0.Args) != 1 || r0.Args[0].IntVal != 2 {
		t.Errorf("rule[0].Args = %+v, want int 2", r0.Args)
	}

	r1 := begin.Rules[1]
	if r1.Name != "KilledNpcs" || r1.Goto != "Victory" {
		t.Errorf("rule[1] = {%q, goto %q}", r1.Name, r1.Goto)
	}
}

