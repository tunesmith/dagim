// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

func newTestModel(t *testing.T, g *graph.Graph) Model {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "test.dagim"), g)
}

func TestNodeViewWrapsTextAndHidesIDs(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("parent-node-id", "Parent node"))
	must(t, g.AddNodeWithID("current-node-id", "Current node"))
	must(t, g.AddNodeWithID("long-child-id", "This child node has enough words to wrap in a narrow terminal display"))
	must(t, g.AddEdge("parent-node-id", "current-node-id"))
	must(t, g.AddEdge("current-node-id", "long-child-id"))

	m := newTestModel(t, g)
	m.current = "current-node-id"
	m.width = 34
	view := m.viewNode()

	for _, hidden := range []string{"parent-node-id", "current-node-id", "long-child-id"} {
		if strings.Contains(view, hidden) {
			t.Fatalf("main node view exposed ID %q:\n%s", hidden, view)
		}
	}
	if !strings.Contains(view, "This child node has enough") || !strings.Contains(view, "\n    words to wrap") {
		t.Fatalf("expected wrapped child text:\n%s", view)
	}
}

func TestWideViewCapsRulesAndAddsGutter(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))

	m := newTestModel(t, g)
	m.mode = modeNode
	m.current = "root-node"
	m.width = 180
	view := m.View()

	if !strings.HasPrefix(view, "    ") {
		t.Fatalf("expected wide view to start with a gutter:\n%s", view)
	}
	if strings.Contains(view, strings.Repeat("-", maxContentWidth+1)) {
		t.Fatalf("rule exceeded content cap:\n%s", view)
	}
	if strings.Contains(view, strings.Repeat("=", maxContentWidth+1)) {
		t.Fatalf("strong rule exceeded content cap:\n%s", view)
	}
	if !strings.Contains(view, strings.Repeat("-", maxContentWidth)) {
		t.Fatalf("expected capped rule width:\n%s", view)
	}
}

func TestViewFitsTerminalDimensions(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("current", "Current"))
	for i := 1; i <= 20; i++ {
		id := graph.NodeID(fmt.Sprintf("child-%02d", i))
		text := fmt.Sprintf("Child item %02d with enough words to wrap in the viewport", i)
		must(t, g.AddNodeWithID(id, text))
		must(t, g.AddEdge("current", id))
	}

	m := newTestModel(t, g)
	m.mode = modeNode
	m.current = "current"
	m.height = 12
	m.width = 44

	view := m.View()
	assertViewFits(t, view, m.width, m.height)
	if !strings.Contains(view, "PgUp/PgDn scroll") {
		t.Fatalf("overflowing free-form view should show scroll controls:\n%s", view)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	scrolled := next.(Model)
	if scrolled.viewScroll == 0 {
		t.Fatal("expected global viewport to scroll")
	}
	assertViewFits(t, scrolled.View(), scrolled.width, scrolled.height)
}

func assertViewFits(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if height > 0 && len(lines) > height {
		t.Fatalf("view has %d lines, height %d:\n%s", len(lines), height, view)
	}
	if width <= 0 {
		return
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line width = %d, want <= %d:\n%s", got, width, view)
		}
	}
}

func TestCheckViewReportsStatsAndTransitiveEdges(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("a", "A"))
	must(t, g.AddNodeWithID("b", "B"))
	must(t, g.AddNodeWithID("c", "C"))
	must(t, g.AddNodeWithID("d", "D"))
	must(t, g.AddEdge("a", "b"))
	must(t, g.AddEdge("b", "c"))
	must(t, g.AddEdge("c", "d"))
	must(t, g.AddEdge("a", "d"))
	m := newTestModel(t, g)
	m.mode = modeCheck

	view := m.View()
	for _, want := range []string{
		"Check",
		"valid: yes",
		"W rewrite ID changes: 0",
		"nodes: 4",
		"edges: 4",
		"transitive edges: 1",
		"1. A",
		"-> D",
		"via alternate path (3 edges)",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
}

func TestCheckViewReportsRewriteIDChanges(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("old-id", "Better node text"))
	must(t, g.AddNodeWithID("other-old-id", "Other better node text"))
	m := newTestModel(t, g)
	m.mode = modeCheck

	view := m.View()
	for _, want := range []string{
		"W rewrite ID changes: 2",
		"W rewrite ID changes",
		"old-id",
		"-> better-node-text",
		"other-old-id",
		"-> other-better-node-text",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
	if !g.HasNode("old-id") {
		t.Fatal("check view mutated graph")
	}
}

func TestCheckViewScrollsWithinTerminalHeight(t *testing.T) {
	g := graph.New()
	for i := 1; i <= 12; i++ {
		id := graph.NodeID(fmt.Sprintf("old-%02d", i))
		text := fmt.Sprintf("Better node text %02d", i)
		must(t, g.AddNodeWithID(id, text))
	}
	m := newTestModel(t, g)
	m.mode = modeCheck
	m.height = 10
	m.width = 50

	view := m.View()
	if !strings.Contains(view, "Check") {
		t.Fatalf("initial check view lost title:\n%s", view)
	}
	if !strings.Contains(view, "PgUp/PgDn") {
		t.Fatalf("overflowing check view should show scroll controls:\n%s", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > m.height {
		t.Fatalf("view has %d lines, height %d:\n%s", lines, m.height, view)
	}

	var next tea.Model = m
	for i := 0; i < 6; i++ {
		next, _ = next.Update(runeKey('j'))
	}
	scrolled := next.(Model)
	if scrolled.checkScroll == 0 {
		t.Fatal("expected check view to scroll")
	}
	view = scrolled.View()
	if lines := strings.Count(view, "\n") + 1; lines > scrolled.height {
		t.Fatalf("scrolled view has %d lines, height %d:\n%s", lines, scrolled.height, view)
	}
}

func TestReadyCanOpenCheckView(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('C'))
	checking := next.(Model)
	if checking.mode != modeCheck {
		t.Fatalf("mode = %v", checking.mode)
	}
	if checking.previous != modeReady {
		t.Fatalf("previous = %v", checking.previous)
	}

	next, _ = checking.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)
	if updated.mode != modeReady {
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
			m := newTestModel(t, g)
			m.mode = modeNode
			return m
		}()},
		{name: "ready", m: newTestModel(t, g)},
		{name: "order", m: func() Model {
			m := newTestModel(t, g)
			m.mode = modeOrder
			m.order = graph.NewOrder(g)
			m.orderReturn = modeReady
			return m
		}()},
		{name: "prompt", m: func() Model {
			m := newTestModel(t, g)
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
	m := newTestModel(t, g)
	m.mode = modeOrder
	m.order = graph.NewOrder(g)
	m.orderReturn = modeReady

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	if _, ok := cmd().(tea.SuspendMsg); !ok {
		t.Fatalf("expected SuspendMsg")
	}
}

func TestNewStartsOnReadyForNonEmptyGraph(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("child-node", "Child node"))
	must(t, g.AddEdge("root-node", "child-node"))

	m := newTestModel(t, g)

	if m.mode != modeReady {
		t.Fatalf("mode = %v", m.mode)
	}
	if m.current != "root-node" {
		t.Fatalf("current = %q", m.current)
	}
	if view := m.View(); !strings.Contains(view, "Ready") || !strings.Contains(view, "Root node") {
		t.Fatalf("expected ready view on startup:\n%s", view)
	}
}

func TestNewStartsEmptyGraphInNodeMode(t *testing.T) {
	m := newTestModel(t, graph.New())

	if m.mode != modeNode {
		t.Fatalf("mode = %v", m.mode)
	}
	if view := m.View(); !strings.Contains(view, "No nodes yet") {
		t.Fatalf("expected empty node state:\n%s", view)
	}
}

func TestNodeCanOpenEditPrompt(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)
	m.mode = modeNode

	next, _ := m.Update(runeKey('e'))
	updated := next.(Model)

	if updated.mode != modePrompt {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.promptAction != promptEdit {
		t.Fatalf("promptAction = %v", updated.promptAction)
	}
	if updated.promptTitle != "Edit node" {
		t.Fatalf("promptTitle = %q", updated.promptTitle)
	}
	if updated.input.Value() != "Root node" {
		t.Fatalf("input = %q", updated.input.Value())
	}
}

func TestNodeCanOpenReadyViewWithR(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)
	m.mode = modeNode

	next, _ := m.Update(runeKey('r'))
	updated := next.(Model)

	if updated.mode != modeReady {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestNodeCanReturnToReadyWithoutLosingReadyCursor(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("first-root", "First root"))
	must(t, g.AddNodeWithID("second-root", "Second root"))
	must(t, g.AddNodeWithID("third-root", "Third root"))
	m := newTestModel(t, g)
	m.readyCursor = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	focused := next.(Model)
	if focused.mode != modeNode {
		t.Fatalf("mode after enter = %v", focused.mode)
	}
	if focused.current != "second-root" {
		t.Fatalf("current after enter = %q", focused.current)
	}

	next, _ = focused.Update(runeKey('r'))
	updated := next.(Model)
	if updated.mode != modeReady {
		t.Fatalf("mode after r = %v", updated.mode)
	}
	if updated.readyCursor != 1 {
		t.Fatalf("readyCursor after r = %d", updated.readyCursor)
	}
}

func TestNodeReordersSelectedChildWithinChildren(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddNodeWithID("first-child", "First child"))
	must(t, g.AddNodeWithID("unrelated", "Unrelated"))
	must(t, g.AddNodeWithID("second-child", "Second child"))
	must(t, g.AddEdge("current", "first-child"))
	must(t, g.AddEdge("current", "second-child"))
	m := newTestModel(t, g)
	m.mode = modeNode
	m.current = "current"

	next, _ := m.Update(runeKey('J'))
	updated := next.(Model)
	children, _ := updated.g.ChildrenOf("current")
	if !reflect.DeepEqual(children, []graph.NodeID{"second-child", "first-child"}) {
		t.Fatalf("children after J = %#v", children)
	}
	if updated.cursor != 1 {
		t.Fatalf("cursor after J = %d", updated.cursor)
	}
	if updated.dirty {
		t.Fatal("reorder should autosave and clear dirty state")
	}

	next, _ = updated.Update(runeKey('K'))
	updated = next.(Model)
	children, _ = updated.g.ChildrenOf("current")
	if !reflect.DeepEqual(children, []graph.NodeID{"first-child", "second-child"}) {
		t.Fatalf("children after K = %#v", children)
	}
	if updated.cursor != 0 {
		t.Fatalf("cursor after K = %d", updated.cursor)
	}
}

func TestNodeReorderDoesNothingWithoutVisibleRelation(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddNodeWithID("other", "Other"))
	m := newTestModel(t, g)
	m.mode = modeNode
	m.current = "current"
	before := g.Order()

	next, _ := m.Update(runeKey('J'))
	updated := next.(Model)

	if !reflect.DeepEqual(updated.g.Order(), before) {
		t.Fatalf("order changed without a visible relation: %#v", updated.g.Order())
	}
	if updated.dirty {
		t.Fatal("invisible no-op reorder should not dirty the graph")
	}
}

func TestCommandGridReflowsToFitWidth(t *testing.T) {
	rows := [][]string{
		{"a add node", "p add parent", "c add child", "x unlink selected"},
		{"e edit", "d delete node", "/ search", "r ready"},
		{"l leaves", "o order", "C check", "u undo"},
		{"Space done/undone", "v completed", "R reset", "W rewrite"},
		{"J/K reorder", "q quit", "? help"},
	}

	view := renderCommandGrid(rows, 36)
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > 36 {
			t.Fatalf("line width = %d, want <= 36:\n%s", got, view)
		}
	}
	if !strings.Contains(view, "x unlink selected") {
		t.Fatalf("expected full command label:\n%s", view)
	}
}

func TestEditAutosaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.dagim")
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New(path, g)
	m.mode = modeNode
	m.current = "root-node"

	if _, err := os.Stat(path); err == nil {
		t.Fatal("file existed before autosave")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}

	next, _ := m.Update(runeKey('e'))
	prompt := next.(Model)
	prompt.input.SetValue("Renamed root")
	next, _ = prompt.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(Model)
	if updated.dirty {
		t.Fatal("edit should autosave and clear dirty state")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Renamed root") {
		t.Fatalf("autosaved file missing edited node text:\n%s", data)
	}
}

func TestUndoRestoresEditAndAutosaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.dagim")
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := New(path, g)
	m.mode = modeNode
	m.current = "root-node"

	next, _ := m.Update(runeKey('e'))
	prompt := next.(Model)
	prompt.input.SetValue("Renamed root")
	next, _ = prompt.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(Model)

	next, _ = updated.Update(runeKey('u'))
	undone := next.(Model)

	node, _ := undone.g.Node("root-node")
	if node.Text != "Root node" {
		t.Fatalf("node text = %q", node.Text)
	}
	if undone.message != "undid last change" {
		t.Fatalf("message = %q", undone.message)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "Root node") || strings.Contains(text, "Renamed root") {
		t.Fatalf("undo was not autosaved:\n%s", text)
	}
}

func TestUndoRestoresDeletedNodeAndEdges(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddNodeWithID("child", "Child"))
	must(t, g.AddEdge("current", "child"))
	m := newTestModel(t, g)
	m.mode = modeNode
	m.current = "current"

	next, _ := m.Update(runeKey('d'))
	confirming := next.(Model)
	next, _ = confirming.Update(runeKey('y'))
	deleted := next.(Model)
	if deleted.g.HasNode("current") {
		t.Fatal("delete did not remove node")
	}

	next, _ = deleted.Update(runeKey('u'))
	undone := next.(Model)
	if !undone.g.HasNode("current") {
		t.Fatal("undo did not restore deleted node")
	}
	children, err := undone.g.ChildrenOf("current")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(children, []graph.NodeID{"child"}) {
		t.Fatalf("children = %#v", children)
	}
	if undone.current != "current" {
		t.Fatalf("current = %q", undone.current)
	}
}

func TestUndoRestoresCompletionCascadeAsOneAction(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("a", "A"))
	must(t, g.AddNodeWithID("b", "B"))
	must(t, g.AddNodeWithID("c", "C"))
	must(t, g.AddEdge("a", "b"))
	must(t, g.AddEdge("b", "c"))
	for _, id := range []graph.NodeID{"a", "b", "c"} {
		must(t, g.MarkComplete(id))
	}
	m := newTestModel(t, g)
	m.mode = modeNode
	m.current = "b"

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	updated := next.(Model)
	b, _ := updated.g.Node("b")
	c, _ := updated.g.Node("c")
	if b.Complete || c.Complete {
		t.Fatalf("cascade did not mark descendants undone: b=%v c=%v", b.Complete, c.Complete)
	}

	next, _ = updated.Update(runeKey('u'))
	undone := next.(Model)
	for _, id := range []graph.NodeID{"a", "b", "c"} {
		node, _ := undone.g.Node(id)
		if !node.Complete {
			t.Fatalf("%s was not restored complete", id)
		}
	}
}

func TestUndoWithEmptyStackShowsMessage(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('u'))
	updated := next.(Model)

	if updated.message != "nothing to undo" {
		t.Fatalf("message = %q", updated.message)
	}
	if !updated.g.HasNode("root-node") {
		t.Fatal("empty undo changed graph")
	}
}

func TestReadyReordersHighlightedNode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("first", "First"))
	must(t, g.AddNodeWithID("second", "Second"))
	must(t, g.AddNodeWithID("third", "Third"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('J'))
	updated := next.(Model)

	if got := updated.g.Ready(); !reflect.DeepEqual(got, []graph.NodeID{"second", "first", "third"}) {
		t.Fatalf("ready after J = %#v", got)
	}
	if updated.readyCursor != 1 {
		t.Fatalf("readyCursor after J = %d", updated.readyCursor)
	}
	if updated.dirty {
		t.Fatal("ready reorder should autosave and clear dirty state")
	}

	next, _ = updated.Update(runeKey('K'))
	updated = next.(Model)
	if got := updated.g.Ready(); !reflect.DeepEqual(got, []graph.NodeID{"first", "second", "third"}) {
		t.Fatalf("ready after K = %#v", got)
	}
	if updated.readyCursor != 0 {
		t.Fatalf("readyCursor after K = %d", updated.readyCursor)
	}
}

func TestReadyMarksCompleteAndAdvances(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("a", "A"))
	must(t, g.AddNodeWithID("b", "B"))
	must(t, g.AddNodeWithID("c", "C"))
	must(t, g.AddEdge("a", "c"))
	must(t, g.AddEdge("b", "c"))
	m := newTestModel(t, g)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	updated := next.(Model)
	node, _ := updated.g.Node("a")
	if !node.Complete {
		t.Fatal("selected ready node was not marked complete")
	}
	if got := updated.g.Ready(); !reflect.DeepEqual(got, []graph.NodeID{"b"}) {
		t.Fatalf("ready = %#v", got)
	}

	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeySpace})
	updated = next.(Model)
	if got := updated.g.Ready(); !reflect.DeepEqual(got, []graph.NodeID{"c"}) {
		t.Fatalf("ready after completing b = %#v", got)
	}
}

func TestReadyCanShowCompletedAndMarkIncomplete(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("a", "A"))
	must(t, g.SetComplete("a", true))
	m := newTestModel(t, g)

	if view := m.View(); strings.Contains(view, "[x] A") {
		t.Fatalf("completed node shown before toggle:\n%s", view)
	}
	next, _ := m.Update(runeKey('v'))
	updated := next.(Model)
	if view := updated.View(); !strings.Contains(view, "[x] A") {
		t.Fatalf("completed node hidden after toggle:\n%s", view)
	}

	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeySpace})
	updated = next.(Model)
	node, _ := updated.g.Node("a")
	if node.Complete {
		t.Fatal("completed node was not marked incomplete")
	}
	if got := updated.g.Ready(); !reflect.DeepEqual(got, []graph.NodeID{"a"}) {
		t.Fatalf("ready = %#v", got)
	}
}

func TestReadyDeletesSelectedNodeWithConfirmation(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("a", "A"))
	must(t, g.AddNodeWithID("b", "B"))
	must(t, g.AddNodeWithID("c", "C"))
	must(t, g.AddEdge("b", "c"))
	m := newTestModel(t, g)
	m.readyCursor = 1

	next, _ := m.Update(runeKey('d'))
	confirming := next.(Model)
	if confirming.mode != modeConfirmDelete {
		t.Fatalf("mode = %v", confirming.mode)
	}
	if confirming.previous != modeReady {
		t.Fatalf("previous = %v", confirming.previous)
	}
	if confirming.current != "b" {
		t.Fatalf("delete target current = %q", confirming.current)
	}
	if view := confirming.viewConfirmDelete(); !strings.Contains(view, "B") {
		t.Fatalf("confirm view did not show selected node:\n%s", view)
	}

	next, _ = confirming.Update(runeKey('n'))
	cancelled := next.(Model)
	if cancelled.mode != modeReady {
		t.Fatalf("mode after cancel = %v", cancelled.mode)
	}
	if !cancelled.g.HasNode("b") {
		t.Fatal("cancel deleted node")
	}

	next, _ = cancelled.Update(runeKey('d'))
	confirming = next.(Model)
	next, _ = confirming.Update(runeKey('y'))
	updated := next.(Model)
	if updated.mode != modeReady {
		t.Fatalf("mode after delete = %v", updated.mode)
	}
	if updated.g.HasNode("b") {
		t.Fatal("selected ready node was not deleted")
	}
	if got := updated.g.Ready(); !reflect.DeepEqual(got, []graph.NodeID{"a", "c"}) {
		t.Fatalf("ready after delete = %#v", got)
	}
	if updated.dirty {
		t.Fatal("delete should autosave and clear dirty state")
	}
}

func TestReadyWindowsItemsToTerminalHeight(t *testing.T) {
	g := graph.New()
	for i := 1; i <= 25; i++ {
		id := graph.NodeID(fmt.Sprintf("ready-%02d", i))
		text := fmt.Sprintf("Ready item %02d", i)
		must(t, g.AddNodeWithID(id, text))
	}
	m := newTestModel(t, g)
	m.height = 12
	m.width = 48

	view := m.viewReady()
	if !strings.Contains(view, "Ready 1-5 of 25") {
		t.Fatalf("expected initial ready window:\n%s", view)
	}
	if strings.Contains(view, "Ready item 06") {
		t.Fatalf("rendered beyond visible ready window:\n%s", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > m.height {
		t.Fatalf("view has %d lines, height %d:\n%s", lines, m.height, view)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	paged := next.(Model)
	if paged.readyCursor != 4 {
		t.Fatalf("readyCursor after PgDown = %d", paged.readyCursor)
	}

	paged.readyCursor = 14
	view = paged.viewReady()
	if !strings.Contains(view, "Ready 13-17 of 25") {
		t.Fatalf("expected centered ready window:\n%s", view)
	}
	if !strings.Contains(view, "Ready item 15") {
		t.Fatalf("selected ready item not visible:\n%s", view)
	}
}

func TestReadyWrapsItemsWithContinuationIndent(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("long-ready", "This ready item has enough words to wrap onto another line cleanly"))
	m := newTestModel(t, g)
	m.height = 14
	m.width = 36

	view := m.viewReady()
	for _, want := range []string{
		"This ready item has enough",
		"\n    words to wrap onto another",
		"\n    line cleanly",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing wrapped text %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "This ready item has enough words...") {
		t.Fatalf("ready item was truncated:\n%s", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > m.height {
		t.Fatalf("view has %d lines, height %d:\n%s", lines, m.height, view)
	}
}

func TestReadyMarkIncompleteCascadesToCompletedDescendants(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("a", "A"))
	must(t, g.AddNodeWithID("b", "B"))
	must(t, g.AddNodeWithID("c", "C"))
	must(t, g.AddEdge("a", "b"))
	must(t, g.AddEdge("b", "c"))
	for _, id := range []graph.NodeID{"a", "b", "c"} {
		must(t, g.MarkComplete(id))
	}
	m := newTestModel(t, g)
	m.showCompleted = true
	m.readyCursor = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	updated := next.(Model)

	a, _ := updated.g.Node("a")
	b, _ := updated.g.Node("b")
	c, _ := updated.g.Node("c")
	if !a.Complete || b.Complete || c.Complete {
		t.Fatalf("complete states: a=%v b=%v c=%v", a.Complete, b.Complete, c.Complete)
	}
	if updated.message != "marked 2 nodes undone" {
		t.Fatalf("message = %q", updated.message)
	}
	if got := updated.g.Ready(); !reflect.DeepEqual(got, []graph.NodeID{"b"}) {
		t.Fatalf("ready = %#v", got)
	}
}

func TestNodeCannotMarkCompleteWithIncompleteParents(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("parent", "Parent"))
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddEdge("parent", "current"))
	m := newTestModel(t, g)
	m.mode = modeNode
	m.current = "current"

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	updated := next.(Model)

	node, _ := updated.g.Node("current")
	if node.Complete {
		t.Fatal("blocked node was marked complete")
	}
	if updated.dirty {
		t.Fatal("blocked completion should not dirty the graph")
	}
	if updated.message != "blocked by 1 undone parent" {
		t.Fatalf("message = %q", updated.message)
	}
}

func TestNodeDeleteConfirmationReturnsToNode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddNodeWithID("child", "Child"))
	must(t, g.AddEdge("current", "child"))
	m := newTestModel(t, g)
	m.mode = modeNode
	m.current = "current"

	next, _ := m.Update(runeKey('d'))
	confirming := next.(Model)
	if confirming.mode != modeConfirmDelete {
		t.Fatalf("mode = %v", confirming.mode)
	}
	if confirming.previous != modeNode {
		t.Fatalf("previous = %v", confirming.previous)
	}

	next, _ = confirming.Update(runeKey('n'))
	cancelled := next.(Model)
	if cancelled.mode != modeNode {
		t.Fatalf("mode after cancel = %v", cancelled.mode)
	}

	next, _ = cancelled.Update(runeKey('d'))
	confirming = next.(Model)
	next, _ = confirming.Update(runeKey('y'))
	updated := next.(Model)
	if updated.mode != modeNode {
		t.Fatalf("mode after delete = %v", updated.mode)
	}
	if updated.g.HasNode("current") {
		t.Fatal("current node was not deleted")
	}
}

func TestAddIncompleteParentToCompletedNodeCascadesUndone(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("new-parent", "New parent"))
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddNodeWithID("child", "Child"))
	must(t, g.AddEdge("current", "child"))
	must(t, g.MarkComplete("current"))
	must(t, g.MarkComplete("child"))
	m := newTestModel(t, g)
	m.current = "current"
	m.mode = modePrompt
	m.promptAction = promptAddParent
	m.input.SetValue("New parent")

	next, _ := m.submitPrompt(false)
	updated := next.(Model)

	parents, _ := updated.g.ParentsOf("current")
	if !reflect.DeepEqual(parents, []graph.NodeID{"new-parent"}) {
		t.Fatalf("parents = %#v", parents)
	}
	current, _ := updated.g.Node("current")
	child, _ := updated.g.Node("child")
	if current.Complete || child.Complete {
		t.Fatalf("complete states: current=%v child=%v", current.Complete, child.Complete)
	}
	if updated.message != "linked; marked 2 nodes undone" {
		t.Fatalf("message = %q", updated.message)
	}
	if err := updated.g.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNodeViewHidesCompletedRelationsUntilToggled(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("parent", "Parent"))
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddEdge("parent", "current"))
	must(t, g.SetComplete("parent", true))
	m := newTestModel(t, g)
	m.mode = modeNode
	m.current = "current"

	if view := m.viewNode(); strings.Contains(view, "  Parent") || strings.Contains(view, "> Parent") {
		t.Fatalf("completed parent shown before toggle:\n%s", view)
	}
	next, _ := m.Update(runeKey('v'))
	updated := next.(Model)
	if view := updated.viewNode(); !strings.Contains(view, "[x] Parent") {
		t.Fatalf("completed parent hidden after toggle:\n%s", view)
	}
}

func TestResetCompletionRequiresConfirm(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("a", "A"))
	must(t, g.SetComplete("a", true))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('R'))
	confirming := next.(Model)
	if confirming.mode != modeConfirmReset {
		t.Fatalf("mode = %v", confirming.mode)
	}
	next, _ = confirming.Update(runeKey('y'))
	updated := next.(Model)
	node, _ := updated.g.Node("a")
	if node.Complete {
		t.Fatal("completion was not reset")
	}
	if updated.dirty {
		t.Fatal("reset should autosave and clear dirty state")
	}
	if updated.mode != modeReady {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestReadyCanOpenAddNodePrompt(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('a'))
	updated := next.(Model)

	if updated.mode != modePrompt {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.previous != modeReady {
		t.Fatalf("previous = %v", updated.previous)
	}
	if updated.promptAction != promptAddNode {
		t.Fatalf("promptAction = %v", updated.promptAction)
	}
	if updated.promptTitle != "Add node" {
		t.Fatalf("promptTitle = %q", updated.promptTitle)
	}
}

func TestReadyCanStartOrder(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('o'))
	updated := next.(Model)

	if updated.mode != modeOrder {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.previous != modeReady {
		t.Fatalf("previous = %v", updated.previous)
	}
	if updated.orderReturn != modeReady {
		t.Fatalf("orderReturn = %v", updated.orderReturn)
	}
	if updated.order == nil {
		t.Fatal("order is nil")
	}
}

func TestOrderEscReturnsToLaunchingMode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('o'))
	ordering := next.(Model)
	next, _ = ordering.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)

	if updated.mode != modeReady {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestOrderQReturnsToLaunchingMode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('o'))
	ordering := next.(Model)
	next, _ = ordering.Update(runeKey('q'))
	updated := next.(Model)

	if updated.mode != modeReady {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestNodeOrderEscReturnsToNode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)
	m.mode = modeNode

	next, _ := m.Update(runeKey('o'))
	ordering := next.(Model)
	next, _ = ordering.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)

	if updated.mode != modeNode {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestReadyEscDoesNotEnterNodeView(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("first-root", "First root"))
	must(t, g.AddNodeWithID("second-root", "Second root"))
	m := newTestModel(t, g)
	m.readyCursor = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)

	if updated.mode != modeReady {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.readyCursor != 1 {
		t.Fatalf("readyCursor = %d", updated.readyCursor)
	}
	if updated.current != "first-root" {
		t.Fatalf("current = %q", updated.current)
	}
}

func TestReadySearchEscReturnsToReady(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('/'))
	searching := next.(Model)
	if searching.mode != modeSearch {
		t.Fatalf("mode = %v", searching.mode)
	}

	next, _ = searching.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)
	if updated.mode != modeReady {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestSearchTreatsPrintableKeysAsInput(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("log-into-bank", "Log into bank"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('/'))
	searching := next.(Model)
	query := "Log into bank / q W R v a p c e d i r l o s x J K ?"
	for _, r := range query {
		next, _ = searching.Update(runeKey(r))
		searching = next.(Model)
	}

	if searching.mode != modeSearch {
		t.Fatalf("mode = %v", searching.mode)
	}
	if searching.input.Value() != query {
		t.Fatalf("input = %q", searching.input.Value())
	}
}

func TestSearchDoesNotMatchHiddenIDAfterEdit(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("shred-chicken-give-curt", "Shred or cut chicken into small pieces, give to Curt"))
	m := newTestModel(t, g)
	m.mode = modeNode
	m.current = "shred-chicken-give-curt"

	must(t, g.EditNodeText("shred-chicken-give-curt", "Shred or cut chicken into small pieces"))

	if results := m.searchResults("Curt"); len(results) != 0 {
		t.Fatalf("results = %v", results)
	}
}

func TestSearchWindowsResultsToTerminalHeight(t *testing.T) {
	g := graph.New()
	for i := 1; i <= 25; i++ {
		id := graph.NodeID(fmt.Sprintf("candidate-%02d", i))
		text := fmt.Sprintf("Candidate %02d with enough extra text to stay compact in search", i)
		must(t, g.AddNodeWithID(id, text))
	}
	m := newTestModel(t, g)
	m.mode = modeSearch
	m.height = 16
	m.width = 48

	view := m.viewSearch()
	if !strings.Contains(view, "Search 1-10 of 25") {
		t.Fatalf("expected initial search window:\n%s", view)
	}
	if strings.Contains(view, "Candidate 11") {
		t.Fatalf("rendered beyond visible search window:\n%s", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > m.height {
		t.Fatalf("view has %d lines, height %d:\n%s", lines, m.height, view)
	}

	m.searchCursor = 14
	view = m.viewSearch()
	if !strings.Contains(view, "Search 10-19 of 25") {
		t.Fatalf("expected centered search window:\n%s", view)
	}
	if !strings.Contains(view, "Candidate 15") {
		t.Fatalf("selected search candidate not visible:\n%s", view)
	}
}

func TestReadyHelpEscReturnsToReady(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('?'))
	helping := next.(Model)
	if helping.mode != modeHelp {
		t.Fatalf("mode = %v", helping.mode)
	}

	next, _ = helping.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(Model)
	if updated.mode != modeReady {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestReadyQuitCancelReturnsToReady(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	m := newTestModel(t, g)
	m.dirty = true

	next, _ := m.Update(runeKey('q'))
	confirming := next.(Model)
	if confirming.mode != modeConfirmQuit {
		t.Fatalf("mode = %v", confirming.mode)
	}

	next, _ = confirming.Update(runeKey('c'))
	updated := next.(Model)
	if updated.mode != modeReady {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestNodeCanOpenLeavesView(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("leaf-node", "Leaf node"))
	must(t, g.AddEdge("root-node", "leaf-node"))
	m := newTestModel(t, g)
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

func TestReadyCanOpenLeavesView(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("leaf-node", "Leaf node"))
	must(t, g.AddEdge("root-node", "leaf-node"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('l'))
	updated := next.(Model)

	if updated.mode != modeLeaves {
		t.Fatalf("mode = %v", updated.mode)
	}
	if updated.previous != modeReady {
		t.Fatalf("previous = %v", updated.previous)
	}
	if updated.leavesReturn != modeReady {
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
	m := newTestModel(t, g)
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
	m := newTestModel(t, g)
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

func TestLeavesReordersHighlightedNode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("first-leaf", "First leaf"))
	must(t, g.AddNodeWithID("second-leaf", "Second leaf"))
	must(t, g.AddEdge("root-node", "first-leaf"))
	must(t, g.AddEdge("root-node", "second-leaf"))
	m := newTestModel(t, g)
	m.mode = modeLeaves

	next, _ := m.Update(runeKey('J'))
	updated := next.(Model)

	if got := updated.g.Leaves(); !reflect.DeepEqual(got, []graph.NodeID{"second-leaf", "first-leaf"}) {
		t.Fatalf("leaves after J = %#v", got)
	}
	if updated.leavesCursor != 1 {
		t.Fatalf("leavesCursor after J = %d", updated.leavesCursor)
	}
	if updated.dirty {
		t.Fatal("leaves reorder should autosave and clear dirty state")
	}

	next, _ = updated.Update(runeKey('K'))
	updated = next.(Model)
	if got := updated.g.Leaves(); !reflect.DeepEqual(got, []graph.NodeID{"first-leaf", "second-leaf"}) {
		t.Fatalf("leaves after K = %#v", got)
	}
	if updated.leavesCursor != 0 {
		t.Fatalf("leavesCursor after K = %d", updated.leavesCursor)
	}
}

func TestLeavesEscReturnsToLaunchingMode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("leaf-node", "Leaf node"))
	must(t, g.AddEdge("root-node", "leaf-node"))
	m := newTestModel(t, g)
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

func TestLeavesSearchEscReturnsToLeaves(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("leaf-node", "Leaf node"))
	must(t, g.AddEdge("root-node", "leaf-node"))
	m := newTestModel(t, g)
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

func TestLeavesCanOpenReadyViewWithR(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("leaf-node", "Leaf node"))
	must(t, g.AddEdge("root-node", "leaf-node"))
	m := newTestModel(t, g)
	m.mode = modeLeaves

	next, _ := m.Update(runeKey('r'))
	updated := next.(Model)

	if updated.mode != modeReady {
		t.Fatalf("mode = %v", updated.mode)
	}
}

func TestLeavesWindowsItemsToTerminalHeight(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	for i := 1; i <= 20; i++ {
		id := graph.NodeID(fmt.Sprintf("leaf-%02d", i))
		text := fmt.Sprintf("Leaf item %02d", i)
		must(t, g.AddNodeWithID(id, text))
		must(t, g.AddEdge("root-node", id))
	}
	m := newTestModel(t, g)
	m.mode = modeLeaves
	m.height = 10
	m.width = 48

	view := m.viewLeaves()
	if !strings.Contains(view, "Leaves 1-2 of 20") {
		t.Fatalf("expected initial leaves window:\n%s", view)
	}
	if strings.Contains(view, "Leaf item 03") {
		t.Fatalf("rendered beyond visible leaves window:\n%s", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > m.height {
		t.Fatalf("view has %d lines, height %d:\n%s", lines, m.height, view)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	paged := next.(Model)
	if paged.leavesCursor != 1 {
		t.Fatalf("leavesCursor after PgDown = %d", paged.leavesCursor)
	}

	paged.leavesCursor = 12
	view = paged.viewLeaves()
	if !strings.Contains(view, "Leaves 12-13 of 20") {
		t.Fatalf("expected centered leaves window:\n%s", view)
	}
	if !strings.Contains(view, "Leaf item 13") {
		t.Fatalf("selected leaf not visible:\n%s", view)
	}
}

func TestLeavesWrapsItemsWithContinuationIndent(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root-node", "Root node"))
	must(t, g.AddNodeWithID("long-leaf", "This leaf item has enough words to wrap onto another line cleanly"))
	must(t, g.AddEdge("root-node", "long-leaf"))
	m := newTestModel(t, g)
	m.mode = modeLeaves
	m.height = 14
	m.width = 36

	view := m.viewLeaves()
	for _, want := range []string{
		"This leaf item has enough",
		"\n    words to wrap onto another",
		"\n    line cleanly",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing wrapped text %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "This leaf item has enough words...") {
		t.Fatalf("leaf item was truncated:\n%s", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > m.height {
		t.Fatalf("view has %d lines, height %d:\n%s", lines, m.height, view)
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

	m := newTestModel(t, g)
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

func TestLinkPromptHidesCycleCandidates(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("grandparent", "Grandparent"))
	must(t, g.AddNodeWithID("parent", "Parent"))
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddNodeWithID("child", "Child"))
	must(t, g.AddNodeWithID("available", "Available"))
	must(t, g.AddEdge("grandparent", "parent"))
	must(t, g.AddEdge("parent", "current"))
	must(t, g.AddEdge("current", "child"))

	m := newTestModel(t, g)
	m.current = "current"

	parentCandidates := m.linkCandidates("", promptAddParent)
	if containsID(parentCandidates, "child") {
		t.Fatalf("cycle-creating parent candidate was shown: %#v", parentCandidates)
	}
	if !containsID(parentCandidates, "available") {
		t.Fatalf("available parent candidate missing: %#v", parentCandidates)
	}

	childCandidates := m.linkCandidates("", promptAddChild)
	if containsID(childCandidates, "grandparent") {
		t.Fatalf("cycle-creating child candidate was shown: %#v", childCandidates)
	}
	if !containsID(childCandidates, "available") {
		t.Fatalf("available child candidate missing: %#v", childCandidates)
	}
}

func TestTypedCycleCandidateStillShowsGraphError(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddNodeWithID("child", "Child"))
	must(t, g.AddEdge("current", "child"))

	m := newTestModel(t, g)
	m.current = "current"
	m.mode = modePrompt
	m.promptAction = promptAddParent
	m.input.SetValue("Child")

	next, _ := m.submitPrompt(false)
	updated := next.(Model)

	if updated.mode != modePrompt {
		t.Fatalf("mode = %v", updated.mode)
	}
	if !strings.Contains(updated.message, "cycle") {
		t.Fatalf("expected cycle error, got %q", updated.message)
	}
}

func TestLinkPromptWindowsMatchesToTerminalHeight(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("current", "Current"))
	for i := 1; i <= 25; i++ {
		id := graph.NodeID(fmt.Sprintf("candidate-%02d", i))
		text := fmt.Sprintf("Candidate %02d with enough extra text to demonstrate single-line truncation", i)
		must(t, g.AddNodeWithID(id, text))
	}

	m := newTestModel(t, g)
	m.current = "current"
	m.mode = modePrompt
	m.promptAction = promptAddChild
	m.promptTitle = "Add/link child"
	m.height = 16
	m.width = 48

	view := m.viewPrompt()
	if !strings.Contains(view, "Matches 1-6 of 25") {
		t.Fatalf("expected initial match window:\n%s", view)
	}
	if strings.Contains(view, "Candidate 07") {
		t.Fatalf("rendered beyond visible window:\n%s", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > m.height {
		t.Fatalf("view has %d lines, height %d:\n%s", lines, m.height, view)
	}

	m.suggestionCursor = 10
	view = m.viewPrompt()
	if !strings.Contains(view, "Matches 8-13 of 25") {
		t.Fatalf("expected centered match window:\n%s", view)
	}
	if !strings.Contains(view, "Candidate 11") {
		t.Fatalf("selected candidate not visible:\n%s", view)
	}
}

func TestLinkPromptShowsNoEligibleMatchesMessage(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("parent", "Existing parent"))
	must(t, g.AddNodeWithID("current", "Current"))
	must(t, g.AddEdge("parent", "current"))

	m := newTestModel(t, g)
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
