// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tunesmith/dagim/internal/graph"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case diskCheckMsg:
		m = m.refreshFromDiskNow()
		m = m.ensureSelectionVisible()
		return m, scheduleDiskCheck()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = inputWidth(msg.Width)
		if m.mode == modeGraphMap {
			m = m.ensureGraphMapVisible()
		} else {
			m = m.ensureSelectionVisible()
		}
		return m, nil
	case tea.KeyMsg:
		m.message = ""
		m = m.refreshFromDiskNow()
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+z":
			return m, tea.Suspend
		}
		if m.usesGlobalViewport() && m.mode != modeNode && m.mode != modeOrder {
			maxScroll := m.globalScrollMax()
			switch msg.String() {
			case "pgdown":
				m = m.withScrollOffset(m.scrollOffset() + m.globalScrollPageSize())
				if m.scrollOffset() > maxScroll {
					m = m.withScrollOffset(maxScroll)
				}
				return m.ensureSelectionVisible(), nil
			case "pgup":
				m = m.withScrollOffset(m.scrollOffset() - m.globalScrollPageSize())
				if m.scrollOffset() < 0 {
					m = m.withScrollOffset(0)
				}
				return m.ensureSelectionVisible(), nil
			case "home":
				m = m.withScrollOffset(0)
				return m.ensureSelectionVisible(), nil
			case "end":
				m = m.withScrollOffset(maxScroll)
				return m.ensureSelectionVisible(), nil
			}
		}
		switch m.mode {
		case modeNode:
			return updateAndEnsure(m.updateNode, msg)
		case modePrompt:
			return updateAndEnsure(m.updatePrompt, msg)
		case modeSearch:
			return updateAndEnsure(m.updateSearch, msg)
		case modeReady:
			return updateAndEnsure(m.updateReady, msg)
		case modeLeaves:
			return updateAndEnsure(m.updateLeaves, msg)
		case modeOrder:
			return updateAndEnsure(m.updateOrder, msg)
		case modeCheck:
			return updateAndEnsure(m.updateCheck, msg)
		case modeConfirmDelete:
			return updateAndEnsure(m.updateConfirmDelete, msg)
		case modeConfirmRewrite:
			return updateAndEnsure(m.updateConfirmRewrite, msg)
		case modeConfirmReset:
			return updateAndEnsure(m.updateConfirmReset, msg)
		case modeConfirmQuit:
			return updateAndEnsure(m.updateConfirmQuit, msg)
		case modeHelp:
			if msg.String() == "esc" || msg.String() == "q" || msg.String() == "?" {
				m.mode = m.previous
			}
			return ensureAfterUpdate(m, nil)
		case modeGraphMap:
			return updateAndEnsure(m.updateGraphMap, msg)
		}
	}
	return m, nil
}

func updateAndEnsure(update func(tea.KeyMsg) (tea.Model, tea.Cmd), msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	next, cmd := update(msg)
	return ensureAfterUpdate(next, cmd)
}

func ensureAfterUpdate(next tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m, ok := next.(Model)
	if !ok {
		return next, cmd
	}
	return m.ensureSelectionVisible(), cmd
}

func (m Model) updateNode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.current == "" {
		switch msg.String() {
		case "a":
			return m.setPrompt(promptAddNode, "Add first node", "")
		case "u":
			m = m.undoGraphChange()
		case "g":
			return m.openGraphMap(modeNode), nil
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
	case "pgdown":
		m.cursor = m.pageRelationCursor(items, 1)
	case "pgup":
		m.cursor = m.pageRelationCursor(items, -1)
	case "home":
		m.cursor = moveListCursor(0, len(items), 0)
	case "end":
		m.cursor = moveListCursor(len(items)-1, len(items), 0)
	case "enter":
		if len(items) > 0 && m.cursor >= 0 && m.cursor < len(items) {
			m.current = items[m.cursor].id
			m.cursor = 0
			m.nodeScroll = 0
		}
	case "C":
		m.previous = modeNode
		m.checkScroll = 0
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
			snapshot := m.undoSnapshot()
			if item.kind == "parent" {
				if err := m.g.RemoveEdge(item.id, m.current); err != nil {
					m.message = err.Error()
				} else {
					m = m.markChangedWithUndo(snapshot)
				}
			} else {
				if err := m.g.RemoveEdge(m.current, item.id); err != nil {
					m.message = err.Error()
				} else {
					m = m.markChangedWithUndo(snapshot)
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
		m = m.startDelete(m.current, modeNode)
	case "/":
		return m.setSearch()
	case "r":
		m.mode = modeReady
	case "l":
		m.previous = modeNode
		m.leavesReturn = modeNode
		m.mode = modeLeaves
		m.leavesCursor = 0
	case "g":
		return m.openGraphMap(modeNode), nil
	case " ":
		m = m.toggleComplete(m.current)
	case "u":
		m = m.undoGraphChange()
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
		m.helpScroll = 0
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
	case "pgup":
		m.suggestionCursor = m.pagePromptCursor(-1)
		return m, nil
	case "pgdown":
		m.suggestionCursor = m.pagePromptCursor(1)
		return m, nil
	case "home":
		m.suggestionCursor = 0
		return m, nil
	case "end":
		m.suggestionCursor = clampedCursor(len(m.promptMatches())-1, len(m.promptMatches()))
		return m, nil
	case "ctrl+n":
		return m.submitPrompt(false)
	case "enter":
		return m.submitPrompt(true)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.suggestionCursor = 0
	m.promptScroll = 0
	return m, cmd
}

func (m Model) submitPrompt(useSuggestion bool) (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	switch m.promptAction {
	case promptAddNode:
		snapshot := m.undoSnapshot()
		duplicateText := m.hasExactText(value)
		id, err := m.g.AddNode(value)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.current = id
		m.cursor = 0
		if duplicateText {
			m.message = "created separate node with duplicate text"
		}
		m = m.markChangedWithUndo(snapshot)
	case promptEdit:
		snapshot := m.undoSnapshot()
		if err := m.g.EditNodeText(m.current, value); err != nil {
			m.message = err.Error()
			return m, nil
		}
		m = m.markChangedWithUndo(snapshot)
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
	case "pgdown":
		m.searchCursor = m.pageSearchCursor(1)
		return m, nil
	case "pgup":
		m.searchCursor = m.pageSearchCursor(-1)
		return m, nil
	case "home":
		m.searchCursor = 0
		return m, nil
	case "end":
		m.searchCursor = clampedCursor(len(results)-1, len(results))
		return m, nil
	case "enter":
		if len(results) > 0 {
			if m.searchCursor >= len(results) {
				m.searchCursor = len(results) - 1
			}
			m.current = results[m.searchCursor]
			m.cursor = 0
			m.nodeScroll = 0
			m.mode = modeNode
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.searchCursor = 0
	m.searchScroll = 0
	return m, cmd
}

func (m Model) updateReady(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.readyItems()
	switch msg.String() {
	case "a":
		return m.setPrompt(promptAddNode, "Add node", "")
	case "u":
		m = m.undoGraphChange()
	case "j", "down":
		m.readyCursor = moveListCursor(m.readyCursor, len(items), 1)
	case "k", "up":
		m.readyCursor = moveListCursor(m.readyCursor, len(items), -1)
	case "pgdown":
		m.readyCursor = m.pageReadyCursor(1)
	case "pgup":
		m.readyCursor = m.pageReadyCursor(-1)
	case "home":
		m.readyCursor = moveListCursor(0, len(items), 0)
	case "end":
		m.readyCursor = moveListCursor(len(items)-1, len(items), 0)
	case "enter":
		if len(items) > 0 {
			if m.readyCursor >= len(items) {
				m.readyCursor = len(items) - 1
			}
			m.current = items[m.readyCursor].id
			m.cursor = 0
			m.nodeScroll = 0
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
	case "d":
		if len(items) > 0 {
			if m.readyCursor >= len(items) {
				m.readyCursor = len(items) - 1
			}
			m = m.startDelete(items[m.readyCursor].id, modeReady)
		}
	case "C":
		m.previous = modeReady
		m.checkScroll = 0
		m.mode = modeCheck
	case "/":
		return m.setSearch()
	case "l":
		m.previous = modeReady
		m.leavesReturn = modeReady
		m.mode = modeLeaves
		m.leavesCursor = 0
	case "g":
		return m.openGraphMap(modeReady), nil
	case "o":
		m.order = graph.NewOrder(m.g)
		m.previous = modeReady
		m.orderReturn = modeReady
		m.mode = modeOrder
		m.cursor = 0
		m.orderScroll = 0
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
		m.helpScroll = 0
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
	case "u":
		m = m.undoGraphChange()
	case "j", "down":
		m.leavesCursor = moveListCursor(m.leavesCursor, len(leaves), 1)
	case "k", "up":
		m.leavesCursor = moveListCursor(m.leavesCursor, len(leaves), -1)
	case "pgdown":
		m.leavesCursor = m.pageLeavesCursor(1)
	case "pgup":
		m.leavesCursor = m.pageLeavesCursor(-1)
	case "home":
		m.leavesCursor = moveListCursor(0, len(leaves), 0)
	case "end":
		m.leavesCursor = moveListCursor(len(leaves)-1, len(leaves), 0)
	case "enter":
		if len(leaves) > 0 {
			if m.leavesCursor >= len(leaves) {
				m.leavesCursor = len(leaves) - 1
			}
			m.current = leaves[m.leavesCursor]
			m.cursor = 0
			m.nodeScroll = 0
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
	case "C":
		m.previous = modeLeaves
		m.checkScroll = 0
		m.mode = modeCheck
	case "/":
		return m.setSearch()
	case "r":
		m.mode = modeReady
	case "g":
		return m.openGraphMap(modeLeaves), nil
	case "o":
		m.order = graph.NewOrder(m.g)
		m.previous = modeLeaves
		m.orderReturn = modeLeaves
		m.mode = modeOrder
		m.cursor = 0
		m.orderScroll = 0
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
		m.helpScroll = 0
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
	case "pgdown":
		m.cursor = m.pageOrderCursor(1)
	case "pgup":
		m.cursor = m.pageOrderCursor(-1)
	case "home":
		m.cursor = moveListCursor(0, len(available), 0)
	case "end":
		m.cursor = moveListCursor(len(available)-1, len(available), 0)
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
	case "C":
		m.previous = modeOrder
		m.checkScroll = 0
		m.mode = modeCheck
	case "u":
		if !m.order.Undo() {
			m.message = "nothing to undo"
		}
	case "r":
		m.order.Reset()
		m.cursor = 0
		m.orderScroll = 0
	case "e":
		return m.setPrompt(promptExportOrder, "Export order", defaultOrderPath(m.path))
	}
	return m, nil
}

func (m Model) updateCheck(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxScroll := m.checkScrollMax()
	switch msg.String() {
	case "j", "down":
		if m.checkScroll < maxScroll {
			m.checkScroll++
		}
	case "k", "up":
		if m.checkScroll > 0 {
			m.checkScroll--
		}
	case "pgdown":
		m.checkScroll += m.checkScrollPageSize()
		if m.checkScroll > maxScroll {
			m.checkScroll = maxScroll
		}
	case "pgup":
		m.checkScroll -= m.checkScrollPageSize()
		if m.checkScroll < 0 {
			m.checkScroll = 0
		}
	case "home":
		m.checkScroll = 0
	case "end":
		m.checkScroll = maxScroll
	case "esc", "enter", "q":
		m.mode = m.previous
	}
	return m, nil
}

func (m Model) updateGraphMap(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "h", "left":
		m = m.moveGraphMapHorizontal(false)
	case "l", "right":
		m = m.moveGraphMapHorizontal(true)
	case "j", "down":
		m = m.moveGraphMapVertical(1)
	case "k", "up":
		m = m.moveGraphMapVertical(-1)
	case "v":
		m.showCompleted = !m.showCompleted
		m.mapHistory = nil
		m = m.ensureGraphMapVisible()
	case "t":
		m.mapTransitive = !m.mapTransitive
		m.mapHistory = nil
		m = m.ensureGraphMapVisible()
	case "enter":
		if m.mapSelected != "" && m.g.HasNode(m.mapSelected) {
			m.current = m.mapSelected
			m.cursor = 0
			m.mode = modeNode
		}
	case "g", "esc", "q":
		m.mode = stableUndoMode(m.mapReturn)
	}
	return m, nil
}

func (m Model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		returnMode := m.previous
		m = m.deleteCurrent()
		m.mode = m.modeAfterDelete(returnMode)
	case "n", "N", "esc":
		m.mode = m.previous
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
	snapshot := m.undoSnapshot()
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
	order = m.g.Order()
	if len(order) == 0 {
		m.current = ""
		m.cursor = 0
		return m.markChangedWithUndo(snapshot)
	}
	if index >= len(order) {
		index = len(order) - 1
	}
	m.current = order[index]
	m.cursor = 0
	return m.markChangedWithUndo(snapshot)
}

func (m Model) startDelete(id graph.NodeID, returnMode mode) Model {
	if id == "" || !m.g.HasNode(id) {
		return m
	}
	m.current = id
	m.cursor = 0
	m.previous = returnMode
	parents, _ := m.g.ParentsOf(id)
	children, _ := m.g.ChildrenOf(id)
	if len(parents)+len(children) > 0 {
		m.mode = modeConfirmDelete
		return m
	}
	m = m.deleteCurrent()
	m.mode = m.modeAfterDelete(returnMode)
	return m
}

func (m Model) modeAfterDelete(returnMode mode) mode {
	if len(m.g.Nodes()) == 0 {
		return modeNode
	}
	return returnMode
}
