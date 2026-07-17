// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tunesmith/dagim/internal/dagimfile"
	"github.com/tunesmith/dagim/internal/graph"
)

func TestAddCommandCreatesReadyNodeAndNewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.dagim")

	var stdout, stderr bytes.Buffer
	args := []string{"add", path, "--text", "First step", "--json"}
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO add: %v", err)
	}
	var result graphEditOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal add JSON: %v\n%s", err, stdout.String())
	}
	if result.Action != "add" || !result.Changed || result.Node == nil {
		t.Fatalf("add result = %#v", result)
	}
	if result.Node.ID != "first-step" || result.Node.State != "ready" {
		t.Fatalf("added node = %#v", result.Node)
	}
	if result.NewlyReady == nil || len(result.NewlyReady) != 0 {
		t.Fatalf("newly_ready = %#v, want an empty array", result.NewlyReady)
	}
	if result.EdgesAdded == nil || result.EdgesRemoved == nil || result.CompletionChanged == nil {
		t.Fatalf("array fields must not encode as null: %#v", result)
	}

	g, err := dagimfile.Load(path)
	if err != nil {
		t.Fatalf("load created graph: %v", err)
	}
	if !g.HasNode("first-step") {
		t.Fatalf("created graph does not contain first-step")
	}
}

func TestAddCommandLinksRepeatedParentsAndChildAtomically(t *testing.T) {
	path := writeCompletedTestGraph(t)

	var stdout, stderr bytes.Buffer
	args := []string{
		"add", path,
		"--parent", "prepare-rice",
		"--text", "New gate",
		"--parent", "chop-vegetables",
		"--child", "cook-dinner",
		"--json",
	}
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO add linked: %v", err)
	}
	var result graphEditOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal linked add JSON: %v", err)
	}
	if result.Node == nil || result.Node.ID != "new-gate" || result.Node.State != "ready" {
		t.Fatalf("added node = %#v", result.Node)
	}
	wantEdges := []edgeOutput{
		{Parent: "prepare-rice", Child: "new-gate"},
		{Parent: "chop-vegetables", Child: "new-gate"},
		{Parent: "new-gate", Child: "cook-dinner"},
	}
	if !reflect.DeepEqual(result.EdgesAdded, wantEdges) {
		t.Fatalf("edges_added = %#v, want %#v", result.EdgesAdded, wantEdges)
	}
	if got, want := outputIDs(result.CompletionChanged), []string{"cook-dinner", "serve-dinner"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("completion_changed = %v, want %v", got, want)
	}
	if got, want := outputIDs(result.NewlyBlocked), []string{"cook-dinner", "serve-dinner"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("newly_blocked = %v, want %v", got, want)
	}

	g, err := dagimfile.Load(path)
	if err != nil {
		t.Fatalf("load graph after linked add: %v", err)
	}
	parents, _ := g.ParentsOf("new-gate")
	if got, want := nodeIDStrings(parents), []string{"prepare-rice", "chop-vegetables"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("new-gate parents = %v, want %v", got, want)
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("saved graph is invalid: %v", err)
	}
}

func TestAddCommandFailureLeavesFileUnchanged(t *testing.T) {
	path := writeTestGraph(t)
	before := readTestFile(t, path)

	var stdout, stderr bytes.Buffer
	args := []string{
		"add", path,
		"--text", "Partially linked node",
		"--parent", "prepare-rice",
		"--child", "missing",
		"--json",
	}
	err := runWithIO(args, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unknown node") {
		t.Fatalf("add error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("failed add stdout = %q", stdout.String())
	}
	if after := readTestFile(t, path); !bytes.Equal(after, before) {
		t.Fatalf("failed add changed the file")
	}
}

func TestEditCommandPreservesIDAndRelationships(t *testing.T) {
	path := writeTestGraph(t)

	var stdout, stderr bytes.Buffer
	args := []string{"edit", path, "cook-dinner", "--text", "Cook supper", "--json"}
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO edit: %v", err)
	}
	var result graphEditOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal edit JSON: %v", err)
	}
	if result.Node == nil || result.Node.ID != "cook-dinner" || result.Node.Text != "Cook supper" {
		t.Fatalf("edited node = %#v", result.Node)
	}
	if result.PreviousText == nil || *result.PreviousText != "Cook dinner" || !result.Changed {
		t.Fatalf("edit metadata = %#v", result)
	}

	g, err := dagimfile.Load(path)
	if err != nil {
		t.Fatalf("load edited graph: %v", err)
	}
	parents, _ := g.ParentsOf("cook-dinner")
	if got, want := nodeIDStrings(parents), []string{"prepare-rice", "chop-vegetables"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parents after edit = %v, want %v", got, want)
	}

	before := readTestFile(t, path)
	stdout.Reset()
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("idempotent edit: %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal idempotent edit: %v", err)
	}
	if result.Changed {
		t.Fatalf("same-text edit reported a change")
	}
	if after := readTestFile(t, path); !bytes.Equal(after, before) {
		t.Fatalf("same-text edit rewrote the file")
	}
}

func TestLinkCommandReopensCompletedChildAndDescendants(t *testing.T) {
	path := writeLinkCascadeGraph(t)

	var stdout, stderr bytes.Buffer
	args := []string{"link", path, "new-parent", "current", "--json"}
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO link: %v", err)
	}
	var result graphEditOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal link JSON: %v", err)
	}
	if got, want := result.EdgesAdded, []edgeOutput{{Parent: "new-parent", Child: "current"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("edges_added = %#v, want %#v", got, want)
	}
	if got, want := outputIDs(result.CompletionChanged), []string{"current", "child"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("completion_changed = %v, want %v", got, want)
	}
	if got, want := outputIDs(result.NewlyBlocked), []string{"current", "child"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("newly_blocked = %v, want %v", got, want)
	}

	g, err := dagimfile.Load(path)
	if err != nil {
		t.Fatalf("load linked graph: %v", err)
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("linked graph invalid: %v", err)
	}
}

func TestLinkCycleFailureLeavesFileUnchanged(t *testing.T) {
	path := writeTestGraph(t)
	before := readTestFile(t, path)

	var stdout, stderr bytes.Buffer
	err := runWithIO([]string{"link", path, "serve-dinner", "cook-dinner"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle error = %v", err)
	}
	if after := readTestFile(t, path); !bytes.Equal(after, before) {
		t.Fatalf("failed cyclic link changed the file")
	}
}

func TestUnlinkCommandReportsNewlyReadyNode(t *testing.T) {
	path := writeTestGraph(t)

	var stdout, stderr bytes.Buffer
	args := []string{"unlink", "--json", path, "chop-vegetables", "cook-dinner"}
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO unlink: %v", err)
	}
	var result graphEditOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal unlink JSON: %v", err)
	}
	if got, want := result.EdgesRemoved, []edgeOutput{{Parent: "chop-vegetables", Child: "cook-dinner"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("edges_removed = %#v, want %#v", got, want)
	}
	if got, want := outputIDs(result.NewlyReady), []string{"cook-dinner"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("newly_ready = %v, want %v", got, want)
	}
}

func TestGraphEditCommandHelp(t *testing.T) {
	for _, command := range []string{"add", "edit", "link", "unlink"} {
		var stdout, stderr bytes.Buffer
		if err := runWithIO([]string{"help", command}, &stdout, &stderr); err != nil {
			t.Fatalf("help %s: %v", command, err)
		}
		if !strings.Contains(stdout.String(), "Usage: dagim "+command) {
			t.Fatalf("help %s output = %q", command, stdout.String())
		}
	}
}

func writeLinkCascadeGraph(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "link-cascade.dagim")
	content := `# dagim v1

node new-parent: New parent

node current: Current
  complete

node child: Child
  complete
  parent current  # Current
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write link cascade graph: %v", err)
	}
	return path
}

func nodeIDStrings(ids []graph.NodeID) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		result = append(result, string(id))
	}
	return result
}
