package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"dagim/internal/dagimfile"
	"dagim/internal/graph"
)

type mode int

const (
	modeNode mode = iota
	modePrompt
	modeSearch
	modeRoots
	modeLeaves
	modeSequence
	modeInspect
	modeConfirmDelete
	modeConfirmRewrite
	modeConfirmQuit
	modeHelp
)

type promptAction int

const (
	promptNone promptAction = iota
	promptAddNode
	promptAddParent
	promptAddChild
	promptEdit
	promptExportSequence
)

type relationItem struct {
	label string
	id    graph.NodeID
	kind  string
}

type Model struct {
	path string
	g    *graph.Graph

	mode     mode
	previous mode
	current  graph.NodeID
	cursor   int

	input            textinput.Model
	promptTitle      string
	promptAction     promptAction
	suggestionCursor int

	seq          *graph.Sequence
	seqReturn    mode
	inspectID    graph.NodeID
	rootsCursor  int
	leavesCursor int
	leavesReturn mode
	searchCursor int

	dirty   bool
	message string
	width   int
	height  int
}

func New(path string, g *graph.Graph) Model {
	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = 4096
	input.Width = 80
	input.Focus()

	m := Model{
		path:  path,
		g:     g,
		mode:  modeNode,
		input: input,
	}
	if nodes := g.Nodes(); len(nodes) > 0 {
		m.current = nodes[0].ID
		m.mode = modeRoots
	}
	return m
}

func Run(path string, g *graph.Graph) error {
	_, err := tea.NewProgram(New(path, g), tea.WithAltScreen()).Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) setPrompt(action promptAction, title, value string) (Model, tea.Cmd) {
	m.previous = m.mode
	m.mode = modePrompt
	m.promptAction = action
	m.promptTitle = title
	m.suggestionCursor = 0
	m.input = textinput.New()
	m.input.Prompt = "> "
	m.input.CharLimit = 4096
	m.input.Width = inputWidth(m.width)
	m.input.SetValue(value)
	m.input.Focus()
	m.message = ""
	return m, textinput.Blink
}

func (m Model) setSearch() (Model, tea.Cmd) {
	m.previous = m.mode
	m.mode = modeSearch
	m.searchCursor = 0
	m.input = textinput.New()
	m.input.Prompt = "/ "
	m.input.CharLimit = 4096
	m.input.Width = inputWidth(m.width)
	m.input.Focus()
	m.message = ""
	return m, textinput.Blink
}

func (m Model) relationItems() []relationItem {
	if m.current == "" {
		return nil
	}
	var items []relationItem
	parents, _ := m.g.ParentsOf(m.current)
	for _, id := range parents {
		node, _ := m.g.Node(id)
		items = append(items, relationItem{kind: "parent", id: id, label: node.Text})
	}
	children, _ := m.g.ChildrenOf(m.current)
	for _, id := range children {
		node, _ := m.g.Node(id)
		items = append(items, relationItem{kind: "child", id: id, label: node.Text})
	}
	return items
}

func (m Model) searchResults(query string) []graph.NodeID {
	query = strings.ToLower(strings.TrimSpace(query))
	var ids []graph.NodeID
	for _, node := range m.g.Nodes() {
		if query == "" || strings.Contains(strings.ToLower(node.Text), query) || strings.Contains(strings.ToLower(string(node.ID)), query) {
			ids = append(ids, node.ID)
		}
	}
	return ids
}

func (m Model) promptMatches() []graph.NodeID {
	if m.promptAction != promptAddParent && m.promptAction != promptAddChild {
		return m.searchResults(m.input.Value())
	}
	return m.linkCandidates(m.input.Value(), m.promptAction)
}

func (m Model) linkCandidates(query string, action promptAction) []graph.NodeID {
	hidden := map[graph.NodeID]bool{}
	if m.current != "" {
		hidden[m.current] = true
	}
	switch action {
	case promptAddParent:
		parents, _ := m.g.ParentsOf(m.current)
		for _, id := range parents {
			hidden[id] = true
		}
	case promptAddChild:
		children, _ := m.g.ChildrenOf(m.current)
		for _, id := range children {
			hidden[id] = true
		}
	}
	var ids []graph.NodeID
	for _, id := range m.searchResults(query) {
		if !hidden[id] && !m.linkWouldCycle(id, action) {
			ids = append(ids, id)
		}
	}
	return ids
}

func (m Model) linkWouldCycle(candidate graph.NodeID, action promptAction) bool {
	if m.current == "" || candidate == "" {
		return false
	}
	switch action {
	case promptAddParent:
		_, found := m.g.Path(m.current, candidate)
		return found
	case promptAddChild:
		_, found := m.g.Path(candidate, m.current)
		return found
	default:
		return false
	}
}

func (m Model) promptMatchWindow(total int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	limit := m.promptMatchLimit()
	if limit >= total {
		return 0, total
	}
	cursor := m.suggestionCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= total {
		cursor = total - 1
	}
	start := cursor - limit/2
	if start < 0 {
		start = 0
	}
	if start+limit > total {
		start = total - limit
	}
	return start, start + limit
}

func (m Model) promptMatchLimit() int {
	const maxMatches = 18
	if m.height <= 0 {
		return 12
	}
	limit := m.height - 10
	if limit < 3 {
		return 3
	}
	if limit > maxMatches {
		return maxMatches
	}
	return limit
}

func (m Model) selectedSuggestion() (graph.NodeID, bool) {
	results := m.promptMatches()
	if len(results) == 0 {
		return "", false
	}
	if m.suggestionCursor < 0 {
		m.suggestionCursor = 0
	}
	if m.suggestionCursor >= len(results) {
		m.suggestionCursor = len(results) - 1
	}
	return results[m.suggestionCursor], true
}

func (m Model) findExactNode(input string) (graph.NodeID, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", false
	}
	id := graph.NodeID(input)
	if m.g.HasNode(id) {
		return id, true
	}
	for _, node := range m.g.Nodes() {
		if node.Text == input {
			return node.ID, true
		}
	}
	return "", false
}

func (m Model) hasExactText(input string) bool {
	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}
	for _, node := range m.g.Nodes() {
		if node.Text == input {
			return true
		}
	}
	return false
}

func (m Model) ensureCurrent() Model {
	if m.current != "" && m.g.HasNode(m.current) {
		return m
	}
	if nodes := m.g.Nodes(); len(nodes) > 0 {
		m.current = nodes[0].ID
	} else {
		m.current = ""
		m.cursor = 0
	}
	return m
}

func (m Model) save() Model {
	if err := m.g.Validate(); err != nil {
		m.message = err.Error()
		return m
	}
	if err := dagimfile.SaveAtomic(m.path, m.g); err != nil {
		m.message = err.Error()
		return m
	}
	m.dirty = false
	m.message = "saved " + m.path
	return m
}

func (m Model) rewrite() Model {
	current := m.current
	mapping, err := m.g.RekeyByText()
	if err != nil {
		m.message = err.Error()
		return m
	}
	if err := dagimfile.SaveAtomic(m.path, m.g); err != nil {
		m.dirty = true
		m.message = err.Error()
		return m
	}
	changed := 0
	for oldID, newID := range mapping {
		if oldID != newID {
			changed++
		}
	}
	m.dirty = false
	m.message = fmt.Sprintf("rewrote %s (%d IDs changed)", m.path, changed)
	if nextCurrent, ok := mapping[current]; ok {
		m.current = nextCurrent
	}
	m = m.ensureCurrent()
	return m
}

func (m Model) exportSequence(path string) Model {
	path = strings.TrimSpace(path)
	if path == "" {
		m.message = "export path cannot be empty"
		return m
	}
	if m.seq == nil {
		m.message = "no sequence to export"
		return m
	}
	var b strings.Builder
	for _, id := range m.seq.Output() {
		node, ok := m.g.Node(id)
		if !ok {
			continue
		}
		b.WriteString(node.Text)
		b.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && filepath.Dir(path) != "." {
		m.message = err.Error()
		return m
	}
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		m.message = err.Error()
		return m
	}
	m.message = "exported " + path
	return m
}

func (m Model) addLinkedNode(text string, asParent bool) Model {
	id, err := m.g.AddNode(strings.TrimSpace(text))
	if err != nil {
		m.message = err.Error()
		return m
	}
	if m.current == "" {
		m.current = id
		m.dirty = true
		return m
	}
	if asParent {
		err = m.g.AddEdge(id, m.current)
	} else {
		err = m.g.AddEdge(m.current, id)
	}
	if err != nil {
		_ = m.g.DeleteNode(id)
		m.message = err.Error()
		return m
	}
	m.dirty = true
	return m
}

func inputWidth(width int) int {
	width = contentWidthForTerminal(width)
	if width < 30 {
		return width
	}
	return width - 4
}

func defaultSequencePath(path string) string {
	if path == "" {
		return "sequence.txt"
	}
	return fmt.Sprintf("%s.sequence.txt", path)
}
