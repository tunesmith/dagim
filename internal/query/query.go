// SPDX-License-Identifier: GPL-3.0-or-later

package query

import (
	"fmt"
	"strings"

	"github.com/tunesmith/dagim/internal/graph"
)

type State string

const (
	StateComplete State = "complete"
	StateReady    State = "ready"
	StateBlocked  State = "blocked"
)

type Filter string

const (
	FilterAll        Filter = "all"
	FilterReady      Filter = "ready"
	FilterBlocked    Filter = "blocked"
	FilterComplete   Filter = "complete"
	FilterIncomplete Filter = "incomplete"
)

type NodeSummary struct {
	Node      graph.Node
	State     State
	BlockedBy []graph.NodeID
}

type Relations struct {
	Node     NodeSummary
	Parents  []NodeSummary
	Children []NodeSummary
}

type Transitions struct {
	CompletionChanged []NodeSummary
	NewlyReady        []NodeSummary
	NewlyBlocked      []NodeSummary
}

func Summarize(g *graph.Graph, node graph.Node) NodeSummary {
	blockedBy, _ := g.IncompleteParentsOf(node.ID)
	state := StateBlocked
	if node.Complete {
		state = StateComplete
	} else if len(blockedBy) == 0 {
		state = StateReady
	}
	if blockedBy == nil {
		blockedBy = []graph.NodeID{}
	}
	return NodeSummary{Node: node, State: state, BlockedBy: blockedBy}
}

func List(g *graph.Graph, filter Filter) []NodeSummary {
	result := make([]NodeSummary, 0)
	for _, node := range g.Nodes() {
		summary := Summarize(g, node)
		if Matches(summary, filter) {
			result = append(result, summary)
		}
	}
	return result
}

func Matches(node NodeSummary, filter Filter) bool {
	switch filter {
	case FilterAll:
		return true
	case FilterIncomplete:
		return !node.Node.Complete
	default:
		return node.State == State(filter)
	}
}

func ValidFilter(filter Filter) bool {
	switch filter {
	case FilterAll, FilterReady, FilterBlocked, FilterComplete, FilterIncomplete:
		return true
	default:
		return false
	}
}

func Ready(g *graph.Graph, includeCompleted bool) []NodeSummary {
	result := List(g, FilterReady)
	if includeCompleted {
		result = append(result, List(g, FilterComplete)...)
	}
	return result
}

func Leaves(g *graph.Graph, includeCompleted bool) []NodeSummary {
	result := make([]NodeSummary, 0)
	for _, id := range g.Leaves() {
		node, _ := g.Node(id)
		if node.Complete && !includeCompleted {
			continue
		}
		result = append(result, Summarize(g, node))
	}
	return result
}

func Search(g *graph.Graph, text string) []NodeSummary {
	text = strings.ToLower(strings.TrimSpace(text))
	result := make([]NodeSummary, 0)
	for _, node := range g.Nodes() {
		if text == "" || strings.Contains(strings.ToLower(node.Text), text) {
			result = append(result, Summarize(g, node))
		}
	}
	return result
}

func RelationsFor(g *graph.Graph, id graph.NodeID) (Relations, error) {
	node, ok := g.Node(id)
	if !ok {
		return Relations{}, fmt.Errorf("%w: %s", graph.ErrUnknownNode, id)
	}
	parents, err := g.ParentsOf(id)
	if err != nil {
		return Relations{}, err
	}
	children, err := g.ChildrenOf(id)
	if err != nil {
		return Relations{}, err
	}
	return Relations{Node: Summarize(g, node), Parents: summarizeIDs(g, parents), Children: summarizeIDs(g, children)}, nil
}

func Compare(before, after *graph.Graph) Transitions {
	result := Transitions{CompletionChanged: []NodeSummary{}, NewlyReady: []NodeSummary{}, NewlyBlocked: []NodeSummary{}}
	for _, node := range after.Nodes() {
		afterSummary := Summarize(after, node)
		beforeNode, existed := before.Node(node.ID)
		if !existed {
			continue
		}
		beforeSummary := Summarize(before, beforeNode)
		if beforeNode.Complete != node.Complete {
			result.CompletionChanged = append(result.CompletionChanged, afterSummary)
		}
		if beforeSummary.State != StateReady && afterSummary.State == StateReady {
			result.NewlyReady = append(result.NewlyReady, afterSummary)
		}
		if beforeSummary.State != StateBlocked && afterSummary.State == StateBlocked {
			result.NewlyBlocked = append(result.NewlyBlocked, afterSummary)
		}
	}
	return result
}

func summarizeIDs(g *graph.Graph, ids []graph.NodeID) []NodeSummary {
	result := make([]NodeSummary, 0, len(ids))
	for _, id := range ids {
		node, _ := g.Node(id)
		result = append(result, Summarize(g, node))
	}
	return result
}
