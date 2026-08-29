// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import "strings"

type lineSpan struct {
	start int
	end   int
}

type renderedText struct {
	lines []string
	spans []lineSpan
}

func renderedItems(items []string) renderedText {
	result := renderedText{lines: []string{}, spans: make([]lineSpan, 0, len(items))}
	for _, item := range items {
		start := len(result.lines)
		result.lines = append(result.lines, strings.Split(item, "\n")...)
		result.spans = append(result.spans, lineSpan{start: start, end: len(result.lines)})
	}
	return result
}

func revealSelection(offset, lineCount int, span lineSpan, budget int) int {
	budget = maxInt(1, budget)
	maxOffset := maxInt(0, lineCount-budget)
	offset = clampInt(offset, 0, maxOffset)
	if span.end <= span.start {
		return offset
	}
	if span.start < offset {
		offset = span.start
	} else if span.end > offset+budget {
		if span.end-span.start > budget {
			offset = span.start
		} else {
			offset = span.end - budget
		}
	}
	return clampInt(offset, 0, maxOffset)
}

func pageSelection(cursor int, spans []lineSpan, budget, direction int) int {
	if len(spans) == 0 {
		return 0
	}
	cursor = clampedCursor(cursor, len(spans))
	distance := maxInt(1, budget-1)
	target := cursor
	if direction > 0 {
		line := spans[cursor].start + distance
		for i := cursor + 1; i < len(spans); i++ {
			if spans[i].start > line && target > cursor {
				break
			}
			target = i
			if spans[i].start > line {
				break
			}
		}
		if target == cursor && cursor < len(spans)-1 {
			target++
		}
		return target
	}
	line := spans[cursor].end - distance
	for i := cursor - 1; i >= 0; i-- {
		if spans[i].end < line && target < cursor {
			break
		}
		target = i
		if spans[i].end < line {
			break
		}
	}
	if target == cursor && cursor > 0 {
		target--
	}
	return target
}

func viewportLines(lines []string, offset, budget int) []string {
	if len(lines) == 0 {
		return nil
	}
	budget = maxInt(1, budget)
	offset = clampInt(offset, 0, maxInt(0, len(lines)-budget))
	end := minInt(len(lines), offset+budget)
	return lines[offset:end]
}

func visibleSpanRange(spans []lineSpan, offset, budget int) (int, int) {
	start, end := -1, -1
	for i, span := range spans {
		if span.end <= offset || span.start >= offset+budget {
			continue
		}
		if start < 0 {
			start = i
		}
		end = i + 1
	}
	if start < 0 {
		return 0, 0
	}
	return start, end
}

func (m Model) scrollOffset() int {
	switch m.mode {
	case modeNode:
		return m.nodeScroll
	case modePrompt:
		return m.promptScroll
	case modeSearch:
		return m.searchScroll
	case modeReady:
		return m.readyScroll
	case modeLeaves:
		return m.leavesScroll
	case modeOrder:
		return m.orderScroll
	case modeCheck:
		return m.checkScroll
	case modeHelp:
		return m.helpScroll
	default:
		return 0
	}
}

func (m Model) withScrollOffset(offset int) Model {
	switch m.mode {
	case modeNode:
		m.nodeScroll = offset
	case modePrompt:
		m.promptScroll = offset
	case modeSearch:
		m.searchScroll = offset
	case modeReady:
		m.readyScroll = offset
	case modeLeaves:
		m.leavesScroll = offset
	case modeOrder:
		m.orderScroll = offset
	case modeCheck:
		m.checkScroll = offset
	case modeHelp:
		m.helpScroll = offset
	}
	return m
}

func (m Model) readyRenderedItems() (renderedText, int) {
	items := m.readyItems()
	rendered := make([]string, len(items))
	for i, item := range items {
		node, _ := m.g.Node(item.id)
		rendered[i] = m.renderSelectableNode(i == clampedCursor(m.readyCursor, len(items)), node)
	}
	footer := m.responsiveFooter(m.readyFooter(), "j/k move  PgUp/PgDn page  Enter focus  q quit", 2)
	return renderedItems(rendered), m.listItemLimitForFooter(footer)
}

func (m Model) leavesRenderedItems() (renderedText, int) {
	leaves := m.visibleLeaves()
	rendered := make([]string, len(leaves))
	for i, id := range leaves {
		node, _ := m.g.Node(id)
		rendered[i] = m.renderSelectableNode(i == clampedCursor(m.leavesCursor, len(leaves)), node)
	}
	footer := m.responsiveFooter(m.leavesFooter(), "j/k move  PgUp/PgDn page  Enter focus  Esc back", 2)
	return renderedItems(rendered), m.listItemLimitForFooter(footer)
}

func (m Model) searchRenderedItems() (renderedText, int) {
	results := m.searchResults(m.input.Value())
	rendered := make([]string, len(results))
	for i, id := range results {
		node, _ := m.g.Node(id)
		rendered[i] = m.renderSelectableLine(i == clampedCursor(m.searchCursor, len(results)), node.Text)
	}
	return renderedItems(rendered), m.searchResultLimit()
}

func (m Model) promptRenderedItems() (renderedText, int) {
	results := m.promptMatches()
	rendered := make([]string, len(results))
	for i, id := range results {
		node, _ := m.g.Node(id)
		rendered[i] = m.renderSelectableLine(i == clampedCursor(m.suggestionCursor, len(results)), node.Text)
	}
	return renderedItems(rendered), m.promptMatchLimit()
}

func (m Model) relationRenderedItems(items []relationItem) (renderedText, int) {
	rendered := make([]string, len(items))
	for i, item := range items {
		node, _ := m.g.Node(item.id)
		rendered[i] = m.renderSelectableNode(i == clampedCursor(m.cursor, len(items)), node)
	}
	return renderedItems(rendered), m.globalScrollPageSize() + 1
}

func (m Model) orderRenderedItems() (renderedText, int) {
	if m.order == nil {
		return renderedText{}, 1
	}
	available := m.order.Available()
	rendered := make([]string, len(available))
	for i, id := range available {
		node, _ := m.g.Node(id)
		rendered[i] = m.renderSelectable(i == clampedCursor(m.cursor, len(available)), node.Text)
	}
	return renderedItems(rendered), m.globalScrollPageSize() + 1
}

func (m Model) ensureSelectionVisible() Model {
	var body renderedText
	var budget, cursor int
	switch m.mode {
	case modeReady:
		body, budget = m.readyRenderedItems()
		cursor = m.readyCursor
	case modeLeaves:
		body, budget = m.leavesRenderedItems()
		cursor = m.leavesCursor
	case modeSearch:
		body, budget = m.searchRenderedItems()
		cursor = m.searchCursor
	case modePrompt:
		if m.promptAction != promptAddParent && m.promptAction != promptAddChild {
			return m
		}
		body, budget = m.promptRenderedItems()
		cursor = m.suggestionCursor
	default:
		return m.ensureGlobalSelectionVisible()
	}
	if len(body.spans) == 0 {
		return m.withScrollOffset(0)
	}
	cursor = clampedCursor(cursor, len(body.spans))
	offset := revealSelection(m.scrollOffset(), len(body.lines), body.spans[cursor], budget)
	return m.withScrollOffset(offset)
}

func (m Model) pageReadyCursor(direction int) int {
	body, budget := m.readyRenderedItems()
	return pageSelection(m.readyCursor, body.spans, budget, direction)
}

func (m Model) pageLeavesCursor(direction int) int {
	body, budget := m.leavesRenderedItems()
	return pageSelection(m.leavesCursor, body.spans, budget, direction)
}

func (m Model) pageSearchCursor(direction int) int {
	body, budget := m.searchRenderedItems()
	return pageSelection(m.searchCursor, body.spans, budget, direction)
}

func (m Model) pagePromptCursor(direction int) int {
	body, budget := m.promptRenderedItems()
	return pageSelection(m.suggestionCursor, body.spans, budget, direction)
}

func (m Model) pageRelationCursor(items []relationItem, direction int) int {
	body, budget := m.relationRenderedItems(items)
	return pageSelection(m.cursor, body.spans, budget, direction)
}

func (m Model) pageOrderCursor(direction int) int {
	body, budget := m.orderRenderedItems()
	return pageSelection(m.cursor, body.spans, budget, direction)
}
