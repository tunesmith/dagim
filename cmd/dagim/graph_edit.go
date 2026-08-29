// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/tunesmith/dagim/internal/dagimfile"
	"github.com/tunesmith/dagim/internal/graph"
	domainquery "github.com/tunesmith/dagim/internal/query"
)

type edgeOutput struct {
	Parent string `json:"parent"`
	Child  string `json:"child"`
}

type graphEditOutput struct {
	Action            string       `json:"action"`
	DryRun            bool         `json:"dry_run"`
	Changed           bool         `json:"changed"`
	Node              *nodeOutput  `json:"node"`
	PreviousText      *string      `json:"previous_text"`
	EdgesAdded        []edgeOutput `json:"edges_added"`
	EdgesRemoved      []edgeOutput `json:"edges_removed"`
	CompletionChanged []nodeOutput `json:"completion_changed"`
	NewlyReady        []nodeOutput `json:"newly_ready"`
	NewlyBlocked      []nodeOutput `json:"newly_blocked"`
	Stats             statsOutput  `json:"stats"`
}

type stringListFlag []string

func (f *stringListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func runAddCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	text := fs.String("text", "", "node text")
	var parents, children stringListFlag
	fs.Var(&parents, "parent", "parent node ID (repeatable)")
	fs.Var(&children, "child", "child node ID (repeatable)")
	fs.Usage = func() { writeAddUsage(fs.Output()) }
	valueFlags := map[string]bool{"text": true, "parent": true, "child": true}
	if err := fs.Parse(flagsFirst(args, valueFlags)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "add")
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return diagnosticError("usage", "add expects exactly one file", "add")
	}
	if strings.TrimSpace(*text) == "" {
		return diagnosticError("text_required", "add requires --text", "--text")
	}
	return addNode(fs.Arg(0), *text, parents, children, *jsonOutput, stdout)
}

func runEditCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim edit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	text := fs.String("text", "", "new node text")
	fs.Usage = func() { writeEditUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args, map[string]bool{"text": true})); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "edit")
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return diagnosticError("usage", "edit expects a file and node ID", "edit")
	}
	if strings.TrimSpace(*text) == "" {
		return diagnosticError("text_required", "edit requires --text", "--text")
	}
	return editNode(fs.Arg(0), graph.NodeID(fs.Arg(1)), *text, *jsonOutput, stdout)
}

func runLinkCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim link", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	fs.Usage = func() { writeLinkUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args, nil)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "link")
	}
	if fs.NArg() != 3 {
		fs.Usage()
		return diagnosticError("usage", "link expects a file, parent ID, and child ID", "link")
	}
	return editEdge(fs.Arg(0), graph.NodeID(fs.Arg(1)), graph.NodeID(fs.Arg(2)), true, *jsonOutput, stdout)
}

func runUnlinkCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim unlink", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	fs.Usage = func() { writeUnlinkUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args, nil)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "unlink")
	}
	if fs.NArg() != 3 {
		fs.Usage()
		return diagnosticError("usage", "unlink expects a file, parent ID, and child ID", "unlink")
	}
	return editEdge(fs.Arg(0), graph.NodeID(fs.Arg(1)), graph.NodeID(fs.Arg(2)), false, *jsonOutput, stdout)
}

func runDeleteCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	dryRun := fs.Bool("dry-run", false, "preview without saving")
	fs.Usage = func() { writeDeleteUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args, nil)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "delete")
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return diagnosticError("usage", "delete expects a file and node ID", "delete")
	}
	return deleteNode(fs.Arg(0), graph.NodeID(fs.Arg(1)), *dryRun, *jsonOutput, stdout)
}

func addNode(path, text string, parents, children []string, jsonOutput bool, stdout io.Writer) error {
	g, existed, err := dagimfile.LoadOrEmptyState(path)
	if err != nil {
		return err
	}
	before := g.Clone()
	id, err := g.AddNode(text)
	if err != nil {
		return err
	}
	edges := make([]edgeOutput, 0, len(parents)+len(children))
	for _, parent := range parents {
		parentID := graph.NodeID(parent)
		if err := addEdgeForEditing(g, parentID, id); err != nil {
			return err
		}
		edges = append(edges, edgeOutput{Parent: string(parentID), Child: string(id)})
	}
	for _, child := range children {
		childID := graph.NodeID(child)
		if err := addEdgeForEditing(g, id, childID); err != nil {
			return err
		}
		edges = append(edges, edgeOutput{Parent: string(id), Child: string(childID)})
	}
	if err := g.Validate(); err != nil {
		return err
	}
	if existed {
		if err := dagimfile.SaveAtomic(path, g); err != nil {
			return err
		}
	} else if err := dagimfile.CreateAtomic(path, g); err != nil {
		return err
	}
	node, _ := g.Node(id)
	result := newGraphEditOutput("add", before, g)
	summary := summarizeNode(domainquery.Summarize(g, node))
	result.Node = &summary
	result.Changed = true
	result.EdgesAdded = edges
	return writeGraphEditResult(stdout, result, jsonOutput)
}

func editNode(path string, id graph.NodeID, text string, jsonOutput bool, stdout io.Writer) error {
	g, err := dagimfile.Load(path)
	if err != nil {
		return err
	}
	before := g.Clone()
	oldNode, ok := g.Node(id)
	if !ok {
		return diagnosticError("unknown_node", fmt.Sprintf("%s: %s", graph.ErrUnknownNode, id), string(id))
	}
	if err := g.EditNodeText(id, text); err != nil {
		return err
	}
	node, _ := g.Node(id)
	changed := oldNode.Text != node.Text
	if changed {
		if err := dagimfile.SaveAtomic(path, g); err != nil {
			return err
		}
	}
	result := newGraphEditOutput("edit", before, g)
	summary := summarizeNode(domainquery.Summarize(g, node))
	result.Node = &summary
	result.PreviousText = &oldNode.Text
	result.Changed = changed
	return writeGraphEditResult(stdout, result, jsonOutput)
}

func editEdge(path string, parent, child graph.NodeID, add, jsonOutput bool, stdout io.Writer) error {
	g, err := dagimfile.Load(path)
	if err != nil {
		return err
	}
	before := g.Clone()
	if err := ensureKnownNode(g, parent); err != nil {
		return err
	}
	if err := ensureKnownNode(g, child); err != nil {
		return err
	}
	edge := edgeOutput{Parent: string(parent), Child: string(child)}
	action := "unlink"
	if add {
		action = "link"
		if err := addEdgeForEditing(g, parent, child); err != nil {
			return err
		}
	} else if err := g.RemoveEdge(parent, child); err != nil {
		return err
	}
	if err := g.Validate(); err != nil {
		return err
	}
	if err := dagimfile.SaveAtomic(path, g); err != nil {
		return err
	}
	result := newGraphEditOutput(action, before, g)
	result.Changed = true
	if add {
		result.EdgesAdded = append(result.EdgesAdded, edge)
	} else {
		result.EdgesRemoved = append(result.EdgesRemoved, edge)
	}
	return writeGraphEditResult(stdout, result, jsonOutput)
}

func deleteNode(path string, id graph.NodeID, dryRun, jsonOutput bool, stdout io.Writer) error {
	g, err := dagimfile.Load(path)
	if err != nil {
		return err
	}
	before := g.Clone()
	node, ok := g.Node(id)
	if !ok {
		return diagnosticError("unknown_node", fmt.Sprintf("%s: %s", graph.ErrUnknownNode, id), string(id))
	}
	deleted := summarizeNode(domainquery.Summarize(g, node))
	edges := make([]edgeOutput, 0)
	for _, edge := range g.Edges() {
		if edge.Parent == id || edge.Child == id {
			edges = append(edges, edgeOutput{Parent: string(edge.Parent), Child: string(edge.Child)})
		}
	}
	if err := g.DeleteNode(id); err != nil {
		return err
	}
	if err := g.Validate(); err != nil {
		return err
	}
	result := newGraphEditOutput("delete", before, g)
	result.DryRun = dryRun
	result.Changed = true
	result.Node = &deleted
	result.EdgesRemoved = edges
	if !dryRun {
		if err := dagimfile.SaveAtomic(path, g); err != nil {
			return err
		}
	}
	return writeGraphEditResult(stdout, result, jsonOutput)
}

func addEdgeForEditing(g *graph.Graph, parent, child graph.NodeID) error {
	if err := ensureKnownNode(g, parent); err != nil {
		return err
	}
	if err := ensureKnownNode(g, child); err != nil {
		return err
	}
	if err := g.AddEdge(parent, child); err != nil {
		return err
	}
	parentNode, _ := g.Node(parent)
	childNode, _ := g.Node(child)
	if childNode.Complete && !parentNode.Complete {
		_, err := g.MarkIncompleteCascade(child)
		return err
	}
	return nil
}

func ensureKnownNode(g *graph.Graph, id graph.NodeID) error {
	if g.HasNode(id) {
		return nil
	}
	return diagnosticError("unknown_node", fmt.Sprintf("%s: %s", graph.ErrUnknownNode, id), string(id))
}

func newGraphEditOutput(action string, before, after *graph.Graph) graphEditOutput {
	transitions := domainquery.Compare(before, after)
	return graphEditOutput{
		Action:            action,
		Node:              nil,
		PreviousText:      nil,
		EdgesAdded:        make([]edgeOutput, 0),
		EdgesRemoved:      make([]edgeOutput, 0),
		CompletionChanged: summarizeNodes(transitions.CompletionChanged),
		NewlyReady:        summarizeNodes(transitions.NewlyReady),
		NewlyBlocked:      summarizeNodes(transitions.NewlyBlocked),
		Stats:             makeStatsOutput(after.Stats()),
	}
}

func writeGraphEditResult(w io.Writer, result graphEditOutput, jsonOutput bool) error {
	if jsonOutput {
		return writeJSON(w, result)
	}
	switch result.Action {
	case "add":
		writeMutationNodes(w, "added", []nodeOutput{*result.Node})
	case "edit":
		if result.Changed {
			fmt.Fprintf(w, "edited: %s\n", result.Node.ID)
			fmt.Fprintf(w, "previous text: %s\n", *result.PreviousText)
			fmt.Fprintf(w, "text: %s\n", result.Node.Text)
		} else {
			fmt.Fprintf(w, "no change: %s already has that text\n", result.Node.ID)
		}
	case "link":
		writeEdges(w, "linked", result.EdgesAdded)
	case "unlink":
		writeEdges(w, "unlinked", result.EdgesRemoved)
	case "delete":
		label := "deleted"
		if result.DryRun {
			label = "would delete"
		}
		writeMutationNodes(w, label, []nodeOutput{*result.Node})
	}
	if result.Action == "add" && len(result.EdgesAdded) > 0 {
		writeEdges(w, "linked", result.EdgesAdded)
	}
	if result.Action == "delete" && len(result.EdgesRemoved) > 0 {
		label := "unlinked"
		if result.DryRun {
			label = "would unlink"
		}
		writeEdges(w, label, result.EdgesRemoved)
	}
	if len(result.CompletionChanged) > 0 {
		writeMutationNodes(w, "reopened", result.CompletionChanged)
	}
	if len(result.NewlyReady) > 0 {
		writeMutationNodes(w, "newly ready", result.NewlyReady)
	}
	if len(result.NewlyBlocked) > 0 {
		writeMutationNodes(w, "newly blocked", result.NewlyBlocked)
	}
	return nil
}

func writeEdges(w io.Writer, label string, edges []edgeOutput) {
	fmt.Fprintf(w, "%s:\n", label)
	for _, edge := range edges {
		fmt.Fprintf(w, "  %s -> %s\n", edge.Parent, edge.Child)
	}
}

func writeAddUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim add [--parent NODE]... [--child NODE]... [--json] FILE --text TEXT")
	fmt.Fprintln(w, "Add a node, optionally linking existing parents and children.")
}

func writeEditUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim edit [--json] FILE NODE --text TEXT")
	fmt.Fprintln(w, "Edit node text without changing its stable ID or relationships.")
}

func writeLinkUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim link [--json] FILE PARENT CHILD")
	fmt.Fprintln(w, "Link existing nodes, reopening invalidated completed work if needed.")
}

func writeUnlinkUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim unlink [--json] FILE PARENT CHILD")
	fmt.Fprintln(w, "Remove an edge between existing nodes.")
}

func writeDeleteUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim delete [--dry-run] [--json] FILE NODE")
	fmt.Fprintln(w, "Delete a node and its incident edges, optionally previewing the result.")
}
