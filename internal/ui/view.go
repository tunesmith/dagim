package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"dagim/internal/graph"
)

const maxContentWidth = 112

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	selectStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	nodeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Bold(true)
	completeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Strikethrough(true)
	commandStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func (m Model) View() string {
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
	case modeInspect:
		body = m.viewInspect()
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
	if m.message != "" {
		body += "\n\n" + errorStyle.Render(m.message)
	}
	return m.padBlock(body)
}

func (m Model) viewNode() string {
	if m.current == "" {
		return strings.Join([]string{
			titleStyle.Render("No nodes yet."),
			"",
			commandStyle.Render("a add first node    ? help    q quit"),
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
	b.WriteString(renderCommandGrid([][]string{
		{"a add node", "p add parent", "c add child", "x unlink"},
		{"i inspect", "e edit", "d delete", "/ search"},
		{"r ready", "l leaves", "o order", "s save"},
		{"Space done/undone", "v completed", "R reset", "W rewrite"},
		{"J/K reorder", "C check", "q quit", "? help"},
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
	var b strings.Builder
	b.WriteString(titleStyle.Render("Search"))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	b.WriteString("\n\n")
	if len(results) == 0 {
		b.WriteString(mutedStyle.Render("  no matches"))
	} else {
		for i, id := range results {
			node, _ := m.g.Node(id)
			b.WriteString(m.renderSelectableNode(i == m.searchCursor, node))
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n")
	b.WriteString(commandStyle.Render("Enter focus    Up/Down select    Esc cancel"))
	return b.String()
}

func (m Model) viewReady() string {
	ready := m.g.Ready()
	var b strings.Builder
	b.WriteString(titleStyle.Render("Ready"))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	index := 0
	if len(ready) == 0 {
		if len(m.g.Nodes()) > 0 && m.g.CompleteCount() == len(m.g.Nodes()) {
			b.WriteString(mutedStyle.Render("  all complete"))
		} else {
			b.WriteString(mutedStyle.Render("  none ready"))
		}
	} else {
		for _, id := range ready {
			node, _ := m.g.Node(id)
			b.WriteString(m.renderSelectableNode(index == m.readyCursor, node))
			b.WriteByte('\n')
			index++
		}
	}
	if m.showCompleted {
		completed := m.g.Completed()
		b.WriteString("\n\n")
		b.WriteString(titleStyle.Render("Completed"))
		b.WriteByte('\n')
		b.WriteString(m.rule())
		b.WriteByte('\n')
		if len(completed) == 0 {
			b.WriteString(mutedStyle.Render("  none complete"))
		} else {
			for _, id := range completed {
				node, _ := m.g.Node(id)
				b.WriteString(m.renderSelectableNode(index == m.readyCursor, node))
				b.WriteByte('\n')
				index++
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(renderCommandGrid([][]string{
		{"a add node", "Enter focus", "Space done/undone", "/ search"},
		{"i inspect", "v completed", "o order", "l leaves"},
		{"R reset", "W rewrite", "J/K reorder", "s save"},
		{"C check", "q quit", "? help"},
	}))
	return b.String()
}

func (m Model) viewLeaves() string {
	leaves := m.visibleLeaves()
	var b strings.Builder
	b.WriteString(titleStyle.Render("Leaves"))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	if len(leaves) == 0 {
		b.WriteString(mutedStyle.Render("  no leaves"))
	} else {
		for i, id := range leaves {
			node, _ := m.g.Node(id)
			b.WriteString(m.renderSelectableNode(i == m.leavesCursor, node))
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n")
	b.WriteString(renderCommandGrid([][]string{
		{"Enter focus", "Space done/undone", "i inspect", "/ search"},
		{"r ready", "v completed", "o order", "J/K reorder"},
		{"R reset", "W rewrite", "s save", "q quit"},
		{"Esc back", "C check", "? help"},
	}))
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
	b.WriteString(renderCommandGrid([][]string{
		{"Space pick", "Enter inspect", "u undo", "r reset"},
		{"e export", "C check", "Esc/q exit"},
	}))
	return b.String()
}

func (m Model) viewCheck() string {
	stats := m.g.Stats()
	transitive := m.g.TransitiveEdges()
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
	if m.dirty {
		b.WriteString("unsaved changes: yes")
	} else {
		b.WriteString("unsaved changes: no")
	}
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("nodes: %d\n", stats.Nodes))
	b.WriteString(fmt.Sprintf("edges: %d\n", stats.Edges))
	b.WriteString(fmt.Sprintf("complete: %d\n", stats.Complete))
	b.WriteString(fmt.Sprintf("ready: %d\n", stats.Ready))
	b.WriteString(fmt.Sprintf("roots: %d\n", stats.Roots))
	b.WriteString(fmt.Sprintf("leaves: %d\n", stats.Leaves))
	b.WriteString(fmt.Sprintf("transitive edges: %d\n", len(transitive)))
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("Transitive edges"))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	if len(transitive) == 0 {
		b.WriteString(mutedStyle.Render("  none"))
	} else {
		for _, edge := range transitive {
			parent, _ := m.g.Node(edge.Parent)
			child, _ := m.g.Node(edge.Child)
			b.WriteString(m.renderWrapped("", fmt.Sprintf("%s -> %s", parent.Text, child.Text), lipgloss.NewStyle()))
			b.WriteByte('\n')
			b.WriteString(mutedStyle.Render("  via " + formatNodePath(edge.Path)))
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n")
	b.WriteString(commandStyle.Render("Esc back"))
	return b.String()
}

func (m Model) viewInspect() string {
	node, ok := m.g.Node(m.inspectID)
	if !ok {
		return "Node missing.\n\n" + commandStyle.Render("Esc back")
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Inspect"))
	b.WriteByte('\n')
	b.WriteString(m.rule())
	b.WriteByte('\n')
	b.WriteString(m.renderWrapped("", node.Text, lipgloss.NewStyle()))
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render(string(node.ID)))
	if node.Complete {
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("complete"))
	}
	b.WriteString("\n\n")
	b.WriteString(m.renderIDList("Parents", node.ID, m.g.ParentsOf))
	b.WriteByte('\n')
	b.WriteString(m.renderIDList("Children", node.ID, m.g.ChildrenOf))
	b.WriteString("\n")
	b.WriteString(commandStyle.Render("Esc back"))
	return b.String()
}

func (m Model) viewConfirmDelete() string {
	node, _ := m.g.Node(m.current)
	return fmt.Sprintf("%s\n%s\n%s\n\n%s",
		titleStyle.Render("Delete connected node?"),
		m.rule(),
		node.Text,
		commandStyle.Render("y delete    n cancel"))
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
		"This removes every complete marker from the graph and saves nothing yet.",
		"",
		commandStyle.Render("y reset    n cancel"),
	}, "\n")
}

func (m Model) viewConfirmQuit() string {
	return strings.Join([]string{
		titleStyle.Render("Unsaved changes."),
		m.rule(),
		commandStyle.Render("y save and quit    n quit without saving    c cancel"),
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
		"x unlink          i inspect            e edit",
		"d delete          / search             r ready",
		"l leaves          o order remaining    J/K reorder",
		"Space done/undone v completed          s save",
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

func formatNodePath(path []graph.NodeID) string {
	parts := make([]string, 0, len(path))
	for _, id := range path {
		parts = append(parts, string(id))
	}
	return strings.Join(parts, " -> ")
}

func renderCommandGrid(rows [][]string) string {
	maxWidth := 0
	for _, row := range rows {
		for _, cell := range row {
			if len(cell) > maxWidth {
				maxWidth = len(cell)
			}
		}
	}
	cellWidth := maxWidth + 4
	var b strings.Builder
	for rowIndex, row := range rows {
		var line strings.Builder
		for i, cell := range row {
			if i == len(row)-1 {
				line.WriteString(cell)
				continue
			}
			line.WriteString(fmt.Sprintf("%-*s", cellWidth, cell))
		}
		b.WriteString(commandStyle.Render(strings.TrimRight(line.String(), " ")))
		if rowIndex < len(rows)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
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

func renderIDListFor(g *graph.Graph, title string, id graph.NodeID, fn func(graph.NodeID) ([]graph.NodeID, error)) string {
	ids, _ := fn(id)
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteByte('\n')
	b.WriteString(genericRule())
	b.WriteByte('\n')
	if len(ids) == 0 {
		b.WriteString(mutedStyle.Render("  none"))
		b.WriteByte('\n')
		return b.String()
	}
	for _, id := range ids {
		node, _ := g.Node(id)
		b.WriteString("  ")
		b.WriteString(node.Text)
		b.WriteString(" ")
		b.WriteString(mutedStyle.Render("[" + string(node.ID) + "]"))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m Model) renderIDList(title string, id graph.NodeID, fn func(graph.NodeID) ([]graph.NodeID, error)) string {
	return renderIDListFor(m.g, title, id, fn)
}

func (m Model) rule() string {
	return mutedStyle.Render(strings.Repeat("-", m.contentWidth()))
}

func (m Model) strongRule() string {
	return selectStyle.Render(strings.Repeat("=", m.contentWidth()))
}

func genericRule() string {
	return mutedStyle.Render(strings.Repeat("-", 42))
}

func (m Model) contentWidth() int {
	return contentWidthForTerminal(m.width)
}

func contentWidthForTerminal(width int) int {
	margin := leftMarginForTerminal(width)
	if width <= 0 {
		width = 80
	}
	width -= margin + 2
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
