// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/displaywidth"

	"github.com/tunesmith/dagim/internal/collection"
	"github.com/tunesmith/dagim/internal/dagimfile"
)

func collectionUIFixture(t *testing.T, extra int) (CollectionModel, string, string) {
	t.Helper()
	dir := t.TempDir()
	a, b, path := filepath.Join(dir, "house"), filepath.Join(dir, "work"), filepath.Join(dir, "daily")
	text := "node d17: D17: First\nnode d18: D18: Next\n  parent d17\n"
	for i := 0; i < extra; i++ {
		text += fmt.Sprintf("node task%d: Task %d %s\n", i, i, strings.Repeat("wide 界 ", 10))
	}
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := collection.Create(path); err != nil {
		t.Fatal(err)
	}
	d, err := collection.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Add(a, "house"); err != nil {
		t.Fatal(err)
	}
	if err := d.Add(b, "work"); err != nil {
		t.Fatal(err)
	}
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	m, err := NewCollection(path)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(CollectionModel), a, b
}

func collectionKey(m CollectionModel, key string) CollectionModel {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	switch key {
	case "pgdown":
		msg = tea.KeyMsg{Type: tea.KeyPgDown}
	case "pgup":
		msg = tea.KeyMsg{Type: tea.KeyPgUp}
	case "end":
		msg = tea.KeyMsg{Type: tea.KeyEnd}
	case "home":
		msg = tea.KeyMsg{Type: tea.KeyHome}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	}
	result, _ := m.Update(msg)
	return result.(CollectionModel)
}

func TestCollectionTUICompletionOrderingAndIndependentWindow(t *testing.T) {
	m, a, b := collectionUIFixture(t, 0)
	g, err := dagimfile.Load(a)
	if err != nil {
		t.Fatal(err)
	}
	individual := New(a, g)
	bBefore, _ := os.ReadFile(b)
	m = collectionKey(m, "J")
	if m.cursor != 1 || m.items[0].Member != "work" || m.items[1].Member != "house" {
		t.Fatal("cross-file reorder lost selection")
	}
	reopened, err := NewCollection(m.path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.items[0].Member != "work" {
		t.Fatal("reorder not persistent")
	}
	m = collectionKey(m, " ")
	if m.reloadError != nil || m.message != "" {
		t.Fatal(m.reloadError, m.message)
	}
	if len(m.items) != 2 || m.items[0].Member != "house" || m.items[0].ID != "d18" || m.items[1].Member != "work" {
		t.Fatalf("completion did not refresh frontier: cursor=%d items=%+v", m.cursor, m.items)
	}
	bAfter, _ := os.ReadFile(b)
	if !bytes.Equal(bBefore, bAfter) {
		t.Fatal("completion changed another member")
	}
	updated, _ := individual.Update(diskCheckMsg{})
	individual = updated.(Model)
	n, _ := individual.g.Node("d17")
	if !n.Complete {
		t.Fatal("individual view failed to reload collection completion")
	}
	updated, _ = individual.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	individual = updated.(Model)
	n, _ = individual.g.Node("d18")
	if !n.Complete {
		t.Fatal("individual completion failed")
	}
	updated, _ = m.Update(diskCheckMsg{})
	m = updated.(CollectionModel)
	if len(m.items) != 1 || m.items[0].Member != "work" {
		t.Fatal("collection failed to reload individual completion")
	}
	m = collectionKey(m, "v")
	for i, item := range m.items {
		if item.Member == "house" && item.ID == "d17" {
			m.cursor = i
		}
	}
	m = collectionKey(m, " ")
	g, _ = dagimfile.Load(a)
	if g.CompleteCount() != 0 {
		t.Fatal("combined reopen failed to cascade")
	}
}

func TestCollectionTUIInvalidReloadDisablesWritesAndRecovers(t *testing.T) {
	m, a, b := collectionUIFixture(t, 0)
	aBefore, _ := os.ReadFile(a)
	bBefore, _ := os.ReadFile(b)
	if err := os.WriteFile(b, []byte("invalid graph"), 0o600); err != nil {
		t.Fatal(err)
	}
	m = collectionKey(m, " ")
	if m.reloadError == nil || !strings.Contains(ansi.Strip(m.View()), "actions disabled") {
		t.Fatal("invalid member was hidden")
	}
	aAfter, _ := os.ReadFile(a)
	if !bytes.Equal(aBefore, aAfter) {
		t.Fatal("action was not disabled")
	}
	if err := os.WriteFile(b, bBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(diskCheckMsg{})
	m = updated.(CollectionModel)
	if m.reloadError != nil {
		t.Fatal("failed to recover", m.reloadError)
	}
	d, _ := collection.Load(m.path)
	if err := d.Remove("work"); err != nil {
		t.Fatal(err)
	}
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	updated, _ = m.Update(diskCheckMsg{})
	m = updated.(CollectionModel)
	if len(m.items) != 1 || m.items[0].Member != "house" {
		t.Fatal("membership changes not reloaded")
	}
}

func TestCollectionTUIFilteringPagingAndTerminalBounds(t *testing.T) {
	m, _, _ := collectionUIFixture(t, 15)
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 40, Height: 12}, {Width: 120, Height: 16}} {
		updated, _ := m.Update(size)
		m = updated.(CollectionModel)
		m = collectionKey(m, "home")
		m = collectionKey(m, "pgdown")
		if m.cursor == 0 {
			t.Fatal("paging did not advance")
		}
		m = collectionKey(m, "end")
		body := m.rendered()
		span := body.spans[m.cursor]
		if span.start < m.scroll || span.start >= m.scroll+m.budget() {
			t.Fatal("selection not visible")
		}
		view := ansi.Strip(m.View())
		if strings.Count(view, "\n")+1 > size.Height {
			t.Fatal("too many rows")
		}
		for _, line := range strings.Split(view, "\n") {
			if displaywidth.String(line) > size.Width {
				t.Fatal("too wide", line)
			}
		}
		if !strings.Contains(view, "reorder") {
			t.Fatal("footer lost", view)
		}
	}
	m = collectionKey(m, "/")
	m = collectionKey(m, "work")
	if len(m.items) != 16 {
		t.Fatalf("member search: %d", len(m.items))
	}
	m = collectionKey(m, "esc")
	if len(m.items) != 32 {
		t.Fatal("search not cleared")
	}
	m = collectionKey(m, "?")
	m = collectionKey(m, "end")
	if !strings.Contains(ansi.Strip(m.View()), "back") {
		t.Fatal("help paging lost footer")
	}
	m = collectionKey(m, "esc")
	m = collectionKey(m, "enter")
	if !strings.Contains(m.overlayBody(), m.items[m.cursor].Ref.String()) {
		t.Fatal("detail missing source-qualified identity")
	}
}
