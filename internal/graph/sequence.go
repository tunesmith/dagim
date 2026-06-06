package graph

import "fmt"

type Sequence struct {
	graph     *Graph
	processed map[NodeID]struct{}
	output    []NodeID
}

func NewSequence(g *Graph) *Sequence {
	return &Sequence{
		graph:     g,
		processed: make(map[NodeID]struct{}),
	}
}

func (s *Sequence) Available() []NodeID {
	available := make([]NodeID, 0)
	for _, id := range s.graph.order {
		if _, done := s.processed[id]; done {
			continue
		}
		blocked := false
		for parent := range s.graph.parents[id] {
			if _, done := s.processed[parent]; !done {
				blocked = true
				break
			}
		}
		if !blocked {
			available = append(available, id)
		}
	}
	return available
}

func (s *Sequence) Pick(id NodeID) error {
	id = cleanID(id)
	if !s.graph.HasNode(id) {
		return fmt.Errorf("%w: %s", ErrUnknownNode, id)
	}
	for _, available := range s.Available() {
		if available == id {
			s.processed[id] = struct{}{}
			s.output = append(s.output, id)
			return nil
		}
	}
	return fmt.Errorf("node %s is blocked", id)
}

func (s *Sequence) Undo() bool {
	if len(s.output) == 0 {
		return false
	}
	last := s.output[len(s.output)-1]
	s.output = s.output[:len(s.output)-1]
	delete(s.processed, last)
	return true
}

func (s *Sequence) Reset() {
	s.processed = make(map[NodeID]struct{})
	s.output = nil
}

func (s *Sequence) Output() []NodeID {
	return append([]NodeID(nil), s.output...)
}

func (s *Sequence) Complete() bool {
	return len(s.processed) == len(s.graph.nodes)
}
