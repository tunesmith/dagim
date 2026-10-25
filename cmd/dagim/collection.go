// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/tunesmith/dagim/internal/collection"
	"github.com/tunesmith/dagim/internal/graph"
	"github.com/tunesmith/dagim/internal/query"
)

type collectionMemberOutput struct {
	collection.Member
	File string `json:"file"`
}

type collectionOutput struct {
	Action  string                   `json:"action"`
	Members []collectionMemberOutput `json:"members"`
	Order   []collection.Ref         `json:"order"`
}

type collectionNodeOutput struct {
	nodeOutput
	Member string `json:"member"`
	File   string `json:"file"`
	Ref    string `json:"ref"`
}

func collectionNode(member, file string, summary query.NodeSummary) collectionNodeOutput {
	return collectionNodeOutput{nodeOutput: summarizeNode(summary), Member: member, File: file, Ref: (collection.Ref{Member: member, ID: summary.Node.ID}).String()}
}

func collectionNodes(member, file string, nodes []query.NodeSummary) []collectionNodeOutput {
	result := make([]collectionNodeOutput, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, collectionNode(member, file, n))
	}
	return result
}

func writeCollectionNodes(s *collection.Snapshot, filter query.Filter, jsonOutput bool, stdout io.Writer) error {
	nodes := []collectionNodeOutput{}
	for _, item := range s.Items(filter) {
		nodes = append(nodes, collectionNode(item.Member, item.File, item.Summary))
	}
	if jsonOutput {
		return writeJSON(stdout, struct {
			Nodes []collectionNodeOutput `json:"nodes"`
		}{nodes})
	}
	for _, n := range nodes {
		fmt.Fprintf(stdout, "%s\t%s\t%s", n.State, n.Ref, n.Text)
		if len(n.BlockedBy) > 0 {
			fmt.Fprintf(stdout, "\tblocked by: %v", n.BlockedBy)
		}
		fmt.Fprintln(stdout)
	}
	return nil
}

func writeMultiFileNodes(paths []string, filter query.Filter, jsonOutput bool, stdout io.Writer) error {
	s, err := collection.ReadPaths(paths)
	if err != nil {
		return err
	}
	return writeCollectionNodes(s, filter, jsonOutput, stdout)
}

func runCollectionCommand(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		writeCollectionUsage(stdout)
		return diagnosticError("usage", "expected a collection command", "collection")
	}
	if args[0] == "--help" || args[0] == "-h" {
		writeCollectionUsage(stdout)
		return nil
	}
	action := args[0]
	fs := flag.NewFlagSet("dagim collection "+action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	name := fs.String("as", "", "short member name (add only)")
	before := fs.String("before", "", "place before MEMBER:NODE (move only)")
	after := fs.String("after", "", "place after MEMBER:NODE (move only)")
	fs.Usage = func() { writeCollectionUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args[1:], map[string]bool{"as": true, "before": true, "after": true})); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "collection")
	}
	count := 1
	switch action {
	case "create", "list":
	case "add", "remove", "move":
		count = 2
	default:
		return diagnosticError("unknown_command", "unknown collection command: "+action, action)
	}
	if fs.NArg() != count {
		return diagnosticError("usage", "wrong arguments; see dagim help collection", action)
	}
	if action != "add" && *name != "" || action != "move" && (*before != "" || *after != "") {
		return diagnosticError("usage", "--as is for add; --before/--after are for move", action)
	}
	path := fs.Arg(0)
	if action == "create" {
		if err := collection.Create(path); err != nil {
			return err
		}
	}
	d, err := collection.Load(path)
	if err != nil {
		return err
	}
	switch action {
	case "add":
		if err := d.Add(fs.Arg(1), *name); err != nil {
			return err
		}
		if err := d.Save(); err != nil {
			return err
		}
	case "remove":
		if err := d.Remove(fs.Arg(1)); err != nil {
			return err
		}
		if err := d.Save(); err != nil {
			return err
		}
	case "move":
		if (*before == "") == (*after == "") {
			return diagnosticError("usage", "move requires exactly one of --before or --after", action)
		}
		r, err := collection.ParseRef(fs.Arg(1))
		if err != nil {
			return err
		}
		anchorText := *before
		if *after != "" {
			anchorText = *after
		}
		anchor, err := collection.ParseRef(anchorText)
		if err != nil {
			return err
		}
		s, err := d.Snapshot()
		if err != nil {
			return err
		}
		if err := s.Move(r, anchor, *after != ""); err != nil {
			return err
		}
	}
	result := collectionOutput{Action: action, Members: []collectionMemberOutput{}, Order: d.Order}
	for _, m := range d.Members {
		result.Members = append(result.Members, collectionMemberOutput{Member: m, File: d.SourcePath(m)})
	}
	if *jsonOutput {
		return writeJSON(stdout, result)
	}
	fmt.Fprintf(stdout, "%s: %s\n", action, path)
	for _, m := range result.Members {
		fmt.Fprintf(stdout, "  %s\t%s\n", m.Name, m.Path)
	}
	return nil
}

func showCollectionNode(path, text string, jsonOutput bool, stdout io.Writer) error {
	s, err := collection.LoadSnapshot(path)
	if err != nil {
		return err
	}
	r, err := collection.ParseRef(text)
	if err != nil {
		return err
	}
	source, err := s.Lookup(r)
	if err != nil {
		return err
	}
	relations, err := query.RelationsFor(source.Graph, r.ID)
	if err != nil {
		return err
	}
	result := struct {
		Node     collectionNodeOutput   `json:"node"`
		Parents  []collectionNodeOutput `json:"parents"`
		Children []collectionNodeOutput `json:"children"`
	}{collectionNode(r.Member, source.Path, relations.Node), collectionNodes(r.Member, source.Path, relations.Parents), collectionNodes(r.Member, source.Path, relations.Children)}
	if jsonOutput {
		return writeJSON(stdout, result)
	}
	fmt.Fprintf(stdout, "member: %s\nfile: %s\n", r.Member, source.Path)
	writeNodeShow(stdout, nodeShowOutput{Node: summarizeNode(relations.Node), Parents: summarizeNodes(relations.Parents), Children: summarizeNodes(relations.Children)})
	return nil
}

func mutateCollectionCompletion(path string, id graph.NodeID, action string, dryRun, jsonOutput bool, stdout io.Writer) error {
	s, err := collection.LoadSnapshot(path)
	if err != nil {
		return err
	}
	r, err := collection.ParseRef(string(id))
	if err != nil {
		return err
	}
	before, after, err := s.SetCompletion(r, action == "reopen", dryRun)
	if err != nil {
		return err
	}
	source := s.Sources[r.Member]
	node, _ := after.Node(r.ID)
	transitions := query.Compare(before, after)
	result := struct {
		Action       string                 `json:"action"`
		DryRun       bool                   `json:"dry_run"`
		Node         collectionNodeOutput   `json:"node"`
		Changed      []collectionNodeOutput `json:"changed"`
		NewlyReady   []collectionNodeOutput `json:"newly_ready"`
		NewlyBlocked []collectionNodeOutput `json:"newly_blocked"`
		Stats        statsOutput            `json:"stats"`
	}{action, dryRun, collectionNode(r.Member, source.Path, query.Summarize(after, node)), collectionNodes(r.Member, source.Path, transitions.CompletionChanged), collectionNodes(r.Member, source.Path, transitions.NewlyReady), collectionNodes(r.Member, source.Path, transitions.NewlyBlocked), makeStatsOutput(after.Stats())}
	if jsonOutput {
		return writeJSON(stdout, result)
	}
	fmt.Fprintf(stdout, "%s: %s\n", action, r.String())
	legacy, err := completionMutationOutput(action, dryRun, before, after, r.ID)
	if err != nil {
		return err
	}
	writeMutationHuman(stdout, legacy)
	return nil
}

func checkCollection(path string, jsonOutput bool, stdout io.Writer) error {
	s, err := collection.LoadSnapshot(path)
	if err != nil {
		return err
	}
	stats := graph.Stats{}
	for _, source := range s.Sources {
		x := source.Graph.Stats()
		stats.Nodes += x.Nodes
		stats.Edges += x.Edges
		stats.Complete += x.Complete
		stats.Ready += x.Ready
		stats.Roots += x.Roots
		stats.Leaves += x.Leaves
	}
	if jsonOutput {
		return writeJSON(stdout, struct {
			OK      bool        `json:"ok"`
			Members int         `json:"members"`
			Stats   statsOutput `json:"stats"`
		}{true, len(s.Document.Members), makeStatsOutput(stats)})
	}
	fmt.Fprintf(stdout, "OK\nmembers: %d\nnodes: %d\nready: %d\n", len(s.Document.Members), stats.Nodes, stats.Ready)
	return nil
}

func writeCollectionUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  dagim collection create [--json] COLLECTION")
	fmt.Fprintln(w, "  dagim collection add [--as NAME] [--json] COLLECTION FILE")
	fmt.Fprintln(w, "  dagim collection remove [--json] COLLECTION NAME")
	fmt.Fprintln(w, "  dagim collection list [--json] COLLECTION")
	fmt.Fprintln(w, "  dagim collection move [--json] COLLECTION MEMBER:NODE (--before MEMBER:NODE | --after MEMBER:NODE)")
	fmt.Fprintln(w, "Collections are detected by contents; filenames and extensions are unrestricted.")
}
