// SPDX-License-Identifier: GPL-3.0-or-later

package query

import (
	"reflect"
	"testing"

	"github.com/tunesmith/dagim/internal/graph"
)

func TestStatesFiltersRelationsAndSearchPreserveOrder(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("first", "First alpha"))
	must(t, g.AddNodeWithID("second", "Second"))
	must(t, g.AddNodeWithID("third", "Third alpha"))
	must(t, g.AddEdge("first", "second"))
	must(t, g.AddEdge("second", "third"))
	must(t, g.MarkComplete("first"))

	if got := ids(List(g, FilterComplete)); !reflect.DeepEqual(got, []graph.NodeID{"first"}) {
		t.Fatalf("complete = %#v", got)
	}
	if got := ids(List(g, FilterReady)); !reflect.DeepEqual(got, []graph.NodeID{"second"}) {
		t.Fatalf("ready = %#v", got)
	}
	blocked := List(g, FilterBlocked)
	if got := ids(blocked); !reflect.DeepEqual(got, []graph.NodeID{"third"}) || !reflect.DeepEqual(blocked[0].BlockedBy, []graph.NodeID{"second"}) {
		t.Fatalf("blocked = %#v", blocked)
	}
	if got := ids(Search(g, "ALPHA")); !reflect.DeepEqual(got, []graph.NodeID{"first", "third"}) {
		t.Fatalf("search = %#v", got)
	}
	relations, err := RelationsFor(g, "second")
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(relations.Parents); !reflect.DeepEqual(got, []graph.NodeID{"first"}) {
		t.Fatalf("parents = %#v", got)
	}
	if got := ids(relations.Children); !reflect.DeepEqual(got, []graph.NodeID{"third"}) {
		t.Fatalf("children = %#v", got)
	}
}

func TestReadyLeavesAndTransitions(t *testing.T) {
	before := graph.New()
	must(t, before.AddNodeWithID("a", "A"))
	must(t, before.AddNodeWithID("b", "B"))
	must(t, before.AddEdge("a", "b"))
	after := before.Clone()
	must(t, after.MarkComplete("a"))

	if got := ids(Ready(before, true)); !reflect.DeepEqual(got, []graph.NodeID{"a"}) {
		t.Fatalf("ready + complete = %#v", got)
	}
	if got := ids(Leaves(before, false)); !reflect.DeepEqual(got, []graph.NodeID{"b"}) {
		t.Fatalf("leaves = %#v", got)
	}
	transitions := Compare(before, after)
	if got := ids(transitions.CompletionChanged); !reflect.DeepEqual(got, []graph.NodeID{"a"}) {
		t.Fatalf("completion changed = %#v", got)
	}
	if got := ids(transitions.NewlyReady); !reflect.DeepEqual(got, []graph.NodeID{"b"}) {
		t.Fatalf("newly ready = %#v", got)
	}
}

func ids(nodes []NodeSummary) []graph.NodeID {
	result := make([]graph.NodeID, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, node.Node.ID)
	}
	return result
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
