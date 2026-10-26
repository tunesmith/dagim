// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tunesmith/dagim/internal/collection"
	"github.com/tunesmith/dagim/internal/query"
)

type collectionNavigationFrame struct {
	current, selected collection.Ref
	cursor, scroll    int
}

type collectionRelation struct {
	collection.Item
	kind string
}

func (m CollectionModel) relations() []collectionRelation {
	if !m.detail || m.snapshot == nil {
		return nil
	}
	source, err := m.snapshot.Lookup(m.current)
	if err != nil {
		return nil
	}
	related, err := query.RelationsFor(source.Graph, m.current.ID)
	if err != nil {
		return nil
	}
	result := []collectionRelation{}
	appendNodes := func(kind string, nodes []query.NodeSummary) {
		for _, summary := range nodes {
			if summary.Node.Complete && !m.showCompleted {
				continue
			}
			result = append(result, collectionRelation{
				Item: collection.Item{Ref: collection.Ref{Member: m.current.Member, ID: summary.Node.ID}, File: source.Path, Summary: summary},
				kind: kind,
			})
		}
	}
	appendNodes("parent", related.Parents)
	appendNodes("child", related.Children)
	return result
}

func (m CollectionModel) selectedRelation() (collection.Ref, bool) {
	relations := m.relations()
	if len(relations) == 0 {
		return collection.Ref{}, false
	}
	return relations[clampedCursor(m.relationCursor, len(relations))].Ref, true
}

func (m CollectionModel) focusNode(ref collection.Ref) CollectionModel {
	if _, err := m.snapshot.Lookup(ref); err != nil {
		m.message = err.Error()
		return m
	}
	if m.detail {
		selected, _ := m.selectedRelation()
		m.navigation = append(m.navigation, collectionNavigationFrame{current: m.current, selected: selected, cursor: m.relationCursor, scroll: m.relationScroll})
	}
	m.current, m.detail = ref, true
	m.relationCursor, m.relationScroll = 0, 0
	return m.ensureNavigationVisible()
}

func (m CollectionModel) readyFromNavigation() CollectionModel {
	for i, item := range m.items {
		if item.Ref == m.current {
			m.cursor = i
			break
		}
	}
	m.current, m.detail = collection.Ref{}, false
	m.navigation = nil
	return m.ensureVisible()
}

func (m CollectionModel) backNavigation() CollectionModel {
	for len(m.navigation) > 0 {
		frame := m.navigation[len(m.navigation)-1]
		m.navigation = m.navigation[:len(m.navigation)-1]
		if _, err := m.snapshot.Lookup(frame.current); err != nil {
			continue
		}
		m.current, m.detail = frame.current, true
		m.relationCursor, m.relationScroll = frame.cursor, frame.scroll
		relations := m.relations()
		m.relationCursor = clampedCursor(m.relationCursor, len(relations))
		for i, relation := range relations {
			if relation.Ref == frame.selected {
				m.relationCursor = i
				break
			}
		}
		return m.ensureNavigationVisible()
	}
	return m.readyFromNavigation()
}

func (m CollectionModel) toggleCollectionCompletion(ref collection.Ref) CollectionModel {
	if m.reloadError != nil {
		return m
	}
	source, err := m.snapshot.Lookup(ref)
	if err != nil {
		m.message = err.Error()
		return m
	}
	node, _ := source.Graph.Node(ref.ID)
	if _, _, err := m.snapshot.SetCompletion(ref, node.Complete, false); err != nil {
		m.message = err.Error()
		return m
	}
	return m.refresh()
}

func (m CollectionModel) updateNavigation(msg tea.KeyMsg) CollectionModel {
	relations := m.relations()
	switch msg.String() {
	case "esc", "backspace":
		return m.backNavigation()
	case "r", "q":
		return m.readyFromNavigation()
	case "?":
		m.help, m.overlayScroll = true, 0
	case "j", "down":
		m.relationCursor = clampedCursor(m.relationCursor+1, len(relations))
	case "k", "up":
		m.relationCursor = clampedCursor(m.relationCursor-1, len(relations))
	case "home":
		m.relationCursor, m.relationScroll = 0, 0
	case "end":
		m.relationCursor = maxInt(0, len(relations)-1)
		if len(relations) == 0 {
			m.relationScroll = maxInt(0, len(m.navigationBody().lines)-m.navigationBudget())
		}
	case "pgdown", "pgup":
		direction := 1
		if msg.String() == "pgup" {
			direction = -1
		}
		body := m.navigationBody()
		if len(relations) > 0 {
			m.relationCursor = pageSelection(m.relationCursor, body.spans, m.navigationBudget(), direction)
		} else {
			m.relationScroll += direction * maxInt(1, m.navigationBudget()-1)
		}
	case "enter":
		if ref, ok := m.selectedRelation(); ok {
			return m.focusNode(ref)
		}
	case " ":
		m = m.toggleCollectionCompletion(m.current)
	case "v":
		m.showCompleted = !m.showCompleted
		m = m.refresh()
	}
	return m.ensureNavigationVisible()
}

func (m CollectionModel) navigationFooter() string {
	f := m.formatter()
	full := f.renderCommandGrid([][]string{
		{"j/k select", "PgUp/PgDn page", "Enter follow", "Space done/undone"},
		{"v completed", "Esc/Backspace back", "r/q ready", "? help"},
	})
	return f.responsiveFooter(full, "Enter follow  Space done  Esc back  r ready", 1)
}

func (m CollectionModel) navigationBudget() int {
	if m.height <= 0 {
		return 20
	}
	rows := 2 + lineCount(m.navigationFooter()) // heading and blank separator
	if m.reloadError != nil || m.message != "" {
		rows += 1 + lineCount(m.status())
	}
	return maxInt(1, m.height-rows)
}

func (m CollectionModel) navigationBody() renderedText {
	f := m.formatter()
	body := renderedText{}
	appendText := func(text string) { body.lines = append(body.lines, strings.Split(text, "\n")...) }
	source, err := m.snapshot.Lookup(m.current)
	if err != nil {
		return body
	}
	node, _ := source.Graph.Node(m.current.ID)
	summary := query.Summarize(source.Graph, node)
	style := nodeStyle
	if node.Complete {
		style = completeStyle
	}
	appendText(f.renderWrapped("", node.Text, style))
	appendText(f.renderWrapped("", "File: "+source.Path, mutedStyle))
	appendText("State: " + string(summary.State))
	if len(summary.BlockedBy) > 0 {
		blockers := make([]string, 0, len(summary.BlockedBy))
		for _, id := range summary.BlockedBy {
			blockers = append(blockers, (collection.Ref{Member: m.current.Member, ID: id}).String())
		}
		appendText(f.renderWrapped("", "Blocked by: "+strings.Join(blockers, ", "), mutedStyle))
	}
	relations := m.relations()
	parents, _ := source.Graph.ParentsOf(m.current.ID)
	children, _ := source.Graph.ChildrenOf(m.current.ID)
	for _, section := range []struct {
		kind, title string
		total       int
	}{{"parent", "Parents", len(parents)}, {"child", "Children", len(children)}} {
		appendText("\n" + titleStyle.Render(section.title))
		count := 0
		for i, relation := range relations {
			if relation.kind != section.kind {
				continue
			}
			node := relation.Summary.Node
			node.Text = fmt.Sprintf("[%s] %s", relation.Summary.State, node.Text)
			start := len(body.lines)
			appendText(f.renderSelectableNode(i == m.relationCursor, node))
			body.spans = append(body.spans, lineSpan{start: start, end: len(body.lines)})
			count++
		}
		if count == 0 {
			text := "  none"
			if section.total > 0 {
				text = "  all complete (v to show)"
			}
			appendText(mutedStyle.Render(text))
		}
	}
	return body
}

func (m CollectionModel) ensureNavigationVisible() CollectionModel {
	if !m.detail {
		return m
	}
	body := m.navigationBody()
	budget := m.navigationBudget()
	if len(body.spans) == 0 {
		m.relationCursor = 0
		m.relationScroll = clampInt(m.relationScroll, 0, maxInt(0, len(body.lines)-budget))
		return m
	}
	m.relationCursor = clampedCursor(m.relationCursor, len(body.spans))
	m.relationScroll = revealSelection(m.relationScroll, len(body.lines), body.spans[m.relationCursor], budget)
	return m
}

func (m CollectionModel) viewNavigation() string {
	f := m.formatter()
	body := m.navigationBody()
	lines := []string{titleStyle.Render(truncateText(m.current.String(), f.contentWidth()))}
	lines = append(lines, viewportLines(body.lines, m.relationScroll, m.navigationBudget())...)
	lines = append(lines, "", m.navigationFooter())
	if m.reloadError != nil || m.message != "" {
		lines = append(lines, "", m.status())
	}
	return f.fitToTerminal(f.padBlock(strings.Join(lines, "\n")))
}
