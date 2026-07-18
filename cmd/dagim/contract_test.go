// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func TestJSONContractReadCommands(t *testing.T) {
	path := writeTestGraph(t)

	check := decodeJSONObject(t, runJSONCommand(t, "check", path, "--json"))
	assertExactJSONKeys(t, check,
		"canonical", "ok", "schema_version", "stats", "transitive_edges",
	)
	assertSchemaVersionOne(t, check)
	assertExactJSONKeys(t, decodeRawObject(t, check["stats"]),
		"complete", "edges", "leaves", "nodes", "ready", "roots",
	)
	runJSONCommand(t, "link", path, "prepare-rice", "serve-dinner", "--json")
	check = decodeJSONObject(t, runJSONCommand(t, "check", path, "--json"))
	transitiveEdges := decodeRawArray(t, check["transitive_edges"])
	if len(transitiveEdges) == 0 {
		t.Fatal("check contract fixture returned no transitive edges")
	}
	assertExactJSONKeys(t, decodeRawObject(t, transitiveEdges[0]), "child", "parent", "path")

	ready := decodeJSONObject(t, runJSONCommand(t, "ready", path, "--json"))
	assertExactJSONKeys(t, ready, "nodes", "schema_version")
	assertSchemaVersionOne(t, ready)
	readyNodes := decodeRawArray(t, ready["nodes"])
	if len(readyNodes) == 0 {
		t.Fatal("ready contract fixture returned no nodes")
	}
	assertNodeOutputKeys(t, decodeRawObject(t, readyNodes[0]))

	show := decodeJSONObject(t, runJSONCommand(t, "show", path, "cook-dinner", "--json"))
	assertExactJSONKeys(t, show, "children", "node", "parents", "schema_version")
	assertSchemaVersionOne(t, show)
	assertNodeOutputKeys(t, decodeRawObject(t, show["node"]))
}

func TestJSONContractCompletionMutation(t *testing.T) {
	path := writeTestGraph(t)
	result := decodeJSONObject(t, runJSONCommand(t,
		"complete", path, "chop-vegetables", "--json",
	))
	assertExactJSONKeys(t, result,
		"action", "changed", "dry_run", "newly_blocked", "newly_ready", "node", "schema_version", "stats",
	)
	assertSchemaVersionOne(t, result)
	assertNodeOutputKeys(t, decodeRawObject(t, result["node"]))
	changed := decodeRawArray(t, result["changed"])
	if len(changed) == 0 {
		t.Fatal("complete contract fixture returned no changed nodes")
	}
	assertNodeOutputKeys(t, decodeRawObject(t, changed[0]))
}

func TestJSONContractGraphEditMutation(t *testing.T) {
	path := writeTestGraph(t)
	result := decodeJSONObject(t, runJSONCommand(t,
		"add", path,
		"--text", "New step",
		"--parent", "prepare-rice",
		"--json",
	))
	assertExactJSONKeys(t, result,
		"action", "changed", "completion_changed", "dry_run", "edges_added", "edges_removed",
		"newly_blocked", "newly_ready", "node", "previous_text", "schema_version", "stats",
	)
	assertSchemaVersionOne(t, result)
	assertNodeOutputKeys(t, decodeRawObject(t, result["node"]))
	edges := decodeRawArray(t, result["edges_added"])
	if len(edges) == 0 {
		t.Fatal("add contract fixture returned no added edges")
	}
	assertExactJSONKeys(t, decodeRawObject(t, edges[0]), "child", "parent")
}

func runJSONCommand(t *testing.T, args ...string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if err := runWithIO(args, &stdout, &stderr); err != nil {
		t.Fatalf("runWithIO %v: %v", args, err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("runWithIO %v stderr = %q", args, stderr.String())
	}
	return stdout.Bytes()
}

func decodeJSONObject(t *testing.T, data []byte) map[string]json.RawMessage {
	t.Helper()
	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode JSON object: %v\n%s", err, data)
	}
	return result
}

func decodeRawObject(t *testing.T, data json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	return decodeJSONObject(t, data)
}

func decodeRawArray(t *testing.T, data json.RawMessage) []json.RawMessage {
	t.Helper()
	var result []json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode JSON array: %v\n%s", err, data)
	}
	return result
}

func assertExactJSONKeys(t *testing.T, object map[string]json.RawMessage, want ...string) {
	t.Helper()
	got := make([]string, 0, len(object))
	for key := range object {
		got = append(got, key)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON keys = %v, want %v", got, want)
	}
}

func assertSchemaVersionOne(t *testing.T, object map[string]json.RawMessage) {
	t.Helper()
	var version int
	if err := json.Unmarshal(object["schema_version"], &version); err != nil {
		t.Fatalf("decode schema_version: %v", err)
	}
	if version != 1 {
		t.Fatalf("schema_version = %d, want 1", version)
	}
}

func assertNodeOutputKeys(t *testing.T, node map[string]json.RawMessage) {
	t.Helper()
	assertExactJSONKeys(t, node, "blocked_by", "complete", "id", "ready", "state", "text")
}
