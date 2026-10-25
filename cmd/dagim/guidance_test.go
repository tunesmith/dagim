// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestGuidanceJSONContract(t *testing.T) {
	result := runCLIProcess(t, "guidance", "--json")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("process = %+v", result)
	}
	envelope, object := decodeSuccessEnvelope(t, []byte(result.stdout))
	assertEnvelope(t, envelope, true)
	assertExactJSONKeys(t, object, "authoring", "operating")
	if string(envelope["diagnostics"]) != "[]" {
		t.Fatalf("diagnostics = %s", envelope["diagnostics"])
	}
	wantIDs := map[string][]string{
		"authoring": {"describe_what_is_tracked", "concise_self_contained_text", "meaningful_completion", "blocking_prerequisites", "avoid_unnecessary_narration", "suggest_cludia_for_reasoning"},
		"operating": {"preserve_stable_ids", "avoid_accidental_duplicates", "complete_satisfied_work"},
	}
	seen := map[string]bool{}
	for section, want := range wantIDs {
		ids := []string{}
		for _, raw := range decodeRawArray(t, object[section]) {
			assertExactJSONKeys(t, decodeRawObject(t, raw), "id", "title", "description")
			var rule guidanceRule
			if err := json.Unmarshal(raw, &rule); err != nil {
				t.Fatal(err)
			}
			if seen[rule.ID] || rule.ID == "" || rule.Title == "" || rule.Description == "" {
				t.Fatalf("invalid rule: %+v", rule)
			}
			seen[rule.ID] = true
			ids = append(ids, rule.ID)
		}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("%s IDs = %v, want %v", section, ids, want)
		}
	}
}

func TestGuidanceHumanAndJSONExposeAgreedGuidelines(t *testing.T) {
	_, object := decodeSuccessEnvelope(t, runJSONCommand(t, "guidance", "--json"))
	rules := map[string]string{}
	var stdout, stderr bytes.Buffer
	if err := runWithIO([]string{"guidance"}, &stdout, &stderr); err != nil || stderr.Len() != 0 {
		t.Fatalf("err=%v stderr=%q", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Authoring guidelines:") || !strings.Contains(stdout.String(), "Agent operating guidelines:") {
		t.Fatal("guidance sections missing")
	}
	for _, section := range []string{"authoring", "operating"} {
		for _, raw := range decodeRawArray(t, object[section]) {
			var rule guidanceRule
			if err := json.Unmarshal(raw, &rule); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout.String(), rule.Title) || !strings.Contains(stdout.String(), rule.Description) {
				t.Fatalf("human guidance differs for %s", rule.ID)
			}
			rules[rule.ID] = rule.Title + ". " + rule.Description
		}
	}
	// Protect the agreed distinctions rather than requiring a task-only template,
	// predetermined research result, automatic deduplication, or proof ceremony.
	want := map[string][]string{
		"describe_what_is_tracked":     {"work, inquiry, or condition", "graph's purpose", "rather than forcing"},
		"concise_self_contained_text":  {"dates", "constraints", "purpose", "without reconstructing a conversation"},
		"meaningful_completion":        {"separate prerequisites or completion tracking", "without reaching a predetermined conclusion"},
		"blocking_prerequisites":       {"B remains blocked until A is complete", "do not by themselves justify an edge"},
		"avoid_unnecessary_narration":  {"conversation history", "progress reports", "Retain context"},
		"suggest_cludia_for_reasoning": {"Suggest using the cludia CLI", "understandable on its own"},
		"preserve_stable_ids":          {"Preserve IDs across text revisions", "do not delete and recreate"},
		"avoid_accidental_duplicates":  {"Inspect the relevant graph", "intentional repetition is valid", "does not authorize automatically merging or deleting"},
		"complete_satisfied_work":      {"user reporting completion", "Readiness, discussion, or a proposed solution alone does not establish completion", "inconclusive finding"},
	}
	for id, phrases := range want {
		for _, phrase := range phrases {
			if !strings.Contains(rules[id], phrase) {
				t.Errorf("%s missing %q", id, phrase)
			}
		}
	}
}

func TestGuidanceHelpAndDiscovery(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help"}, {"--help", "--json"}} {
		result := runCLIProcess(t, args...)
		if result.exitCode != 0 || result.stderr != "" || !strings.Contains(result.stdout, "dagim guidance [--json]") {
			t.Fatalf("%v: %+v", args, result)
		}
	}
	for _, args := range [][]string{{"help", "guidance"}, {"guidance", "--help"}, {"guidance", "-h"}, {"guidance", "--json", "--help"}} {
		result := runCLIProcess(t, args...)
		if result.exitCode != 0 || result.stderr != "" || !strings.HasPrefix(result.stdout, "Usage: dagim guidance [--json]\n") {
			t.Fatalf("%v: %+v", args, result)
		}
		if strings.Contains(result.stdout, "schema_version") {
			t.Fatal("help must take precedence over JSON")
		}
	}
}

func TestGuidanceUsageFailureContract(t *testing.T) {
	for _, args := range [][]string{{"guidance", "unexpected", "--json"}, {"guidance", "--unknown", "--json"}} {
		result := runCLIProcess(t, args...)
		if result.exitCode != 1 || result.stderr != "" {
			t.Fatalf("%v: %+v", args, result)
		}
		envelope := decodeJSONObject(t, []byte(result.stdout))
		assertEnvelope(t, envelope, false)
		if string(envelope["result"]) != "null" {
			t.Fatal("failed invocation returned a result")
		}
		diagnostics := decodeRawArray(t, envelope["diagnostics"])
		if len(diagnostics) != 1 || string(decodeRawObject(t, diagnostics[0])["code"]) != `"usage"` {
			t.Fatal("usage diagnostic missing")
		}
	}
	result := runCLIProcess(t, "guidance", "unexpected")
	if result.exitCode != 1 || result.stdout != "" || !strings.Contains(result.stderr, "Usage: dagim guidance") {
		t.Fatalf("human usage failure: %+v", result)
	}
}
