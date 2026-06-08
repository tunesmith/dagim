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
		case modeReady:
			return m.updateReady(msg)
		case modeLeaves:
			return m.updateLeaves(msg)
		case modeOrder:
			return m.updateOrder(msg)
		case modeInspect:
			return m.updateInspect(msg)
		case modeCheck:
			return m.updateCheck(msg)
		case modeConfirmDelete:
			return m.updateConfirmDelete(msg)
		case modeConfirmRewrite:
			return m.updateConfirmRewrite(msg)
		case modeConfirmReset:
			return m.updateConfirmReset(msg)
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
	case "C":
		m.previous = modeNode
		m.mode = modeCheck
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
		m.mode = modeReady
		m.readyCursor = 0
	case "l":
		m.previous = modeNode
		m.leavesReturn = modeNode
		m.mode = modeLeaves
		m.leavesCursor = 0
	case " ":
		m = m.toggleComplete(m.current)
	case "J":
		m = m.reorderSelected(items, 1)
	case "K":
		m = m.reorderSelected(items, -1)
	case "o":
		m.order = graph.NewOrder(m.g)
		m.previous = modeNode
		m.orderReturn = modeNode
		m.mode = modeOrder
		m.cursor = 0
	case "s":
		m = m.save()
	case "R":
		m.previous = modeNode
		m.mode = modeConfirmReset
	case "W":
		m.previous = modeNode
		m.mode = modeConfirmRewrite
	case "v":
		m.showCompleted = !m.showCompleted
		items = m.relationItems()
		if m.cursor >= len(items) {
			m.cursor = len(items) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
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
				m, err = m.addEdge(id, m.current)
			} else {
				m, err = m.addEdge(m.current, id)
			}
			if err != nil {
				m.message = err.Error()
				return m, nil
			}
		} else {
			m = m.addLinkedNode(value, asParent)
		}
	case promptExportOrder:
		m = m.exportOrder(value)
		m.mode = modeOrder
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
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.searchCursor = 0
	return m, cmd
}

func (m Model) updateReady(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.readyItems()
	switch msg.String() {
	case "a":
		return m.setPrompt(promptAddNode, "Add node", "")
	case "j", "down":
		if m.readyCursor < len(items)-1 {
			m.readyCursor++
		}
	case "k", "up":
		if m.readyCursor > 0 {
			m.readyCursor--
		}
	case "enter":
		if len(items) > 0 {
			if m.readyCursor >= len(items) {
				m.readyCursor = len(items) - 1
			}
			m.current = items[m.readyCursor].id
			m.cursor = 0
			m.mode = modeNode
		}
	case " ":
		if len(items) > 0 {
			if m.readyCursor >= len(items) {
				m.readyCursor = len(items) - 1
			}
			m = m.toggleComplete(items[m.readyCursor].id)
			if m.readyCursor >= len(m.readyItems()) && m.readyCursor > 0 {
				m.readyCursor--
			}
		}
	case "i":
		if len(items) > 0 {
			if m.readyCursor >= len(items) {
				m.readyCursor = len(items) - 1
			}
			m.inspectID = items[m.readyCursor].id
			m.previous = modeReady
			m.mode = modeInspect
		}
	case "C":
		m.previous = modeReady
		m.mode = modeCheck
	case "/":
		return m.setSearch()
	case "l":
		m.previous = modeReady
		m.leavesReturn = modeReady
		m.mode = modeLeaves
		m.leavesCursor = 0
	case "o":
		m.order = graph.NewOrder(m.g)
		m.previous = modeReady
		m.orderReturn = modeReady
		m.mode = modeOrder
		m.cursor = 0
	case "s":
		m = m.save()
	case "v":
		m.showCompleted = !m.showCompleted
		m.readyCursor = 0
	case "J":
		m = m.reorderReadyItem(1)
	case "K":
		m = m.reorderReadyItem(-1)
	case "R":
		m.previous = modeReady
		m.mode = modeConfirmReset
	case "W":
		m.previous = modeReady
		m.mode = modeConfirmRewrite
	case "?":
		m.previous = modeReady
		m.mode = modeHelp
	case "q", "ctrl+c":
		if m.dirty {
			m.previous = modeReady
			m.mode = modeConfirmQuit
			return m, nil
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateLeaves(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	leaves := m.visibleLeaves()
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
	case " ":
		if len(leaves) > 0 {
			if m.leavesCursor >= len(leaves) {
				m.leavesCursor = len(leaves) - 1
			}
			m = m.toggleComplete(leaves[m.leavesCursor])
			if m.leavesCursor >= len(m.visibleLeaves()) && m.leavesCursor > 0 {
				m.leavesCursor--
			}
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
	case "C":
		m.previous = modeLeaves
		m.mode = modeCheck
	case "/":
		return m.setSearch()
	case "r":
		m.mode = modeReady
		m.readyCursor = 0
	case "o":
		m.order = graph.NewOrder(m.g)
		m.previous = modeLeaves
		m.orderReturn = modeLeaves
		m.mode = modeOrder
		m.cursor = 0
	case "s":
		m = m.save()
	case "v":
		m.showCompleted = !m.showCompleted
		m.leavesCursor = 0
	case "J":
		m, m.leavesCursor = m.reorderListItem(leaves, m.leavesCursor, 1)
	case "K":
		m, m.leavesCursor = m.reorderListItem(leaves, m.leavesCursor, -1)
	case "R":
		m.previous = modeLeaves
		m.mode = modeConfirmReset
	case "W":
		m.previous = modeLeaves
		m.mode = modeConfirmRewrite
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

func (m Model) updateOrder(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.order == nil {
		m.order = graph.NewOrder(m.g)
	}
	available := m.order.Available()
	switch msg.String() {
	case "esc", "q":
		m.mode = m.orderReturn
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
			if err := m.order.Pick(available[m.cursor]); err != nil {
				m.message = err.Error()
			}
			if m.cursor >= len(m.order.Available()) && m.cursor > 0 {
				m.cursor--
			}
		}
	case "enter":
		if len(available) > 0 {
			if m.cursor >= len(available) {
				m.cursor = len(available) - 1
			}
			m.inspectID = available[m.cursor]
			m.previous = modeOrder
			m.mode = modeInspect
		}
	case "C":
		m.previous = modeOrder
		m.mode = modeCheck
	case "u":
		if !m.order.Undo() {
			m.message = "nothing to undo"
		}
	case "r":
		m.order.Reset()
		m.cursor = 0
	case "e":
		return m.setPrompt(promptExportOrder, "Export order", defaultOrderPath(m.path))
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

func (m Model) updateCheck(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "q":
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
		m.mode = m.previous
	case "n", "N", "esc":
		m.mode = m.previous
	}
	return m, nil
}

func (m Model) updateConfirmReset(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m = m.resetCompletion()
		m.mode = m.previous
	case "n", "N", "esc":
		m.mode = m.previous
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
