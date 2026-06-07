package graph

import "fmt"

type Order struct {
	graph     *Graph
	base      map[NodeID]struct{}
	processed map[NodeID]struct{}
	output    []NodeID
}

func NewOrder(g *Graph) *Order {
	base := make(map[NodeID]struct{})
	for _, id := range g.order {
		if g.nodes[id].Complete {
			base[id] = struct{}{}
		}
	}
	o := &Order{graph: g, base: base}
	o.Reset()
	return o
}

func (o *Order) Available() []NodeID {
	available := make([]NodeID, 0)
	for _, id := range o.graph.order {
		if _, done := o.processed[id]; done {
			continue
		}
		blocked := false
		for parent := range o.graph.parents[id] {
			if _, done := o.processed[parent]; !done {
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

func (o *Order) Pick(id NodeID) error {
	id = cleanID(id)
	if !o.graph.HasNode(id) {
		return fmt.Errorf("%w: %s", ErrUnknownNode, id)
	}
	for _, available := range o.Available() {
		if available == id {
			o.processed[id] = struct{}{}
			o.output = append(o.output, id)
			return nil
		}
	}
	return fmt.Errorf("node %s is blocked", id)
}

func (o *Order) Undo() bool {
	if len(o.output) == 0 {
		return false
	}
	last := o.output[len(o.output)-1]
	o.output = o.output[:len(o.output)-1]
	delete(o.processed, last)
	return true
}

func (o *Order) Reset() {
	o.processed = make(map[NodeID]struct{}, len(o.base))
	for id := range o.base {
		o.processed[id] = struct{}{}
	}
	o.output = nil
}

func (o *Order) Output() []NodeID {
	return append([]NodeID(nil), o.output...)
}

func (o *Order) Complete() bool {
	return len(o.processed) == len(o.graph.nodes)
}
