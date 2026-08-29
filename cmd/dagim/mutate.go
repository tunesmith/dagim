// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/tunesmith/dagim/internal/dagimfile"
	"github.com/tunesmith/dagim/internal/graph"
	domainquery "github.com/tunesmith/dagim/internal/query"
)

type mutationOutput struct {
	Action       string       `json:"action"`
	DryRun       bool         `json:"dry_run"`
	Node         nodeOutput   `json:"node"`
	Changed      []nodeOutput `json:"changed"`
	NewlyReady   []nodeOutput `json:"newly_ready"`
	NewlyBlocked []nodeOutput `json:"newly_blocked"`
	Stats        statsOutput  `json:"stats"`
}

func runCompleteCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim complete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	fs.Usage = func() { writeCompleteUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args, nil)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "complete")
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return diagnosticError("usage", "complete expects a file and node ID", "complete")
	}
	return mutateCompletion(fs.Arg(0), graph.NodeID(fs.Arg(1)), "complete", false, *jsonOutput, stdout)
}

func runReopenCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim reopen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	dryRun := fs.Bool("dry-run", false, "preview without saving")
	fs.Usage = func() { writeReopenUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args, nil)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "reopen")
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return diagnosticError("usage", "reopen expects a file and node ID", "reopen")
	}
	return mutateCompletion(fs.Arg(0), graph.NodeID(fs.Arg(1)), "reopen", *dryRun, *jsonOutput, stdout)
}

func mutateCompletion(path string, id graph.NodeID, action string, dryRun, jsonOutput bool, stdout io.Writer) error {
	g, err := dagimfile.Load(path)
	if err != nil {
		return err
	}
	if !g.HasNode(id) {
		return diagnosticError("unknown_node", fmt.Sprintf("%s: %s", graph.ErrUnknownNode, id), string(id))
	}
	before := g.Clone()

	switch action {
	case "complete":
		if err := g.MarkComplete(id); err != nil {
			return err
		}
	case "reopen":
		if _, err := g.MarkIncompleteCascade(id); err != nil {
			return err
		}
	default:
		return diagnosticError("internal_error", fmt.Sprintf("unknown completion action %q", action), action)
	}

	result, err := completionMutationOutput(action, dryRun, before, g, id)
	if err != nil {
		return err
	}
	if !dryRun && len(result.Changed) > 0 {
		if err := dagimfile.SaveAtomic(path, g); err != nil {
			return err
		}
	}
	if jsonOutput {
		return writeJSON(stdout, result)
	}
	writeMutationHuman(stdout, result)
	return nil
}

func completionMutationOutput(action string, dryRun bool, before, after *graph.Graph, id graph.NodeID) (mutationOutput, error) {
	target, ok := after.Node(id)
	if !ok {
		return mutationOutput{}, fmt.Errorf("%w: %s", graph.ErrUnknownNode, id)
	}
	result := mutationOutput{
		Action:       action,
		DryRun:       dryRun,
		Node:         summarizeNode(domainquery.Summarize(after, target)),
		Changed:      make([]nodeOutput, 0),
		NewlyReady:   make([]nodeOutput, 0),
		NewlyBlocked: make([]nodeOutput, 0),
		Stats:        makeStatsOutput(after.Stats()),
	}
	transitions := domainquery.Compare(before, after)
	result.Changed = summarizeNodes(transitions.CompletionChanged)
	result.NewlyReady = summarizeNodes(transitions.NewlyReady)
	result.NewlyBlocked = summarizeNodes(transitions.NewlyBlocked)
	return result, nil
}

func makeStatsOutput(stats graph.Stats) statsOutput {
	return statsOutput{
		Nodes:    stats.Nodes,
		Edges:    stats.Edges,
		Complete: stats.Complete,
		Ready:    stats.Ready,
		Roots:    stats.Roots,
		Leaves:   stats.Leaves,
	}
}

func writeMutationHuman(w io.Writer, result mutationOutput) {
	if len(result.Changed) == 0 {
		fmt.Fprintf(w, "no change: %s is already %s\n", result.Node.ID, result.Node.State)
	} else {
		label := "completed"
		if result.Action == "reopen" {
			label = "reopened"
			if result.DryRun {
				label = "would reopen"
			}
		}
		writeMutationNodes(w, label, result.Changed)
	}
	if len(result.NewlyReady) > 0 {
		writeMutationNodes(w, "newly ready", result.NewlyReady)
	}
	if len(result.NewlyBlocked) > 0 {
		writeMutationNodes(w, "newly blocked", result.NewlyBlocked)
	}
}

func writeMutationNodes(w io.Writer, label string, nodes []nodeOutput) {
	fmt.Fprintf(w, "%s:\n", label)
	for _, node := range nodes {
		fmt.Fprintf(w, "  %s\t%s\t%s\n", node.State, node.ID, node.Text)
	}
}

func writeCompleteUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim complete [--json] FILE NODE")
	fmt.Fprintln(w, "Mark a ready node complete and report newly ready nodes.")
}

func writeReopenUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim reopen [--dry-run] [--json] FILE NODE")
	fmt.Fprintln(w, "Mark a node and its completed descendants incomplete.")
}
