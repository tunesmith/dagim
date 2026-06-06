package graph

import (
	"errors"
	"reflect"
	"testing"
)

func TestAddNodeGeneratesReadableUniqueID(t *testing.T) {
	g := New()
	first, err := g.AddNode("People think in graphs")
	if err != nil {
		t.Fatal(err)
	}
	second, err := g.AddNode("People think in graphs")
	if err != nil {
		t.Fatal(err)
	}
	if first != "people-think-in-graphs" {
		t.Fatalf("first id = %q", first)
	}
	if second != "people-think-in-graphs-2" {
		t.Fatalf("second id = %q", second)
	}
}

func TestSlugifyFallback(t *testing.T) {
	g := New()
	id, err := g.AddNode("!!!")
	if err != nil {
		t.Fatal(err)
	}
	if id != "node" {
		t.Fatalf("id = %q", id)
	}
}

func TestRenamePreservesIDAndEdges(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B")
	if err := g.AddEdge("a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := g.RenameNode("a", "A2"); err != nil {
		t.Fatal(err)
	}
	node, _ := g.Node("a")
	if node.Text != "A2" {
		t.Fatalf("text = %q", node.Text)
	}
	parents, err := g.ParentsOf("b")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parents, []NodeID{"a"}) {
		t.Fatalf("parents = %#v", parents)
	}
}

func TestDeleteConnectedNodeRemovesIncidentEdges(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B", "c", "C")
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", "c")
	if err := g.DeleteNode("b"); err != nil {
		t.Fatal(err)
	}
	if g.HasNode("b") {
		t.Fatal("b still exists")
	}
	children, err := g.ChildrenOf("a")
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 0 {
		t.Fatalf("children of a = %#v", children)
	}
	parents, err := g.ParentsOf("c")
	if err != nil {
		t.Fatal(err)
	}
	if len(parents) != 0 {
		t.Fatalf("parents of c = %#v", parents)
	}
}

func TestAddEdgeRejectsDuplicateSelfAndCycle(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B", "c", "C")
	mustAddEdge(t, g, "a", "b")
	if err := g.AddEdge("a", "b"); !errors.Is(err, ErrDuplicateEdge) {
		t.Fatalf("duplicate err = %v", err)
	}
	if err := g.AddEdge("a", "a"); !errors.Is(err, ErrSelfEdge) {
		t.Fatalf("self err = %v", err)
	}
	mustAddEdge(t, g, "b", "c")
	err := g.AddEdge("c", "a")
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("cycle err = %v", err)
	}
	var cycle CycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("cycle did not expose path: %v", err)
	}
}

func TestRootsAndOrdering(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B", "c", "C")
	mustAddEdge(t, g, "a", "c")
	if got := g.Roots(); !reflect.DeepEqual(got, []NodeID{"a", "b"}) {
		t.Fatalf("roots = %#v", got)
	}
	if !g.MoveLater("a") {
		t.Fatal("move later failed")
	}
	if got := g.Order(); !reflect.DeepEqual(got, []NodeID{"b", "a", "c"}) {
		t.Fatalf("order = %#v", got)
	}
	if got := g.Roots(); !reflect.DeepEqual(got, []NodeID{"b", "a"}) {
		t.Fatalf("roots after reorder = %#v", got)
	}
}

func TestManualSequence(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B", "c", "C")
	mustAddEdge(t, g, "a", "c")
	mustAddEdge(t, g, "b", "c")
	seq := NewSequence(g)
	if got := seq.Available(); !reflect.DeepEqual(got, []NodeID{"a", "b"}) {
		t.Fatalf("available = %#v", got)
	}
	if err := seq.Pick("a"); err != nil {
		t.Fatal(err)
	}
	if got := seq.Available(); !reflect.DeepEqual(got, []NodeID{"b"}) {
		t.Fatalf("available after a = %#v", got)
	}
	if err := seq.Pick("c"); err == nil {
		t.Fatal("expected blocked c")
	}
	if err := seq.Pick("b"); err != nil {
		t.Fatal(err)
	}
	if got := seq.Available(); !reflect.DeepEqual(got, []NodeID{"c"}) {
		t.Fatalf("available after b = %#v", got)
	}
	if !seq.Undo() {
		t.Fatal("undo failed")
	}
	if got := seq.Available(); !reflect.DeepEqual(got, []NodeID{"b"}) {
		t.Fatalf("available after undo = %#v", got)
	}
	seq.Reset()
	if got := seq.Output(); len(got) != 0 {
		t.Fatalf("output after reset = %#v", got)
	}
}

func mustGraph(t *testing.T, pairs ...string) *Graph {
	t.Helper()
	g := New()
	for i := 0; i < len(pairs); i += 2 {
		if err := g.AddNodeWithID(NodeID(pairs[i]), pairs[i+1]); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func mustAddEdge(t *testing.T, g *Graph, parent, child NodeID) {
	t.Helper()
	if err := g.AddEdge(parent, child); err != nil {
		t.Fatal(err)
	}
}
