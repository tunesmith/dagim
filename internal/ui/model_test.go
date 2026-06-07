package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dagim/internal/graph"
)

func TestRewriteRegeneratesIDsAndSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gumbo.dagim")
	g := graph.New()
	must(t, g.AddNodeWithID("in-gumbo-pot-combine-1-2-cup-butter-and-1-2-cup-white-rice-flour", "In gumbo pot, combine 1/2 cup butter and 1/2 cup white rice flour"))
	must(t, g.AddNodeWithID("cook-over-medium-heat-frequently-stirring-to-make-a-dark-brown-roux-about-15-minutes", "Cook over medium heat, frequently stirring to make a dark brown roux, about 15 minutes"))
	must(t, g.AddEdge(
		"in-gumbo-pot-combine-1-2-cup-butter-and-1-2-cup-white-rice-flour",
		"cook-over-medium-heat-frequently-stirring-to-make-a-dark-brown-roux-about-15-minutes",
	))

	m := New(path, g)
	m.current = "cook-over-medium-heat-frequently-stirring-to-make-a-dark-brown-roux-about-15-minutes"
	m.dirty = true
	m = m.rewrite()

	if m.dirty {
		t.Fatal("rewrite should save and clear dirty state")
	}
	if m.current != "cook-medium-heat-stirring-dark-brown-roux" {
		t.Fatalf("current = %q", m.current)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "in-gumbo-pot-combine-1-2-cup-butter") {
		t.Fatalf("old long ID remains:\n%s", text)
	}
	if !strings.Contains(text, "parent gumbo-pot-combine-butter-white-rice-flour") {
		t.Fatalf("parent reference was not rewritten:\n%s", text)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
