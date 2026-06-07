package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"dagim/internal/graph"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = inputWidth(msg.Width)
		return m, nil
	case tea.KeyMsg:
		m.message = ""
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+z":
			return m, tea.Suspend
		}
		switch m.mode {
		case modeNode:
			return m.updateNode(msg)
		case modePrompt:
			return m.updatePrompt(msg)
		case modeSearch:
			return m.updateSearch(msg)
		case modeRoots:
			return m.updateRoots(msg)
		case modeLeaves:
			return m.updateLeaves(msg)
		case modeSequence:
			return m.updateSequence(msg)
		case modeInspect:
			return m.updateInspect(msg)
		case modeConfirmDelete:
			return m.updateConfirmDelete(msg)
		case modeConfirmRewrite:
			return m.updateConfirmRewrite(msg)
		case modeConfirmQuit:
			return m.updateConfirmQuit(msg)
		case modeHelp:
			if msg.String() == "esc" || msg.String() == "q" || msg.String() == "?" {
				m.mode = m.previous
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) updateNode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.current == "" {
		switch msg.String() {
		case "a":
			return m.setPrompt(promptAddNode, "Add first node", "")
		case "?":
			m.previous = modeNode
			m.mode = modeHelp
			return m, nil
		case "q", "ctrl+c":
			if m.dirty {
				m.previous = modeNode
				m.mode = modeConfirmQuit
				return m, nil
			}
			return m, tea.Quit
		}
		return m, nil
	}

	items := m.relationItems()
	switch msg.String() {
	case "j", "down":
		if len(items) > 0 && m.cursor < len(items)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		if len(items) > 0 && m.cursor >= 0 && m.cursor < len(items) {
			m.current = items[m.cursor].id
			m.cursor = 0
		}
	case "i":
		m.inspectID = m.selectedNodeOrCurrent(items)
		m.previous = modeNode
		m.mode = modeInspect
	case "a":
		return m.setPrompt(promptAddNode, "Add node", "")
	case "p":
		return m.setPrompt(promptAddParent, "Add/link parent", "")
	case "c":
		return m.setPrompt(promptAddChild, "Add/link child", "")
	case "x":
		if len(items) > 0 && m.cursor >= 0 && m.cursor < len(items) {
			item := items[m.cursor]
			if item.kind == "parent" {
				if err := m.g.RemoveEdge(item.id, m.current); err != nil {
					m.message = err.Error()
				} else {
					m.dirty = true
				}
			} else {
				if err := m.g.RemoveEdge(m.current, item.id); err != nil {
					m.message = err.Error()
				} else {
					m.dirty = true
				}
			}
			if m.cursor >= len(m.relationItems()) && m.cursor > 0 {
				m.cursor--
			}
		}
	case "e":
		node, _ := m.g.Node(m.current)
		return m.setPrompt(promptEdit, "Edit node", node.Text)
	case "d":
		parents, _ := m.g.ParentsOf(m.current)
		children, _ := m.g.ChildrenOf(m.current)
		if len(parents)+len(children) > 0 {
			m.mode = modeConfirmDelete
			return m, nil
		}
		m = m.deleteCurrent()
	case "/":
		return m.setSearch()
	case "r":
		m.mode = modeRoots
		m.rootsCursor = 0
	case "l":
		m.previous = modeNode
		m.leavesReturn = modeNode
		m.mode = modeLeaves
		m.leavesCursor = 0
	case "J":
		if m.g.MoveLater(m.current) {
			m.dirty = true
		}
	case "K":
		if m.g.MoveEarlier(m.current) {
			m.dirty = true
		}
	case "m":
		m.seq = graph.NewSequence(m.g)
		m.previous = modeNode
		m.seqReturn = modeNode
		m.mode = modeSequence
		m.cursor = 0
	case "s":
		m = m.save()
	case "R":
		m.mode = modeConfirmRewrite
	case "?":
		m.previous = modeNode
		m.mode = modeHelp
	case "q", "ctrl+c":
		if m.dirty {
			m.previous = modeNode
			m.mode = modeConfirmQuit
			return m, nil
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = m.previous
		m.promptAction = promptNone
		return m, nil
	case "up":
		if m.suggestionCursor > 0 {
			m.suggestionCursor--
		}
		return m, nil
	case "down":
		results := m.promptMatches()
		if m.suggestionCursor < len(results)-1 {
			m.suggestionCursor++
		}
		return m, nil
	case "ctrl+n":
		return m.submitPrompt(false)
	case "enter":
		return m.submitPrompt(true)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.suggestionCursor = 0
	return m, cmd
}

func (m Model) submitPrompt(useSuggestion bool) (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	switch m.promptAction {
	case promptAddNode:
		duplicateText := m.hasExactText(value)
		id, err := m.g.AddNode(value)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.current = id
		m.cursor = 0
		m.dirty = true
		if duplicateText {
			m.message = "created separate node with duplicate text"
		}
	case promptEdit:
		if err := m.g.EditNodeText(m.current, value); err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.dirty = true
	case promptAddParent, promptAddChild:
		asParent := m.promptAction == promptAddParent
		var id graph.NodeID
		var ok bool
		if useSuggestion {
			id, ok = m.selectedSuggestion()
		}
		if !ok {
			id, ok = m.findExactNode(value)
		}
		if ok {
			var err error
			if asParent {
				err = m.g.AddEdge(id, m.current)
			} else {
				err = m.g.AddEdge(m.current, id)
			}
			if err != nil {
				m.message = err.Error()
				return m, nil
			}
			m.dirty = true
		} else {
			m = m.addLinkedNode(value, asParent)
		}
	case promptExportSequence:
		m = m.exportSequence(value)
		m.mode = modeSequence
		m.promptAction = promptNone
		return m, nil
	}
	m.mode = modeNode
	m.promptAction = promptNone
	m = m.ensureCurrent()
	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	results := m.searchResults(m.input.Value())
	switch msg.String() {
	case "esc":
		m.mode = m.previous
		return m, nil
	case "up":
		if m.searchCursor > 0 {
			m.searchCursor--
		}
		return m, nil
	case "down":
		if m.searchCursor < len(results)-1 {
			m.searchCursor++
		}
		return m, nil
	case "enter":
		if len(results) > 0 {
			if m.searchCursor >= len(results) {
				m.searchCursor = len(results) - 1
			}
			m.current = results[m.searchCursor]
			m.cursor = 0
			m.mode = modeNode
		}
		return m, nil
	case "i":
		if len(results) > 0 {
			if m.searchCursor >= len(results) {
				m.searchCursor = len(results) - 1
			}
			m.inspectID = results[m.searchCursor]
			m.previous = modeSearch
			m.mode = modeInspect
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.searchCursor = 0
	return m, cmd
}

func (m Model) updateRoots(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	roots := m.g.Roots()
	switch msg.String() {
	case "a":
		return m.setPrompt(promptAddNode, "Add node", "")
	case "j", "down":
		if m.rootsCursor < len(roots)-1 {
			m.rootsCursor++
		}
	case "k", "up":
		if m.rootsCursor > 0 {
			m.rootsCursor--
		}
	case "enter":
		if len(roots) > 0 {
			if m.rootsCursor >= len(roots) {
				m.rootsCursor = len(roots) - 1
			}
			m.current = roots[m.rootsCursor]
			m.cursor = 0
			m.mode = modeNode
		}
	case "i":
		if len(roots) > 0 {
			if m.rootsCursor >= len(roots) {
				m.rootsCursor = len(roots) - 1
			}
			m.inspectID = roots[m.rootsCursor]
			m.previous = modeRoots
			m.mode = modeInspect
		}
	case "/":
		return m.setSearch()
	case "l":
		m.previous = modeRoots
		m.leavesReturn = modeRoots
		m.mode = modeLeaves
		m.leavesCursor = 0
	case "m":
		m.seq = graph.NewSequence(m.g)
		m.previous = modeRoots
		m.seqReturn = modeRoots
		m.mode = modeSequence
		m.cursor = 0
	case "s":
		m = m.save()
	case "?":
		m.previous = modeRoots
		m.mode = modeHelp
	case "q", "ctrl+c":
		if m.dirty {
			m.previous = modeRoots
			m.mode = modeConfirmQuit
			return m, nil
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateLeaves(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	leaves := m.g.Leaves()
	switch msg.String() {
	case "esc":
		m.mode = m.leavesReturn
	case "j", "down":
		if m.leavesCursor < len(leaves)-1 {
			m.leavesCursor++
		}
	case "k", "up":
		if m.leavesCursor > 0 {
			m.leavesCursor--
		}
	case "enter":
		if len(leaves) > 0 {
			if m.leavesCursor >= len(leaves) {
				m.leavesCursor = len(leaves) - 1
			}
			m.current = leaves[m.leavesCursor]
			m.cursor = 0
			m.mode = modeNode
		}
	case "i":
		if len(leaves) > 0 {
			if m.leavesCursor >= len(leaves) {
				m.leavesCursor = len(leaves) - 1
			}
			m.inspectID = leaves[m.leavesCursor]
			m.previous = modeLeaves
			m.mode = modeInspect
		}
	case "/":
		return m.setSearch()
	case "r":
		m.mode = modeRoots
		m.rootsCursor = 0
	case "s":
		m = m.save()
	case "?":
		m.previous = modeLeaves
		m.mode = modeHelp
	case "q":
		if m.dirty {
			m.previous = modeLeaves
			m.mode = modeConfirmQuit
			return m, nil
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateSequence(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.seq == nil {
		m.seq = graph.NewSequence(m.g)
	}
	available := m.seq.Available()
	switch msg.String() {
	case "esc", "q":
		m.mode = m.seqReturn
	case "j", "down":
		if m.cursor < len(available)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case " ":
		if len(available) > 0 {
			if m.cursor >= len(available) {
				m.cursor = len(available) - 1
			}
			if err := m.seq.Pick(available[m.cursor]); err != nil {
				m.message = err.Error()
			}
			if m.cursor >= len(m.seq.Available()) && m.cursor > 0 {
				m.cursor--
			}
		}
	case "enter":
		if len(available) > 0 {
			if m.cursor >= len(available) {
				m.cursor = len(available) - 1
			}
			m.inspectID = available[m.cursor]
			m.previous = modeSequence
			m.mode = modeInspect
		}
	case "u":
		if !m.seq.Undo() {
			m.message = "nothing to undo"
		}
	case "r":
		m.seq.Reset()
		m.cursor = 0
	case "e":
		return m.setPrompt(promptExportSequence, "Export sequence", defaultSequencePath(m.path))
	}
	return m, nil
}

func (m Model) updateInspect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.mode = m.previous
	}
	return m, nil
}

func (m Model) selectedNodeOrCurrent(items []relationItem) graph.NodeID {
	if len(items) > 0 && m.cursor >= 0 && m.cursor < len(items) {
		return items[m.cursor].id
	}
	return m.current
}

func (m Model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m = m.deleteCurrent()
		m.mode = modeNode
	case "n", "N", "esc":
		m.mode = modeNode
	}
	return m, nil
}

func (m Model) updateConfirmRewrite(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m = m.rewrite()
		m.mode = modeNode
	case "n", "N", "esc":
		m.mode = modeNode
	}
	return m, nil
}

func (m Model) updateConfirmQuit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m = m.save()
		if !m.dirty {
			return m, tea.Quit
		}
	case "n", "N":
		return m, tea.Quit
	case "c", "C", "esc":
		m.mode = m.previous
	}
	return m, nil
}

func (m Model) deleteCurrent() Model {
	if m.current == "" {
		return m
	}
	order := m.g.Order()
	index := 0
	for i, id := range order {
		if id == m.current {
			index = i
			break
		}
	}
	if err := m.g.DeleteNode(m.current); err != nil {
		m.message = err.Error()
		return m
	}
	m.dirty = true
	order = m.g.Order()
	if len(order) == 0 {
		m.current = ""
		m.cursor = 0
		return m
	}
	if index >= len(order) {
		index = len(order) - 1
	}
	m.current = order[index]
	m.cursor = 0
	return m
}
