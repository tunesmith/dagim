// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tunesmith/dagim/internal/dagimfile"
)

const diskCheckInterval = 500 * time.Millisecond

var errDiskChanged = errors.New("file changed on disk")

type diskVersion struct {
	exists bool
	digest [sha256.Size]byte
}

type diskContents struct {
	data    []byte
	version diskVersion
	err     error
}

type diskCheckMsg struct{}

func readDisk(path string) diskContents {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return diskContents{version: diskVersion{}}
	}
	if err != nil {
		return diskContents{err: err}
	}
	return diskContents{
		data: data,
		version: diskVersion{
			exists: true,
			digest: sha256.Sum256(data),
		},
	}
}

func versionForSavedGraph(serialized string) diskVersion {
	return diskVersion{exists: true, digest: sha256.Sum256([]byte(serialized))}
}

func scheduleDiskCheck() tea.Cmd {
	return tea.Tick(diskCheckInterval, func(time.Time) tea.Msg {
		return diskCheckMsg{}
	})
}

func (m Model) refreshFromDisk(check diskContents) Model {
	if check.err != nil {
		m.message = "reload failed: " + check.err.Error()
		return m
	}
	if m.diskVersionKnown && check.version == m.seenDiskVersion {
		return m
	}
	m.seenDiskVersion = check.version
	if m.diskVersionKnown && check.version == m.diskVersion {
		return m
	}
	if m.dirty {
		m.message = "file changed on disk; unsaved TUI changes were not written"
		return m
	}

	g, err := dagimfile.Parse(string(check.data))
	if err != nil {
		m.message = "external change is invalid: " + err.Error()
		return m
	}

	oldCurrent := m.current
	oldCurrentNode, hadOldCurrent := m.g.Node(oldCurrent)
	m.g = g
	m.diskVersion = check.version
	m.seenDiskVersion = check.version
	m.diskVersionKnown = true
	m.dirty = false
	m.undoStack = nil
	m.order = nil
	m = m.ensureCurrent()
	m.cursor = clampedCursor(m.cursor, len(m.relationItems()))
	m.readyCursor = clampedCursor(m.readyCursor, len(m.readyItems()))
	m.leavesCursor = clampedCursor(m.leavesCursor, len(m.visibleLeaves()))
	if m.mode == modeGraphMap {
		m.mapHistory = nil
		m = m.ensureGraphMapSelection()
		m = m.ensureGraphMapVisible()
	}

	switch m.mode {
	case modeOrder:
		m.mode = stableUndoMode(m.orderReturn)
	case modeConfirmDelete, modeConfirmRewrite, modeConfirmReset, modeConfirmQuit:
		m.mode = stableUndoMode(m.previous)
	case modePrompt:
		needsCurrent := m.promptAction == promptEdit || m.promptAction == promptAddParent || m.promptAction == promptAddChild
		if needsCurrent && (oldCurrent == "" || !m.g.HasNode(oldCurrent)) {
			m.mode = stableUndoMode(m.previous)
			m.promptAction = promptNone
		} else if m.promptAction == promptEdit {
			newCurrentNode, _ := m.g.Node(oldCurrent)
			if !hadOldCurrent || newCurrentNode != oldCurrentNode {
				m.mode = stableUndoMode(m.previous)
				m.promptAction = promptNone
			}
		}
	}
	if len(m.g.Nodes()) == 0 {
		m.mode = modeNode
	}
	m.message = "reloaded changes from disk"
	return m
}

func (m Model) refreshFromDiskNow() Model {
	return m.refreshFromDisk(readDisk(m.path))
}

func (m Model) diskStillCurrent() error {
	if !m.diskVersionKnown {
		return fmt.Errorf("cannot verify the file version")
	}
	check := readDisk(m.path)
	if check.err != nil {
		return check.err
	}
	if check.version != m.diskVersion {
		return errDiskChanged
	}
	return nil
}
