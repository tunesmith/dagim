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

func TestCompleteCommandSavesAndReportsNewlyReady(t *testing.T) {
	path := writeTestGraph(t)

	var stdout, stderr bytes.Buffer
	args := []string{"complete", path, "chop-vegetables", "--json"}
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO complete: %v", err)
	}
	var result mutationOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal complete JSON: %v\n%s", err, stdout.String())
	}
	if result.SchemaVersion != 1 || result.Action != "complete" || result.DryRun {
		t.Fatalf("complete result metadata = %#v", result)
	}
	if got, want := outputIDs(result.Changed), []string{"chop-vegetables"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("changed = %v, want %v", got, want)
	}
	if got, want := outputIDs(result.NewlyReady), []string{"cook-dinner"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("newly_ready = %v, want %v", got, want)
	}
	if result.NewlyBlocked == nil || len(result.NewlyBlocked) != 0 {
		t.Fatalf("newly_blocked = %#v", result.NewlyBlocked)
	}
	if result.Stats.Complete != 2 || result.Stats.Ready != 1 {
		t.Fatalf("stats = %#v", result.Stats)
	}

	g, err := dagimfile.Load(path)
	if err != nil {
		t.Fatalf("load saved graph: %v", err)
	}
	chop, _ := g.Node("chop-vegetables")
	if !chop.Complete {
		t.Fatalf("chop-vegetables was not persisted complete")
	}
}

func TestCompleteCommandRejectsBlockedNodeWithoutChangingFile(t *testing.T) {
	path := writeTestGraph(t)
	before := readTestFile(t, path)

	var stdout, stderr bytes.Buffer
	err := runWithIO([]string{"complete", "--json", path, "serve-dinner"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "blocked by incomplete parents") {
		t.Fatalf("complete blocked error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("complete blocked stdout = %q", stdout.String())
	}
	if after := readTestFile(t, path); !bytes.Equal(after, before) {
		t.Fatalf("blocked completion changed the file")
	}
}

func TestCompleteCommandIsIdempotent(t *testing.T) {
	path := writeTestGraph(t)
	var stdout, stderr bytes.Buffer
	args := []string{"complete", path, "chop-vegetables", "--json"}
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("first complete: %v", err)
	}
	before := readTestFile(t, path)

	stdout.Reset()
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("second complete: %v", err)
	}
	var result mutationOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal second complete: %v", err)
	}
	if len(result.Changed) != 0 || !result.Node.Complete {
		t.Fatalf("idempotent result = %#v", result)
	}
	if after := readTestFile(t, path); !bytes.Equal(after, before) {
		t.Fatalf("idempotent completion rewrote the file")
	}
}

func TestReopenDryRunReportsCascadeWithoutSaving(t *testing.T) {
	path := writeCompletedTestGraph(t)
	before := readTestFile(t, path)

	var stdout, stderr bytes.Buffer
	args := []string{"reopen", path, "chop-vegetables", "--dry-run", "--json"}
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO reopen --dry-run: %v", err)
	}
	var result mutationOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal reopen JSON: %v\n%s", err, stdout.String())
	}
	if result.Action != "reopen" || !result.DryRun {
		t.Fatalf("reopen result metadata = %#v", result)
	}
	if got, want := outputIDs(result.Changed), []string{"chop-vegetables", "cook-dinner", "serve-dinner"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("changed = %v, want %v", got, want)
	}
	if got, want := outputIDs(result.NewlyReady), []string{"chop-vegetables"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("newly_ready = %v, want %v", got, want)
	}
	if got, want := outputIDs(result.NewlyBlocked), []string{"cook-dinner", "serve-dinner"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("newly_blocked = %v, want %v", got, want)
	}
	if after := readTestFile(t, path); !bytes.Equal(after, before) {
		t.Fatalf("dry run changed the file")
	}
}

func TestReopenCommandSavesCascade(t *testing.T) {
	path := writeCompletedTestGraph(t)

	var stdout, stderr bytes.Buffer
	if err := runWithIO([]string{"reopen", path, "chop-vegetables"}, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO reopen: %v", err)
	}
	for _, want := range []string{
		"reopened:",
		"ready\tchop-vegetables\tChop vegetables",
		"blocked\tcook-dinner\tCook dinner",
		"newly blocked:",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("reopen output missing %q:\n%s", want, stdout.String())
		}
	}

	g, err := dagimfile.Load(path)
	if err != nil {
		t.Fatalf("load reopened graph: %v", err)
	}
	for _, id := range []string{"chop-vegetables", "cook-dinner", "serve-dinner"} {
		node, _ := g.Node(graph.NodeID(id))
		if node.Complete {
			t.Fatalf("%s remained complete after cascade", id)
		}
	}
	prepare, _ := g.Node("prepare-rice")
	if !prepare.Complete {
		t.Fatalf("unrelated prepare-rice completion was cleared")
	}
}

func TestMutationCommandHelp(t *testing.T) {
	for _, command := range []string{"complete", "reopen"} {
		var stdout, stderr bytes.Buffer
		if err := runWithIO([]string{"help", command}, &stdout, &stderr); err != nil {
			t.Fatalf("help %s: %v", command, err)
		}
		if !strings.Contains(stdout.String(), "Usage: dagim "+command) {
			t.Fatalf("help %s output = %q", command, stdout.String())
		}
	}
}

func outputIDs(nodes []nodeOutput) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}

func writeCompletedTestGraph(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "completed-dinner.dagim")
	content := `# dagim v1

node prepare-rice: Prepare rice
  complete

node chop-vegetables: Chop vegetables
  complete

node cook-dinner: Cook dinner
  complete
  parent prepare-rice  # Prepare rice
  parent chop-vegetables  # Chop vegetables

node serve-dinner: Serve dinner
  complete
  parent cook-dinner  # Cook dinner
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write completed graph: %v", err)
	}
	return path
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
