package graph

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type NodeID string

type Node struct {
	ID       NodeID
	Text     string
	Complete bool
}

type Graph struct {
	nodes   map[NodeID]Node
	order   []NodeID
	parents map[NodeID]map[NodeID]struct{}
}

type Stats struct {
	Nodes    int
	Edges    int
	Complete int
	Ready    int
	Roots    int
	Leaves   int
}

type Edge struct {
	Parent NodeID
	Child  NodeID
}

type TransitiveEdge struct {
	Parent NodeID
	Child  NodeID
	Path   []NodeID
}

var (
	ErrEmptyNodeID   = errors.New("empty node id")
	ErrEmptyNodeText = errors.New("empty node text")
	ErrDuplicateNode = errors.New("duplicate node id")
	ErrUnknownNode   = errors.New("unknown node")
	ErrDuplicateEdge = errors.New("duplicate edge")
	ErrSelfEdge      = errors.New("self edge")
	ErrCycle         = errors.New("cycle")
	ErrBlocked       = errors.New("blocked by incomplete parents")
)

func New() *Graph {
	return &Graph{
		nodes:   make(map[NodeID]Node),
		parents: make(map[NodeID]map[NodeID]struct{}),
	}
}

func (g *Graph) Clone() *Graph {
	next := New()
	for _, id := range g.order {
		n := g.nodes[id]
		next.nodes[id] = n
		next.order = append(next.order, id)
		next.parents[id] = make(map[NodeID]struct{}, len(g.parents[id]))
		for parent := range g.parents[id] {
			next.parents[id][parent] = struct{}{}
		}
	}
	return next
}

func (g *Graph) Nodes() []Node {
	nodes := make([]Node, 0, len(g.order))
	for _, id := range g.order {
		nodes = append(nodes, g.nodes[id])
	}
	return nodes
}

func (g *Graph) Order() []NodeID {
	return append([]NodeID(nil), g.order...)
}

func (g *Graph) HasNode(id NodeID) bool {
	_, ok := g.nodes[id]
	return ok
}

func (g *Graph) Node(id NodeID) (Node, bool) {
	n, ok := g.nodes[id]
	return n, ok
}

func (g *Graph) AddNode(text string) (NodeID, error) {
	text = strings.TrimSpace(text)
	id := g.UniqueID(text)
	return id, g.AddNodeWithID(id, text)
}

func (g *Graph) AddNodeWithID(id NodeID, text string) error {
	id, text = cleanID(id), strings.TrimSpace(text)
	if id == "" {
		return ErrEmptyNodeID
	}
	if !ValidID(id) {
		return fmt.Errorf("invalid node id %q", id)
	}
	if text == "" {
		return ErrEmptyNodeText
	}
	if _, exists := g.nodes[id]; exists {
		return ErrDuplicateNode
	}
	g.nodes[id] = Node{ID: id, Text: text}
	g.order = append(g.order, id)
	g.parents[id] = make(map[NodeID]struct{})
	return nil
}

func (g *Graph) EditNodeText(id NodeID, newText string) error {
	id, newText = cleanID(id), strings.TrimSpace(newText)
	if newText == "" {
		return ErrEmptyNodeText
	}
	node, ok := g.nodes[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownNode, id)
	}
	node.Text = newText
	g.nodes[id] = node
	return nil
}

func (g *Graph) SetComplete(id NodeID, complete bool) error {
	id = cleanID(id)
	node, ok := g.nodes[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownNode, id)
	}
	node.Complete = complete
	g.nodes[id] = node
	return nil
}

func (g *Graph) MarkComplete(id NodeID) error {
	id = cleanID(id)
	incomplete, err := g.IncompleteParentsOf(id)
	if err != nil {
		return err
	}
	if len(incomplete) > 0 {
		return BlockedError{Node: id, Parents: incomplete}
	}
	node := g.nodes[id]
	node.Complete = true
	g.nodes[id] = node
	return nil
}

func (g *Graph) MarkIncompleteCascade(id NodeID) (int, error) {
	id = cleanID(id)
	if !g.HasNode(id) {
		return 0, fmt.Errorf("%w: %s", ErrUnknownNode, id)
	}
	affected := map[NodeID]struct{}{id: {}}
	var walk func(NodeID)
	walk = func(current NodeID) {
		children, _ := g.ChildrenOf(current)
		for _, child := range children {
			if _, seen := affected[child]; seen {
				continue
			}
			affected[child] = struct{}{}
			walk(child)
		}
	}
	walk(id)

	count := 0
	for _, current := range g.order {
		if _, ok := affected[current]; !ok {
			continue
		}
		node := g.nodes[current]
		if !node.Complete {
			continue
		}
		node.Complete = false
		g.nodes[current] = node
		count++
	}
	return count, nil
}

func (g *Graph) IncompleteParentsOf(id NodeID) ([]NodeID, error) {
	parents, err := g.ParentsOf(id)
	if err != nil {
		return nil, err
	}
	incomplete := make([]NodeID, 0)
	for _, parent := range parents {
		if !g.nodes[parent].Complete {
			incomplete = append(incomplete, parent)
		}
	}
	return incomplete, nil
}

func (g *Graph) RekeyByText() (map[NodeID]NodeID, error) {
	next := New()
	mapping := make(map[NodeID]NodeID, len(g.nodes))
	for _, oldID := range g.order {
		node := g.nodes[oldID]
		newID := next.UniqueID(node.Text)
		if err := next.AddNodeWithID(newID, node.Text); err != nil {
			return nil, err
		}
		if node.Complete {
			if err := next.SetComplete(newID, true); err != nil {
				return nil, err
			}
		}
		mapping[oldID] = newID
	}
	for _, oldChild := range g.order {
		newChild := mapping[oldChild]
		parents, err := g.ParentsOf(oldChild)
		if err != nil {
			return nil, err
		}
		for _, oldParent := range parents {
			newParent, ok := mapping[oldParent]
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrUnknownNode, oldParent)
			}
			if err := next.AddEdge(newParent, newChild); err != nil {
				return nil, err
			}
		}
	}
	*g = *next
	return mapping, nil
}

func (g *Graph) RekeyPreview() (map[NodeID]NodeID, error) {
	return g.Clone().RekeyByText()
}

func (g *Graph) DeleteNode(id NodeID) error {
	id = cleanID(id)
	if !g.HasNode(id) {
		return fmt.Errorf("%w: %s", ErrUnknownNode, id)
	}
	delete(g.nodes, id)
	delete(g.parents, id)
	for child := range g.parents {
		delete(g.parents[child], id)
	}
	for i, existing := range g.order {
		if existing == id {
			g.order = append(g.order[:i], g.order[i+1:]...)
			break
		}
	}
	return nil
}

func (g *Graph) AddEdge(parent, child NodeID) error {
	parent, child = cleanID(parent), cleanID(child)
	if parent == child {
		return ErrSelfEdge
	}
	if !g.HasNode(parent) {
		return fmt.Errorf("%w: %s", ErrUnknownNode, parent)
	}
	if !g.HasNode(child) {
		return fmt.Errorf("%w: %s", ErrUnknownNode, child)
	}
	if _, exists := g.parents[child][parent]; exists {
		return ErrDuplicateEdge
	}
	if path, found := g.Path(child, parent); found {
		return CycleError{Path: append([]NodeID{parent}, path...)}
	}
	g.parents[child][parent] = struct{}{}
	return nil
}

func (g *Graph) RemoveEdge(parent, child NodeID) error {
	parent, child = cleanID(parent), cleanID(child)
	if !g.HasNode(parent) {
		return fmt.Errorf("%w: %s", ErrUnknownNode, parent)
	}
	if !g.HasNode(child) {
		return fmt.Errorf("%w: %s", ErrUnknownNode, child)
	}
	if _, exists := g.parents[child][parent]; !exists {
		return fmt.Errorf("edge %s -> %s does not exist", parent, child)
	}
	delete(g.parents[child], parent)
	return nil
}

func (g *Graph) ParentsOf(id NodeID) ([]NodeID, error) {
	id = cleanID(id)
	if !g.HasNode(id) {
		return nil, fmt.Errorf("%w: %s", ErrUnknownNode, id)
	}
	return g.sortedIDs(g.parents[id]), nil
}

func (g *Graph) ChildrenOf(id NodeID) ([]NodeID, error) {
	id = cleanID(id)
	if !g.HasNode(id) {
		return nil, fmt.Errorf("%w: %s", ErrUnknownNode, id)
	}
	children := make(map[NodeID]struct{})
	for child, parents := range g.parents {
		if _, ok := parents[id]; ok {
			children[child] = struct{}{}
		}
	}
	return g.sortedIDs(children), nil
}

func (g *Graph) Edges() []Edge {
	edges := make([]Edge, 0)
	for _, child := range g.order {
		parents, _ := g.ParentsOf(child)
		for _, parent := range parents {
			edges = append(edges, Edge{Parent: parent, Child: child})
		}
	}
	return edges
}

func (g *Graph) Roots() []NodeID {
	roots := make([]NodeID, 0)
	for _, id := range g.order {
		if len(g.parents[id]) == 0 {
			roots = append(roots, id)
		}
	}
	return roots
}

func (g *Graph) Ready() []NodeID {
	ready := make([]NodeID, 0)
	for _, id := range g.order {
		if g.nodes[id].Complete {
			continue
		}
		blocked := false
		for parent := range g.parents[id] {
			if !g.nodes[parent].Complete {
				blocked = true
				break
			}
		}
		if !blocked {
			ready = append(ready, id)
		}
	}
	return ready
}

func (g *Graph) Completed() []NodeID {
	completed := make([]NodeID, 0)
	for _, id := range g.order {
		if g.nodes[id].Complete {
			completed = append(completed, id)
		}
	}
	return completed
}

func (g *Graph) CompleteCount() int {
	count := 0
	for _, node := range g.nodes {
		if node.Complete {
			count++
		}
	}
	return count
}

func (g *Graph) ResetCompletion() int {
	count := 0
	for _, id := range g.order {
		node := g.nodes[id]
		if !node.Complete {
			continue
		}
		node.Complete = false
		g.nodes[id] = node
		count++
	}
	return count
}

func (g *Graph) Leaves() []NodeID {
	childCounts := make(map[NodeID]int, len(g.nodes))
	for _, parents := range g.parents {
		for parent := range parents {
			childCounts[parent]++
		}
	}
	leaves := make([]NodeID, 0)
	for _, id := range g.order {
		if childCounts[id] == 0 {
			leaves = append(leaves, id)
		}
	}
	return leaves
}

func (g *Graph) MoveTo(id NodeID, index int) bool {
	id = cleanID(id)
	from := g.indexOf(id)
	if from < 0 {
		return false
	}
	if index < 0 {
		index = 0
	}
	if index >= len(g.order) {
		index = len(g.order) - 1
	}
	if from == index {
		return false
	}
	g.order = append(g.order[:from], g.order[from+1:]...)
	g.order = append(g.order[:index], append([]NodeID{id}, g.order[index:]...)...)
	return true
}

func (g *Graph) Stats() Stats {
	edges := 0
	for _, parents := range g.parents {
		edges += len(parents)
	}
	return Stats{
		Nodes:    len(g.nodes),
		Edges:    edges,
		Complete: g.CompleteCount(),
		Ready:    len(g.Ready()),
		Roots:    len(g.Roots()),
		Leaves:   len(g.Leaves()),
	}
}

func (g *Graph) Validate() error {
	for _, id := range g.order {
		if !ValidID(id) {
			return fmt.Errorf("invalid node id %q", id)
		}
		node := g.nodes[id]
		if strings.TrimSpace(node.Text) == "" {
			return fmt.Errorf("%w: %s", ErrEmptyNodeText, id)
		}
		for parent := range g.parents[id] {
			if parent == id {
				return ErrSelfEdge
			}
			if !g.HasNode(parent) {
				return fmt.Errorf("%w: %s", ErrUnknownNode, parent)
			}
		}
		if node.Complete {
			incomplete, err := g.IncompleteParentsOf(id)
			if err != nil {
				return err
			}
			if len(incomplete) > 0 {
				return BlockedError{Node: id, Parents: incomplete}
			}
		}
	}
	if cycle, found := g.findCycle(); found {
		return CycleError{Path: cycle}
	}
	return nil
}

func (g *Graph) TransitiveEdges() []TransitiveEdge {
	edges := make([]TransitiveEdge, 0)
	for _, edge := range g.Edges() {
		path, found := g.pathSkipping(edge.Parent, edge.Child, edge)
		if found {
			edges = append(edges, TransitiveEdge{
				Parent: edge.Parent,
				Child:  edge.Child,
				Path:   path,
			})
		}
	}
	return edges
}

func (g *Graph) Path(from, to NodeID) ([]NodeID, bool) {
	return g.pathSkipping(from, to, Edge{})
}

func (g *Graph) pathSkipping(from, to NodeID, skip Edge) ([]NodeID, bool) {
	from, to = cleanID(from), cleanID(to)
	if !g.HasNode(from) || !g.HasNode(to) {
		return nil, false
	}
	visited := map[NodeID]bool{}
	var walk func(NodeID, []NodeID) ([]NodeID, bool)
	walk = func(current NodeID, path []NodeID) ([]NodeID, bool) {
		if current == to {
			return append(path, current), true
		}
		if visited[current] {
			return nil, false
		}
		visited[current] = true
		children, _ := g.ChildrenOf(current)
		for _, child := range children {
			if current == skip.Parent && child == skip.Child {
				continue
			}
			if foundPath, ok := walk(child, append(path, current)); ok {
				return foundPath, true
			}
		}
		return nil, false
	}
	return walk(from, nil)
}

func (g *Graph) UniqueID(text string) NodeID {
	base := Slugify(text)
	if base == "" {
		base = "node"
	}
	id := NodeID(base)
	if !g.HasNode(id) {
		return id
	}
	for i := 2; ; i++ {
		candidate := NodeID(fmt.Sprintf("%s-%d", base, i))
		if !g.HasNode(candidate) {
			return candidate
		}
	}
}

func ValidID(id NodeID) bool {
	if id == "" {
		return false
	}
	for i, r := range string(id) {
		valid := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_'
		if !valid {
			return false
		}
		if i == 0 && (r == '-' || r == '_') {
			return false
		}
	}
	return true
}

type CycleError struct {
	Path []NodeID
}

type BlockedError struct {
	Node    NodeID
	Parents []NodeID
}

func (e CycleError) Error() string {
	if len(e.Path) == 0 {
		return ErrCycle.Error()
	}
	parts := make([]string, 0, len(e.Path))
	for _, id := range e.Path {
		parts = append(parts, string(id))
	}
	return fmt.Sprintf("%s: %s", ErrCycle, strings.Join(parts, " -> "))
}

func (e CycleError) Is(target error) bool {
	return target == ErrCycle
}

func (e BlockedError) Error() string {
	parts := make([]string, 0, len(e.Parents))
	for _, id := range e.Parents {
		parts = append(parts, string(id))
	}
	return fmt.Sprintf("%s: %s needs %s", ErrBlocked, e.Node, strings.Join(parts, ", "))
}

func (e BlockedError) Is(target error) bool {
	return target == ErrBlocked
}

func (g *Graph) sortedIDs(set map[NodeID]struct{}) []NodeID {
	pos := make(map[NodeID]int, len(g.order))
	for i, id := range g.order {
		pos[id] = i
	}
	ids := make([]NodeID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.SliceStable(ids, func(i, j int) bool {
		return pos[ids[i]] < pos[ids[j]]
	})
	return ids
}

func (g *Graph) indexOf(id NodeID) int {
	for i, existing := range g.order {
		if existing == id {
			return i
		}
	}
	return -1
}

func (g *Graph) findCycle() ([]NodeID, bool) {
	const (
		unseen = iota
		visiting
		done
	)
	state := make(map[NodeID]int, len(g.nodes))
	stack := make([]NodeID, 0, len(g.nodes))
	var walk func(NodeID) ([]NodeID, bool)
	walk = func(id NodeID) ([]NodeID, bool) {
		state[id] = visiting
		stack = append(stack, id)
		children, _ := g.ChildrenOf(id)
		for _, child := range children {
			switch state[child] {
			case unseen:
				if cycle, found := walk(child); found {
					return cycle, true
				}
			case visiting:
				for i, stacked := range stack {
					if stacked == child {
						cycle := append([]NodeID(nil), stack[i:]...)
						cycle = append(cycle, child)
						return cycle, true
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = done
		return nil, false
	}
	for _, id := range g.order {
		if state[id] == unseen {
			if cycle, found := walk(id); found {
				return cycle, true
			}
		}
	}
	return nil, false
}

func cleanID(id NodeID) NodeID {
	return NodeID(strings.TrimSpace(string(id)))
}
