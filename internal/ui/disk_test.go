// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tunesmith/dagim/internal/dagimfile"
	"github.com/tunesmith/dagim/internal/graph"
)

func TestDiskCheckReloadsExternalChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.dagim")
	initial := graph.New()
	must(t, initial.AddNodeWithID("root", "Root"))
	mustSaveGraph(t, path, initial)

	m := New(path, initial.Clone())
	m.current = "root"
	m.undoStack = append(m.undoStack, m.undoSnapshot())

	external := initial.Clone()
	must(t, external.AddNodeWithID("agent-change", "Added by agent"))
	mustSaveGraph(t, path, external)

	nextModel, cmd := m.Update(diskCheckMsg{})
	next := nextModel.(Model)
	if cmd == nil {
		t.Fatal("disk check should schedule the next check")
	}
	if !next.g.HasNode("agent-change") {
		t.Fatal("external node was not reloaded")
	}
	if next.current != "root" {
		t.Fatalf("current = %q, want preserved root", next.current)
	}
	if len(next.undoStack) != 0 {
		t.Fatal("external reload should clear undo history")
	}
	if next.message != "reloaded changes from disk" {
		t.Fatalf("message = %q", next.message)
	}
}

func TestNewUsesLatestDiskContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.dagim")
	stale := graph.New()
	must(t, stale.AddNodeWithID("stale", "Stale"))
	latest := graph.New()
	must(t, latest.AddNodeWithID("latest", "Latest"))
	mustSaveGraph(t, path, latest)

	m := New(path, stale)
	if !m.g.HasNode("latest") || m.g.HasNode("stale") {
		t.Fatal("new model did not use the latest on-disk graph")
	}
}

func TestAutosaveBlocksStaleWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.dagim")
	initial := graph.New()
	must(t, initial.AddNodeWithID("root", "Root"))
	mustSaveGraph(t, path, initial)
	m := New(path, initial.Clone())

	external := initial.Clone()
	must(t, external.AddNodeWithID("agent-change", "Added by agent"))
	mustSaveGraph(t, path, external)

	snapshot := m.undoSnapshot()
	must(t, m.g.AddNodeWithID("tui-change", "Added in TUI"))
	m = m.markChangedWithUndo(snapshot)

	if !m.dirty {
		t.Fatal("blocked autosave should retain the unsaved TUI change")
	}
	if !strings.Contains(m.message, "autosave blocked: file changed on disk") {
		t.Fatalf("message = %q", m.message)
	}
	onDisk, err := dagimfile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !onDisk.HasNode("agent-change") || onDisk.HasNode("tui-change") {
		t.Fatal("stale TUI graph overwrote the external change")
	}
}

func TestAutosaveUpdatesTrackedDiskVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.dagim")
	m := New(path, graph.New())
	snapshot := m.undoSnapshot()
	must(t, m.g.AddNodeWithID("root", "Root"))
	m = m.markChangedWithUndo(snapshot)
	if m.dirty {
		t.Fatalf("autosave failed: %s", m.message)
	}
	if len(m.undoStack) != 1 {
		t.Fatalf("undo entries = %d, want 1", len(m.undoStack))
	}

	m = m.refreshFromDiskNow()
	if len(m.undoStack) != 1 {
		t.Fatal("the TUI treated its own autosave as an external reload")
	}
}

func TestInvalidExternalChangeIsNotLoadedOrOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.dagim")
	initial := graph.New()
	must(t, initial.AddNodeWithID("root", "Root"))
	mustSaveGraph(t, path, initial)
	m := New(path, initial.Clone())

	invalid := []byte("# dagim v1\nnode broken\n")
	if err := os.WriteFile(path, invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	m = m.refreshFromDiskNow()
	if !m.g.HasNode("root") {
		t.Fatal("invalid external contents replaced the in-memory graph")
	}
	if !strings.Contains(m.message, "external change is invalid") {
		t.Fatalf("message = %q", m.message)
	}

	snapshot := m.undoSnapshot()
	must(t, m.g.AddNodeWithID("tui-change", "Added in TUI"))
	m = m.markChangedWithUndo(snapshot)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(invalid) {
		t.Fatal("autosave overwrote invalid external contents")
	}
}

func TestExternalEditCancelsStaleEditPrompt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.dagim")
	initial := graph.New()
	must(t, initial.AddNodeWithID("root", "Original text"))
	mustSaveGraph(t, path, initial)
	m := New(path, initial.Clone())
	m.current = "root"
	m, _ = m.setPrompt(promptEdit, "Edit node", "Original text")

	external := initial.Clone()
	must(t, external.EditNodeText("root", "Changed by agent"))
	mustSaveGraph(t, path, external)
	m = m.refreshFromDiskNow()

	if m.mode == modePrompt || m.promptAction != promptNone {
		t.Fatal("stale edit prompt remained active after its node changed")
	}
	node, ok := m.g.Node("root")
	if !ok || node.Text != "Changed by agent" {
		t.Fatalf("reloaded node = %#v, found = %v", node, ok)
	}
}

func mustSaveGraph(t *testing.T, path string, g *graph.Graph) {
	t.Helper()
	if err := dagimfile.SaveAtomic(path, g); err != nil {
		t.Fatal(err)
	}
}
