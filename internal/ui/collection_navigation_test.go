// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/displaywidth"

	"github.com/tunesmith/dagim/internal/collection"
	"github.com/tunesmith/dagim/internal/dagimfile"
	"github.com/tunesmith/dagim/internal/graph"
)

func navigationTick(m CollectionModel) CollectionModel {
	updated, _ := m.Update(diskCheckMsg{})
	return updated.(CollectionModel)
}

func TestCollectionNavigationTraversesBlockedNodesAndRetraces(t *testing.T) {
	m, a, b := collectionUIFixture(t, 0)
	aBefore, _ := os.ReadFile(a)
	bBefore, _ := os.ReadFile(b)
	m = collectionKey(m, "enter")
	if !m.detail || m.current != (collection.Ref{Member: "house", ID: "d17"}) {
		t.Fatal("did not focus the selected source node")
	}
	if ref, ok := m.selectedRelation(); !ok || ref.ID != "d18" {
		t.Fatal("child not selectable")
	}
	m = collectionKey(m, "enter")
	if m.current.ID != "d18" || !strings.Contains(ansi.Strip(m.View()), "Blocked by: house:d17") {
		t.Fatal("blocked child not reachable or blockers missing", m.View())
	}
	m = collectionKey(m, " ")
	if !strings.Contains(m.message, "blocked") {
		t.Fatal("blocked completion not rejected", m.message)
	}
	// Follow the incomplete parent, then retrace the exact path in both directions.
	m = collectionKey(m, "enter")
	if m.current.ID != "d17" || len(m.navigation) != 2 {
		t.Fatal("parent traversal failed")
	}
	m = collectionKey(m, "esc")
	if m.current.ID != "d18" {
		t.Fatal("backtracking lost child focus")
	}
	m = collectionKey(m, "esc")
	if ref, ok := m.selectedRelation(); m.current.ID != "d17" || !ok || ref.ID != "d18" {
		t.Fatal("backtracking lost relation selection")
	}
	m = collectionKey(m, "esc")
	if m.detail || len(m.navigation) != 0 || m.items[m.cursor].Member != "house" {
		t.Fatal("did not return to the original ready selection")
	}
	for path, before := range map[string][]byte{a: aBefore, b: bBefore} {
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("navigation or rejected completion changed source", path)
		}
	}
}

func TestCollectionNavigationCompletionKeepsFocusAndOtherBlockers(t *testing.T) {
	m, a, b := collectionUIFixture(t, 0)
	g, _ := dagimfile.Load(a)
	if err := g.AddNodeWithID("other", "Other prerequisite"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddEdge("other", "d18"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddNodeWithID("leaf", "Last task"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddEdge("d18", "leaf"); err != nil {
		t.Fatal(err)
	}
	if err := dagimfile.SaveAtomic(a, g); err != nil {
		t.Fatal(err)
	}
	m = navigationTick(m)
	bBefore, _ := os.ReadFile(b)
	collectionBefore, _ := os.ReadFile(m.path)
	m = collectionKey(m, "enter")
	m = collectionKey(m, " ")
	if !m.detail || m.current.ID != "d17" {
		t.Fatal("completion displaced focused node")
	}
	if relations := m.relations(); len(relations) != 1 || relations[0].Summary.State != "blocked" {
		t.Fatal("child incorrectly became ready while another parent is incomplete")
	}
	m = collectionKey(m, "enter")
	if !strings.Contains(ansi.Strip(m.View()), "Blocked by: house:other") {
		t.Fatal("remaining blocker missing")
	}
	// The completed parent is hidden; follow the other parent and satisfy it.
	if ref, _ := m.selectedRelation(); ref.ID != "other" {
		t.Fatal("wrong parent selected")
	}
	m = collectionKey(m, "enter")
	m = collectionKey(m, " ")
	m = collectionKey(m, "enter")
	if m.current.ID != "d18" || m.relations()[0].ID != "leaf" {
		t.Fatal("ready descendant not reachable")
	}
	m = collectionKey(m, " ")
	m = collectionKey(m, "enter")
	m = collectionKey(m, " ")
	if !m.detail || m.current.ID != "leaf" || len(m.items) != 1 || m.items[0].Member != "work" {
		t.Fatal("completing the last source task lost focus")
	}
	// Show and follow the completed parent, then reopen it and its descendant.
	m = collectionKey(m, "v")
	m = collectionKey(m, "enter")
	if m.current.ID != "d18" {
		t.Fatal("completed parent not navigable")
	}
	m = collectionKey(m, " ")
	g, _ = dagimfile.Load(a)
	for _, id := range []graph.NodeID{"d18", "leaf"} {
		node, _ := g.Node(id)
		if node.Complete {
			t.Fatal("reopen did not cascade", id)
		}
	}
	bAfter, _ := os.ReadFile(b)
	collectionAfter, _ := os.ReadFile(m.path)
	if !bytes.Equal(bBefore, bAfter) || !bytes.Equal(collectionBefore, collectionAfter) {
		t.Fatal("node completion wrote outside its source graph")
	}
}

func TestCollectionNavigationSurvivesReadyChangesAndPreservesFilteredList(t *testing.T) {
	m, _, b := collectionUIFixture(t, 0)
	m = collectionKey(m, "/")
	m = collectionKey(m, "First")
	m = collectionKey(m, "enter") // finish filtering
	m = collectionKey(m, "j")
	m = collectionKey(m, "enter")
	m = collectionKey(m, "enter")
	if m.current != (collection.Ref{Member: "work", ID: "d18"}) {
		t.Fatal("source identity lost")
	}
	m = collectionKey(m, "?")
	if !m.help || !m.detail {
		t.Fatal("help replaced navigation")
	}
	m = collectionKey(m, "esc")
	if m.help || !m.detail || m.current.ID != "d18" {
		t.Fatal("help did not restore focus")
	}
	// An external completion changes the ready list; the blocked focus is independent.
	g, _ := dagimfile.Load(b)
	if err := g.MarkComplete("d17"); err != nil {
		t.Fatal(err)
	}
	if err := dagimfile.SaveAtomic(b, g); err != nil {
		t.Fatal(err)
	}
	m = navigationTick(m)
	if m.current != (collection.Ref{Member: "work", ID: "d18"}) || len(m.items) != 1 || m.items[0].Member != "house" {
		t.Fatal("ready-list change retargeted focused node")
	}
	if !strings.Contains(ansi.Strip(m.View()), "State: ready") {
		t.Fatal("focused state not refreshed")
	}
	m = collectionKey(m, "r")
	if m.detail || m.input.Value() != "First" || m.items[m.cursor].Member != "house" {
		t.Fatal("return to filtered ready list failed")
	}
}

func TestCollectionNavigationDeletionConsumesPendingActionAndSkipsMissingHistory(t *testing.T) {
	m, a, _ := collectionUIFixture(t, 0)
	m = collectionKey(m, "enter")
	m = collectionKey(m, "enter")
	g, _ := dagimfile.Load(a)
	if err := g.DeleteNode("d18"); err != nil {
		t.Fatal(err)
	}
	if err := dagimfile.SaveAtomic(a, g); err != nil {
		t.Fatal(err)
	}
	m = collectionKey(m, " ")
	if !m.detail || m.current.ID != "d17" || m.message == "" {
		t.Fatal("deleted focus not handled")
	}
	g, _ = dagimfile.Load(a)
	node, _ := g.Node("d17")
	if node.Complete {
		t.Fatal("key for deleted focus completed a different node")
	}
	d, _ := collection.Load(m.path)
	if err := d.Remove("house"); err != nil {
		t.Fatal(err)
	}
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	m = collectionKey(m, " ")
	if m.detail || len(m.navigation) != 0 || m.items[m.cursor].Member != "work" {
		t.Fatal("removed member did not return to ready list")
	}
	source, _ := m.snapshot.Lookup(collection.Ref{Member: "work", ID: "d17"})
	node, _ = source.Graph.Node("d17")
	if node.Complete {
		t.Fatal("member removal retargeted completion to duplicate ID")
	}
}

func TestCollectionNavigationInvalidReloadDisablesCompletionAndRecovers(t *testing.T) {
	m, a, b := collectionUIFixture(t, 0)
	m = collectionKey(m, "enter")
	aBefore, _ := os.ReadFile(a)
	bBefore, _ := os.ReadFile(b)
	if err := os.WriteFile(b, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	m = collectionKey(m, " ")
	aAfter, _ := os.ReadFile(a)
	if m.reloadError == nil || !m.detail || !bytes.Equal(aBefore, aAfter) || !strings.Contains(ansi.Strip(m.View()), "actions disabled") {
		t.Fatal("invalid reload allowed completion or hid focus")
	}
	if err := os.WriteFile(b, bBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	m = navigationTick(m)
	m = collectionKey(m, " ")
	g, _ := dagimfile.Load(a)
	node, _ := g.Node("d17")
	if m.reloadError != nil || !node.Complete || m.current.ID != "d17" {
		t.Fatal("valid reload did not recover focused completion")
	}
}

func TestCollectionNavigationRelationSelectionAndViewportSurviveReload(t *testing.T) {
	m, a, _ := collectionUIFixture(t, 0)
	g, _ := dagimfile.Load(a)
	for i := 0; i < 20; i++ {
		id := graph.NodeID(fmt.Sprintf("child%d", i))
		if err := g.AddNodeWithID(id, fmt.Sprintf("Child %d %s", i, strings.Repeat("wide 界 ", 8))); err != nil {
			t.Fatal(err)
		}
		if err := g.AddEdge("d17", id); err != nil {
			t.Fatal(err)
		}
	}
	if err := dagimfile.SaveAtomic(a, g); err != nil {
		t.Fatal(err)
	}
	m = navigationTick(m)
	m = collectionKey(m, "enter")
	for _, size := range []tea.WindowSizeMsg{{Width: 40, Height: 12}, {Width: 80, Height: 24}, {Width: 120, Height: 16}} {
		updated, _ := m.Update(size)
		m = updated.(CollectionModel)
		m = collectionKey(m, "home")
		m = collectionKey(m, "pgdown")
		if m.relationCursor == 0 {
			t.Fatal("relation paging did not advance")
		}
		m = collectionKey(m, "end")
		body := m.navigationBody()
		span := body.spans[m.relationCursor]
		if span.start < m.relationScroll || span.start >= m.relationScroll+m.navigationBudget() {
			t.Fatal("selected relation not visible")
		}
		view := ansi.Strip(m.View())
		if strings.Count(view, "\n")+1 > size.Height {
			t.Fatal("navigation exceeds terminal height")
		}
		for _, line := range strings.Split(view, "\n") {
			if displaywidth.String(line) > size.Width {
				t.Fatal("navigation exceeds terminal width")
			}
		}
		if !strings.Contains(view, "follow") {
			t.Fatal("navigation footer not visible", view)
		}
	}
	selected, _ := m.selectedRelation()
	// Changing source order must preserve selected identity, not its old index.
	if !g.MoveTo(selected.ID, 0) {
		t.Fatal("failed to change fixture order")
	}
	if err := dagimfile.SaveAtomic(a, g); err != nil {
		t.Fatal(err)
	}
	m = navigationTick(m)
	if got, _ := m.selectedRelation(); got != selected {
		t.Fatal("reload changed selected relation", got, selected)
	}
	scroll := m.relationScroll
	m = collectionKey(m, "enter")
	m = collectionKey(m, "esc")
	if got, _ := m.selectedRelation(); got != selected || m.relationScroll != scroll {
		t.Fatal("backtracking lost selection or viewport")
	}
}
