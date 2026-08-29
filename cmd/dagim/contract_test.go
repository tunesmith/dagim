// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestJSONContractReadCommands(t *testing.T) {
	path := writeTestGraph(t)

	checkEnvelope, check := decodeSuccessEnvelope(t, runJSONCommand(t, "check", path, "--json"))
	assertEnvelope(t, checkEnvelope, true)
	assertExactJSONKeys(t, check, "canonical", "ok", "stats", "transitive_edges")
	assertExactJSONKeys(t, decodeRawObject(t, check["stats"]),
		"complete", "edges", "leaves", "nodes", "ready", "roots",
	)
	runJSONCommand(t, "link", path, "prepare-rice", "serve-dinner", "--json")
	_, check = decodeSuccessEnvelope(t, runJSONCommand(t, "check", path, "--json"))
	transitiveEdges := decodeRawArray(t, check["transitive_edges"])
	if len(transitiveEdges) == 0 {
		t.Fatal("check contract fixture returned no transitive edges")
	}
	assertExactJSONKeys(t, decodeRawObject(t, transitiveEdges[0]), "child", "parent", "path")

	readyEnvelope, ready := decodeSuccessEnvelope(t, runJSONCommand(t, "ready", path, "--json"))
	assertEnvelope(t, readyEnvelope, true)
	assertExactJSONKeys(t, ready, "nodes")
	readyNodes := decodeRawArray(t, ready["nodes"])
	if len(readyNodes) == 0 {
		t.Fatal("ready contract fixture returned no nodes")
	}
	assertNodeOutputKeys(t, decodeRawObject(t, readyNodes[0]))

	showEnvelope, show := decodeSuccessEnvelope(t, runJSONCommand(t, "show", path, "cook-dinner", "--json"))
	assertEnvelope(t, showEnvelope, true)
	assertExactJSONKeys(t, show, "children", "node", "parents")
	assertNodeOutputKeys(t, decodeRawObject(t, show["node"]))
}

func TestJSONContractCompletionMutation(t *testing.T) {
	path := writeTestGraph(t)
	envelope, result := decodeSuccessEnvelope(t, runJSONCommand(t,
		"complete", path, "chop-vegetables", "--json",
	))
	assertExactJSONKeys(t, result,
		"action", "changed", "dry_run", "newly_blocked", "newly_ready", "node", "stats",
	)
	assertEnvelope(t, envelope, true)
	assertNodeOutputKeys(t, decodeRawObject(t, result["node"]))
	changed := decodeRawArray(t, result["changed"])
	if len(changed) == 0 {
		t.Fatal("complete contract fixture returned no changed nodes")
	}
	assertNodeOutputKeys(t, decodeRawObject(t, changed[0]))
}

func TestJSONContractGraphEditMutation(t *testing.T) {
	path := writeTestGraph(t)
	envelope, result := decodeSuccessEnvelope(t, runJSONCommand(t,
		"add", path,
		"--text", "New step",
		"--parent", "prepare-rice",
		"--json",
	))
	assertExactJSONKeys(t, result,
		"action", "changed", "completion_changed", "dry_run", "edges_added", "edges_removed",
		"newly_blocked", "newly_ready", "node", "previous_text", "stats",
	)
	assertEnvelope(t, envelope, true)
	assertNodeOutputKeys(t, decodeRawObject(t, result["node"]))
	edges := decodeRawArray(t, result["edges_added"])
	if len(edges) == 0 {
		t.Fatal("add contract fixture returned no added edges")
	}
	assertExactJSONKeys(t, decodeRawObject(t, edges[0]), "child", "parent")
}

func TestJSONContractFailureEnvelope(t *testing.T) {
	path := writeTestGraph(t)
	var stdout, stderr bytes.Buffer
	err := runWithIO([]string{"show", path, "missing", "--json"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("show missing succeeded")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
	envelope := decodeJSONObject(t, stdout.Bytes())
	assertEnvelope(t, envelope, false)
	if string(envelope["result"]) != "null" {
		t.Fatalf("result = %s", envelope["result"])
	}
	diagnostics := decodeRawArray(t, envelope["diagnostics"])
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	item := decodeRawObject(t, diagnostics[0])
	assertExactJSONKeys(t, item, "code", "element", "message", "severity")
	var code string
	var element string
	if err := json.Unmarshal(item["code"], &code); err != nil || code != "unknown_node" {
		t.Fatalf("code = %q, err %v", code, err)
	}
	if err := json.Unmarshal(item["element"], &element); err != nil || element != "missing" {
		t.Fatalf("element = %q, err %v", element, err)
	}
}

func TestJSONContractParseFailureCarriesLineAndVersionUsesEnvelope(t *testing.T) {
	path := writeTestGraph(t)
	if err := os.WriteFile(path, []byte("# dagim v1\n\nnot valid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := runWithIO([]string{"check", path, "--json"}, &stdout, &stderr)
	if err == nil || stderr.Len() != 0 {
		t.Fatalf("err=%v stderr=%q", err, stderr.String())
	}
	envelope := decodeJSONObject(t, stdout.Bytes())
	assertEnvelope(t, envelope, false)
	diagnostics := decodeRawArray(t, envelope["diagnostics"])
	item := decodeRawObject(t, diagnostics[0])
	assertExactJSONKeys(t, item, "code", "line", "message", "severity")
	var code string
	var line int
	if err := json.Unmarshal(item["code"], &code); err != nil || code != "malformed_line" {
		t.Fatalf("code=%q err=%v", code, err)
	}
	if err := json.Unmarshal(item["line"], &line); err != nil || line != 3 {
		t.Fatalf("line=%d err=%v", line, err)
	}

	stdout.Reset()
	if err := runWithIO([]string{"--version", "--json"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	_, result := decodeSuccessEnvelope(t, stdout.Bytes())
	assertExactJSONKeys(t, result, "version")
}

func decodeSuccessEnvelope(t *testing.T, data []byte) (map[string]json.RawMessage, map[string]json.RawMessage) {
	t.Helper()
	envelope := decodeJSONObject(t, data)
	return envelope, decodeRawObject(t, envelope["result"])
}

func assertEnvelope(t *testing.T, envelope map[string]json.RawMessage, ok bool) {
	t.Helper()
	assertExactJSONKeys(t, envelope, "diagnostics", "ok", "result", "schema_version")
	var version int
	if err := json.Unmarshal(envelope["schema_version"], &version); err != nil || version != 2 {
		t.Fatalf("schema_version = %d, err %v", version, err)
	}
	var actualOK bool
	if err := json.Unmarshal(envelope["ok"], &actualOK); err != nil || actualOK != ok {
		t.Fatalf("ok = %v, err %v", actualOK, err)
	}
	if envelope["diagnostics"] == nil {
		t.Fatal("diagnostics missing")
	}
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

func assertNodeOutputKeys(t *testing.T, node map[string]json.RawMessage) {
	t.Helper()
	assertExactJSONKeys(t, node, "blocked_by", "complete", "id", "ready", "state", "text")
}
