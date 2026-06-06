package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"dagim/internal/graph"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	selectStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	nodeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Bold(true)
	commandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
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
	case modeRoots:
		body = m.viewRoots()
	case modeSequence:
		body = m.viewSequence()
	case modeInspect:
		body = m.viewInspect()
	case modeConfirmDelete:
		body = m.viewConfirmDelete()
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
	return body
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
	b.WriteString(rule())
	b.WriteByte('\n')
	if len(parents) == 0 {
		b.WriteString(mutedStyle.Render("  none"))
		b.WriteByte('\n')
	} else {
		for _, id := range parents {
			parent, _ := m.g.Node(id)
			b.WriteString(renderSelectable(cursor == m.cursor, parent.Text, parent.ID))
			b.WriteByte('\n')
			cursor++
		}
	}
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("Current node"))
	b.WriteByte('\n')
	b.WriteString(strongRule())
	b.WriteByte('\n')
	b.WriteString(nodeStyle.Render(node.Text))
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render(string(node.ID)))
	b.WriteString("\n\n")

	children, _ := m.g.ChildrenOf(node.ID)
	b.WriteString(titleStyle.Render("Children"))
	b.WriteByte('\n')
	b.WriteString(rule())
	b.WriteByte('\n')
	if len(children) == 0 {
		b.WriteString(mutedStyle.Render("  none"))
		b.WriteByte('\n')
	} else {
		for _, id := range children {
			child, _ := m.g.Node(id)
			b.WriteString(renderSelectable(cursor == m.cursor, child.Text, child.ID))
			b.WriteByte('\n')
			cursor++
		}
	}
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("Commands"))
	b.WriteByte('\n')
	b.WriteString(rule())
	b.WriteByte('\n')
	b.WriteString(commandStyle.Render("a add node    p add parent    c add child    x unlink"))
	b.WriteByte('\n')
	b.WriteString(commandStyle.Render("r rename      d delete        / search       m sequence"))
	b.WriteByte('\n')
	b.WriteString(commandStyle.Render("f roots       J/K reorder     w save         q quit    ? help"))
	return b.String()
}

func (m Model) viewPrompt() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.promptTitle))
	b.WriteByte('\n')
	b.WriteString(rule())
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	if m.promptAction == promptAddParent || m.promptAction == promptAddChild {
		results := m.searchResults(m.input.Value())
		b.WriteString("\n\n")
		b.WriteString(titleStyle.Render("Matches"))
		b.WriteByte('\n')
		b.WriteString(rule())
		b.WriteByte('\n')
		if len(results) == 0 {
			b.WriteString(mutedStyle.Render("  no matches; Enter creates a new node"))
		} else {
			for i, id := range results {
				node, _ := m.g.Node(id)
				b.WriteString(renderSelectable(i == m.suggestionCursor, node.Text, node.ID))
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
	b.WriteString(rule())
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	b.WriteString("\n\n")
	if len(results) == 0 {
		b.WriteString(mutedStyle.Render("  no matches"))
	} else {
		for i, id := range results {
			node, _ := m.g.Node(id)
			b.WriteString(renderSelectable(i == m.searchCursor, node.Text, node.ID))
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n")
	b.WriteString(commandStyle.Render("Enter focus    Up/Down select    Esc cancel"))
	return b.String()
}

func (m Model) viewRoots() string {
	roots := m.g.Roots()
	var b strings.Builder
	b.WriteString(titleStyle.Render("Roots"))
	b.WriteByte('\n')
	b.WriteString(rule())
	b.WriteByte('\n')
	if len(roots) == 0 {
		b.WriteString(mutedStyle.Render("  no roots"))
	} else {
		for i, id := range roots {
			node, _ := m.g.Node(id)
			b.WriteString(renderSelectable(i == m.rootsCursor, node.Text, node.ID))
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n")
	b.WriteString(commandStyle.Render("Enter focus    Up/Down select    / search    Esc back"))
	return b.String()
}

func (m Model) viewSequence() string {
	if m.seq == nil {
		return "No sequence state."
	}
	var b strings.Builder
	if m.seq.Complete() {
		b.WriteString(titleStyle.Render("Sequence complete"))
	} else {
		b.WriteString(titleStyle.Render("Manual sequence"))
	}
	b.WriteByte('\n')
	b.WriteString(rule())
	b.WriteByte('\n')
	output := m.seq.Output()
	if len(output) == 0 {
		b.WriteString(mutedStyle.Render("  none yet"))
		b.WriteByte('\n')
	} else {
		for i, id := range output {
			node, _ := m.g.Node(id)
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, node.Text))
		}
	}
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("Available now"))
	b.WriteByte('\n')
	b.WriteString(rule())
	b.WriteByte('\n')
	available := m.seq.Available()
	if len(available) == 0 {
		if m.seq.Complete() {
			b.WriteString(mutedStyle.Render("  complete"))
		} else {
			b.WriteString(mutedStyle.Render("  none available"))
		}
		b.WriteByte('\n')
	} else {
		for i, id := range available {
			node, _ := m.g.Node(id)
			b.WriteString(renderSelectable(i == m.cursor, node.Text, node.ID))
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(commandStyle.Render("Space pick    Enter inspect    u undo    r reset    e export    Esc exit"))
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
	b.WriteString(rule())
	b.WriteByte('\n')
	b.WriteString(node.Text)
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render(string(node.ID)))
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
		rule(),
		node.Text,
		commandStyle.Render("y delete    n cancel"))
}

func (m Model) viewConfirmQuit() string {
	return strings.Join([]string{
		titleStyle.Render("Unsaved changes."),
		rule(),
		commandStyle.Render("y save and quit    n quit without saving    c cancel"),
	}, "\n")
}

func (m Model) viewHelp() string {
	return strings.Join([]string{
		titleStyle.Render("Help"),
		rule(),
		"dagim edits one plain-text DAG file.",
		"",
		"a add node        p add/link parent    c add/link child",
		"x unlink          r rename             d delete",
		"/ search          f roots              m manual sequence",
		"J/K reorder       w save               q quit",
		"",
		"In parent/child prompts, Enter links the selected match and Ctrl+N creates the typed text.",
		"Manual sequence is temporary; export writes one node text per line.",
		"",
		commandStyle.Render("Esc back"),
	}, "\n")
}

func renderSelectable(selected bool, text string, id graph.NodeID) string {
	prefix := "  "
	if selected {
		prefix = "> "
	}
	line := fmt.Sprintf("%s%s %s", prefix, text, mutedStyle.Render("["+string(id)+"]"))
	if selected {
		return selectStyle.Render(line)
	}
	return line
}

func renderIDListFor(g *graph.Graph, title string, id graph.NodeID, fn func(graph.NodeID) ([]graph.NodeID, error)) string {
	ids, _ := fn(id)
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteByte('\n')
	b.WriteString(rule())
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

func rule() string {
	return mutedStyle.Render(strings.Repeat("-", 42))
}

func strongRule() string {
	return selectStyle.Render(strings.Repeat("=", 42))
}
