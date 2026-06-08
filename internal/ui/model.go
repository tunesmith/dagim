package ui

import (
	"errors"
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
	modeReady
	modeLeaves
	modeOrder
	modeInspect
	modeCheck
	modeConfirmDelete
	modeConfirmRewrite
	modeConfirmReset
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
	promptExportOrder
)

type relationItem struct {
	id   graph.NodeID
	kind string
}

type completionItem struct {
	id graph.NodeID
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

	order         *graph.Order
	orderReturn   mode
	inspectID     graph.NodeID
	readyCursor   int
	leavesCursor  int
	leavesReturn  mode
	searchCursor  int
	showCompleted bool

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
		m.mode = modeReady
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
		if node.Complete && !m.showCompleted {
			continue
		}
		items = append(items, relationItem{kind: "parent", id: id})
	}
	children, _ := m.g.ChildrenOf(m.current)
	for _, id := range children {
		node, _ := m.g.Node(id)
		if node.Complete && !m.showCompleted {
			continue
		}
		items = append(items, relationItem{kind: "child", id: id})
	}
	return items
}

func (m Model) searchResults(query string) []graph.NodeID {
	query = strings.ToLower(strings.TrimSpace(query))
	var ids []graph.NodeID
	for _, node := range m.g.Nodes() {
		if query == "" || strings.Contains(strings.ToLower(node.Text), query) {
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

func (m Model) readyItems() []completionItem {
	items := make([]completionItem, 0)
	for _, id := range m.g.Ready() {
		items = append(items, completionItem{id: id})
	}
	if m.showCompleted {
		for _, id := range m.g.Completed() {
			items = append(items, completionItem{id: id})
		}
	}
	return items
}

func (m Model) visibleLeaves() []graph.NodeID {
	leaves := m.g.Leaves()
	if m.showCompleted {
		return leaves
	}
	filtered := make([]graph.NodeID, 0, len(leaves))
	for _, id := range leaves {
		node, _ := m.g.Node(id)
		if !node.Complete {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

func (m Model) toggleComplete(id graph.NodeID) Model {
	node, ok := m.g.Node(id)
	if !ok {
		m.message = "node missing"
		return m
	}
	if node.Complete {
		count, err := m.g.MarkIncompleteCascade(id)
		if err != nil {
			m.message = err.Error()
			return m
		}
		if count == 0 {
			m.message = "already undone"
			return m
		}
		m.dirty = true
		if count == 1 {
			m.message = "marked undone"
		} else {
			m.message = fmt.Sprintf("marked %d nodes undone", count)
		}
		return m
	}
	if err := m.g.MarkComplete(id); err != nil {
		var blocked graph.BlockedError
		if errors.As(err, &blocked) {
			if len(blocked.Parents) == 1 {
				m.message = "blocked by 1 undone parent"
			} else {
				m.message = fmt.Sprintf("blocked by %d undone parents", len(blocked.Parents))
			}
			return m
		}
		m.message = err.Error()
		return m
	}
	m.dirty = true
	m.message = "marked done"
	return m
}

func (m Model) resetCompletion() Model {
	count := m.g.ResetCompletion()
	if count == 0 {
		m.message = "no completed nodes"
		return m
	}
	m.dirty = true
	m.message = fmt.Sprintf("reset %d completed nodes", count)
	return m
}

func (m Model) reorderSelected(items []relationItem, direction int) Model {
	if len(items) == 0 || m.cursor < 0 || m.cursor >= len(items) {
		return m
	}

	selected := items[m.cursor]
	targetIndex := -1
	for i := m.cursor + direction; i >= 0 && i < len(items); i += direction {
		if items[i].kind == selected.kind {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return m
	}
	orderIndex := indexOfID(m.g.Order(), items[targetIndex].id)
	if orderIndex < 0 {
		return m
	}
	if m.g.MoveTo(selected.id, orderIndex) {
		m.cursor = targetIndex
		m.dirty = true
	}
	return m
}

func (m Model) reorderListItem(ids []graph.NodeID, cursor, direction int) (Model, int) {
	if len(ids) == 0 || cursor < 0 || cursor >= len(ids) {
		return m, cursor
	}
	target := cursor + direction
	if target < 0 || target >= len(ids) {
		return m, cursor
	}
	orderIndex := indexOfID(m.g.Order(), ids[target])
	if orderIndex < 0 {
		return m, cursor
	}
	if m.g.MoveTo(ids[cursor], orderIndex) {
		m.dirty = true
		cursor = target
	}
	return m, cursor
}

func (m Model) reorderReadyItem(direction int) Model {
	ready := m.g.Ready()
	if m.readyCursor < len(ready) {
		next, cursor := m.reorderListItem(ready, m.readyCursor, direction)
		next.readyCursor = cursor
		return next
	}
	if !m.showCompleted {
		return m
	}
	completedCursor := m.readyCursor - len(ready)
	completed := m.g.Completed()
	next, cursor := m.reorderListItem(completed, completedCursor, direction)
	next.readyCursor = len(ready) + cursor
	return next
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

func (m Model) exportOrder(path string) Model {
	path = strings.TrimSpace(path)
	if path == "" {
		m.message = "export path cannot be empty"
		return m
	}
	if m.order == nil {
		m.message = "no order to export"
		return m
	}
	var b strings.Builder
	for _, id := range m.order.Output() {
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
		m, err = m.addEdge(id, m.current)
	} else {
		m, err = m.addEdge(m.current, id)
	}
	if err != nil {
		_ = m.g.DeleteNode(id)
		m.message = err.Error()
		return m
	}
	return m
}

func (m Model) addEdge(parent, child graph.NodeID) (Model, error) {
	if err := m.g.AddEdge(parent, child); err != nil {
		return m, err
	}
	m.dirty = true
	parentNode, parentOK := m.g.Node(parent)
	childNode, childOK := m.g.Node(child)
	if parentOK && childOK && childNode.Complete && !parentNode.Complete {
		count, err := m.g.MarkIncompleteCascade(child)
		if err != nil {
			return m, err
		}
		if count == 1 {
			m.message = "linked; marked 1 node undone"
		} else if count > 1 {
			m.message = fmt.Sprintf("linked; marked %d nodes undone", count)
		}
	}
	return m, nil
}

func inputWidth(width int) int {
	width = contentWidthForTerminal(width)
	if width < 30 {
		return width
	}
	return width - 4
}

func defaultOrderPath(path string) string {
	if path == "" {
		return "order.txt"
	}
	return fmt.Sprintf("%s.order.txt", path)
}

func indexOfID(ids []graph.NodeID, want graph.NodeID) int {
	for i, id := range ids {
		if id == want {
			return i
		}
	}
	return -1
}
