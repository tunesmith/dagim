package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"dagim/internal/graph"
)

func TestRewriteRegeneratesIDsAndSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gumbo.dagim")
	g := graph.New()
	must(t, g.AddNodeWithID("in-gumbo-pot-combine-1-2-cup-butter-and-1-2-cup-white-rice-flour", "In gumbo pot, combine 1/2 cup butter and 1/2 cup white rice flour"))
	must(t, g.AddNodeWithID("cook-over-medium-heat-frequently-stirring-to-make-a-dark-brown-roux-about-15-minutes", "Cook over medium heat, frequently stirring to make a dark brown roux, about 15 minutes"))
	must(t, g.AddEdge(
		"in-gumbo-pot-combine-1-2-cup-butter-and-1-2-cup-white-rice-flour",
		"cook-over-medium-heat-frequently-stirring-to-make-a-dark-brown-roux-about-15-minutes",
	))

	m := New(path, g)
	m.current = "cook-over-medium-heat-frequently-stirring-to-make-a-dark-brown-roux-about-15-minutes"
	m.dirty = true
	m = m.rewrite()

	if m.dirty {
		t.Fatal("rewrite should save and clear dirty state")
	}
	if m.current != "cook-medium-heat-stirring-dark-brown-roux" {
		t.Fatalf("current = %q", m.current)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "in-gumbo-pot-combine-1-2-cup-butter") {
		t.Fatalf("old long ID remains:\n%s", text)
	}
	if !strings.Contains(text, "parent gumbo-pot-combine-butter-white-rice-flour") {
		t.Fatalf("parent reference was not rewritten:\n%s", text)
	}
}

func TestNodeViewWrapsTextAndHidesIDs(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("parent-node-id", "Parent node"))
	must(t, g.AddNodeWithID("current-node-id", "Current node"))
	must(t, g.AddNodeWithID("long-child-id", "This child node has enough words to wrap in a narrow terminal display"))
	must(t, g.AddEdge("parent-node-id", "current-node-id"))
	must(t, g.AddEdge("current-node-id", "long-child-id"))

	m := New("test.dagim", g)
	m.current = "current-node-id"
	m.width = 34
	view := m.viewNode()

	for _, hidden := range []string{"parent-node-id", "current-node-id", "long-child-id"} {
		if strings.Contains(view, hidden) {
			t.Fatalf("main node view exposed ID %q:\n%s", hidden, view)
		}
	}
	if !strings.Contains(view, "This child node has enough") || !strings.Contains(view, "  words to wrap") {
		t.Fatalf("expected wrapped child text:\n%s", view)
	}
}

func TestInspectShowsIDs(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("current-node-id", "Current node"))
	m := New("test.dagim", g)
	m.inspectID = "current-node-id"
	m.mode = modeInspect
	view := m.View()

	if !strings.Contains(view, "current-node-id") {
		t.Fatalf("inspect should expose ID:\n%s", view)
	}
}

func TestInspectReturnsToPreviousMode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("current-node-id", "Current node"))
	m := New("test.dagim", g)
	m.mode = modeInspect
	m.previous = modeNode

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)
	if updated.mode != modeNode {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestCtrlCQuitsFromAnyMode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))

	for _, tc := range []struct {
		name string
		m    Model
	}{
		{name: "node", m: func() Model {
			m := New("test.dagim", g)
			m.mode = modeNode
			return m
		}()},
		{name: "roots", m: New("test.dagim", g)},
		{name: "sequence", m: func() Model {
			m := New("test.dagim", g)
			m.mode = modeSequence
			m.seq = graph.NewSequence(g)
			m.seqReturn = modeRoots
			return m
		}()},
		{name: "prompt", m: func() Model {
			m := New("test.dagim", g)
			m.mode = modePrompt
			return m
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, cmd := tc.m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			if cmd == nil {
				t.Fatal("cmd is nil")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("expected QuitMsg")
			}
		})
	}
}

func TestCtrlZSuspendsFromAnyMode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New("test.dagim", g)
	m.mode = modeSequence
	m.seq = graph.NewSequence(g)
	m.seqReturn = modeRoots

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	if _, ok := cmd().(tea.SuspendMsg); !ok {
		t.Fatalf("expected SuspendMsg")
	}
}

func TestNewStartsOnRootsForNonEmptyGraph(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("child-node", "Child node"))
	must(t, g.AddEdge("root-node", "child-node"))

	m := New("test.dagim", g)

	if m.mode != modeRoots {
		t.Fatalf("mode = %v", m.mode)
	}
	if m.current != "root-node" {
		t.Fatalf("current = %q", m.current)
	}
	if view := m.View(); !strings.Contains(view, "Roots") || !strings.Contains(view, "Root node") {
		t.Fatalf("expected roots view on startup:\n%s", view)
	}
}

func TestNewStartsEmptyGraphInNodeMode(t *testing.T) {
	m := New("test.dagim", graph.New())

	if m.mode != modeNode {
		t.Fatalf("mode = %v", m.mode)
	}
	if view := m.View(); !strings.Contains(view, "No nodes yet") {
		t.Fatalf("expected empty node state:\n%s", view)
	}
}

func TestRootsCanOpenAddNodePrompt(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New("test.dagim", g)

	next, _ := m.Update(runeKey('a'))
	updated := next.(Model)

	if updated.mode != modePrompt {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.previous != modeRoots {
		t.Fatalf("previous = %v", updated.previous)
	}
	if updated.promptAction != promptAddNode {
		t.Fatalf("promptAction = %v", updated.promptAction)
	}
	if updated.promptTitle != "Add node" {
		t.Fatalf("promptTitle = %q", updated.promptTitle)
	}
}

func TestRootsCanStartSequence(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New("test.dagim", g)

	next, _ := m.Update(runeKey('m'))
	updated := next.(Model)

	if updated.mode != modeSequence {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.previous != modeRoots {
		t.Fatalf("previous = %v", updated.previous)
	}
	if updated.seqReturn != modeRoots {
		t.Fatalf("seqReturn = %v", updated.seqReturn)
	}
	if updated.seq == nil {
		t.Fatal("seq is nil")
	}
}

func TestSequenceEscReturnsToLaunchingMode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New("test.dagim", g)

	next, _ := m.Update(runeKey('m'))
	sequencing := next.(Model)
	next, _ = sequencing.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)

	if updated.mode != modeRoots {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestSequenceInspectDoesNotLoseRootsReturnMode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New("test.dagim", g)

	next, _ := m.Update(runeKey('m'))
	sequencing := next.(Model)
	next, _ = sequencing.Update(tea.KeyMsg{Type: tea.KeyEnter})
	inspecting := next.(Model)
	if inspecting.mode != modeInspect {
		t.Fatalf("mode = %v", inspecting.mode)
	}

	next, _ = inspecting.Update(tea.KeyMsg{Type: tea.KeyEsc})
	sequencing = next.(Model)
	if sequencing.mode != modeSequence {
		t.Fatalf("mode = %v", sequencing.mode)
	}

	next, _ = sequencing.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)
	if updated.mode != modeRoots {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestSequenceQReturnsToLaunchingMode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New("test.dagim", g)

	next, _ := m.Update(runeKey('m'))
	sequencing := next.(Model)
	next, _ = sequencing.Update(runeKey('q'))
	updated := next.(Model)

	if updated.mode != modeRoots {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestNodeSequenceEscReturnsToNode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New("test.dagim", g)
	m.mode = modeNode

	next, _ := m.Update(runeKey('m'))
	sequencing := next.(Model)
	next, _ = sequencing.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)

	if updated.mode != modeNode {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestRootsEscDoesNotEnterNodeView(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("first-root", "First root"))
	must(t, g.AddNodeWithID("second-root", "Second root"))
	m := New("test.dagim", g)
	m.rootsCursor = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)

	if updated.mode != modeRoots {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.rootsCursor != 1 {
		t.Fatalf("rootsCursor = %d", updated.rootsCursor)
	}
	if updated.current != "first-root" {
		t.Fatalf("current = %q", updated.current)
	}
}

func TestRootsSearchEscReturnsToRoots(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New("test.dagim", g)

	next, _ := m.Update(runeKey('/'))
	searching := next.(Model)
	if searching.mode != modeSearch {
		t.Fatalf("mode = %v", searching.mode)
	}

	next, _ = searching.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)
	if updated.mode != modeRoots {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestRootsHelpEscReturnsToRoots(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New("test.dagim", g)

	next, _ := m.Update(runeKey('?'))
	helping := next.(Model)
	if helping.mode != modeHelp {
		t.Fatalf("mode = %v", helping.mode)
	}

	next, _ = helping.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)
	if updated.mode != modeRoots {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestRootsQuitCancelReturnsToRoots(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New("test.dagim", g)
	m.dirty = true

	next, _ := m.Update(runeKey('q'))
	confirming := next.(Model)
	if confirming.mode != modeConfirmQuit {
		t.Fatalf("mode = %v", confirming.mode)
	}

	next, _ = confirming.Update(runeKey('c'))
	updated := next.(Model)
	if updated.mode != modeRoots {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestNodeCanOpenLeavesView(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("leaf-node", "Leaf node"))
	must(t, g.AddEdge("root-node", "leaf-node"))
	m := New("test.dagim", g)
	m.mode = modeNode
	m.current = "root-node"

	next, _ := m.Update(runeKey('l'))
	updated := next.(Model)

	if updated.mode != modeLeaves {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.previous != modeNode {
		t.Fatalf("previous = %v", updated.previous)
	}
	if updated.leavesReturn != modeNode {
		t.Fatalf("leavesReturn = %v", updated.leavesReturn)
	}
	view := updated.View()
	if !strings.Contains(view, "Leaves") || !strings.Contains(view, "Leaf node") {
		t.Fatalf("expected leaves view:\n%s", view)
	}
	if strings.Contains(view, "Root node") {
		t.Fatalf("non-leaf was shown:\n%s", view)
	}
}

func TestRootsCanOpenLeavesView(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("leaf-node", "Leaf node"))
	must(t, g.AddEdge("root-node", "leaf-node"))
	m := New("test.dagim", g)

	next, _ := m.Update(runeKey('l'))
	updated := next.(Model)

	if updated.mode != modeLeaves {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.previous != modeRoots {
		t.Fatalf("previous = %v", updated.previous)
	}
	if updated.leavesReturn != modeRoots {
		t.Fatalf("leavesReturn = %v", updated.leavesReturn)
	}
}

func TestLeavesEnterFocusesSelectedLeaf(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("first-leaf", "First leaf"))
	must(t, g.AddNodeWithID("second-leaf", "Second leaf"))
	must(t, g.AddEdge("root-node", "first-leaf"))
	must(t, g.AddEdge("root-node", "second-leaf"))
	m := New("test.dagim", g)
	m.mode = modeLeaves
	m.leavesCursor = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(Model)

	if updated.mode != modeNode {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.current != "second-leaf" {
		t.Fatalf("current = %q", updated.current)
	}
}

func TestLeavesEnterClampsStaleCursor(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("leaf-node", "Leaf node"))
	must(t, g.AddEdge("root-node", "leaf-node"))
	m := New("test.dagim", g)
	m.mode = modeLeaves
	m.leavesCursor = 10

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(Model)

	if updated.mode != modeNode {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.current != "leaf-node" {
		t.Fatalf("current = %q", updated.current)
	}
}

func TestLeavesEscReturnsToLaunchingMode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("leaf-node", "Leaf node"))
	must(t, g.AddEdge("root-node", "leaf-node"))
	m := New("test.dagim", g)
	m.mode = modeNode
	m.current = "root-node"

	next, _ := m.Update(runeKey('l'))
	leaves := next.(Model)
	next, _ = leaves.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)

	if updated.mode != modeNode {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.current != "root-node" {
		t.Fatalf("current = %q", updated.current)
	}
}

func TestLeavesInspectDoesNotLoseLaunchingMode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("leaf-node", "Leaf node"))
	must(t, g.AddEdge("root-node", "leaf-node"))
	m := New("test.dagim", g)
	m.mode = modeNode
	m.current = "root-node"

	next, _ := m.Update(runeKey('l'))
	leaves := next.(Model)
	next, _ = leaves.Update(runeKey('i'))
	inspecting := next.(Model)
	if inspecting.mode != modeInspect {
		t.Fatalf("mode = %v", inspecting.mode)
	}

	next, _ = inspecting.Update(tea.KeyMsg{Type: tea.KeyEsc})
	leaves = next.(Model)
	if leaves.mode != modeLeaves {
		t.Fatalf("mode = %v", leaves.mode)
	}

	next, _ = leaves.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)
	if updated.mode != modeNode {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestLeavesSearchEscReturnsToLeaves(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("leaf-node", "Leaf node"))
	must(t, g.AddEdge("root-node", "leaf-node"))
	m := New("test.dagim", g)
	m.mode = modeLeaves

	next, _ := m.Update(runeKey('/'))
	searching := next.(Model)
	if searching.mode != modeSearch {
		t.Fatalf("mode = %v", searching.mode)
	}

	next, _ = searching.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)
	if updated.mode != modeLeaves {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestLinkPromptHidesDuplicateEdgeCandidates(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("parent", "Existing parent"))
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddNodeWithID("child", "Existing child"))
	must(t, g.AddNodeWithID("available", "Available"))
	must(t, g.AddEdge("parent", "current"))
	must(t, g.AddEdge("current", "child"))

	m := New("test.dagim", g)
	m.current = "current"

	m.promptAction = promptAddParent
	parentCandidates := m.linkCandidates("", promptAddParent)
	if containsID(parentCandidates, "parent") {
		t.Fatalf("already-linked parent was shown: %#v", parentCandidates)
	}
	if containsID(parentCandidates, "current") {
		t.Fatalf("current node was shown as parent candidate: %#v", parentCandidates)
	}
	if !containsID(parentCandidates, "available") {
		t.Fatalf("available node missing from parent candidates: %#v", parentCandidates)
	}

	m.promptAction = promptAddChild
	childCandidates := m.linkCandidates("", promptAddChild)
	if containsID(childCandidates, "child") {
		t.Fatalf("already-linked child was shown: %#v", childCandidates)
	}
	if containsID(childCandidates, "current") {
		t.Fatalf("current node was shown as child candidate: %#v", childCandidates)
	}
	if !containsID(childCandidates, "available") {
		t.Fatalf("available node missing from child candidates: %#v", childCandidates)
	}
}

func TestLinkPromptShowsNoEligibleMatchesMessage(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("parent", "Existing parent"))
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddEdge("parent", "current"))

	m := New("test.dagim", g)
	m.current = "current"
	m.promptAction = promptAddParent
	m.promptTitle = "Add/link parent"
	m.input.SetValue("Existing")

	view := m.viewPrompt()
	if !strings.Contains(view, "no eligible matches") {
		t.Fatalf("expected no eligible matches message:\n%s", view)
	}
	if strings.Contains(view, "Existing parent") {
		t.Fatalf("already-linked parent was shown:\n%s", view)
	}
}

func containsID(ids []graph.NodeID, want graph.NodeID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
