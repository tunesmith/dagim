// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionLineUsesInjectedVersion(t *testing.T) {
	original := version
	t.Cleanup(func() {
		version = original
	})

	version = "v1.2.3"

	if got, want := versionLine(), "dagim v1.2.3"; got != want {
		t.Fatalf("versionLine() = %q, want %q", got, want)
	}
}

func TestReadyCommandHumanAndJSON(t *testing.T) {
	path := writeTestGraph(t)

	var stdout, stderr bytes.Buffer
	if err := runWithIO([]string{"ready", path}, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO ready: %v", err)
	}
	if got, want := stdout.String(), "ready\tchop-vegetables\tChop vegetables\n"; got != want {
		t.Fatalf("ready output = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("ready stderr = %q", stderr.String())
	}

	stdout.Reset()
	if err := runWithIO([]string{"ready", path, "--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO ready --json: %v", err)
	}
	var result nodeListOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal ready JSON: %v\n%s", err, stdout.String())
	}
	if result.SchemaVersion != 1 || len(result.Nodes) != 1 {
		t.Fatalf("ready JSON = %#v", result)
	}
	node := result.Nodes[0]
	if node.ID != "chop-vegetables" || node.State != "ready" || !node.Ready || node.BlockedBy == nil {
		t.Fatalf("ready node = %#v", node)
	}
}

func TestListCommandFiltersBlockedNodes(t *testing.T) {
	path := writeTestGraph(t)

	var stdout, stderr bytes.Buffer
	args := []string{"list", path, "--state", "blocked", "--json"}
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO list: %v", err)
	}
	var result nodeListOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal list JSON: %v", err)
	}
	if got, want := len(result.Nodes), 2; got != want {
		t.Fatalf("blocked node count = %d, want %d", got, want)
	}
	if got, want := strings.Join(result.Nodes[0].BlockedBy, ","), "chop-vegetables"; got != want {
		t.Fatalf("first blocked_by = %q, want %q", got, want)
	}
	if got, want := strings.Join(result.Nodes[1].BlockedBy, ","), "cook-dinner"; got != want {
		t.Fatalf("second blocked_by = %q, want %q", got, want)
	}
}

func TestShowCommandIncludesRelationships(t *testing.T) {
	path := writeTestGraph(t)

	var stdout, stderr bytes.Buffer
	if err := runWithIO([]string{"show", "--json", path, "cook-dinner"}, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO show: %v", err)
	}
	var result nodeShowOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal show JSON: %v", err)
	}
	if result.Node.ID != "cook-dinner" || result.Node.State != "blocked" {
		t.Fatalf("shown node = %#v", result.Node)
	}
	if got, want := len(result.Parents), 2; got != want {
		t.Fatalf("parent count = %d, want %d", got, want)
	}
	if got, want := len(result.Children), 1; got != want || result.Children[0].ID != "serve-dinner" {
		t.Fatalf("children = %#v", result.Children)
	}
}

func TestCheckCommandJSONAndLegacyAlias(t *testing.T) {
	path := writeTestGraph(t)

	for _, args := range [][]string{
		{"check", path, "--json"},
		{"--check", path, "--json"},
	} {
		var stdout, stderr bytes.Buffer
		if err := runWithIO(args, &stdout, &stderr); err != nil {
			t.Fatalf("runWithIO %v: %v", args, err)
		}
		var result checkOutput
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal check JSON for %v: %v", args, err)
		}
		if !result.OK || !result.Canonical {
			t.Fatalf("check result for %v = %#v", args, result)
		}
		if result.Stats.Nodes != 4 || result.Stats.Edges != 3 || result.Stats.Ready != 1 {
			t.Fatalf("check stats for %v = %#v", args, result.Stats)
		}
		if result.TransitiveEdges == nil {
			t.Fatalf("transitive_edges should encode as an empty array")
		}
	}
}

func TestReadCommandsRejectUnknownInputs(t *testing.T) {
	path := writeTestGraph(t)

	var stdout, stderr bytes.Buffer
	if err := runWithIO([]string{"list", "--state", "mystery", path}, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "unknown state") {
		t.Fatalf("list unknown state error = %v", err)
	}
	if err := runWithIO([]string{"show", path, "missing"}, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "unknown node") {
		t.Fatalf("show unknown node error = %v", err)
	}
}

func writeTestGraph(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dinner.dagim")
	content := `# dagim v1

node prepare-rice: Prepare rice
  complete

node chop-vegetables: Chop vegetables

node cook-dinner: Cook dinner
  parent prepare-rice  # Prepare rice
  parent chop-vegetables  # Chop vegetables

node serve-dinner: Serve dinner
  parent cook-dinner  # Cook dinner
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write graph: %v", err)
	}
	return path
}
