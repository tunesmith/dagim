// SPDX-License-Identifier: GPL-3.0-or-later

package dagimfile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tunesmith/dagim/internal/graph"
)

func TestParseEmptyGraph(t *testing.T) {
	g, err := Parse("# dagim v1\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes()) != 0 {
		t.Fatalf("nodes = %#v", g.Nodes())
	}
}

func TestParseNodesParentsAndLaterReference(t *testing.T) {
	g, err := Parse(`# dagim v1

node c: C
  complete
  parent a  # stale ignored hint
  parent b

node a: A
  complete

node b: B
  complete
`)
	if err != nil {
		t.Fatal(err)
	}
	parents, err := g.ParentsOf("c")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parents, []graph.NodeID{"a", "b"}) {
		t.Fatalf("parents = %#v", parents)
	}
	node, _ := g.Node("c")
	if !node.Complete {
		t.Fatal("complete line was not parsed")
	}
}

func TestParseIgnoresStandaloneCommentsAnywhere(t *testing.T) {
	g, err := Parse(`# dagim v1

node a: A
  # comment inside node block
  parent b

# between nodes
node b: B
`)
	if err != nil {
		t.Fatal(err)
	}
	parents, _ := g.ParentsOf("a")
	if !reflect.DeepEqual(parents, []graph.NodeID{"b"}) {
		t.Fatalf("parents = %#v", parents)
	}
}

func TestParseRejectsInvalidInput(t *testing.T) {
	tests := []string{
		"node a: A\nnode a: A2\n",
		"node a: A\n  parent missing\n",
		"node : A\n",
		"node a:\n",
		"  parent a\nnode a: A\n",
		"  complete\nnode a: A\n",
		"node a: A\n  parent a\n",
		"node a: A\nnode b: B\n  complete\n  parent a\n",
		"node a: A\n  parent b\n  parent b\nnode b: B\n",
		"not valid\n",
	}
	for _, input := range tests {
		t.Run(strings.ReplaceAll(input, "\n", `\n`), func(t *testing.T) {
			if _, err := Parse(input); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseRejectsCycle(t *testing.T) {
	_, err := Parse(`node a: A
  parent c
node b: B
  parent a
node c: C
  parent b
`)
	if !errors.Is(err, graph.ErrCycle) {
		t.Fatalf("err = %v", err)
	}
}

func TestSerializeCanonical(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("b", "B"))
	must(t, g.AddNodeWithID("a", "A"))
	must(t, g.AddNodeWithID("c", "C"))
	must(t, g.SetComplete("a", true))
	must(t, g.AddEdge("a", "c"))
	must(t, g.AddEdge("b", "c"))

	got := Serialize(g)
	want := `# dagim v1

node b: B

node a: A
  complete

node c: C
  parent b  # B
  parent a  # A
`
	if got != want {
		t.Fatalf("serialize:\n%s", got)
	}
}

func TestSerializeShortensParentHints(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("roux-base", "In gumbo pot, combine 1/2 cup butter and 1/2 cup white rice flour"))
	must(t, g.AddNodeWithID("cook-roux", "Cook roux"))
	must(t, g.AddEdge("roux-base", "cook-roux"))

	got := Serialize(g)
	parentLine := ""
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "parent roux-base") {
			parentLine = line
			break
		}
	}
	if parentLine != "  parent roux-base  # In gumbo pot, combine 1/2 cup butter and 1/2 cup white rice..." {
		t.Fatalf("missing shortened parent hint:\n%s", got)
	}
}

func TestCheckReportsCanonicalFormatting(t *testing.T) {
	input := "node a: A\n"
	result, err := Check(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsCanonical {
		t.Fatal("expected noncanonical input")
	}
	if result.Stats.Nodes != 1 || result.Stats.Edges != 0 || result.Stats.Ready != 1 || result.Stats.Roots != 1 || result.Stats.Leaves != 1 {
		t.Fatalf("stats = %#v", result.Stats)
	}
	if result.Canonical != "# dagim v1\n\nnode a: A\n" {
		t.Fatalf("canonical = %q", result.Canonical)
	}
}

func TestCheckReportsTransitiveEdges(t *testing.T) {
	result, err := Check(`# dagim v1

node a: A

node b: B
  parent a

node c: C
  parent b

node d: D
  parent a
  parent c
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.TransitiveEdges) != 1 {
		t.Fatalf("transitive edges = %#v", result.TransitiveEdges)
	}
	edge := result.TransitiveEdges[0]
	if edge.Parent != "a" || edge.Child != "d" || !reflect.DeepEqual(edge.Path, []graph.NodeID{"a", "b", "c", "d"}) {
		t.Fatalf("edge = %#v", edge)
	}
}

func TestRoundTrip(t *testing.T) {
	input := `# dagim v1

node a: A

node b: B
  parent a  # A
`
	first, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Parse(Serialize(first))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Order(), second.Order()) {
		t.Fatalf("orders = %#v %#v", first.Order(), second.Order())
	}
}

func TestSaveAtomicPreservesModeAndCreateRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.dagim")
	if err := os.WriteFile(path, []byte(Header+"\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	g := graph.New()
	must(t, g.AddNodeWithID("a", "A"))
	if err := SaveAtomic(path, g); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("mode = %o, want 640", got)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	other := graph.New()
	must(t, other.AddNodeWithID("b", "B"))
	if err := CreateAtomic(path, other); !errors.Is(err, os.ErrExist) {
		t.Fatalf("CreateAtomic error = %v, want exists", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("CreateAtomic changed existing file:\n%s", after)
	}
}

func TestCreateAtomicWritesParseableFileAndSaveRejectsInvalidGraph(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.dagim")
	g := graph.New()
	must(t, g.AddNodeWithID("parent", "Parent"))
	must(t, g.AddNodeWithID("child", "Child"))
	must(t, g.SetComplete("child", true))
	must(t, g.AddEdge("parent", "child"))
	if err := SaveAtomic(path, g); !errors.Is(err, graph.ErrBlocked) {
		t.Fatalf("SaveAtomic invalid error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid save created file: %v", err)
	}
	must(t, g.SetComplete("child", false))
	if err := CreateAtomic(path, g); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.HasNode("child") {
		t.Fatal("created graph missing child")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
