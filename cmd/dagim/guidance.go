// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

type guidanceRule struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type guidanceOutput struct {
	Authoring []guidanceRule `json:"authoring"`
	Operating []guidanceRule `json:"operating"`
}

// Both output modes use the same rules so the installed executable exposes the
// same guidance to human readers and agents without repository access.
func dagimGuidance() guidanceOutput {
	return guidanceOutput{
		Authoring: []guidanceRule{
			{
				ID: "describe_what_is_tracked", Title: "Name what is being tracked",
				Description: "Clearly describe the work, inquiry, or condition. Match the graph's purpose rather than forcing every node into an imperative task template.",
			},
			{
				ID: "concise_self_contained_text", Title: "Use concise, self-contained descriptions",
				Description: "Preserve the scope, dates, quantities, constraints, and purpose needed to understand the node without reconstructing a conversation.",
			},
			{
				ID: "meaningful_completion", Title: "Give each node a meaningful completion boundary",
				Description: "Split items when they need separate prerequisites or completion tracking. An investigation can be completed without reaching a predetermined conclusion.",
			},
			{
				ID: "blocking_prerequisites", Title: "Represent blocking prerequisites with edges",
				Description: "A -> B means B remains blocked until A is complete. Association, hierarchy, and preferred presentation order do not by themselves justify an edge.",
			},
			{
				ID: "avoid_unnecessary_narration", Title: "Avoid unnecessary narration",
				Description: "Usually omit conversation history, transient progress reports, repeated justification, and speculative downstream benefits. Retain context that defines the actual work or inquiry.",
			},
			{
				ID: "suggest_cludia_for_reasoning", Title: "Suggest Cludia when supporting reasoning deserves a separate record",
				Description: "Suggest using the cludia CLI to capture rationale, evidence, or competing claims alongside the Dagim graph. Keep the Dagim node understandable on its own.",
			},
		},
		Operating: []guidanceRule{
			{
				ID: "preserve_stable_ids", Title: "Preserve stable node IDs",
				Description: "Use existing stable node IDs when editing or linking nodes. Preserve IDs across text revisions; do not delete and recreate a node merely to reword it.",
			},
			{
				ID: "avoid_accidental_duplicates", Title: "Inspect existing nodes before adding new ones to avoid accidental duplication",
				Description: "Inspect the relevant graph for work already represented there. Similar wording does not necessarily mean duplication, and intentional repetition is valid. This does not authorize automatically merging or deleting nodes.",
			},
			{
				ID: "complete_satisfied_work", Title: "Mark nodes complete when their work or condition has been satisfied",
				Description: "Completion can be supported by the agent's work, an observed result, or the user reporting completion. Readiness, discussion, or a proposed solution alone does not establish completion. An investigation can finish with an inconclusive finding.",
			},
		},
	}
}

func runGuidanceCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim guidance", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	fs.Usage = func() { writeGuidanceUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args, nil)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "guidance")
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return diagnosticError("usage", "guidance does not accept positional arguments", "guidance")
	}
	result := dagimGuidance()
	if *jsonOutput {
		return writeJSON(stdout, result)
	}
	writeRules := func(title string, rules []guidanceRule) {
		fmt.Fprintln(stdout, title)
		for _, rule := range rules {
			fmt.Fprintf(stdout, "- %s. %s\n", rule.Title, rule.Description)
		}
	}
	writeRules("Authoring guidelines:", result.Authoring)
	fmt.Fprintln(stdout)
	writeRules("Agent operating guidelines:", result.Operating)
	return nil
}

func writeGuidanceUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim guidance [--json]")
	fmt.Fprintln(w, "Print node-authoring and agent operating guidelines without opening a file.")
}
