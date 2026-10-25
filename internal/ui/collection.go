// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tunesmith/dagim/internal/collection"
	"github.com/tunesmith/dagim/internal/query"
)

// CollectionModel is deliberately separate from Model: the graph editor's
// navigation, ordering and mutations keep their existing single-file meaning.
type CollectionModel struct {
	path                                   string
	snapshot                               *collection.Snapshot
	items                                  []collection.Item
	cursor, scroll, overlayScroll          int
	width, height                          int
	showCompleted, help, detail, searching bool
	input                                  textinput.Model
	message                                string
	reloadError                            error
}

func NewCollection(path string) (CollectionModel, error) {
	input := textinput.New()
	input.Prompt = "/ "
	input.CharLimit = 4096
	m := CollectionModel{path: path, input: input}
	m = m.refresh()
	return m, m.reloadError
}

func RunCollection(path string) error {
	m, err := NewCollection(path)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m CollectionModel) Init() tea.Cmd { return scheduleDiskCheck() }

func (m CollectionModel) refresh() CollectionModel {
	var selected collection.Ref
	if len(m.items) > 0 {
		selected = m.items[clampedCursor(m.cursor, len(m.items))].Ref
	}
	s, err := collection.LoadSnapshot(m.path)
	m.reloadError = err
	if err != nil {
		return m
	}
	m.snapshot = s
	m.items = nil
	for _, item := range s.Items(query.FilterAll) {
		if item.Summary.State != query.StateReady && !(m.showCompleted && item.Summary.State == query.StateComplete) {
			continue
		}
		if text := strings.ToLower(m.input.Value()); text != "" && !strings.Contains(strings.ToLower(item.Member+" "+item.Summary.Node.Text+" "+string(item.ID)), text) {
			continue
		}
		m.items = append(m.items, item)
	}
	m.cursor = clampedCursor(m.cursor, len(m.items))
	for i, item := range m.items {
		if item.Ref == selected {
			m.cursor = i
			break
		}
	}
	return m.ensureVisible()
}

func (m CollectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case diskCheckMsg:
		return m.refresh(), scheduleDiskCheck()
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = inputWidth(msg.Width)
		return m.ensureVisible(), nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "ctrl+z" {
			return m, tea.Suspend
		}
		if m.searching {
			switch msg.String() {
			case "esc":
				m.input.SetValue("")
				m.searching = false
				m.input.Blur()
			case "enter":
				m.searching = false
				m.input.Blur()
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m.refresh(), cmd
			}
			return m.refresh(), nil
		}
		if m.help || m.detail {
			f := m.formatter()
			body := m.overlayBody()
			footer := m.overlayFooter()
			switch msg.String() {
			case "q", "esc", "?", "enter":
				m.help, m.detail = false, false
			case "j", "down":
				m.overlayScroll++
			case "k", "up":
				m.overlayScroll--
			case "pgdown":
				m.overlayScroll += maxInt(1, f.scrollBodyHeight(footer)-1)
			case "pgup":
				m.overlayScroll -= maxInt(1, f.scrollBodyHeight(footer)-1)
			case "home":
				m.overlayScroll = 0
			case "end":
				m.overlayScroll = f.scrollMax(body, footer)
			}
			m.overlayScroll = clampInt(m.overlayScroll, 0, f.scrollMax(body, footer))
			return m, nil
		}
		m.message = ""
		m = m.refresh()
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "?":
			m.help = true
			m.overlayScroll = 0
		case "enter":
			m.detail = len(m.items) > 0
			m.overlayScroll = 0
		case "/":
			m.searching = true
			return m, m.input.Focus()
		case "esc":
			m.input.SetValue("")
			m = m.refresh()
		case "v":
			m.showCompleted = !m.showCompleted
			m = m.refresh()
		case "j", "down":
			m.cursor = clampedCursor(m.cursor+1, len(m.items))
		case "k", "up":
			m.cursor = clampedCursor(m.cursor-1, len(m.items))
		case "home":
			m.cursor = 0
		case "end":
			m.cursor = maxInt(0, len(m.items)-1)
		case "pgdown", "pgup":
			direction := 1
			if msg.String() == "pgup" {
				direction = -1
			}
			m.cursor = pageSelection(m.cursor, m.rendered().spans, m.budget(), direction)
		case " ":
			if m.reloadError != nil || len(m.items) == 0 {
				break
			}
			item := m.items[m.cursor]
			_, _, err := m.snapshot.SetCompletion(item.Ref, item.Summary.Node.Complete, false)
			if err != nil {
				m.message = err.Error()
			} else {
				m = m.refresh()
			}
		case "J", "K":
			if m.reloadError != nil || len(m.items) == 0 {
				break
			}
			direction := 1
			if msg.String() == "K" {
				direction = -1
			}
			target := m.cursor + direction
			if target >= 0 && target < len(m.items) {
				err := m.snapshot.Move(m.items[m.cursor].Ref, m.items[target].Ref, direction > 0)
				if err != nil {
					m.message = err.Error()
				} else {
					m = m.refresh()
				}
			}
		}
		return m.ensureVisible(), nil
	}
	if m.searching {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m CollectionModel) formatter() Model { return Model{width: m.width, height: m.height} }

func (m CollectionModel) budget() int {
	rows := 4 + lineCount(m.footer()) // title, path, filter, blank separator
	if m.reloadError != nil || m.message != "" {
		rows += 1 + lineCount(m.status())
	}
	if m.height <= 0 {
		return 20
	}
	return maxInt(1, m.height-rows)
}

func (m CollectionModel) footer() string {
	f := m.formatter()
	full := f.renderCommandGrid([][]string{
		{"j/k move", "PgUp/PgDn page", "Space done/undone", "J/K reorder"},
		{"v completed", "/ filter", "Enter inspect", "? help / q quit"},
	})
	return f.responsiveFooter(full, "Space done  J/K reorder  ? help  q quit", 3)
}

func (m CollectionModel) status() string {
	f := m.formatter()
	text := m.message
	if m.reloadError != nil {
		text = "Reload failed; actions disabled: " + m.reloadError.Error()
	}
	return f.renderWrapped("", text, errorStyle)
}

func (m CollectionModel) overlayFooter() string {
	f := m.formatter()
	return truncateText("j/k scroll  PgUp/PgDn page  Esc/q back", f.contentWidth())
}

func (m CollectionModel) rendered() renderedText {
	lines := make([]string, len(m.items))
	f := m.formatter()
	for i, item := range m.items {
		node := item.Summary.Node
		node.Text = "[" + item.Member + "] " + node.Text
		lines[i] = f.renderSelectableNode(i == m.cursor, node)
	}
	return renderedItems(lines)
}

func (m CollectionModel) ensureVisible() CollectionModel {
	body := m.rendered()
	if len(body.spans) == 0 {
		m.scroll = 0
		return m
	}
	m.cursor = clampedCursor(m.cursor, len(body.spans))
	m.scroll = revealSelection(m.scroll, len(body.lines), body.spans[m.cursor], m.budget())
	return m
}

func (m CollectionModel) View() string {
	f := m.formatter()
	if m.help || m.detail {
		body := f.renderScrollable(m.overlayBody(), m.overlayScroll, m.overlayFooter())
		return f.fitToTerminal(f.padBlock(body))
	}
	title := "Ready"
	if m.showCompleted {
		title = "Ready + Completed"
	}
	members := 0
	if m.snapshot != nil {
		members = len(m.snapshot.Document.Members)
	}
	lines := []string{titleStyle.Render(truncateText(fmt.Sprintf("%s · %d tasks · %d files", title, len(m.items), members), f.contentWidth())), mutedStyle.Render(truncateText(m.path, f.contentWidth()))}
	if m.searching {
		lines = append(lines, m.input.View())
	} else if m.input.Value() != "" {
		lines = append(lines, truncateText("Filter: "+m.input.Value(), f.contentWidth()))
	} else {
		lines = append(lines, "")
	}
	body := m.rendered()
	if len(m.items) == 0 {
		text := "  none ready"
		if members == 0 {
			text = "  no files; use dagim collection add"
		}
		if m.input.Value() != "" {
			text = "  no matching tasks"
		}
		lines = append(lines, truncateText(text, f.contentWidth()))
	} else {
		lines = append(lines, viewportLines(body.lines, m.scroll, m.budget())...)
	}
	lines = append(lines, "", m.footer())
	if m.reloadError != nil || m.message != "" {
		lines = append(lines, "", m.status())
	}
	return f.fitToTerminal(f.padBlock(strings.Join(lines, "\n")))
}

func (m CollectionModel) overlayBody() string {
	f := m.formatter()
	var lines []string
	switch {
	case m.help:
		lines = []string{"Collection commands", "", "j/k or arrows: select task", "PgUp/PgDn, Home/End: navigate", "Space: complete selected task (or reopen a completed task)", "J/K: reorder across files; saved in the collection", "v: show/hide completed tasks", "/: filter by member, task text, or node ID; Esc clears", "Enter: inspect task and dependencies", "q: quit", "", "Membership: dagim collection add/remove/list COLLECTION ...", "Members remain independent graphs; dependencies stay within a file.", "", "Esc or q: back"}
	case m.detail && len(m.items) > 0:
		item := m.items[clampedCursor(m.cursor, len(m.items))]
		source := m.snapshot.Sources[item.Member]
		lines = []string{item.Ref.String(), "", item.Summary.Node.Text, "", "File: " + item.File, "State: " + string(item.Summary.State), "", "Parents"}
		parents, _ := source.Graph.ParentsOf(item.ID)
		children, _ := source.Graph.ChildrenOf(item.ID)
		for _, id := range parents {
			n, _ := source.Graph.Node(id)
			lines = append(lines, fmt.Sprintf("  %s: %s", id, n.Text))
		}
		if len(parents) == 0 {
			lines = append(lines, "  none")
		}
		lines = append(lines, "", "Children")
		for _, id := range children {
			n, _ := source.Graph.Node(id)
			lines = append(lines, fmt.Sprintf("  %s: %s", id, n.Text))
		}
		if len(children) == 0 {
			lines = append(lines, "  none")
		}
		lines = append(lines, "", "Esc or q: back")
	}
	if m.reloadError != nil {
		lines = append(lines, "", "Reload failed; actions disabled: "+m.reloadError.Error())
	} else if m.message != "" {
		lines = append(lines, "", m.message)
	}
	wrapped := make([]string, len(lines))
	for i, line := range lines {
		wrapped[i] = f.renderWrapped("", line, nodeStyle.Copy().Bold(false))
	}
	return strings.Join(wrapped, "\n")
}
