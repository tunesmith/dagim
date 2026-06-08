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

func TestSlugifyCompactsLongRecipeSteps(t *testing.T) {
	tests := map[string]string{
		"In gumbo pot, combine 1/2 cup butter and 1/2 cup white rice flour":         "gumbo-pot-combine-butter-white-rice-flour",
		"Prepare rice in rice cooker and press start":                               "prepare-rice-cooker-press-start",
		"Put two quarts of hot water in the chicken pot":                            "put-hot-water-chicken-pot",
		"Cook over medium heat, frequently stirring to make a dark brown roux":      "cook-medium-heat-stirring-dark-brown-roux",
		"Slice 12 ounces Andouille sausage":                                         "slice-andouille-sausage",
		"Review RFC #42":                                                            "review-rfc-42",
		"Investigate extraordinarilylongwordthatexceedstheindividualtokenlimit now": "investigate-extraordinarilylongwordt-now",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := Slugify(input); got != want {
				t.Fatalf("Slugify(%q) = %q, want %q", input, got, want)
			}
		})
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

func TestEditNodeTextPreservesIDAndEdges(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B")
	if err := g.AddEdge("a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := g.EditNodeText("a", "A2"); err != nil {
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

func TestRekeyByTextRegeneratesIDsAndPreservesEdges(t *testing.T) {
	g := mustGraph(t,
		"in-gumbo-pot-combine-1-2-cup-butter-and-1-2-cup-white-rice-flour", "In gumbo pot, combine 1/2 cup butter and 1/2 cup white rice flour",
		"cook-over-medium-heat-frequently-stirring-to-make-a-dark-brown-roux-about-15-minutes", "Cook over medium heat, frequently stirring to make a dark brown roux, about 15 minutes",
	)
	mustAddEdge(t, g,
		"in-gumbo-pot-combine-1-2-cup-butter-and-1-2-cup-white-rice-flour",
		"cook-over-medium-heat-frequently-stirring-to-make-a-dark-brown-roux-about-15-minutes",
	)
	mapping, err := g.RekeyByText()
	if err != nil {
		t.Fatal(err)
	}
	if mapping["in-gumbo-pot-combine-1-2-cup-butter-and-1-2-cup-white-rice-flour"] != "gumbo-pot-combine-butter-white-rice-flour" {
		t.Fatalf("mapping = %#v", mapping)
	}
	if mapping["cook-over-medium-heat-frequently-stirring-to-make-a-dark-brown-roux-about-15-minutes"] != "cook-medium-heat-stirring-dark-brown-roux" {
		t.Fatalf("mapping = %#v", mapping)
	}
	parents, err := g.ParentsOf("cook-medium-heat-stirring-dark-brown-roux")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parents, []NodeID{"gumbo-pot-combine-butter-white-rice-flour"}) {
		t.Fatalf("parents = %#v", parents)
	}
}

func TestRekeyPreviewDoesNotMutateGraph(t *testing.T) {
	g := mustGraph(t,
		"old-id", "Better node text",
		"child", "Child",
	)
	mustAddEdge(t, g, "old-id", "child")

	mapping, err := g.RekeyPreview()
	if err != nil {
		t.Fatal(err)
	}
	if mapping["old-id"] != "better-node-text" {
		t.Fatalf("mapping = %#v", mapping)
	}
	if !g.HasNode("old-id") {
		t.Fatal("preview mutated original node ID")
	}
	parents, err := g.ParentsOf("child")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parents, []NodeID{"old-id"}) {
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
	if !g.MoveTo("a", 1) {
		t.Fatal("move later failed")
	}
	if got := g.Order(); !reflect.DeepEqual(got, []NodeID{"b", "a", "c"}) {
		t.Fatalf("order = %#v", got)
	}
	if got := g.Roots(); !reflect.DeepEqual(got, []NodeID{"b", "a"}) {
		t.Fatalf("roots after reorder = %#v", got)
	}
}

func TestLeavesAndOrdering(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B", "c", "C", "d", "D")
	mustAddEdge(t, g, "a", "c")
	mustAddEdge(t, g, "b", "c")
	if got := g.Leaves(); !reflect.DeepEqual(got, []NodeID{"c", "d"}) {
		t.Fatalf("leaves = %#v", got)
	}
	if !g.MoveTo("d", 2) {
		t.Fatal("move earlier failed")
	}
	if got := g.Order(); !reflect.DeepEqual(got, []NodeID{"a", "b", "d", "c"}) {
		t.Fatalf("order = %#v", got)
	}
	if got := g.Leaves(); !reflect.DeepEqual(got, []NodeID{"d", "c"}) {
		t.Fatalf("leaves after reorder = %#v", got)
	}
}

func TestReadyUsesCompletionState(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B", "c", "C")
	mustAddEdge(t, g, "a", "c")
	mustAddEdge(t, g, "b", "c")

	if got := g.Ready(); !reflect.DeepEqual(got, []NodeID{"a", "b"}) {
		t.Fatalf("ready = %#v", got)
	}
	if err := g.SetComplete("a", true); err != nil {
		t.Fatal(err)
	}
	if got := g.Ready(); !reflect.DeepEqual(got, []NodeID{"b"}) {
		t.Fatalf("ready after a complete = %#v", got)
	}
	if err := g.SetComplete("b", true); err != nil {
		t.Fatal(err)
	}
	if got := g.Ready(); !reflect.DeepEqual(got, []NodeID{"c"}) {
		t.Fatalf("ready after b complete = %#v", got)
	}
	if count := g.ResetCompletion(); count != 2 {
		t.Fatalf("reset count = %d", count)
	}
	if got := g.Ready(); !reflect.DeepEqual(got, []NodeID{"a", "b"}) {
		t.Fatalf("ready after reset = %#v", got)
	}
}

func TestMarkCompleteRequiresCompleteParents(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B", "c", "C")
	mustAddEdge(t, g, "a", "c")
	mustAddEdge(t, g, "b", "c")

	err := g.MarkComplete("c")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v", err)
	}
	var blocked BlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("blocked error did not expose parents: %v", err)
	}
	if !reflect.DeepEqual(blocked.Parents, []NodeID{"a", "b"}) {
		t.Fatalf("blocked parents = %#v", blocked.Parents)
	}
	node, _ := g.Node("c")
	if node.Complete {
		t.Fatal("blocked node was marked complete")
	}

	if err := g.MarkComplete("a"); err != nil {
		t.Fatal(err)
	}
	if err := g.MarkComplete("b"); err != nil {
		t.Fatal(err)
	}
	if err := g.MarkComplete("c"); err != nil {
		t.Fatal(err)
	}
	node, _ = g.Node("c")
	if !node.Complete {
		t.Fatal("unblocked node was not marked complete")
	}
}

func TestMarkIncompleteCascadeClearsCompletedDescendants(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B", "alt", "Alt", "c", "C", "sibling", "Sibling")
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", "c")
	mustAddEdge(t, g, "alt", "c")
	mustAddEdge(t, g, "a", "sibling")
	for _, id := range []NodeID{"a", "alt", "b", "c", "sibling"} {
		if err := g.MarkComplete(id); err != nil {
			t.Fatal(err)
		}
	}

	count, err := g.MarkIncompleteCascade("b")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count = %d", count)
	}
	for _, id := range []NodeID{"b", "c"} {
		node, _ := g.Node(id)
		if node.Complete {
			t.Fatalf("%s remained complete", id)
		}
	}
	for _, id := range []NodeID{"a", "alt", "sibling"} {
		node, _ := g.Node(id)
		if !node.Complete {
			t.Fatalf("%s was unexpectedly marked incomplete", id)
		}
	}
	if got := g.Ready(); !reflect.DeepEqual(got, []NodeID{"b"}) {
		t.Fatalf("ready = %#v", got)
	}
}

func TestValidateRejectsBlockedCompletionState(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B")
	mustAddEdge(t, g, "a", "b")
	if err := g.SetComplete("b", true); err != nil {
		t.Fatal(err)
	}

	if err := g.Validate(); !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v", err)
	}
}

func TestTransitiveEdgesReportsDirectEdgesWithAlternatePaths(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B", "c", "C", "d", "D", "x", "X")
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", "c")
	mustAddEdge(t, g, "c", "d")
	mustAddEdge(t, g, "a", "d")
	mustAddEdge(t, g, "x", "d")

	got := g.TransitiveEdges()
	want := []TransitiveEdge{{
		Parent: "a",
		Child:  "d",
		Path:   []NodeID{"a", "b", "c", "d"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("transitive edges = %#v", got)
	}
}

func TestOrderStartsFromCompletionState(t *testing.T) {
	g := mustGraph(t, "a", "A", "b", "B", "c", "C")
	mustAddEdge(t, g, "a", "c")
	mustAddEdge(t, g, "b", "c")
	if err := g.SetComplete("a", true); err != nil {
		t.Fatal(err)
	}
	order := NewOrder(g)
	if got := order.Available(); !reflect.DeepEqual(got, []NodeID{"b"}) {
		t.Fatalf("available = %#v", got)
	}
	if err := order.Pick("c"); err == nil {
		t.Fatal("expected blocked c")
	}
	if err := order.Pick("b"); err != nil {
		t.Fatal(err)
	}
	if got := order.Available(); !reflect.DeepEqual(got, []NodeID{"c"}) {
		t.Fatalf("available after b = %#v", got)
	}
	if !order.Undo() {
		t.Fatal("undo failed")
	}
	if got := order.Available(); !reflect.DeepEqual(got, []NodeID{"b"}) {
		t.Fatalf("available after undo = %#v", got)
	}
	order.Reset()
	if got := order.Output(); len(got) != 0 {
		t.Fatalf("output after reset = %#v", got)
	}
	if got := order.Available(); !reflect.DeepEqual(got, []NodeID{"b"}) {
		t.Fatalf("available after reset = %#v", got)
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
