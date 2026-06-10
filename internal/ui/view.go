// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/tunesmith/dagim/internal/graph"
)

const (
	maxContentWidth = 112
	checkFooter     = "j/k scroll    PgUp/PgDn page    Home/End    Esc back"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	selectStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("178")).Bold(true)
	nodeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Bold(true)
	completeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Strikethrough(true)
	commandStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func (m Model) View() string {
	body := m.viewBody()
	if m.message != "" {
		body += "\n\n" + errorStyle.Render(m.message)
	}
	if m.usesGlobalViewport() {
		body = m.renderGlobalViewport(body)
	}
	return m.fitToTerminal(m.padBlock(body))
}

func (m Model) viewBody() string {
	var body string
	switch m.mode {
	case modeNode:
		body = m.viewNode()
	case modePrompt:
		body = m.viewPrompt()
	case modeSearch:
		body = m.viewSearch()
	case modeReady:
		body = m.viewReady()
	case modeLeaves:
		body = m.viewLeaves()
	case modeOrder:
		body = m.viewOrder()
	case modeCheck:
		body = m.viewCheck()
	case modeConfirmDelete:
		body = m.viewConfirmDelete()
	case modeConfirmRewrite:
		body = m.viewConfirmRewrite()
	case modeConfirmReset:
		body = m.viewConfirmReset()
	case modeConfirmQuit:
		body = m.viewConfirmQuit()
	case modeHelp:
		body = m.viewHelp()
	default:
		body = m.viewNode()
	}
	return body
}

func (m Model) viewNode() string {
	if m.current == "" {
		return strings.Join([]string{
			titleStyle.Render("No nodes yet."),
			"",
			commandStyle.Render("a add first node    u undo    ? help    q quit"),
		}, "\n")
	}
	node, ok := m.g.Node(m.current)
	if !ok {
		return "Current node is missing."
	}
	var b strings.Builder
	cursor := 0
	parents, _ := m.g.ParentsOf(node.ID)
	b.WriteString(titleStyle.Render("Parents"))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	renderedParents := 0
	for _, id := range parents {
		parent, _ := m.g.Node(id)
		if parent.Complete && !m.showCompleted {
			continue
		}
		b.WriteString(m.renderSelectableNode(cursor == m.cursor, parent))
		b.WriteByte('\n')
		cursor++
		renderedParents++
	}
	if renderedParents == 0 {
		b.WriteString(mutedStyle.Render("  none"))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	title := "Current node"
	if node.Complete {
		title += " [complete]"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteByte('\n')
	b.WriteString(m.strongRule())
	b.WriteByte('\n')
	style := nodeStyle
	if node.Complete {
		style = completeStyle
	}
	b.WriteString(m.renderWrapped("", node.Text, style))
	b.WriteString("\n\n")

	children, _ := m.g.ChildrenOf(node.ID)
	b.WriteString(titleStyle.Render("Children"))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	renderedChildren := 0
	for _, id := range children {
		child, _ := m.g.Node(id)
		if child.Complete && !m.showCompleted {
			continue
		}
		b.WriteString(m.renderSelectableNode(cursor == m.cursor, child))
		b.WriteByte('\n')
		cursor++
		renderedChildren++
	}
	if renderedChildren == 0 {
		b.WriteString(mutedStyle.Render("  none"))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("Commands"))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	b.WriteString(m.renderCommandGrid([][]string{
		{"a add node", "p add parent", "c add child", "x unlink selected"},
		{"e edit", "d delete node", "/ search", "r ready"},
		{"l leaves", "o order", "C check", "u undo"},
		{"Space done/undone", "v completed", "R reset", "W rewrite"},
		{"J/K reorder", "q quit", "? help"},
	}))
	return b.String()
}

func (m Model) viewPrompt() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.promptTitle))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	if m.promptAction == promptAddParent || m.promptAction == promptAddChild {
		results := m.promptMatches()
		b.WriteString("\n\n")
		matchTitle := "Matches"
		if len(results) > 0 {
			start, end := m.promptMatchWindow(len(results))
			matchTitle = fmt.Sprintf("Matches %d-%d of %d", start+1, end, len(results))
		}
		b.WriteString(titleStyle.Render(matchTitle))
		b.WriteByte('\n')
		b.WriteString(m.rule())
		b.WriteByte('\n')
		if len(results) == 0 {
			if len(m.searchResults(m.input.Value())) == 0 {
				b.WriteString(mutedStyle.Render("  no matches; Enter creates a new node"))
			} else {
				b.WriteString(mutedStyle.Render("  no eligible matches; existing links or cycles are hidden"))
			}
		} else {
			start, end := m.promptMatchWindow(len(results))
			for i := start; i < end; i++ {
				id := results[i]
				node, _ := m.g.Node(id)
				b.WriteString(m.renderSelectableLine(i == m.suggestionCursor, node.Text))
				b.WriteByte('\n')
			}
			b.WriteString(mutedStyle.Render("Enter links selected match; Ctrl+N creates typed text"))
		}
	}
	b.WriteString("\n\n")
	b.WriteString(commandStyle.Render("Enter accept    Esc cancel"))
	if m.promptAction == promptAddParent || m.promptAction == promptAddChild {
		b.WriteString(commandStyle.Render("    Up/Down select    Ctrl+N create"))
	}
	return b.String()
}

func (m Model) viewSearch() string {
	results := m.searchResults(m.input.Value())
	start, end := m.searchResultWindow(len(results))
	title := "Search"
	if len(results) > end-start {
		title = fmt.Sprintf("Search %d-%d of %d", start+1, end, len(results))
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	b.WriteString("\n\n")
	if len(results) == 0 {
		b.WriteString(mutedStyle.Render("  no matches"))
	} else {
		for i := start; i < end; i++ {
			id := results[i]
			node, _ := m.g.Node(id)
			b.WriteString(m.renderSelectableLine(i == m.searchCursor, node.Text))
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n")
	b.WriteString(commandStyle.Render("Enter focus    Up/Down select    Esc cancel"))
	return b.String()
}

func (m Model) viewReady() string {
	items := m.readyItems()
	cursor := clampedCursor(m.readyCursor, len(items))
	footer := m.readyFooter()
	rendered := make([]string, len(items))
	for i, item := range items {
		node, _ := m.g.Node(item.id)
		rendered[i] = m.renderSelectableNode(i == cursor, node)
	}
	start, end := lineWindowForRendered(rendered, cursor, m.listItemLimitForFooter(footer))
	title := "Ready"
	if m.showCompleted {
		title = "Ready + Completed"
	}
	title = listTitle(title, start, end, len(items))
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	if len(items) == 0 {
		if len(m.g.Nodes()) > 0 && m.g.CompleteCount() == len(m.g.Nodes()) {
			b.WriteString(mutedStyle.Render("  all complete"))
		} else {
			b.WriteString(mutedStyle.Render("  none ready"))
		}
	} else {
		for i := start; i < end; i++ {
			b.WriteString(rendered[i])
			b.WriteByte('\n')
		}
	}
	if len(items) == 0 {
		b.WriteByte('\n')
	}
	b.WriteString(footer)
	return b.String()
}

func (m Model) viewLeaves() string {
	leaves := m.visibleLeaves()
	cursor := clampedCursor(m.leavesCursor, len(leaves))
	footer := m.leavesFooter()
	rendered := make([]string, len(leaves))
	for i, id := range leaves {
		node, _ := m.g.Node(id)
		rendered[i] = m.renderSelectableNode(i == cursor, node)
	}
	start, end := lineWindowForRendered(rendered, cursor, m.listItemLimitForFooter(footer))
	title := listTitle("Leaves", start, end, len(leaves))
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	if len(leaves) == 0 {
		b.WriteString(mutedStyle.Render("  no leaves"))
	} else {
		for i := start; i < end; i++ {
			b.WriteString(rendered[i])
			b.WriteByte('\n')
		}
	}
	if len(leaves) == 0 {
		b.WriteByte('\n')
	}
	b.WriteString(footer)
	return b.String()
}

func (m Model) viewOrder() string {
	if m.order == nil {
		return "No order state."
	}
	var b strings.Builder
	if m.order.Complete() {
		b.WriteString(titleStyle.Render("Order complete"))
	} else {
		b.WriteString(titleStyle.Render("Order remaining"))
	}
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	output := m.order.Output()
	if len(output) == 0 {
		b.WriteString(mutedStyle.Render("  none yet"))
		b.WriteByte('\n')
	} else {
		for i, id := range output {
			node, _ := m.g.Node(id)
			b.WriteString(m.renderWrapped(fmt.Sprintf("%d. ", i+1), node.Text, lipgloss.NewStyle()))
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("Available now"))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	available := m.order.Available()
	if len(available) == 0 {
		if m.order.Complete() {
			b.WriteString(mutedStyle.Render("  complete"))
		} else {
			b.WriteString(mutedStyle.Render("  none available"))
		}
		b.WriteByte('\n')
	} else {
		for i, id := range available {
			node, _ := m.g.Node(id)
			b.WriteString(m.renderSelectable(i == m.cursor, node.Text))
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(m.renderCommandGrid([][]string{
		{"j/k move", "Space pick", "u undo", "r reset"},
		{"e export", "C check", "Esc/q exit"},
	}))
	return b.String()
}

func (m Model) viewCheck() string {
	body := m.checkBody()
	footer := "Esc back"
	if m.scrollMax(body, checkFooter) > 0 {
		footer = checkFooter
	}
	return m.renderScrollable(body, m.checkScroll, footer)
}

func (m Model) checkBody() string {
	stats := m.g.Stats()
	transitive := m.g.TransitiveEdges()
	rekeyMapping, rekeyChanged, rekeyErr := m.rekeyChanges()
	rekeyChanges := rekeyChangesInOrder(m.g.Order(), rekeyMapping)
	var b strings.Builder
	b.WriteString(titleStyle.Render("Check"))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	if err := m.g.Validate(); err != nil {
		b.WriteString(errorStyle.Render("invalid: " + err.Error()))
	} else {
		b.WriteString("valid: yes")
	}
	b.WriteByte('\n')
	if rekeyErr != nil {
		b.WriteString(errorStyle.Render("rewrite check failed: " + rekeyErr.Error()))
	} else {
		b.WriteString(fmt.Sprintf("W rewrite ID changes: %d", rekeyChanged))
	}
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("nodes: %d\n", stats.Nodes))
	b.WriteString(fmt.Sprintf("edges: %d\n", stats.Edges))
	b.WriteString(fmt.Sprintf("complete: %d\n", stats.Complete))
	b.WriteString(fmt.Sprintf("ready: %d\n", stats.Ready))
	b.WriteString(fmt.Sprintf("roots: %d\n", stats.Roots))
	b.WriteString(fmt.Sprintf("leaves: %d\n", stats.Leaves))
	b.WriteString(fmt.Sprintf("transitive edges: %d\n", len(transitive)))
	if len(rekeyChanges) > 0 {
		b.WriteByte('\n')
		b.WriteString(titleStyle.Render("W rewrite ID changes"))
		b.WriteByte('\n')
		b.WriteString(m.rule())
		b.WriteByte('\n')
		for _, change := range rekeyChanges {
			b.WriteString(m.renderWrapped("  ", string(change.oldID), mutedStyle))
			b.WriteByte('\n')
			b.WriteString(m.renderWrappedWithContinuation("    -> ", string(change.newID), mutedStyle, 7))
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("Transitive edges"))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	if len(transitive) == 0 {
		b.WriteString(mutedStyle.Render("  none"))
	} else {
		for i, edge := range transitive {
			parent, _ := m.g.Node(edge.Parent)
			child, _ := m.g.Node(edge.Child)
			b.WriteString(m.renderWrapped(fmt.Sprintf("%d. ", i+1), parent.Text, lipgloss.NewStyle()))
			b.WriteByte('\n')
			b.WriteString(m.renderWrappedWithContinuation("   -> ", child.Text, lipgloss.NewStyle(), 6))
			b.WriteByte('\n')
			b.WriteString(mutedStyle.Render(fmt.Sprintf("   via alternate path (%d edges)", len(edge.Path)-1)))
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n")
	return b.String()
}

func (m Model) viewConfirmDelete() string {
	node, _ := m.g.Node(m.current)
	return fmt.Sprintf("%s\n%s\nCurrent node:\n%s\n\nThis removes the node and all links to and from it.\n\n%s",
		titleStyle.Render("Delete current node?"),
		m.rule(),
		node.Text,
		commandStyle.Render("y delete node    n cancel"))
}

func (m Model) viewConfirmRewrite() string {
	return strings.Join([]string{
		titleStyle.Render("Rewrite file?"),
		m.rule(),
		"This regenerates node IDs from current text, updates parent references,",
		"writes the current canonical format, and saves the file.",
		"",
		commandStyle.Render("y rewrite and save    n cancel"),
	}, "\n")
}

func (m Model) viewConfirmReset() string {
	return strings.Join([]string{
		titleStyle.Render("Reset completion?"),
		m.rule(),
		"This removes every complete marker from the graph and autosaves.",
		"",
		commandStyle.Render("y reset    n cancel"),
	}, "\n")
}

func (m Model) viewConfirmQuit() string {
	return strings.Join([]string{
		titleStyle.Render("Autosave failed."),
		m.rule(),
		"Some changes are only in memory.",
		"",
		commandStyle.Render("y retry save and quit    n quit without saving    c cancel"),
	}, "\n")
}

func (m Model) viewHelp() string {
	return strings.Join([]string{
		titleStyle.Render("Help"),
		m.rule(),
		"dagim edits one plain-text DAG file.",
		"",
		"Non-empty files open to ready. Enter focuses a selected node.",
		"",
		"a add node        p add/link parent    c add/link child",
		"x unlink selected e edit              d delete node",
		"/ search          r ready             l leaves",
		"o order remaining J/K reorder",
		"Space done/undone u undo              v completed",
		"R reset done      W rewrite file       C check",
		"q quit",
		"ctrl+c force quit ctrl+z suspend",
		"",
		"In parent/child prompts, Enter links the selected match and Ctrl+N creates the typed text.",
		"Order remaining is temporary; export writes one node text per line.",
		"",
		commandStyle.Render("Esc back"),
	}, "\n")
}

type rekeyChange struct {
	oldID graph.NodeID
	newID graph.NodeID
}

func rekeyChangesInOrder(order []graph.NodeID, mapping map[graph.NodeID]graph.NodeID) []rekeyChange {
	changes := make([]rekeyChange, 0)
	for _, oldID := range order {
		newID, ok := mapping[oldID]
		if ok && oldID != newID {
			changes = append(changes, rekeyChange{oldID: oldID, newID: newID})
		}
	}
	return changes
}

func (m Model) renderCommandGrid(rows [][]string) string {
	return renderCommandGrid(rows, m.contentWidth())
}

func (m Model) readyFooter() string {
	return m.renderCommandGrid([][]string{
		{"j/k move", "PgUp/PgDn page", "Enter focus", "Space done"},
		{"d delete", "/ search", "v completed", "l leaves"},
		{"o order", "R reset", "W rewrite", "C check"},
		{"u undo", "q quit", "? help"},
	})
}

func (m Model) leavesFooter() string {
	return m.renderCommandGrid([][]string{
		{"j/k move", "PgUp/PgDn page", "Enter focus", "Space done"},
		{"r ready", "/ search", "v completed", "o order"},
		{"J/K reorder", "R reset", "W rewrite", "C check"},
		{"u undo", "q quit", "Esc back", "? help"},
	})
}

func renderCommandGrid(rows [][]string, width int) string {
	const gap = 4
	var cells []string
	maxColumns := 0
	for _, row := range rows {
		if len(row) > maxColumns {
			maxColumns = len(row)
		}
		cells = append(cells, row...)
	}
	if len(cells) == 0 {
		return ""
	}
	if width <= 0 {
		width = 80
	}
	columns := commandGridColumns(cells, maxColumns, width, gap)
	widths := commandGridColumnWidths(cells, columns)
	var b strings.Builder
	for start := 0; start < len(cells); start += columns {
		var line strings.Builder
		end := start + columns
		if end > len(cells) {
			end = len(cells)
		}
		for i := start; i < end; i++ {
			column := i - start
			if column > 0 {
				line.WriteString(strings.Repeat(" ", gap))
			}
			cell := cells[i]
			if i < end-1 {
				line.WriteString(fmt.Sprintf("%-*s", widths[column], cell))
			} else {
				line.WriteString(cell)
			}
		}
		b.WriteString(commandStyle.Render(strings.TrimRight(line.String(), " ")))
		if end < len(cells) {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func commandGridColumns(cells []string, maxColumns, width, gap int) int {
	if maxColumns < 1 {
		maxColumns = 1
	}
	for columns := maxColumns; columns > 1; columns-- {
		widths := commandGridColumnWidths(cells, columns)
		total := gap * (columns - 1)
		for _, w := range widths {
			total += w
		}
		if total <= width {
			return columns
		}
	}
	return 1
}

func commandGridColumnWidths(cells []string, columns int) []int {
	widths := make([]int, columns)
	for i, cell := range cells {
		column := i % columns
		if len(cell) > widths[column] {
			widths[column] = len(cell)
		}
	}
	return widths
}

func (m Model) renderSelectable(selected bool, text string) string {
	return m.renderSelectableText(selected, text, false)
}

func (m Model) renderSelectableNode(selected bool, node graph.Node) string {
	return m.renderSelectableText(selected, node.Text, node.Complete)
}

func (m Model) renderSelectableText(selected bool, text string, complete bool) string {
	prefix := "  "
	style := lipgloss.NewStyle()
	if complete {
		text = "[x] " + text
		style = completeStyle
	}
	if selected {
		prefix = "> "
		style = selectStyle
	}
	return m.renderWrappedWithContinuation(prefix, text, style, len(prefix)+2)
}

func (m Model) renderSelectableLine(selected bool, text string) string {
	prefix := "  "
	style := lipgloss.NewStyle()
	if selected {
		prefix = "> "
		style = selectStyle
	}
	width := m.contentWidth() - len(prefix)
	if width < 12 {
		width = 12
	}
	return prefix + style.Render(truncateText(text, width))
}

func (m Model) rule() string {
	return mutedStyle.Render(strings.Repeat("-", m.contentWidth()))
}

func (m Model) strongRule() string {
	return selectStyle.Render(strings.Repeat("=", m.contentWidth()))
}

func (m Model) contentWidth() int {
	return contentWidthForTerminal(m.width)
}

func contentWidthForTerminal(width int) int {
	margin := leftMarginForTerminal(width)
	if width <= 0 {
		width = 80
	}
	width -= margin + 4
	if width > maxContentWidth {
		width = maxContentWidth
	}
	if width < 24 {
		return 24
	}
	return width
}

func (m Model) padBlock(text string) string {
	margin := leftMarginForTerminal(m.width)
	if margin == 0 || text == "" {
		return text
	}
	pad := strings.Repeat(" ", margin)
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}

func (m Model) usesGlobalViewport() bool {
	switch m.mode {
	case modeReady, modeLeaves, modeSearch, modePrompt, modeCheck:
		return false
	default:
		return true
	}
}

func (m Model) renderGlobalViewport(body string) string {
	if m.height <= 0 {
		return body
	}
	footer := ""
	if m.globalScrollMaxFor(body) > 0 {
		footer = "PgUp/PgDn scroll    Home/End"
	}
	return m.renderScrollable(body, m.viewScroll, footer)
}

func (m Model) globalScrollMax() int {
	if !m.usesGlobalViewport() || m.height <= 0 {
		return 0
	}
	body := m.viewBody()
	if m.message != "" {
		body += "\n\n" + errorStyle.Render(m.message)
	}
	return m.globalScrollMaxFor(body)
}

func (m Model) globalScrollMaxFor(body string) int {
	if m.height <= 0 {
		return 0
	}
	footer := ""
	if scrollMaxForLines(strings.Split(body, "\n"), m.height) > 0 {
		footer = "PgUp/PgDn scroll    Home/End"
	}
	return scrollMaxForLines(strings.Split(body, "\n"), m.scrollBodyHeight(footer))
}

func (m Model) globalScrollPageSize() int {
	size := m.scrollBodyHeight("PgUp/PgDn scroll    Home/End") - 1
	if size < 1 {
		return 1
	}
	return size
}

func (m Model) fitToTerminal(text string) string {
	lines := strings.Split(text, "\n")
	if m.width > 0 {
		width := m.width - 1
		if width < 1 {
			width = 1
		}
		style := lipgloss.NewStyle().MaxWidth(width)
		for i := range lines {
			lines[i] = style.Render(lines[i])
		}
	}
	if m.height > 0 && len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderScrollable(body string, offset int, footer string) string {
	if m.height <= 0 {
		return body + "\n\n" + commandStyle.Render("Esc back")
	}
	lines := strings.Split(body, "\n")
	bodyHeight := m.scrollBodyHeight(footer)
	maxScroll := scrollMaxForLines(lines, bodyHeight)
	offset = clampInt(offset, 0, maxScroll)
	end := offset + bodyHeight
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	for i := offset; i < end; i++ {
		if i > offset {
			b.WriteByte('\n')
		}
		b.WriteString(lines[i])
	}
	if footer != "" && m.height > bodyHeight {
		b.WriteByte('\n')
		b.WriteString(commandStyle.Render(footer))
	}
	return b.String()
}

func (m Model) checkScrollMax() int {
	return m.scrollMax(m.checkBody(), checkFooter)
}

func (m Model) checkScrollPageSize() int {
	size := m.scrollBodyHeight(checkFooter) - 1
	if size < 1 {
		return 1
	}
	return size
}

func (m Model) scrollMax(body, footer string) int {
	if m.height <= 0 {
		return 0
	}
	return scrollMaxForLines(strings.Split(body, "\n"), m.scrollBodyHeight(footer))
}

func (m Model) scrollBodyHeight(footer string) int {
	if m.height <= 0 {
		return 0
	}
	bodyHeight := m.height
	if footer != "" {
		bodyHeight--
	}
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	return bodyHeight
}

func scrollMaxForLines(lines []string, bodyHeight int) int {
	maxScroll := len(lines) - bodyHeight
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func clampedCursor(cursor, total int) int {
	if total <= 0 {
		return 0
	}
	return clampInt(cursor, 0, total-1)
}

func listTitle(base string, start, end, total int) string {
	if total > end-start {
		return fmt.Sprintf("%s %d-%d of %d", base, start+1, end, total)
	}
	return base
}

func (m Model) listItemLimitForFooter(footer string) int {
	if m.height <= 0 {
		return m.listItemLimit()
	}
	limit := m.height - 2 - lineCount(footer) // title and rule
	if limit < 1 {
		return 1
	}
	return limit
}

func lineWindowForRendered(rendered []string, cursor, limit int) (int, int) {
	heights := make([]int, len(rendered))
	for i, text := range rendered {
		heights[i] = lineCount(text)
	}
	return lineWindowAroundCursor(heights, cursor, limit)
}

func lineWindowAroundCursor(heights []int, cursor, limit int) (int, int) {
	total := len(heights)
	if total == 0 {
		return 0, 0
	}
	cursor = clampedCursor(cursor, total)
	if limit < 1 {
		limit = 1
	}
	start := cursor
	end := cursor + 1
	used := heights[cursor]
	beforeTarget := limit / 2
	beforeUsed := 0
	for start > 0 && beforeUsed+heights[start-1] <= beforeTarget && used+heights[start-1] <= limit {
		start--
		beforeUsed += heights[start]
		used += heights[start]
	}
	for end < total && used+heights[end] <= limit {
		used += heights[end]
		end++
	}
	for start > 0 && used+heights[start-1] <= limit {
		start--
		used += heights[start]
	}
	return start, end
}

func lineCount(text string) int {
	if text == "" {
		return 1
	}
	return strings.Count(text, "\n") + 1
}

func leftMarginForTerminal(width int) int {
	switch {
	case width >= maxContentWidth+30:
		return 4
	case width >= 90:
		return 2
	default:
		return 0
	}
}

func (m Model) renderWrapped(prefix, text string, style lipgloss.Style) string {
	return m.renderWrappedWithContinuation(prefix, text, style, len(prefix))
}

func (m Model) renderWrappedWithContinuation(prefix, text string, style lipgloss.Style, continuationIndent int) string {
	width := m.contentWidth() - len(prefix)
	if width < 12 {
		width = 12
	}
	lines := wrapWords(text, width)
	if len(lines) == 0 {
		lines = []string{""}
	}
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(" ", continuationIndent))
		} else {
			b.WriteString(prefix)
		}
		b.WriteString(style.Render(line))
	}
	return b.String()
}

func wrapWords(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var line strings.Builder
	for _, word := range words {
		if line.Len() == 0 {
			for len(word) > width {
				lines = append(lines, word[:width])
				word = word[width:]
			}
			line.WriteString(word)
			continue
		}
		if line.Len()+1+len(word) <= width {
			line.WriteByte(' ')
			line.WriteString(word)
			continue
		}
		lines = append(lines, line.String())
		line.Reset()
		for len(word) > width {
			lines = append(lines, word[:width])
			word = word[width:]
		}
		line.WriteString(word)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return lines
}

func truncateText(text string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
