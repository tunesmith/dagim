// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func collectionFixture(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	a, b, path := filepath.Join(dir, "house"), filepath.Join(dir, "work.dagim"), filepath.Join(dir, "daily.custom")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("node d17: D17: First\nnode d18: D18: Next\n  parent d17\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runJSONCommand(t, "collection", "create", path, "--json")
	runJSONCommand(t, "collection", "add", path, a, "--as", "house", "--json")
	runJSONCommand(t, "collection", "add", path, b, "--as", "work", "--json")
	return path, a, b
}

func TestCollectionJSONContractsAndWorkflow(t *testing.T) {
	path, a, b := collectionFixture(t)
	aBefore, _ := os.ReadFile(a)
	bBefore, _ := os.ReadFile(b)
	envelope, result := decodeSuccessEnvelope(t, runJSONCommand(t, "collection", "list", path, "--json"))
	assertEnvelope(t, envelope, true)
	assertExactJSONKeys(t, result, "action", "members", "order")
	members := decodeRawArray(t, result["members"])
	if len(members) != 2 {
		t.Fatal("members", len(members))
	}
	assertExactJSONKeys(t, decodeRawObject(t, members[0]), "name", "path", "file")
	order := decodeRawArray(t, result["order"])
	assertExactJSONKeys(t, decodeRawObject(t, order[0]), "member", "id")
	_, result = decodeSuccessEnvelope(t, runJSONCommand(t, "ready", path, "--json"))
	nodes := decodeRawArray(t, result["nodes"])
	if len(nodes) != 2 {
		t.Fatal("ready", len(nodes))
	}
	node := decodeRawObject(t, nodes[0])
	assertExactJSONKeys(t, node, "id", "text", "state", "complete", "ready", "blocked_by", "member", "file", "ref")
	if string(node["ref"]) != `"house:d17"` {
		t.Fatalf("ref = %s", node["ref"])
	}
	var source string
	realA, err := filepath.EvalSymlinks(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(node["file"], &source); err != nil || source != realA {
		t.Fatalf("file = %q: %v", source, err)
	}
	runJSONCommand(t, "collection", "move", path, "work:d17", "--before", "house:d17", "--json")
	_, result = decodeSuccessEnvelope(t, runJSONCommand(t, "ready", path, "--json"))
	nodes = decodeRawArray(t, result["nodes"])
	if string(decodeRawObject(t, nodes[0])["ref"]) != `"work:d17"` {
		t.Fatal("move lost")
	}
	for p, expected := range map[string][]byte{a: aBefore, b: bBefore} {
		raw, _ := os.ReadFile(p)
		if !bytes.Equal(raw, expected) {
			t.Fatal("move changed source", p)
		}
	}
	_, result = decodeSuccessEnvelope(t, runJSONCommand(t, "show", path, "house:d18", "--json"))
	parents := decodeRawArray(t, result["parents"])
	if len(parents) != 1 || string(decodeRawObject(t, parents[0])["ref"]) != `"house:d17"` {
		t.Fatal("parents not scoped")
	}
	_, result = decodeSuccessEnvelope(t, runJSONCommand(t, "complete", path, "house:d17", "--json"))
	assertExactJSONKeys(t, result, "action", "dry_run", "node", "changed", "newly_ready", "newly_blocked", "stats")
	newlyReady := decodeRawArray(t, result["newly_ready"])
	if len(newlyReady) != 1 || string(decodeRawObject(t, newlyReady[0])["ref"]) != `"house:d18"` {
		t.Fatal("wrong newly ready")
	}
	raw, _ := os.ReadFile(b)
	if !bytes.Equal(raw, bBefore) {
		t.Fatal("completion changed other source")
	}
	_, result = decodeSuccessEnvelope(t, runJSONCommand(t, "list", path, "--state", "blocked", "--json"))
	if len(decodeRawArray(t, result["nodes"])) != 1 {
		t.Fatal("wrong blocked list")
	}
	runJSONCommand(t, "reopen", path, "house:d17", "--dry-run", "--json")
	_, result = decodeSuccessEnvelope(t, runJSONCommand(t, "show", a, "d17", "--json"))
	if string(decodeRawObject(t, result["node"])["complete"]) != "true" {
		t.Fatal("dry run saved")
	}
	runJSONCommand(t, "reopen", path, "house:d17", "--json")
	_, result = decodeSuccessEnvelope(t, runJSONCommand(t, "check", path, "--json"))
	assertExactJSONKeys(t, result, "ok", "members", "stats")
	runJSONCommand(t, "--check", path, "--json")
	runJSONCommand(t, "collection", "remove", path, "house", "--json")
	if _, err := os.Stat(a); err != nil {
		t.Fatal("removed source", err)
	}
}

func TestCollectionFailuresHaveOneJSONEnvelopeAndNoWrites(t *testing.T) {
	path, a, _ := collectionFixture(t)
	for _, args := range [][]string{
		{"complete", path, "d17"},
		{"complete", path, "unknown:d17"},
		{"complete", path, "house:d18"},
		{"collection", "move", path, "house:d17", "--before", "work:d17", "--after", "work:d17"},
		{"collection", "add", path, a, "--as", "duplicate"},
		{"collection", "remove", path, "unknown"},
		{"collection", "create", path},
		{"collection", "unknown", path},
		{"collection", "list"},
		{"add", path, "--text", "Must not become a graph"},
	} {
		before, _ := os.ReadFile(path)
		var stdout, stderr bytes.Buffer
		err := runWithIO(append(args, "--json"), &stdout, &stderr)
		if err == nil {
			t.Fatalf("succeeded: %v", args)
		}
		if stderr.Len() > 0 {
			t.Fatalf("stderr for %v: %s", args, stderr.String())
		}
		envelope := decodeJSONObject(t, stdout.Bytes())
		assertEnvelope(t, envelope, false)
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatalf("failed command changed collection: %v", args)
		}
	}
}

func TestMultiFileReadsAndCollectionHelp(t *testing.T) {
	_, a, b := collectionFixture(t)
	for _, command := range []string{"ready", "list"} {
		_, result := decodeSuccessEnvelope(t, runJSONCommand(t, command, a, b, "--json"))
		nodes := decodeRawArray(t, result["nodes"])
		if len(nodes) < 2 {
			t.Fatal("missing multi-file nodes")
		}
		if string(decodeRawObject(t, nodes[0])["ref"]) != `"house:d17"` {
			t.Fatal("unexpected member")
		}
	}
	for _, args := range [][]string{{"help", "collection"}, {"collection", "--help", "--json"}, {"collection", "add", "--help", "--json"}} {
		var stdout, stderr bytes.Buffer
		if err := runWithIO(args, &stdout, &stderr); err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(stdout.Bytes(), []byte("Usage:")) || stderr.Len() != 0 {
			t.Fatal("help stream contract")
		}
	}
}
